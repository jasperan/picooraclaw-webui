package server

import (
	"context"
	"encoding/json"
	"net/http"

	"nhooyr.io/websocket"

	"github.com/jasperan/picooraclaw-webui/internal/auth"
	"github.com/jasperan/picooraclaw-webui/internal/bridge"
	"github.com/jasperan/picooraclaw-webui/internal/ws"
)

type Deps struct {
	Gate   *auth.Gate
	Client *bridge.Client
	Hub    *ws.Hub
	Static http.Handler
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
			OriginPatterns: []string{"*"},
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
	defer d.Hub.Unregister(conn)
	defer conn.Close()

	var currentSession string

	for {
		f, err := ws.ReadFrame(ctx, c)
		if err != nil {
			return
		}
		switch f.Type {
		case "subscribe":
			if currentSession != "" {
				d.Hub.Unregister(conn)
			}
			currentSession = f.SessionID
			d.Hub.Register(conn, currentSession)
		case "send":
			if currentSession == "" {
				// Protocol: must subscribe before sending.
				continue
			}
			if f.SessionID == "" || f.Text == "" {
				continue
			}
			// Pin to the subscribed session to prevent session spoofing.
			_, _ = d.Client.PostChat(ctx, currentSession, f.Text, "")
		}
	}
}
