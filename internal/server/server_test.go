package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/jasperan/picooraclaw-webui/internal/auth"
	"github.com/jasperan/picooraclaw-webui/internal/bridge"
	"github.com/jasperan/picooraclaw-webui/internal/ws"
)

func TestMux_AuthRequiredForAPI(t *testing.T) {
	gate := auth.NewGate("pw", "secretXXXXXXXXXXXXXXXXXXXXXXXXXX")
	defer gate.Stop()
	hub := ws.NewHub()
	defer hub.Close()
	client := bridge.NewClient("http://upstream.invalid", "")

	m := NewMux(Deps{
		Gate:   gate,
		Client: client,
		Hub:    hub,
		Static: http.NotFoundHandler(),
	})
	srv := httptest.NewServer(m)
	defer srv.Close()

	// /api/sessions without cookie → 401
	resp, _ := http.Get(srv.URL + "/api/sessions")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sessions unauth: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// /api/login GET → 405
	resp, _ = http.Get(srv.URL + "/api/login")
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("login GET: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// /api/login POST with valid password → 204
	resp, _ = http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(`{"password":"pw"}`))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("login POST: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWS_SendWithoutSubscribe_Ignored(t *testing.T) {
	gate := auth.NewGate("", "secretXXXXXXXXXXXXXXXXXXXXXXXXXX")
	defer gate.Stop()
	hub := ws.NewHub()
	defer hub.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("upstream must NOT be called: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(500)
	}))
	defer upstream.Close()
	client := bridge.NewClient(upstream.URL, "")

	m := NewMux(Deps{Gate: gate, Client: client, Hub: hub, Static: http.NotFoundHandler()})
	srv := httptest.NewServer(m)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// Send without subscribe; handler must drop it (no upstream call).
	_ = c.Write(ctx, websocket.MessageText,
		[]byte(`{"type":"send","session_id":"x","text":"hi"}`))

	// Give handler time to process + NOT call upstream.
	time.Sleep(100 * time.Millisecond)
}
