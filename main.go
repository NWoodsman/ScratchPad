// ScratchPad is a self-hosted scratch pad that syncs text in real time
// between browsers on a local network. One binary serves both the page
// and the WebSocket on a single port.
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
)

//go:embed web/index.html
var indexTemplate string

const writeTimeout = 10 * time.Second

type config struct {
	listen       string
	dataDir      string
	saveInterval time.Duration
	maxBytes     int64 // <= 0 means unlimited
	origins      []string
}

// envOr returns the first non-empty environment variable among keys, or def.
func envOr(def string, keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return def
}

func parseConfig() (config, error) {
	var c config
	var origins string
	maxMiB, err := strconv.ParseInt(envOr("0", "SCRATCHPAD_MAX_MIB"), 10, 64)
	if err != nil {
		return c, fmt.Errorf("SCRATCHPAD_MAX_MIB: %w", err)
	}
	flag.StringVar(&c.listen, "listen", envOr(":63001", "SCRATCHPAD_LISTEN"),
		"address to listen on, e.g. :63001 or 127.0.0.1:63001 (env SCRATCHPAD_LISTEN)")
	flag.StringVar(&c.dataDir, "data-dir", envOr("", "SCRATCHPAD_DATA_DIR", "DATA_DIR"),
		"directory for content.txt (env SCRATCHPAD_DATA_DIR; default: OS user data dir)")
	flag.DurationVar(&c.saveInterval, "save-interval", 5*time.Second,
		"how often unsaved changes are written to disk")
	flag.Int64Var(&maxMiB, "max-mib", maxMiB,
		"largest document accepted, in MiB; 0 means no limit (env SCRATCHPAD_MAX_MIB)")
	flag.StringVar(&origins, "allowed-origins", envOr("", "SCRATCHPAD_ALLOWED_ORIGINS"),
		"comma-separated extra origin hosts allowed to connect, e.g. pad.example.com "+
			"(same-origin is always allowed; env SCRATCHPAD_ALLOWED_ORIGINS)")
	flag.Parse()

	if c.dataDir == "" {
		if c.dataDir, err = defaultDataDir(); err != nil {
			return c, fmt.Errorf("finding data dir: %w", err)
		}
	}
	if maxMiB < 0 || c.saveInterval <= 0 {
		return c, errors.New("-max-mib must be >= 0 and -save-interval positive")
	}
	c.maxBytes = maxMiB << 20
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.origins = append(c.origins, o)
		}
	}
	return c, nil
}

type server struct {
	hub      *Hub
	maxBytes int64 // <= 0 means unlimited
	origins  []string
	index    []byte
}

func newServer(hub *Hub, maxBytes int64, origins []string) *server {
	if maxBytes <= 0 {
		maxBytes = 0 // the page treats 0 as "no limit"
	}
	page := strings.ReplaceAll(indexTemplate, "{{MAX_BYTES}}", strconv.FormatInt(maxBytes, 10))
	return &server{hub: hub, maxBytes: maxBytes, origins: origins, index: []byte(page)}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	return mux
}

func (s *server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	w.Write(s.index)
}

func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Accept rejects cross-origin requests unless listed in OriginPatterns,
	// so other websites open in a LAN browser can't read or overwrite the pad.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.origins})
	if err != nil {
		log.Printf("websocket accept from %s: %v", r.RemoteAddr, err)
		return
	}
	defer conn.CloseNow()
	if s.maxBytes > 0 {
		conn.SetReadLimit(s.maxBytes)
	} else {
		conn.SetReadLimit(-1) // the library's own default is only 32 KiB
	}

	c := s.hub.Join()
	defer s.hub.Leave(c)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case text := <-c.send:
				wctx, wcancel := context.WithTimeout(ctx, writeTimeout)
				err := conn.Write(wctx, websocket.MessageText, []byte(text))
				wcancel()
				if err != nil {
					return
				}
			}
		}
	}()

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			// Oversized messages are closed with 1009 by the library;
			// the client shows that instead of silently losing the text.
			return
		}
		if typ != websocket.MessageText || !utf8.Valid(data) {
			conn.Close(websocket.StatusUnsupportedData, "expected UTF-8 text")
			return
		}
		s.hub.Update(c, string(data))
	}
}

func run() error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}
	store, err := OpenStore(cfg.dataDir)
	if err != nil {
		return fmt.Errorf("opening data dir: %w", err)
	}
	initial, err := store.Load()
	if err != nil {
		return fmt.Errorf("loading %s: %w", store.path, err)
	}
	hub := NewHub(initial)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	saverDone := make(chan struct{})
	go func() {
		RunSaver(ctx, hub, store, cfg.saveInterval)
		close(saverDone)
	}()

	ln, err := net.Listen("tcp", cfg.listen)
	if err != nil {
		stop()
		<-saverDone
		return err
	}
	srv := &http.Server{
		Handler:           newServer(hub, cfg.maxBytes, cfg.origins).routes(),
		ReadHeaderTimeout: 10 * time.Second,
		// Cancelling ctx also ends the hijacked WebSocket connections.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	log.Printf("ScratchPad listening on http://%s (data: %s)", ln.Addr(), store.path)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err = <-serveErr:
		stop()
	case <-ctx.Done():
		log.Print("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = srv.Shutdown(shutdownCtx)
	}
	<-saverDone // final flush to disk
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return err
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
