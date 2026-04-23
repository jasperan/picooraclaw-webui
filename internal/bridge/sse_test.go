package bridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSSE_StreamsParsedEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("session_id") != "s1" {
			t.Errorf("missing session_id")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		fmt.Fprintf(w, "data: {\"type\":\"message_start\",\"session_id\":\"s1\",\"message_id\":\"m1\"}\n\n")
		fl.Flush()
		fmt.Fprintf(w, "data: {\"type\":\"message_end\",\"session_id\":\"s1\",\"message_id\":\"m1\"}\n\n")
		fl.Flush()
		// Hold the connection so the client has time to read both events.
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events := make(chan Event, 16)
	done := make(chan error, 1)
	go func() {
		done <- c.Stream(ctx, "s1", "", events)
	}()

	// Collect events until Stream returns.
	var types []string
	deadline := time.After(1500 * time.Millisecond)
loop:
	for {
		select {
		case ev := <-events:
			types = append(types, ev.Type)
			if len(types) >= 2 {
				break loop
			}
		case err := <-done:
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
				t.Fatalf("stream err: %v", err)
			}
			break loop
		case <-deadline:
			t.Fatalf("timeout; got types=%v", types)
		}
	}

	if len(types) < 2 || types[0] != "message_start" || types[1] != "message_end" {
		t.Fatalf("events: %v", types)
	}
}
