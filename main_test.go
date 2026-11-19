package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// msg is the union of all server messages, for decoding in tests.
type msg struct {
	T    string `json:"t"`
	V    uint64 `json:"v"`
	Text string `json:"text"`
	At   int    `json:"at"`
	Del  int    `json:"del"`
	Ins  string `json:"ins"`
	Len  int    `json:"len"`
}

func testServer(t *testing.T, initial string, maxBytes int64) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(newServer(NewHub(initial, maxBytes), maxBytes, nil).routes())
	t.Cleanup(ts.Close)
	return ts
}

func testCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func wsURL(srvURL string) string { return "ws" + strings.TrimPrefix(srvURL, "http") + "/ws" }

func dial(t *testing.T, ctx context.Context, srvURL string, origin string) *websocket.Conn {
	t.Helper()
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	conn, _, err := websocket.Dial(ctx, wsURL(srvURL), &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.SetReadLimit(-1)
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func read(t *testing.T, ctx context.Context, c *websocket.Conn) msg {
	t.Helper()
	_, b, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m msg
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode %q: %v", b, err)
	}
	return m
}

func sendPatch(t *testing.T, ctx context.Context, c *websocket.Conn, base uint64, at, del int, ins string) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"t": "patch", "base": base, "at": at, "del": del, "ins": ins})
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

func TestSync(t *testing.T) {
	ctx := testCtx(t)
	ts := testServer(t, "hello", 0)

	a := dial(t, ctx, ts.URL, "")
	if m := read(t, ctx, a); m.T != "full" || m.Text != "hello" || m.V != 0 || m.Len != 5 {
		t.Fatalf("initial = %+v", m)
	}
	b := dial(t, ctx, ts.URL, "")
	read(t, ctx, b)

	sendPatch(t, ctx, a, 0, 5, 0, " world")
	if m := read(t, ctx, a); m.T != "ack" || m.V != 1 || m.Len != 11 {
		t.Fatalf("a got %+v, want ack v1", m)
	}
	if m := read(t, ctx, b); m.T != "patch" || m.V != 1 || m.At != 5 || m.Ins != " world" {
		t.Fatalf("b got %+v", m)
	}

	// Late joiner gets the current text.
	c := dial(t, ctx, ts.URL, "")
	if m := read(t, ctx, c); m.Text != "hello world" || m.V != 1 {
		t.Fatalf("late joiner got %+v", m)
	}

	// Deleting everything still syncs, and an empty doc is still sent in full.
	sendPatch(t, ctx, b, 1, 0, 11, "")
	read(t, ctx, b)
	d := dial(t, ctx, ts.URL, "")
	if m := read(t, ctx, d); m.T != "full" || m.Text != "" || m.V != 2 {
		t.Fatalf("empty doc joiner got %+v", m)
	}
}

func TestStaleBaseIsNacked(t *testing.T) {
	ctx := testCtx(t)
	ts := testServer(t, "abc", 0)
	a := dial(t, ctx, ts.URL, "")
	b := dial(t, ctx, ts.URL, "")
	read(t, ctx, a)
	read(t, ctx, b)

	sendPatch(t, ctx, a, 0, 0, 0, "A") // wins
	read(t, ctx, a)                    // ack
	sendPatch(t, ctx, b, 0, 3, 0, "B") // stale base
	if m := read(t, ctx, b); m.T != "patch" || m.Ins != "A" {
		t.Fatalf("b should first see a's patch, got %+v", m)
	}
	if m := read(t, ctx, b); m.T != "nack" {
		t.Fatalf("b got %+v, want nack", m)
	}
	// Rebased retry succeeds.
	sendPatch(t, ctx, b, 1, 4, 0, "B")
	if m := read(t, ctx, b); m.T != "ack" || m.Len != 5 {
		t.Fatalf("retry got %+v", m)
	}
	if m := read(t, ctx, a); m.T != "patch" || m.At != 4 {
		t.Fatalf("a got %+v", m)
	}
}

func TestUTF16Offsets(t *testing.T) {
	h := NewHub("😀b", 0) // the emoji is 2 UTF-16 units, as in JavaScript
	c := h.Join()
	if err := h.Apply(c, 0, 2, 1, "é"); err != nil {
		t.Fatal(err)
	}
	if text, _, _ := h.Snapshot(); text != "😀é" {
		t.Fatalf("got %q", text)
	}
	if err := h.Apply(c, 1, 3, 1, ""); err != errBadPatch {
		t.Fatalf("out-of-range patch: %v", err)
	}
}

func TestBadPatchCloses(t *testing.T) {
	ctx := testCtx(t)
	ts := testServer(t, "abc", 0)
	a := dial(t, ctx, ts.URL, "")
	read(t, ctx, a)
	sendPatch(t, ctx, a, 0, 2, 5, "")
	if _, _, err := a.Read(ctx); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("want 1008, got %v", err)
	}
}

func TestSlowClientIsKicked(t *testing.T) {
	h := NewHub("", 0)
	slow := h.Join() // never drained
	fast := h.Join()
	for i := 0; i <= sendBuffer; i++ {
		<-fast.send
		if err := h.Apply(fast, uint64(i), 0, 0, "x"); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-slow.kick:
	default:
		t.Fatal("slow client not kicked")
	}
}

func TestRejectsCrossOrigin(t *testing.T) {
	ctx := testCtx(t)
	ts := testServer(t, "", 0)

	h := http.Header{"Origin": {"http://evil.example"}}
	_, resp, err := websocket.Dial(ctx, wsURL(ts.URL), &websocket.DialOptions{HTTPHeader: h})
	if err == nil {
		t.Fatal("cross-origin dial succeeded")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %v", resp)
	}
	dial(t, ctx, ts.URL, ts.URL) // same-origin works
}

func TestTooLargeClosesWith1009(t *testing.T) {
	ctx := testCtx(t)
	ts := testServer(t, "", 16)
	a := dial(t, ctx, ts.URL, "")
	read(t, ctx, a)
	sendPatch(t, ctx, a, 0, 0, 0, strings.Repeat("x", 100))
	if _, _, err := a.Read(ctx); websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
		t.Fatalf("want 1009 close, got %v", err)
	}
}

func TestUnlimitedByDefault(t *testing.T) {
	ctx := testCtx(t)
	ts := testServer(t, "", 0)
	a := dial(t, ctx, ts.URL, "")
	b := dial(t, ctx, ts.URL, "")
	read(t, ctx, a)
	read(t, ctx, b)

	big := strings.Repeat("large paste ", 2<<20/12) // ~2 MiB, far past the library's 32 KiB default
	sendPatch(t, ctx, a, 0, 0, 0, big)
	if m := read(t, ctx, b); m.Ins != big {
		t.Fatalf("b got %d bytes, want %d", len(m.Ins), len(big))
	}
}

func TestIndexHasLimit(t *testing.T) {
	ts := testServer(t, "", 1234)
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

	hub := NewHub("", 0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { RunSaver(ctx, hub, s, time.Hour); close(done) }()
	if err := hub.Apply(hub.Join(), 0, 0, 0, "ünïcode\n\ttext 😀"); err != nil {
		t.Fatal(err)
	}
	cancel() // shutdown must flush even though the ticker never fired
	<-done

	if got, _ := s.Load(); got != "ünïcode\n\ttext 😀" {
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
