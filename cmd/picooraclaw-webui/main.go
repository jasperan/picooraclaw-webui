package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jasperan/picooraclaw-webui/internal/auth"
	"github.com/jasperan/picooraclaw-webui/internal/bridge"
	"github.com/jasperan/picooraclaw-webui/internal/config"
	"github.com/jasperan/picooraclaw-webui/internal/server"
	"github.com/jasperan/picooraclaw-webui/internal/ws"
)

func isLoopbackListen(addr string) bool {
	// Accepts "127.0.0.1:3000", "localhost:3000" and "[::1]:3000"; rejects ":3000" and
	// "0.0.0.0:3000", which expose every interface.
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	switch strings.ToLower(host) {
	case "localhost", "::1":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Fail closed on the dangerous combination: with no password the gate allows every
	// request (see auth.Gate.Authorized), so listening beyond loopback would publish an
	// unprotected dashboard. Setting PICOORACLAW_WEBUI_PASSWORD/--password enables remote use.
	if cfg.Password == "" && !isLoopbackListen(cfg.Listen) {
		log.Fatalf(
			"refusing to listen on %s without a password: set PICOORACLAW_WEBUI_PASSWORD "+
				"(or --password), or bind 127.0.0.1. The UI has no other access control.",
			cfg.Listen,
		)
	}

	client, err := bridge.NewClientChecked(cfg.PicooraclawURL, cfg.UpstreamToken)
	if err != nil {
		log.Fatalf("upstream client: %v", err)
	}
	hub := ws.NewHub()
	gate := auth.NewGate(cfg.Password, cfg.Secret)
	defer gate.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// One SSE pump per subscribed session. Pre-warm "default" for legacy
	// clients and the v1 protocol; everything else is lazy on subscribe.
	pumps := NewSessionPumps(ctx, client, hub)
	pumps.Ensure("default")

	mux := server.NewMux(server.Deps{
		Gate:       gate,
		Client:     client,
		Hub:        hub,
		Subscriber: pumps,
		Static:     staticHandler(),
	})

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	}()

	log.Printf("picooraclaw-webui listening on %s (upstream=%s)", cfg.Listen, cfg.PicooraclawURL)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}

// runSSEPump opens an SSE stream to the upstream for sessionID and broadcasts
// every event into the hub keyed by e.SessionID. Reconnects on upstream close.
func runSSEPump(ctx context.Context, client *bridge.Client, hub *ws.Hub, sessionID string) {
	for {
		events := make(chan bridge.Event, 64)
		streamCtx, streamCancel := context.WithCancel(ctx)
		go func() {
			// The spawning goroutine owns events and closes it once Stream
			// returns (sse.go documents this contract), so the drain below is a
			// single range that naturally finishes any buffered events.
			err := client.Stream(streamCtx, sessionID, "", events)
			if err != nil && ctx.Err() == nil {
				log.Printf("sse stream: %v (retrying)", err)
			}
			close(events)
		}()

		for e := range events {
			buf, err := json.Marshal(e)
			if err != nil {
				log.Printf("sse pump: marshal: %v", err)
				continue
			}
			hub.Broadcast(e.SessionID, ws.Frame{Type: "event", Payload: buf})
		}
		streamCancel()

		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
}
