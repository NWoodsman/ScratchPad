package main

import "sync"

// client is one connected browser. send holds at most one pending message:
// every message is the full document, so a slow client only ever needs the
// newest one and older ones are dropped instead of queueing up.
type client struct {
	send chan string
}

// Hub owns the shared document and the set of connected clients.
type Hub struct {
	mu      sync.Mutex
	content string
	version uint64 // bumped on every change
	saved   uint64 // version last written to disk
	clients map[*client]struct{}
}

func NewHub(initial string) *Hub {
	return &Hub{content: initial, clients: make(map[*client]struct{})}
}

// push replaces whatever is pending for c with text. Must hold h.mu: the hub
// is the only writer to c.send, so after draining, the send never blocks.
func push(c *client, text string) {
	select {
	case <-c.send:
	default:
	}
	c.send <- text
}

// Join registers a new client and queues the current document for it,
// even when the document is empty, so the server is always the source of truth.
func (h *Hub) Join() *client {
	c := &client{send: make(chan string, 1)}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	push(c, h.content)
	return c
}

func (h *Hub) Leave(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

// Update stores text as the new document and forwards it to every client
// except the sender. Last writer wins.
func (h *Hub) Update(from *client, text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if text == h.content {
		return
	}
	h.content = text
	h.version++
	for c := range h.clients {
		if c != from {
			push(c, text)
		}
	}
}

// Snapshot returns the document and its version if it has unsaved changes.
func (h *Hub) Snapshot() (text string, version uint64, dirty bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.content, h.version, h.version != h.saved
}

func (h *Hub) MarkSaved(version uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if version > h.saved {
		h.saved = version
	}
}
