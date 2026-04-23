package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
