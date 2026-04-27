package main

import (
	"context"
	"sync"

	"github.com/jasperan/picooraclaw-webui/internal/bridge"
	"github.com/jasperan/picooraclaw-webui/internal/ws"
)

// SessionPumps lazily starts one runSSEPump goroutine per subscribed
// session. The original v1 design ran a single pump pinned to "default";
// the gateway now mints fresh session IDs per chat (e.g. "s-mogjm0ue"),
// so events for those sessions never reached the hub. Ensure() is the
// fix: every time a WS client subscribes, we make sure a pump exists
// for that session.
//
// Pumps are started on first subscribe and run for the lifetime of the
// process (the goroutine exits when the parent ctx is cancelled at
// shutdown). We don't refcount and stop pumps when the last subscriber
// leaves: a pump that's not consumed costs a TCP connection plus a
// goroutine, and re-establishing the SSE adds visible latency on every
// resubscribe. Memory grows with distinct session IDs over the process
// lifetime — acceptable for current usage; revisit with refcount + idle
// timeout if it becomes a problem.
type SessionPumps struct {
	ctx    context.Context
	client *bridge.Client
	hub    *ws.Hub

	mu    sync.Mutex
	known map[string]struct{}
}

func NewSessionPumps(ctx context.Context, client *bridge.Client, hub *ws.Hub) *SessionPumps {
	return &SessionPumps{
		ctx:    ctx,
		client: client,
		hub:    hub,
		known:  make(map[string]struct{}),
	}
}

func (p *SessionPumps) Ensure(sessionID string) {
	if sessionID == "" {
		return
	}
	p.mu.Lock()
	if _, ok := p.known[sessionID]; ok {
		p.mu.Unlock()
		return
	}
	p.known[sessionID] = struct{}{}
	p.mu.Unlock()
	go runSSEPump(p.ctx, p.client, p.hub, sessionID)
}
