package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func dial(t *testing.T, ctx context.Context, srvURL string, origin string) *websocket.Conn {
	t.Helper()
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srvURL, "http")+"/ws",
		&websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func read(t *testing.T, ctx context.Context, c *websocket.Conn) string {
	t.Helper()
	_, b, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func TestSync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hub := NewHub("hello")
	ts := httptest.NewServer(newServer(hub, 1<<20, nil).routes())
	defer ts.Close()

	a := dial(t, ctx, ts.URL, "")
	if got := read(t, ctx, a); got != "hello" {
		t.Fatalf("initial state = %q, want %q", got, "hello")
	}
	b := dial(t, ctx, ts.URL, "")
	read(t, ctx, b)

	if err := a.Write(ctx, websocket.MessageText, []byte("hello world")); err != nil {
		t.Fatal(err)
	}
	if got := read(t, ctx, b); got != "hello world" {
		t.Fatalf("b got %q", got)
	}

	// A late joiner gets the latest state, and an empty doc is still sent.
	if err := b.Write(ctx, websocket.MessageText, []byte("")); err != nil {
		t.Fatal(err)
	}
	if got := read(t, ctx, a); got != "" {
		t.Fatalf("a got %q, want empty", got)
	}
	c := dial(t, ctx, ts.URL, "")
	if got := read(t, ctx, c); got != "" {
		t.Fatalf("late joiner got %q, want empty", got)
	}
}

func TestRejectsCrossOrigin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ts := httptest.NewServer(newServer(NewHub(""), 1<<20, nil).routes())
	defer ts.Close()

	h := http.Header{"Origin": {"http://evil.example"}}
	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/ws",
		&websocket.DialOptions{HTTPHeader: h})
	if err == nil {
		t.Fatal("cross-origin dial succeeded")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %v", resp)
	}

	// Same-origin works.
	dial(t, ctx, ts.URL, ts.URL)
}

func TestTooLargeClosesWith1009(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ts := httptest.NewServer(newServer(NewHub(""), 16, nil).routes())
	defer ts.Close()

	a := dial(t, ctx, ts.URL, "")
	read(t, ctx, a)
	a.Write(ctx, websocket.MessageText, []byte(strings.Repeat("x", 100)))
	_, _, err := a.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
		t.Fatalf("want 1009 close, got %v", err)
	}
}

func TestIndexHasLimit(t *testing.T) {
	ts := httptest.NewServer(newServer(NewHub(""), 1234, nil).routes())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `content="1234"`) {
		t.Fatal("max bytes not injected into page")
	}
	if r, _ := http.Get(ts.URL + "/main.go"); r.StatusCode != http.StatusNotFound {
		t.Fatalf("serving arbitrary paths: %d", r.StatusCode)
	}
}

func TestStoreAndSaver(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Load(); err != nil || got != "" {
		t.Fatalf("empty load = %q, %v", got, err)
	}

	hub := NewHub("")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { RunSaver(ctx, hub, s, time.Hour); close(done) }()
	hub.Update(nil, "ünïcode\n\ttext")
	cancel() // shutdown must flush even though the ticker never fired
	<-done

	if got, _ := s.Load(); got != "ünïcode\n\ttext" {
		t.Fatalf("after flush = %q", got)
	}
	if _, _, dirty := hub.Snapshot(); dirty {
		t.Fatal("still dirty after save")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("leftover temp files: %v", entries)
	}
}
