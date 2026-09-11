package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"nhooyr.io/websocket"

	"github.com/jasperan/picooraclaw-webui/internal/auth"
	"github.com/jasperan/picooraclaw-webui/internal/bridge"
	"github.com/jasperan/picooraclaw-webui/internal/ws"
)

// SessionSubscriber starts (or reuses) an upstream event pump for a
// session ID. main.go injects an implementation that lazily spawns
// runSSEPump goroutines per session.
type SessionSubscriber interface {
	Ensure(sessionID string)
	Release(sessionID string)
}

type Deps struct {
	Gate       *auth.Gate
	Client     *bridge.Client
	Hub        *ws.Hub
	Subscriber SessionSubscriber
	Static     http.Handler
}

func NewMux(d Deps) *http.ServeMux {
	m := http.NewServeMux()

	m.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		d.Gate.HandleLogin(w, r)
	})

	m.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		if !d.Gate.Authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		sessions, err := d.Client.ListSessions(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if sessions == nil {
			sessions = []bridge.Session{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sessions)
	})

	m.HandleFunc("/api/memory", func(w http.ResponseWriter, r *http.Request) {
		if !d.Gate.Authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		q := r.URL.Query().Get("q")
		results, err := d.Client.SearchMemory(r.Context(), q, 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if results == nil {
			results = []bridge.MemoryResult{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(results)
	})

	m.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if !d.Gate.Authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			// Only the host serving this UI may open the socket. AcceptOptions always
			// authorises the request host, so omitting wildcard patterns blocks cross-site
			// WebSocket hijacking while keeping LAN access (Origin host == Host) working.
			OriginPatterns: nil,
		})
		if err != nil {
			return
		}
		handleWS(r.Context(), c, d)
	})

	// Static must be last so it doesn't eat /api/ or /ws routes.
	m.Handle("/", d.Static)
	return m
}

func handleWS(ctx context.Context, c *websocket.Conn, d Deps) {
	conn := ws.NewWSConn(ctx, c)

	var currentSession string
	defer func() {
		if currentSession != "" && d.Subscriber != nil {
			d.Subscriber.Release(currentSession)
		}
		d.Hub.Unregister(conn)
		conn.Close()
	}()

	for {
		f, err := ws.ReadFrame(ctx, c)
		if err != nil {
			return
		}
		switch f.Type {
		case "subscribe":
			if f.SessionID == "" {
				sendError(conn, "", f.ClientID, "subscribe requires session_id")
				continue
			}
			if currentSession != "" {
				d.Hub.Unregister(conn)
				if d.Subscriber != nil {
					d.Subscriber.Release(currentSession)
				}
			}
			currentSession = f.SessionID
			d.Hub.Register(conn, currentSession)
			if d.Subscriber != nil {
				d.Subscriber.Ensure(currentSession)
			}
		case "send":
			if currentSession == "" {
				sendError(conn, f.SessionID, f.ClientID, "subscribe before sending")
				continue
			}
			if f.SessionID == "" || f.Text == "" {
				sendError(conn, currentSession, f.ClientID, "send requires session_id and text")
				continue
			}
			// Pin to the subscribed session to prevent session spoofing.
			if _, err := d.Client.PostChat(ctx, currentSession, f.Text, ""); err != nil {
				sendError(conn, currentSession, f.ClientID, err.Error())
			}
		}
	}
}

func sendError(conn ws.Conn, sessionID string, clientID string, message string) {
	payload, err := json.Marshal(bridge.Event{
		Type:      "error",
		SessionID: sessionID,
		ClientID:  clientID,
		Error:     message,
		Timestamp: time.Now(),
	})
	if err != nil {
		return
	}
	_ = conn.Send(ws.Frame{Type: "event", Payload: payload})
}
