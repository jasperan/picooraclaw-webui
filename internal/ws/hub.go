package ws

import (
	"encoding/json"
	"sync"
)

type Frame struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Conn interface {
	Send(Frame) error
	Close()
}

type Hub struct {
	mu    sync.RWMutex
	conns map[string]map[Conn]struct{}
	done  chan struct{}
	once  sync.Once
}

func NewHub() *Hub {
	return &Hub{
		conns: make(map[string]map[Conn]struct{}),
		done:  make(chan struct{}),
	}
}

func (h *Hub) Register(c Conn, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.conns[sessionID]; !ok {
		h.conns[sessionID] = make(map[Conn]struct{})
	}
	h.conns[sessionID][c] = struct{}{}
}

// Unregister removes c from every session's set. Callers own the connection
// lifecycle — Unregister does not call c.Close().
func (h *Hub) Unregister(c Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sid, set := range h.conns {
		if _, ok := set[c]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.conns, sid)
			}
		}
	}
}

func (h *Hub) Broadcast(sessionID string, f Frame) {
	h.mu.RLock()
	targets := make([]Conn, 0, len(h.conns[sessionID]))
	for c := range h.conns[sessionID] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		_ = c.Send(f)
	}
}

func (h *Hub) Close() {
	h.once.Do(func() { close(h.done) })
}
