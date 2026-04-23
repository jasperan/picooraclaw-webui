package server

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestE2E_WSSubscribeSendAndReceive(t *testing.T) {
	// Fake upstream picooraclaw that ACKs /v1/chat and serves SSE events.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"message_id":"m1"}`))
		case "/v1/events":
			w.Header().Set("Content-Type", "text/event-stream")
			fl := w.(http.Flusher)
			fmt.Fprintf(w, "data: {\"type\":\"message_end\",\"session_id\":\"default\",\"message_id\":\"m1\",\"text\":\"hi\"}\n\n")
			fl.Flush()
			time.Sleep(200 * time.Millisecond)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	client := bridge.NewClient(upstream.URL, "")
	hub := ws.NewHub()
	defer hub.Close()
	gate := auth.NewGate("", "secretXXXXXXXXXXXXXXXXXXXXXXXXXX")
	defer gate.Stop()

	srv := httptest.NewServer(NewMux(Deps{
		Gate:   gate,
		Client: client,
		Hub:    hub,
		Static: http.NewServeMux(),
	}))
	defer srv.Close()

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// Subscribe to session "default"
	if err := c.Write(ctx, websocket.MessageText, []byte(`{"type":"subscribe","session_id":"default"}`)); err != nil {
		t.Fatal(err)
	}

	// Send a message — this triggers POST /v1/chat on the upstream.
	if err := c.Write(ctx, websocket.MessageText, []byte(`{"type":"send","session_id":"default","text":"hi"}`)); err != nil {
		t.Fatal(err)
	}

	// Give the server a moment to register the subscription and process send.
	time.Sleep(100 * time.Millisecond)

	// Since this test file doesn't run main()'s SSE pump, manually fire a hub
	// broadcast to validate the WS fan-out path end-to-end.
	hub.Broadcast("default", ws.Frame{Type: "event", Payload: json.RawMessage(`{"type":"message_end","text":"hi"}`)})

	_, buf, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(buf), "message_end") {
		t.Fatalf("unexpected frame: %s", string(buf))
	}
}
