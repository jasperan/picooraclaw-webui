package main

import (
	"context"
	"sync"
	"time"

	"github.com/jasperan/picooraclaw-webui/internal/bridge"
	"github.com/jasperan/picooraclaw-webui/internal/ws"
)

const pumpIdleTimeout = 2 * time.Minute

type sessionPump struct {
	cancel context.CancelFunc
	refs   int
	timer  *time.Timer
}

// SessionPumps lazily starts one runSSEPump goroutine per subscribed session.
// Pumps are reference counted and cancelled after a short idle timeout so a
// process that sees many distinct session IDs does not keep a goroutine and
// upstream SSE connection alive forever for each one.
type SessionPumps struct {
	ctx    context.Context
	client *bridge.Client
	hub    *ws.Hub

	mu    sync.Mutex
	pumps map[string]*sessionPump
}

func NewSessionPumps(ctx context.Context, client *bridge.Client, hub *ws.Hub) *SessionPumps {
	return &SessionPumps{
		ctx:    ctx,
		client: client,
		hub:    hub,
		pumps:  make(map[string]*sessionPump),
	}
}

func (p *SessionPumps) Ensure(sessionID string) {
	if sessionID == "" {
		return
	}
	p.mu.Lock()
	if pump, ok := p.pumps[sessionID]; ok {
		pump.refs++
		if pump.timer != nil {
			pump.timer.Stop()
			pump.timer = nil
		}
		p.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(p.ctx)
	p.pumps[sessionID] = &sessionPump{cancel: cancel, refs: 1}
	p.mu.Unlock()
	go runSSEPump(ctx, p.client, p.hub, sessionID)
}

func (p *SessionPumps) Release(sessionID string) {
	if sessionID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	pump, ok := p.pumps[sessionID]
	if !ok {
		return
	}
	if pump.refs > 0 {
		pump.refs--
	}
	if pump.refs != 0 || pump.timer != nil {
		return
	}
	pump.timer = time.AfterFunc(pumpIdleTimeout, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		current, ok := p.pumps[sessionID]
		if !ok || current != pump || current.refs != 0 {
			return
		}
		current.cancel()
		delete(p.pumps, sessionID)
	})
}

func (p *SessionPumps) Active() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.pumps)
}
