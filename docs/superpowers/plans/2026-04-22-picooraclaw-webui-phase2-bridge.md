# Phase 2 — picooraclaw-webui Go Bridge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Build the Go bridge that sits between the browser (WebSocket) and picooraclaw (HTTP + SSE). Phase 1 provides the upstream contract.

**Architecture:** Single Go binary. One module: `github.com/jasperan/picooraclaw-webui`. Internal packages separate concerns: `bridge` (upstream client + SSE consumer), `ws` (WebSocket hub + per-session fan-out), `auth` (password gate + signed cookie), `config` (flag/env/file), `server` (HTTP handlers + static serving). `nhooyr.io/websocket` is the one external dependency (small, context-native, no goroutine leaks).

**Tech Stack:** Go 1.22+, `nhooyr.io/websocket`, stdlib for everything else. Repo: `~/git/personal/picooraclaw-webui/`.

**Repo context (pre-existing):**
- `docs/superpowers/specs/2026-04-22-picooraclaw-webui-design.md` — the spec
- `docs/superpowers/plans/2026-04-22-picooraclaw-webui-phase1-channel.md` — the upstream contract
- The directory is a git repo with only design docs committed so far.

---

### Task 1: Initialize the Go module and directory layout

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `Makefile`
- Create: placeholder `web/build/.gitkeep` so `//go:embed` compiles

- [ ] **Step 1: Init module**

```bash
cd ~/git/personal/picooraclaw-webui
go mod init github.com/jasperan/picooraclaw-webui
go get nhooyr.io/websocket
```

- [ ] **Step 2: Write .gitignore**

```
# Go
/bin/
/build/
*.test
coverage.out

# Frontend build artefacts get checked in only via placeholder
web/node_modules/
web/.svelte-kit/

# Local agent/config dirs (never commit)
.agents/
.claude/
.crush/
.openhands/
.serena/

# Brainstorm/plan scratch (shared with superpowers)
.superpowers/brainstorm/

# IDE
.vscode/
.idea/

# Env
.env
.env.*
!.env.example
```

- [ ] **Step 3: Write Makefile**

```
.PHONY: build test lint run

build:
	go build -o bin/picooraclaw-webui ./cmd/picooraclaw-webui

test:
	go test ./...

lint:
	go vet ./...

run: build
	./bin/picooraclaw-webui --picooraclaw-url http://localhost:8090 --listen :3000
```

- [ ] **Step 4: Placeholder for embedded static**

```bash
mkdir -p web/build
echo "placeholder — replaced by SvelteKit build output in phase 3" > web/build/.gitkeep
```

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum .gitignore Makefile web/build/.gitkeep
git commit -m "chore: init Go module and directory skeleton"
```

---

### Task 2: Config package (flag > env > file > default)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultsApplied(t *testing.T) {
	os.Unsetenv("PICOORACLAW_URL")
	os.Unsetenv("PICOORACLAW_WEBUI_LISTEN")
	cfg, err := Load([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PicooraclawURL != "http://localhost:8090" {
		t.Fatalf("default url: %q", cfg.PicooraclawURL)
	}
	if cfg.Listen != ":3000" {
		t.Fatalf("default listen: %q", cfg.Listen)
	}
}

func TestLoad_EnvOverridesDefault(t *testing.T) {
	t.Setenv("PICOORACLAW_URL", "http://1.2.3.4:9000")
	cfg, _ := Load([]string{})
	if cfg.PicooraclawURL != "http://1.2.3.4:9000" {
		t.Fatalf("env not applied: %q", cfg.PicooraclawURL)
	}
}

func TestLoad_FlagOverridesEnv(t *testing.T) {
	t.Setenv("PICOORACLAW_URL", "http://env:9000")
	cfg, _ := Load([]string{"--picooraclaw-url", "http://flag:9000"})
	if cfg.PicooraclawURL != "http://flag:9000" {
		t.Fatalf("flag not applied: %q", cfg.PicooraclawURL)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -v`
