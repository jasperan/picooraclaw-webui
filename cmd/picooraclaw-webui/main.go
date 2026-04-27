package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jasperan/picooraclaw-webui/internal/auth"
	"github.com/jasperan/picooraclaw-webui/internal/bridge"
	"github.com/jasperan/picooraclaw-webui/internal/config"
	"github.com/jasperan/picooraclaw-webui/internal/server"
	"github.com/jasperan/picooraclaw-webui/internal/ws"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	client := bridge.NewClient(cfg.PicooraclawURL, cfg.UpstreamToken)
	hub := ws.NewHub()
	defer hub.Close()
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
		done := make(chan struct{})
		go func() {
			err := client.Stream(streamCtx, sessionID, "", events)
			if err != nil && ctx.Err() == nil {
				log.Printf("sse stream: %v (retrying)", err)
			}
			close(done)
		}()

		drain := func() {
			for {
				select {
				case e, ok := <-events:
					if !ok {
						return
					}
					buf, err := json.Marshal(e)
					if err != nil {
						log.Printf("sse pump: marshal: %v", err)
						continue
					}
					hub.Broadcast(e.SessionID, ws.Frame{Type: "event", Payload: buf})
				case <-done:
					// Drain any remaining buffered events before reconnecting.
					for {
						select {
						case e, ok := <-events:
							if !ok {
								return
							}
							buf, err := json.Marshal(e)
							if err != nil {
								log.Printf("sse pump: marshal: %v", err)
								continue
							}
							hub.Broadcast(e.SessionID, ws.Frame{Type: "event", Payload: buf})
						default:
							return
						}
					}
				case <-ctx.Done():
					return
				}
			}
		}
		drain()
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
