package main

import (
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"unicode/utf16"
)

// Wire protocol (JSON text messages). Positions and lengths are in UTF-16
// code units, matching JavaScript string indexing.
//
//	server → client
//	  {"t":"full","v":N,"text":"…","len":L}           on connect
//	  {"t":"patch","v":N,"at":A,"del":D,"ins":"…","len":L}  someone else's edit
//	  {"t":"ack","v":N,"len":L}                          your patch was applied
//	  {"t":"nack"}                                       your base was stale; rebase and retry
//	client → server
//	  {"t":"patch","base":N,"at":A,"del":D,"ins":"…"}
//
// "len" is the document length after the change, so clients can detect drift.
type fullMsg struct {
	T    string `json:"t"`
	V    uint64 `json:"v"`
	Text string `json:"text"`
	Len  int    `json:"len"`
}

type patchMsg struct {
	T   string `json:"t"`
	V   uint64 `json:"v"`
	At  int    `json:"at"`
	Del int    `json:"del"`
	Ins string `json:"ins"`
	Len int    `json:"len"`
}

type ackMsg struct {
	T   string `json:"t"`
	V   uint64 `json:"v"`
	Len int    `json:"len"`
}

// clientPatch is the only message clients send.
type clientPatch struct {
	T    string  `json:"t"`
	Base *uint64 `json:"base"`
	At   int     `json:"at"`
	Del  int     `json:"del"`
	Ins  string  `json:"ins"`
}

func encode(m any) []byte {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err) // plain structs; cannot fail
	}
	return b
}

var nackMsg = []byte(`{"t":"nack"}`)

var (
	errBadPatch = errors.New("patch out of range")
	errTooLarge = errors.New("document too large")
)

// sendBuffer is how many messages a client may fall behind before it is
// disconnected. It reconnects and resyncs from a full copy.
const sendBuffer = 1024

type client struct {
	send   chan []byte
	kick   chan struct{} // closed when the client fell too far behind
	kicked bool
}

// push queues msg for c without blocking. Must hold h.mu.
func (c *client) push(msg []byte) {
	if c.kicked {
		return
	}
	select {
	case c.send <- msg:
	default:
		c.kicked = true
		close(c.kick)
	}
}

// Hub owns the shared document and the set of connected clients.
type Hub struct {
	mu       sync.Mutex
	content  []uint16 // UTF-16, so patch offsets match the browser's
	bytes    int      // UTF-8 size of content, tracked when maxBytes > 0
	maxBytes int64    // <= 0 means unlimited
	version  uint64   // bumped on every change
	saved    uint64   // version last written to disk
	clients  map[*client]struct{}
}

func NewHub(initial string, maxBytes int64) *Hub {
	h := &Hub{
		content:  utf16.Encode([]rune(initial)),
		maxBytes: maxBytes,
		clients:  make(map[*client]struct{}),
	}
	if maxBytes > 0 {
		h.bytes = utf8Len(h.content)
	}
	return h
}

// Join registers a new client and queues the full document for it.
func (h *Hub) Join() *client {
	c := &client{send: make(chan []byte, sendBuffer), kick: make(chan struct{})}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	text := string(utf16.Decode(h.content))
	c.push(encode(fullMsg{"full", h.version, text, len(h.content)}))
	return c
}

func (h *Hub) Leave(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

// Apply replaces del units at position at with ins, if base is the current
// version. A stale base gets a nack so the client rebases and retries.
func (h *Hub) Apply(from *client, base uint64, at, del int, ins string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if base != h.version {
		from.push(nackMsg)
		return nil
	}
	if at < 0 || del < 0 || at > len(h.content) || del > len(h.content)-at {
		return errBadPatch
	}
	insU := utf16.Encode([]rune(ins))
	if h.maxBytes > 0 {
		newBytes := h.bytes - utf8Len(h.content[at:at+del]) + len(ins)
		if int64(newBytes) > h.maxBytes {
			return errTooLarge
		}
		h.bytes = newBytes
	}
	h.content = slices.Replace(h.content, at, at+del, insU...)
	h.version++

	patch := encode(patchMsg{"patch", h.version, at, del, ins, len(h.content)})
	for c := range h.clients {
		if c != from {
			c.push(patch)
		}
	}
	from.push(encode(ackMsg{"ack", h.version, len(h.content)}))
	return nil
}

// utf8Len is the UTF-8 size of a UTF-16 slice.
func utf8Len(s []uint16) int {
	n := 0
	for _, r := range utf16.Decode(s) {
		switch {
		case r < 0x80:
			n++
		case r < 0x800:
			n += 2
		case r < 0x10000:
			n += 3
		default:
			n += 4
		}
	}
	return n
}

// Snapshot returns the document and its version if it has unsaved changes.
func (h *Hub) Snapshot() (text string, version uint64, dirty bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.version == h.saved {
		return "", h.version, false
	}
	return string(utf16.Decode(h.content)), h.version, true
}

func (h *Hub) MarkSaved(version uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if version > h.saved {
		h.saved = version
	}
}