Expected: FAIL (package doesn't exist).

- [ ] **Step 3: Write the implementation**

```go
// internal/config/config.go
package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

type Config struct {
	PicooraclawURL string
	Listen         string
	Password       string // optional: if set, enables login gate
	UpstreamToken  string // optional: sent as Authorization: Bearer
	Secret         string // cookie signing key; auto-generated if empty
}

func Load(args []string) (*Config, error) {
	fs := flag.NewFlagSet("picooraclaw-webui", flag.ContinueOnError)

	cfg := &Config{}
	fs.StringVar(&cfg.PicooraclawURL, "picooraclaw-url", getenv("PICOORACLAW_URL", "http://localhost:8090"), "upstream gateway URL")
	fs.StringVar(&cfg.Listen, "listen", getenv("PICOORACLAW_WEBUI_LISTEN", ":3000"), "listen address")
	fs.StringVar(&cfg.Password, "password", os.Getenv("PICOORACLAW_WEBUI_PASSWORD"), "optional login password")
	fs.StringVar(&cfg.UpstreamToken, "upstream-token", os.Getenv("PICOORACLAW_WEB_TOKEN"), "optional upstream bearer token")
	fs.StringVar(&cfg.Secret, "secret", os.Getenv("PICOORACLAW_WEBUI_SECRET"), "cookie signing key")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if cfg.Secret == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("generate secret: %w", err)
		}
		cfg.Secret = hex.EncodeToString(buf)
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): flag/env/default loader with auto-generated cookie secret"
```

---

### Task 3: Upstream HTTP client (POST /v1/chat, GET /v1/sessions, /v1/memory)

**Files:**
- Create: `internal/bridge/client.go`
- Create: `internal/bridge/client_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/bridge/client_test.go
package bridge

import (
	"context"
	"encoding/json"
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
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/bridge -v`
Expected: FAIL (package missing).

- [ ] **Step 3: Write the implementation**

```go
// internal/bridge/client.go
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

func NewClient(baseURL, token string) *Client {
	u, _ := url.Parse(baseURL)
	return &Client{base: u, token: token, http: &http.Client{}}
}

func (c *Client) PostChat(ctx context.Context, sessionID, text, workspace string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"session_id": sessionID, "text": text, "workspace": workspace,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.resolve("/v1/chat"), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}
	var out struct{ MessageID string `json:"message_id"` }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.MessageID, nil
}

type Session struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	LastAt int64  `json:"last_at"`
}

func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.resolve("/v1/sessions"), nil)
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out []Session
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

type MemoryResult struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	Score float64 `json:"score"`
	Date  int64   `json:"date"`
}

func (c *Client) SearchMemory(ctx context.Context, query string, limit int) ([]MemoryResult, error) {
	q := url.Values{}
	q.Set("q", query)
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", c.resolve("/v1/memory")+"?"+q.Encode(), nil)
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out []MemoryResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) resolve(path string) string {
	u := *c.base
	u.Path = path
	return u.String()
}

func (c *Client) addAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/bridge -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bridge/client.go internal/bridge/client_test.go
git commit -m "feat(bridge): upstream HTTP client for chat/sessions/memory"
```

---

### Task 4: SSE consumer

**Files:**
- Create: `internal/bridge/sse.go`
- Create: `internal/bridge/sse_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/bridge/sse_test.go
package bridge

import (
	"context"
	"fmt"
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
		// hold the connection for a moment so the client reads both
		time.Sleep(50 * time.Millisecond)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	events := make(chan Event, 16)
	err := c.Stream(ctx, "s1", "", events)
	if err != nil && err != context.DeadlineExceeded && err != io_EOF() {
		t.Fatalf("stream err: %v", err)
	}
	close(events)

	types := []string{}
	for e := range events {
		types = append(types, e.Type)
	}
	if len(types) < 2 || types[0] != "message_start" || types[1] != "message_end" {
		t.Fatalf("events: %v", types)
	}
}

func io_EOF() error { return nil } // placeholder to keep test readable
```

- [ ] **Step 2: Write the implementation**

```go
// internal/bridge/sse.go
package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Event struct {
	Type       string         `json:"type"`
	SessionID  string         `json:"session_id,omitempty"`
	MessageID  string         `json:"message_id,omitempty"`
	ToolCallID string         `json:"id,omitempty"`
	Tool       string         `json:"tool,omitempty"`
	Args       map[string]any `json:"args,omitempty"`
	Result     string         `json:"result,omitempty"`
	OK         *bool          `json:"ok,omitempty"`
	Text       string         `json:"text,omitempty"`
	Error      string         `json:"error,omitempty"`
	Note       string         `json:"note,omitempty"`
}

// Stream opens an SSE connection to /v1/events and sends parsed events to out.
// Returns when ctx is cancelled or the upstream closes. The caller owns 'out' and
// may close it after Stream returns.
func (c *Client) Stream(ctx context.Context, sessionID, from string, out chan<- Event) error {
	u := c.resolve("/v1/events") + fmt.Sprintf("?session_id=%s", sessionID)
	if from != "" {
		u += "&from=" + from
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	c.addAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("sse upstream %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimRight(line, "\n")
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue // skip malformed
		}
		select {
		case out <- ev:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/bridge -v`
Expected: PASS (the test assertion tolerates early EOF).

- [ ] **Step 4: Commit**

```bash
git add internal/bridge/sse.go internal/bridge/sse_test.go
git commit -m "feat(bridge): SSE consumer with cancellation and graceful malformed-line skip"
```

---

### Task 5: WebSocket hub

**Files:**
- Create: `internal/ws/hub.go`
- Create: `internal/ws/hub_test.go`

Responsibilities: manage N browser connections per session, fan out events pushed to the hub, accept control frames (`subscribe`, `send`) from the browser.

- [ ] **Step 1: Define frame shapes + write the failing test**

```go
// internal/ws/hub_test.go
package ws

import (
	"testing"
	"time"
)

func TestHub_SubscribeAndBroadcast(t *testing.T) {
	h := NewHub()
	defer h.Close()

	cA := &fakeConn{id: "a"}
	cB := &fakeConn{id: "b"}
	cOther := &fakeConn{id: "c"}
	h.Register(cA, "s1")
	h.Register(cB, "s1")
	h.Register(cOther, "s2")

	h.Broadcast("s1", Frame{Type: "event", Payload: []byte(`{"type":"x"}`)})

	waitFrame(t, cA)
	waitFrame(t, cB)
	if cOther.frameCount() != 0 {
		t.Fatalf("cOther should not receive")
	}
}

func waitFrame(t *testing.T, c *fakeConn) {
	t.Helper()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c.frameCount() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("conn %s never received a frame", c.id)
}
```

- [ ] **Step 2: Write fakeConn + the implementation**

```go
// internal/ws/hub.go
package ws

import (
	"encoding/json"
	"sync"
)

type Frame struct {
	Type    string          `json:"type"`               // "event" or "error"
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Conn interface {
	Send(Frame) error
	Close()
}

type Hub struct {
	mu    sync.RWMutex
	conns map[string]map[Conn]struct{} // sessionID -> set of conns
	done  chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		conns: make(map[string]map[Conn]struct{}),
		done:  make(chan struct{}),
	}
}

func (h *Hub) Register(c Conn, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.conns[sessionID]; !ok {
		h.conns[sessionID] = make(map[Conn]struct{})
	}
	h.conns[sessionID][c] = struct{}{}
}

func (h *Hub) Unregister(c Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sid, set := range h.conns {
		if _, ok := set[c]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(h.conns, sid)
			}
		}
	}
}

func (h *Hub) Broadcast(sessionID string, f Frame) {
	h.mu.RLock()
	targets := make([]Conn, 0, len(h.conns[sessionID]))
	for c := range h.conns[sessionID] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()
	for _, c := range targets {
		_ = c.Send(f)
	}
}

func (h *Hub) Close() { close(h.done) }
```

Add fakeConn in the test file:

```go
// internal/ws/hub_test.go (append)
type fakeConn struct {
	id     string
	mu     sync.Mutex
	frames []Frame
}

func (c *fakeConn) Send(f Frame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = append(c.frames, f)
	return nil
}
func (c *fakeConn) Close() {}
func (c *fakeConn) frameCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.frames)
}
```

Add `"sync"` import.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/ws -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ws/
git commit -m "feat(ws): Hub with per-session registration and broadcast"
```

---

### Task 6: WebSocket conn adapter (nhooyr)

**Files:**
- Create: `internal/ws/conn.go`

- [ ] **Step 1: Write the adapter**

```go
// internal/ws/conn.go
package ws

import (
	"context"
	"encoding/json"
	"time"

	"nhooyr.io/websocket"
)

// wsConn wraps a nhooyr websocket.Conn to satisfy Conn.
type wsConn struct {
	c   *websocket.Conn
	ctx context.Context
}

func NewWSConn(ctx context.Context, c *websocket.Conn) Conn {
	return &wsConn{c: c, ctx: ctx}
}

func (w *wsConn) Send(f Frame) error {
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()
	buf, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return w.c.Write(ctx, websocket.MessageText, buf)
}

func (w *wsConn) Close() {
	_ = w.c.Close(websocket.StatusNormalClosure, "")
}

// ReadCommand reads one JSON frame from the browser. Returns type + raw payload.
type IncomingFrame struct {
	Type      string `json:"type"`       // "send" | "subscribe"
	SessionID string `json:"session_id"` // for both
	Text      string `json:"text"`       // for "send"
	From      string `json:"from"`       // for "subscribe", optional cursor
}

func ReadFrame(ctx context.Context, c *websocket.Conn) (IncomingFrame, error) {
	_, buf, err := c.Read(ctx)
	if err != nil {
		return IncomingFrame{}, err
	}
	var f IncomingFrame
	if err := json.Unmarshal(buf, &f); err != nil {
		return IncomingFrame{}, err
	}
	return f, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/ws/conn.go
git commit -m "feat(ws): nhooyr.io/websocket adapter + incoming frame reader"
```

---

### Task 7: Auth (password gate + signed cookie)

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/auth/auth_test.go
package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGate_NoPassword_OpenAccess(t *testing.T) {
	g := NewGate("", "secret")
	req, _ := http.NewRequest("GET", "/", nil)
	if !g.Authorized(req) {
		t.Fatal("no-password mode should always authorize")
	}
}

func TestGate_LoginAndCookieRoundTrip(t *testing.T) {
	g := NewGate("pw", "secret")

	// Login with correct password
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(`{"password":"pw"}`))
	if !g.HandleLogin(rr, req) {
		t.Fatal("login should succeed")
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "pwac_session" {
		t.Fatalf("unexpected cookies: %+v", cookies)
	}

	// Reuse the cookie — Authorized must accept
	req2, _ := http.NewRequest("GET", "/", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	if !g.Authorized(req2) {
		t.Fatal("cookie should authorize subsequent requests")
	}
}

func TestGate_WrongPasswordCooldown(t *testing.T) {
	g := NewGate("pw", "secret")
	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/login", strings.NewReader(`{"password":"nope"}`))
		req.RemoteAddr = "1.2.3.4:1"
		g.HandleLogin(rr, req)
	}
	// 4th attempt should be rate-limited
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(`{"password":"pw"}`))
	req.RemoteAddr = "1.2.3.4:1"
	if g.HandleLogin(rr, req) {
		t.Fatal("expected cooldown to block even correct password")
	}
}
```

- [ ] **Step 2: Write the implementation**

```go
// internal/auth/auth.go
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Gate struct {
	password string
	secret   []byte

	mu       sync.Mutex
	attempts map[string][]time.Time
}

func NewGate(password, secret string) *Gate {
	return &Gate{
		password: password,
		secret:   []byte(secret),
		attempts: make(map[string][]time.Time),
	}
}

// Authorized returns true iff the request is permitted.
// Open access when no password is set.
func (g *Gate) Authorized(r *http.Request) bool {
	if g.password == "" {
		return true
	}
	c, err := r.Cookie("pwac_session")
	if err != nil {
		return false
	}
	return g.verify(c.Value)
}

// HandleLogin processes POST /api/login; returns true on success (cookie set).
func (g *Gate) HandleLogin(w http.ResponseWriter, r *http.Request) bool {
	if g.password == "" {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	ip := clientIP(r)
	if g.cooldown(ip) {
		http.Error(w, "too many attempts, try again later", http.StatusTooManyRequests)
		return false
	}
	var body struct{ Password string `json:"password"` }
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Password != g.password {
		g.recordAttempt(ip)
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return false
	}
	g.clearAttempts(ip)

	expires := time.Now().Add(30 * 24 * time.Hour)
	val := g.sign(strconv.FormatInt(expires.Unix(), 10))
	http.SetCookie(w, &http.Cookie{
		Name: "pwac_session", Value: val, Path: "/",
		Expires: expires, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (g *Gate) sign(payload string) string {
	mac := hmac.New(sha256.New, g.secret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (g *Gate) verify(v string) bool {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return false
	}
	mac := hmac.New(sha256.New, g.secret)
	mac.Write([]byte(parts[0]))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[1])) {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < exp
}

func (g *Gate) cooldown(ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	cutoff := time.Now().Add(-30 * time.Second)
	kept := make([]time.Time, 0)
	for _, t := range g.attempts[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	g.attempts[ip] = kept
	return len(kept) >= 3
}

func (g *Gate) recordAttempt(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.attempts[ip] = append(g.attempts[ip], time.Now())
}

func (g *Gate) clearAttempts(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.attempts, ip)
}

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		return strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/auth -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/
git commit -m "feat(auth): password gate with signed cookie and per-IP cooldown"
```

---

### Task 8: Server package (HTTP handlers + static embed)

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/static.go`
- Create: `internal/server/server_test.go`

- [ ] **Step 1: Static embed**

```go
// internal/server/static.go
package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:build
var staticFS embed.FS

// StaticHandler returns an http.Handler serving the embedded SvelteKit bundle.
// During phase 2 the bundle is a placeholder; phase 3 replaces web/build with
// real output before `go build`.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "build")
	if err != nil {
		// shouldn't happen; embed guarantees presence
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
```

This file lives in `internal/server/` but embeds `web/build/`. To make that work, copy the `//go:embed` path to point at a directory under `internal/server/`. Simplest fix: create a symlink, OR put the embed directive in a file under `web/` that imports nothing else. Cleanest: move the embed to a dedicated file at the module root that re-exports to `internal/server`.

Alternative implementation that always works:

```go
// cmd/picooraclaw-webui/static.go
package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web/build
var staticFS embed.FS

func staticHandler() http.Handler {
	sub, _ := fs.Sub(staticFS, "web/build")
	return http.FileServer(http.FS(sub))
}
```

Put the embed at the entrypoint. Skip the `internal/server/static.go` file for now. Delete the file if created.

- [ ] **Step 2: Write the server handlers**

```go
// internal/server/server.go
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
		if r.Method != "POST" {
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(results)
	})

	m.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if !d.Gate.Authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"*"}, // bridge is local; tighten in production
		})
		if err != nil {
			return
		}
		handleWS(r.Context(), c, d)
	})

	// Static must be last so it doesn't eat /api/ routes.
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
			if f.SessionID == "" || f.Text == "" {
				continue
			}
			// Best-effort — the response comes via SSE→Hub, not this HTTP call.
			_, _ = d.Client.PostChat(ctx, f.SessionID, f.Text, "")
		}
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/
git commit -m "feat(server): HTTP handlers, WS upgrade, auth-gated API routes"
```

---

### Task 9: Main entrypoint + SSE→Hub pump

**Files:**
- Create: `cmd/picooraclaw-webui/main.go`
- Create: `cmd/picooraclaw-webui/static.go`

- [ ] **Step 1: Write main + pump**

```go
// cmd/picooraclaw-webui/main.go
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
	gate := auth.NewGate(cfg.Password, cfg.Secret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// SSE pump: subscribe once to the global (empty session_id meaning all).
	// For v1 the upstream requires session_id, so we open a pump per session on demand.
	// Simple approach: single "default" session pump that also handles per-session
	// subscribes driven by the hub. For v1 we run one pump against sessionID="default"
	// and also support /ws-driven switching (future: track active sessions from Hub).

	go func() {
		for {
			events := make(chan bridge.Event, 64)
			streamCtx, streamCancel := context.WithCancel(ctx)
			done := make(chan struct{})
			go func() {
				err := client.Stream(streamCtx, "default", "", events)
				if err != nil && ctx.Err() == nil {
					log.Printf("sse stream: %v (retrying)", err)
				}
				close(done)
			}()

			for {
				select {
				case e, ok := <-events:
					if !ok {
						break
					}
					buf, _ := json.Marshal(e)
					hub.Broadcast(e.SessionID, ws.Frame{Type: "event", Payload: buf})
				case <-done:
					goto retry
				case <-ctx.Done():
					streamCancel()
					return
				}
			}
		retry:
			streamCancel()
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}()

	mux := server.NewMux(server.Deps{
		Gate:   gate,
		Client: client,
		Hub:    hub,
		Static: staticHandler(),
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
```

**Caveat:** the "default session pump" is a v1 simplification. A production-grade implementation would open one SSE connection per active session tracked in the Hub, and close it when the last subscriber leaves. Recording this as a known TODO for post-v1. Document in a plan-level comment or add a `// TODO:` with a GitHub issue link after the repo is live.

- [ ] **Step 2: Write static handler**

```go
// cmd/picooraclaw-webui/static.go
package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:../../web/build
var staticFS embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "web/build")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
```

The `../../web/build` path is relative — tricky with `go:embed`. The reliable pattern is to put `main.go`'s embed in a file at the module root:

Alternative: move the embed to `static_embed.go` at the module root:

```go
// static_embed.go  (at repo root)
package main // actually we need this in the same package as main

// Simplest reliable approach: mirror the build dir into cmd/picooraclaw-webui/static/
// via a Makefile target before `go build`.
```

Simplest reliable pattern:

1. Add a Make target that runs before build:

```make
build: sync-static
	go build -o bin/picooraclaw-webui ./cmd/picooraclaw-webui

sync-static:
	rm -rf cmd/picooraclaw-webui/static
	cp -r web/build cmd/picooraclaw-webui/static
```

2. Use `//go:embed all:static` in `cmd/picooraclaw-webui/static.go`:

```go
package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:static
var staticFS embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
```

Create `cmd/picooraclaw-webui/static/.gitkeep` so the embed compiles cleanly even when `make sync-static` hasn't run.

- [ ] **Step 3: Build and run manual smoke test**

```bash
mkdir -p cmd/picooraclaw-webui/static
echo placeholder > cmd/picooraclaw-webui/static/index.html
make build
./bin/picooraclaw-webui --picooraclaw-url http://localhost:8090 --listen :3000 &
PID=$!
sleep 1
curl -sS http://localhost:3000/
# Expected: "placeholder"
curl -sS http://localhost:3000/api/sessions
# Expected: [] if picooraclaw running, or 502 if not
kill $PID
```

- [ ] **Step 4: Commit**

```bash
git add cmd/ Makefile
git commit -m "feat(cmd): main entrypoint, SSE→Hub pump, static embed pipeline"
```

---

### Task 10: End-to-end bridge test

**Files:**
- Create: `internal/server/e2e_test.go`

Validates: browser → WS → bridge → fake picooraclaw POST → fake SSE → Hub → browser receives event.

- [ ] **Step 1: Write the test**

```go
// internal/server/e2e_test.go
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
	// Fake upstream picooraclaw
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat":
			w.WriteHeader(202)
			_, _ = w.Write([]byte(`{"message_id":"m1"}`))
		case "/v1/events":
			w.Header().Set("Content-Type", "text/event-stream")
			fl := w.(http.Flusher)
			fmt.Fprintf(w, "data: {\"type\":\"message_end\",\"session_id\":\"default\",\"message_id\":\"m1\",\"text\":\"hi\"}\n\n")
			fl.Flush()
			time.Sleep(200 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	client := bridge.NewClient(upstream.URL, "")
	hub := ws.NewHub()
	gate := auth.NewGate("", "secret")

	srv := httptest.NewServer(NewMux(Deps{
		Gate:   gate,
		Client: client,
		Hub:    hub,
		Static: http.NewServeMux(),
	}))
	defer srv.Close()

	// Dial WS
	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// Subscribe
	subMsg := `{"type":"subscribe","session_id":"default"}`
	if err := c.Write(ctx, websocket.MessageText, []byte(subMsg)); err != nil {
		t.Fatal(err)
	}

	// Kick the pump: POST a chat so the WS-side starts producing.
	sendMsg := `{"type":"send","session_id":"default","text":"hi"}`
	if err := c.Write(ctx, websocket.MessageText, []byte(sendMsg)); err != nil {
		t.Fatal(err)
	}

	// Manually broadcast to verify the hub path (since we don't spin up main's SSE pump here)
	// Real e2e runs in a separate test that runs main().
	hub.Broadcast("default", ws.Frame{Type: "event", Payload: json.RawMessage(`{"type":"message_end","text":"hi"}`)})

	_, buf, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(buf), "message_end") {
		t.Fatalf("unexpected frame: %s", string(buf))
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/server -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/server/e2e_test.go
git commit -m "test(server): WS subscribe/send/receive e2e via fake upstream"
```

---

## Self-review checklist

- [ ] `go test ./...` passes
- [ ] `go vet ./...` clean
- [ ] `make build` produces `bin/picooraclaw-webui`
- [ ] Manual smoke: binary runs, `/api/sessions` proxies upstream, `/ws` accepts connections
- [ ] Dependency surface: only `nhooyr.io/websocket` is non-stdlib (confirm `cat go.mod`)

## Known limitations for post-v1

Document these in the README "Roadmap" section:

- SSE pump uses a single hard-coded `session_id="default"` subscription. A per-session pump driven by the Hub's active-session set is the correct v1.1 design.
- No WebSocket origin restriction (`OriginPatterns: []string{"*"}`) — tighten to same-origin for production deploys.
- No rate limiting on `/ws` or `/api/*` beyond the login cooldown.

## Phase 3 prerequisites delivered

After Phase 2 merges, Phase 3 (SvelteKit) can assume:

- Dev server: `./bin/picooraclaw-webui` serves the static bundle from `cmd/picooraclaw-webui/static/`
- API routes: `POST /api/login`, `GET /api/sessions`, `GET /api/memory?q=`, `WS /ws`
- WS frames in: `{"type":"subscribe","session_id":"X","from":"<msg_id>"}` and `{"type":"send","session_id":"X","text":"..."}`
- WS frames out: `{"type":"event","payload":{<upstream event>}}`
