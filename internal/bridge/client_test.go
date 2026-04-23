package bridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_PostChat(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat" || r.Method != "POST" {
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"message_id":"m_42"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	mid, err := c.PostChat(context.Background(), "s1", "hi", "")
	if err != nil {
		t.Fatal(err)
	}
	if mid != "m_42" {
		t.Fatalf("got message_id %q", mid)
	}
	if !strings.Contains(gotBody, `"text":"hi"`) {
		t.Fatalf("body: %s", gotBody)
	}
}

func TestClient_UpstreamTokenHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing auth header")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message_id": "m"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	_, _ = c.PostChat(context.Background(), "s", "t", "")
}

func TestClient_ListSessions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sessions" {
			t.Errorf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"id":"s1","title":"one","last_at":10}]`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "")
	s, err := c.ListSessions(context.Background())
	if err != nil || len(s) != 1 || s[0].ID != "s1" {
		t.Fatalf("got %+v err=%v", s, err)
	}
}

func TestClient_SearchMemory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "go" {
			t.Errorf("q: %q", r.URL.Query().Get("q"))
		}
		if r.URL.Query().Get("limit") != "5" {
			t.Errorf("limit: %q", r.URL.Query().Get("limit"))
		}
		_, _ = w.Write([]byte(`[{"id":"m1","text":"likes go","score":0.9,"date":1}]`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "")
	r, err := c.SearchMemory(context.Background(), "go", 5)
	if err != nil || len(r) != 1 || r[0].ID != "m1" {
		t.Fatalf("got %+v err=%v", r, err)
	}
}
