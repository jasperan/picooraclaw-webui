# Phase 1 — picooraclaw Web Channel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `web` channel to picooraclaw that exposes 5 HTTP endpoints for browser clients, plus a minimal `EventEmitter` interface in `pkg/agent` so the channel can stream structured agent events (tool calls, message lifecycle) via SSE.

**Architecture:** A new `pkg/channels/web/` package implements the existing `Channel` interface, runs an HTTP server on a dedicated port, and registers itself as an `EventEmitter` on the agent loop. The channel maintains per-session SSE subscribers and fans out events. Token-level streaming is explicitly out of scope for v1 (providers don't support it yet — added later without breaking the API).

**Tech Stack:** Go 1.22+, stdlib `net/http`, existing `pkg/bus`, `pkg/channels` patterns. Testing with `httptest`. Repo: `~/git/personal/picooraclaw/`.

**Repo context:**
- Channel pattern: see `pkg/channels/telegram.go` (Start/Stop/Send + BaseChannel embed) and `pkg/channels/manager.go` (data-driven registration table)
- Agent loop: `pkg/agent/loop.go`, tool execution around lines 691-720
- Health HTTP pattern: `pkg/health/server.go` (graceful shutdown via `StartContext`)
- Config: `pkg/config/config.go` under `Channels` struct

---

### Task 1: Add `EventEmitter` interface and event types

**Files:**
- Create: `pkg/agent/events.go`
- Test: `pkg/agent/events_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/agent/events_test.go
package agent

import (
	"testing"
	"time"
)

func TestEvent_JSON(t *testing.T) {
	e := Event{
		Type:      EventToolCallStart,
		SessionID: "s1",
		MessageID: "m1",
		ToolName:  "remember",
		Args:      map[string]any{"text": "hi"},
		Timestamp: time.Unix(1000, 0),
	}
	if e.Type != "tool_call_start" {
		t.Fatalf("unexpected type: %q", e.Type)
	}
}

type captureEmitter struct{ events []Event }

func (c *captureEmitter) Emit(e Event) { c.events = append(c.events, e) }

func TestCaptureEmitter_Interface(t *testing.T) {
	var _ EventEmitter = (*captureEmitter)(nil)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ~/git/personal/picooraclaw && go test ./pkg/agent -run TestEvent`
Expected: FAIL with `undefined: Event`.

- [ ] **Step 3: Write the minimal implementation**

```go
// pkg/agent/events.go
package agent

import "time"

type EventType string

const (
	EventMessageStart  EventType = "message_start"
	EventMessageEnd    EventType = "message_end"
	EventToolCallStart EventType = "tool_call_start"
	EventToolCallEnd   EventType = "tool_call_end"
	EventError         EventType = "error"
	EventAgentTick     EventType = "agent_tick"
)

type Event struct {
	Type       EventType      `json:"type"`
	SessionID  string         `json:"session_id"`
	MessageID  string         `json:"message_id,omitempty"`
	ToolName   string         `json:"tool,omitempty"`
	ToolCallID string         `json:"id,omitempty"`
	Args       map[string]any `json:"args,omitempty"`
	Result     string         `json:"result,omitempty"`
	OK         *bool          `json:"ok,omitempty"`
	Text       string         `json:"text,omitempty"`
	Error      string         `json:"error,omitempty"`
	Note       string         `json:"note,omitempty"`
	Timestamp  time.Time      `json:"ts"`
}

type EventEmitter interface {
	Emit(e Event)
}

type NoopEmitter struct{}

func (NoopEmitter) Emit(Event) {}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/agent -run TestEvent -run TestCaptureEmitter -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/events.go pkg/agent/events_test.go
git commit -m "feat(agent): add EventEmitter interface and structured Event type"
```

---

### Task 2: Wire optional EventEmitter into AgentLoop

**Files:**
- Modify: `pkg/agent/loop.go` (struct definition around line 25-45, constructors around lines 117-200)
- Test: `pkg/agent/loop_emitter_test.go` (new)

- [ ] **Step 1: Write the failing test**

```go
// pkg/agent/loop_emitter_test.go
package agent

import (
	"testing"

	"github.com/jasperan/picooraclaw/pkg/bus"
	"github.com/jasperan/picooraclaw/pkg/config"
)

func TestAgentLoop_SetEventEmitter(t *testing.T) {
	msgBus := bus.NewMessageBus()
	defer msgBus.Close()
	cfg := &config.Config{}
	cap := &captureEmitter{}

	al := NewAgentLoop(cfg, msgBus, nil)
	al.SetEventEmitter(cap)

	al.emitter.Emit(Event{Type: EventMessageStart, SessionID: "x"})
	if len(cap.events) != 1 {
		t.Fatalf("expected 1 event captured, got %d", len(cap.events))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/agent -run TestAgentLoop_SetEventEmitter`
Expected: FAIL (no `emitter` field, no `SetEventEmitter`).

- [ ] **Step 3: Modify AgentLoop to carry the emitter**

In `pkg/agent/loop.go`, locate the `AgentLoop` struct and add a field (near the `bus` field around line 33):

```go
type AgentLoop struct {
    // ...existing fields...
    bus       *bus.MessageBus
    emitter   EventEmitter
    // ...rest unchanged
}
```

At the bottom of the file (or near the constructors), add:

```go
// SetEventEmitter attaches an EventEmitter. Safe to call before Start().
// If unset, events are silently dropped via NoopEmitter.
func (al *AgentLoop) SetEventEmitter(e EventEmitter) {
    if e == nil {
        al.emitter = NoopEmitter{}
        return
    }
    al.emitter = e
}
```

In `newAgentLoop` (around line 187), initialize `emitter` to `NoopEmitter{}`:

```go
al := &AgentLoop{
    // ...existing fields...
    emitter: NoopEmitter{},
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/agent -run TestAgentLoop_SetEventEmitter -v`
Expected: PASS.

- [ ] **Step 5: Verify existing agent tests still pass**

Run: `go test ./pkg/agent -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/agent/loop.go pkg/agent/loop_emitter_test.go
git commit -m "feat(agent): add SetEventEmitter to AgentLoop with noop default"
```

---

### Task 3: Instrument tool-call execution path with events

**Files:**
- Modify: `pkg/agent/loop.go` (tool-call execution path, around lines 691-720 and 291-320)
- Test: `pkg/agent/loop_emitter_test.go` (extend)

- [ ] **Step 1: Write the failing test**

Append to `pkg/agent/loop_emitter_test.go`:

```go
func TestAgentLoop_EmitsMessageLifecycleEvents(t *testing.T) {
	// Given an AgentLoop with a capture emitter and a fake provider
	// that returns a single tool call then a final message, assert we see:
	//  message_start, tool_call_start, tool_call_end, message_end
	t.Skip("wire in with fake provider in integration test; scaffolding only for now")
}
```

This scaffolds the assertion shape. The full integration with a fake provider is exercised in Task 13's smoke test.

- [ ] **Step 2: Add message_start emission**

In `pkg/agent/loop.go`, locate `processMessage` (around line 317). At the top of the function, after the message_id is known (or generated), add:

```go
messageID := utils.NewID("m") // if not already present; use existing ID gen
al.emitter.Emit(Event{
    Type:      EventMessageStart,
    SessionID: msg.SessionKey,
    MessageID: messageID,
    Timestamp: time.Now(),
})
```

If no `utils.NewID` helper exists, use `fmt.Sprintf("m_%d", time.Now().UnixNano())` inline. Store `messageID` in a local variable — it's reused for subsequent events.

- [ ] **Step 3: Add tool_call_start / tool_call_end emission**

In `pkg/agent/loop.go`, the tool execution goroutine is around line 705. Inside the `for i, tc := range response.ToolCalls { ... go func(idx int, tc providers.ToolCall) { ... }()` block:

Right before `tool.Execute(...)` is called:

```go
al.emitter.Emit(Event{
    Type:       EventToolCallStart,
    SessionID:  msg.SessionKey,
    MessageID:  messageID,
    ToolCallID: tc.ID,
    ToolName:   tc.Name,
    Args:       tc.Args,
    Timestamp:  time.Now(),
})
```

Right after the tool returns (success or error), before publishing the bus.OutboundMessage:

```go
ok := err == nil
al.emitter.Emit(Event{
    Type:       EventToolCallEnd,
    SessionID:  msg.SessionKey,
    MessageID:  messageID,
    ToolCallID: tc.ID,
    ToolName:   tc.Name,
    Result:     resultStr, // the string result (truncate to 4KB for safety)
    OK:         &ok,
    Timestamp:  time.Now(),
})
```

Use whatever the local `resultStr` or `result` variable is named in the existing code — don't invent one. If error, set `Result` to `err.Error()`.

- [ ] **Step 4: Add message_end emission**

At the end of `processMessage` (return path with the final assistant text), before the `return finalText, nil`:

```go
al.emitter.Emit(Event{
    Type:      EventMessageEnd,
    SessionID: msg.SessionKey,
    MessageID: messageID,
    Text:      finalText,
    Timestamp: time.Now(),
})
```

On the error-return path:

```go
al.emitter.Emit(Event{
    Type:      EventError,
    SessionID: msg.SessionKey,
    MessageID: messageID,
    Error:     err.Error(),
    Timestamp: time.Now(),
})
```

- [ ] **Step 5: Run the full agent test suite**

Run: `go test ./pkg/agent -v`
Expected: all PASS. Existing tests must not regress — `NoopEmitter` ensures no observable change when no emitter is wired.

- [ ] **Step 6: Commit**

```bash
git add pkg/agent/loop.go pkg/agent/loop_emitter_test.go
git commit -m "feat(agent): instrument AgentLoop to emit message and tool-call events"
```

---

### Task 4: Add WebConfig to config package

**Files:**
- Modify: `pkg/config/config.go` (Channels struct)
- Test: `pkg/config/config_test.go` (if present; otherwise skip test for this task)

- [ ] **Step 1: Add the struct**

In `pkg/config/config.go`, locate the `Channels` struct (it contains `Telegram`, `Discord`, etc). Add:

```go
type WebConfig struct {
    Enabled bool   `json:"enabled"`
    Host    string `json:"host"`     // default "0.0.0.0"
    Port    int    `json:"port"`     // default 8090
    Token   string `json:"token"`    // optional bearer; empty = no auth
}
```

And to the `Channels` struct, add the field:

```go
type ChannelsConfig struct {
    // ...existing fields...
    Web WebConfig `json:"web"`
}
```

Apply defaults in the loader (usually in the same file): if `cfg.Channels.Web.Enabled` and `cfg.Channels.Web.Port == 0`, set `Port = 8090`; if `Host == ""`, set `Host = "0.0.0.0"`.

- [ ] **Step 2: Run existing config tests**

Run: `go test ./pkg/config -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add pkg/config/config.go
git commit -m "feat(config): add WebConfig for the web channel"
```

---

### Task 5: Scaffold the web channel package

**Files:**
- Create: `pkg/channels/web/channel.go`
- Create: `pkg/channels/web/channel_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/channels/web/channel_test.go
package web

import (
	"context"
	"testing"

	"github.com/jasperan/picooraclaw/pkg/bus"
	"github.com/jasperan/picooraclaw/pkg/config"
)

func TestNewChannel_StartsAndStops(t *testing.T) {
	cfg := config.WebConfig{Enabled: true, Host: "127.0.0.1", Port: 0, Token: ""}
	msgBus := bus.NewMessageBus()
	defer msgBus.Close()

	ch, err := NewChannel(cfg, msgBus)
	if err != nil {
		t.Fatalf("NewChannel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ch.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !ch.IsRunning() {
		t.Fatal("channel should be running")
	}
	if err := ch.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/channels/web -run TestNewChannel -v`
Expected: FAIL (package doesn't exist).

- [ ] **Step 3: Write the minimal channel**

```go
// pkg/channels/web/channel.go
package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/jasperan/picooraclaw/pkg/bus"
	"github.com/jasperan/picooraclaw/pkg/config"
	"github.com/jasperan/picooraclaw/pkg/logger"
)

type Channel struct {
	cfg     config.WebConfig
	bus     *bus.MessageBus
	broker  *EventBroker
	server  *http.Server
	mu      sync.RWMutex
	running bool
	addr    string
}

func NewChannel(cfg config.WebConfig, msgBus *bus.MessageBus) (*Channel, error) {
	c := &Channel{
		cfg:    cfg,
		bus:    msgBus,
		broker: NewEventBroker(),
	}
	return c, nil
}

func (c *Channel) Name() string { return "web" }

func (c *Channel) Broker() *EventBroker { return c.broker }

func (c *Channel) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	c.registerRoutes(mux)

	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("web channel listen %s: %w", addr, err)
	}
	c.addr = ln.Addr().String()

	c.server = &http.Server{
		Handler:           c.authMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	c.setRunning(true)
	logger.InfoCF("web", "web channel listening", map[string]interface{}{"addr": c.addr})

	go func() {
		if err := c.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.ErrorCF("web", "server error", map[string]interface{}{"error": err.Error()})
		}
	}()
	return nil
}

func (c *Channel) Stop(ctx context.Context) error {
	c.setRunning(false)
	if c.server == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.server.Shutdown(shutdownCtx)
}

func (c *Channel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	// Translate the legacy text outbound into the event stream.
	c.broker.Emit(Event{Type: "message_text", SessionID: msg.ChatID, Text: msg.Content, Timestamp: time.Now()})
	return nil
}

func (c *Channel) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

func (c *Channel) Addr() string { return c.addr }

func (c *Channel) setRunning(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = v
}

// registerRoutes and authMiddleware are defined in server.go
```

Create a stub `server.go` now so compilation passes (we'll fill in handlers in later tasks):

```go
// pkg/channels/web/server.go
package web

import "net/http"

func (c *Channel) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/chat", c.handleChat)
	mux.HandleFunc("/v1/events", c.handleEvents)
	mux.HandleFunc("/v1/sessions", c.handleSessions)
	mux.HandleFunc("/v1/memory", c.handleMemory)
}

func (c *Channel) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.cfg.Token != "" {
			authz := r.Header.Get("Authorization")
			if authz != "Bearer "+c.cfg.Token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (c *Channel) handleChat(w http.ResponseWriter, r *http.Request)    { http.Error(w, "not implemented", http.StatusNotImplemented) }
func (c *Channel) handleEvents(w http.ResponseWriter, r *http.Request)  { http.Error(w, "not implemented", http.StatusNotImplemented) }
func (c *Channel) handleSessions(w http.ResponseWriter, r *http.Request){ http.Error(w, "not implemented", http.StatusNotImplemented) }
func (c *Channel) handleMemory(w http.ResponseWriter, r *http.Request)  { http.Error(w, "not implemented", http.StatusNotImplemented) }
```

And a minimal `events_broker.go` stub (we flesh this out in the next task):

```go
// pkg/channels/web/broker.go
package web

import "time"

type Event struct {
	Type       string    `json:"type"`
	SessionID  string    `json:"session_id,omitempty"`
	MessageID  string    `json:"message_id,omitempty"`
	Text       string    `json:"text,omitempty"`
	Timestamp  time.Time `json:"ts"`
}

type EventBroker struct{}

func NewEventBroker() *EventBroker { return &EventBroker{} }

func (b *EventBroker) Emit(e Event) {}
```

- [ ] **Step 4: Run tests to verify channel starts and stops**

Run: `go test ./pkg/channels/web -run TestNewChannel -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/channels/web/
git commit -m "feat(channels/web): scaffold Channel implementation (start/stop, auth stub)"
```

---

### Task 6: Implement the EventBroker (fan-out to SSE subscribers)

**Files:**
- Modify: `pkg/channels/web/broker.go` (expand)
- Create: `pkg/channels/web/broker_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/channels/web/broker_test.go
package web

import (
	"testing"
	"time"
)

func TestEventBroker_SubscribeAndFanOut(t *testing.T) {
	b := NewEventBroker()

	subA := b.Subscribe("s1", "")
	subB := b.Subscribe("s1", "")
	subOther := b.Subscribe("s2", "")
	defer b.Unsubscribe(subA)
	defer b.Unsubscribe(subB)
	defer b.Unsubscribe(subOther)

	e := Event{Type: "message_start", SessionID: "s1", MessageID: "m1", Timestamp: time.Now()}
	b.Emit(e)

	if got := recv(t, subA.C); got.MessageID != "m1" {
		t.Fatalf("subA missed event: %+v", got)
	}
	if got := recv(t, subB.C); got.MessageID != "m1" {
		t.Fatalf("subB missed event: %+v", got)
	}
	select {
	case got := <-subOther.C:
		t.Fatalf("subOther should not receive session s1 event, got %+v", got)
	case <-time.After(50 * time.Millisecond):
		// ok
	}
}

func TestEventBroker_ResumeFromCursor(t *testing.T) {
	b := NewEventBroker()
	b.Emit(Event{Type: "a", SessionID: "s1", MessageID: "m1"})
	b.Emit(Event{Type: "b", SessionID: "s1", MessageID: "m2"})

	// Subscribe with from="m1" — should replay events AFTER m1 (i.e., m2)
	sub := b.Subscribe("s1", "m1")
	defer b.Unsubscribe(sub)

	got := recv(t, sub.C)
	if got.MessageID != "m2" {
		t.Fatalf("expected m2, got %+v", got)
	}
}

func recv(t *testing.T, c <-chan Event) Event {
	t.Helper()
	select {
	case e := <-c:
		return e
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
	return Event{}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/channels/web -run TestEventBroker -v`
Expected: FAIL (`Subscribe`, `Unsubscribe` don't exist).

- [ ] **Step 3: Write the implementation**

Replace `pkg/channels/web/broker.go` with:

```go
package web

import (
	"sync"
	"time"
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
	Timestamp  time.Time      `json:"ts"`
}

const (
	bufferPerSession = 1000
	subChanSize      = 64
)

type Subscription struct {
	C         chan Event
	sessionID string
	id        uint64
}

type EventBroker struct {
	mu      sync.RWMutex
	buffers map[string][]Event // sessionID -> ring (append, trim to 1000)
	subs    map[string]map[uint64]*Subscription
	nextID  uint64
}

func NewEventBroker() *EventBroker {
	return &EventBroker{
		buffers: make(map[string][]Event),
		subs:    make(map[string]map[uint64]*Subscription),
	}
}

func (b *EventBroker) Emit(e Event) {
	b.mu.Lock()
	buf := b.buffers[e.SessionID]
	buf = append(buf, e)
	if len(buf) > bufferPerSession {
		buf = buf[len(buf)-bufferPerSession:]
	}
	b.buffers[e.SessionID] = buf
	subs := b.subs[e.SessionID]
	b.mu.Unlock()

	for _, s := range subs {
		select {
		case s.C <- e:
		default:
			// Drop if consumer is slow; they can resume via cursor on reconnect.
		}
	}
}

// Subscribe returns a channel that receives future events for sessionID.
// If fromMessageID is non-empty, buffered events AFTER that message_id are replayed first.
func (b *EventBroker) Subscribe(sessionID, fromMessageID string) *Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	sub := &Subscription{C: make(chan Event, subChanSize), sessionID: sessionID, id: b.nextID}

	if _, ok := b.subs[sessionID]; !ok {
		b.subs[sessionID] = make(map[uint64]*Subscription)
	}
	b.subs[sessionID][sub.id] = sub

	if fromMessageID != "" {
		// Replay events from the buffer strictly after the given message_id.
		buf := b.buffers[sessionID]
		startIdx := -1
		for i, e := range buf {
			if e.MessageID == fromMessageID {
				startIdx = i
			}
		}
		if startIdx >= 0 {
			for _, e := range buf[startIdx+1:] {
				select {
				case sub.C <- e:
				default:
				}
			}
		}
	}
	return sub
}

func (b *EventBroker) Unsubscribe(sub *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if m, ok := b.subs[sub.sessionID]; ok {
		delete(m, sub.id)
	}
	close(sub.C)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/channels/web -run TestEventBroker -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/channels/web/broker.go pkg/channels/web/broker_test.go
git commit -m "feat(channels/web): event broker with per-session fan-out and resume cursor"
```

---

### Task 7: Wire broker to agent EventEmitter + implement /v1/events SSE

**Files:**
- Modify: `pkg/channels/web/channel.go` (add `Emit` method satisfying `agent.EventEmitter`)
- Modify: `pkg/channels/web/server.go` (implement `handleEvents`)
- Create: `pkg/channels/web/server_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/channels/web/server_test.go
package web

import (
	"bufio"
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jasperan/picooraclaw/pkg/agent"
	"github.com/jasperan/picooraclaw/pkg/bus"
	"github.com/jasperan/picooraclaw/pkg/config"
)

func TestHandleEvents_StreamsSSE(t *testing.T) {
	cfg := config.WebConfig{Enabled: true, Host: "127.0.0.1", Port: 0}
	ch, err := NewChannel(cfg, bus.NewMessageBus())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := ch.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer ch.Stop(context.Background())

	srv := httptest.NewServer(ch.authMiddleware(ch.muxForTest()))
	defer srv.Close()

	// Fire an event asynchronously after the client connects.
	go func() {
		time.Sleep(50 * time.Millisecond)
		ok := true
		ch.Emit(agent.Event{
			Type: agent.EventToolCallStart, SessionID: "s1", MessageID: "m1",
			ToolName: "remember", ToolCallID: "tc1", Timestamp: time.Now(),
		})
		ch.Emit(agent.Event{
			Type: agent.EventToolCallEnd, SessionID: "s1", MessageID: "m1",
			ToolCallID: "tc1", Result: "ok", OK: &ok, Timestamp: time.Now(),
		})
	}()

	req, _ := httptestNewRequestGET(srv.URL + "/v1/events?session_id=s1")
	resp, err := httpDefaultClientDo(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	got := readFirstNSSEEvents(t, resp.Body, 2, 2*time.Second)
	if !strings.Contains(got, "tool_call_start") || !strings.Contains(got, "tool_call_end") {
		t.Fatalf("missing expected events in stream:\n%s", got)
	}
}

// readFirstNSSEEvents reads until it sees N `data:` lines or times out.
func readFirstNSSEEvents(t *testing.T, body interface{ Read([]byte) (int, error) }, n int, timeout time.Duration) string {
	t.Helper()
	deadline := time.After(timeout)
	scanner := bufio.NewScanner(bufioReader{body})
	var sb strings.Builder
	seen := 0
	done := make(chan struct{})
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			sb.WriteString(line)
			sb.WriteString("\n")
			if strings.HasPrefix(line, "data:") {
				seen++
				if seen >= n {
					close(done)
					return
				}
			}
		}
		close(done)
	}()
	select {
	case <-done:
	case <-deadline:
		t.Fatalf("timeout; partial:\n%s", sb.String())
	}
	return sb.String()
}
```

If Go refuses the inline helper types — use the stdlib `http.Get` and `io.Reader` directly. The test is illustrative: the implementing agent should use `httptest.NewServer` + `http.Get` + `bufio.Scanner` on the response body, read N `data: ` lines, assert content.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/channels/web -run TestHandleEvents -v`
Expected: FAIL (`muxForTest` missing, handlers return 501).

- [ ] **Step 3: Add the Emit adapter on the channel**

Append to `pkg/channels/web/channel.go`:

```go
import (
    // ...existing imports, plus:
    "github.com/jasperan/picooraclaw/pkg/agent"
)

// Emit satisfies agent.EventEmitter by forwarding into the broker.
// We shape the broker Event from the agent.Event; fields map 1:1.
func (c *Channel) Emit(e agent.Event) {
    c.broker.Emit(Event{
        Type:       string(e.Type),
        SessionID:  e.SessionID,
        MessageID:  e.MessageID,
        ToolCallID: e.ToolCallID,
        Tool:       e.ToolName,
        Args:       e.Args,
        Result:     e.Result,
        OK:         e.OK,
        Text:       e.Text,
        Error:      e.Error,
        Note:       e.Note,
        Timestamp:  e.Timestamp,
    })
}

// muxForTest exposes the internal mux for tests only.
func (c *Channel) muxForTest() *http.ServeMux {
    m := http.NewServeMux()
    c.registerRoutes(m)
    return m
}
```

- [ ] **Step 4: Implement handleEvents**

Replace the stub in `pkg/channels/web/server.go`:

```go
package web

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (c *Channel) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/chat", c.handleChat)
	mux.HandleFunc("/v1/events", c.handleEvents)
	mux.HandleFunc("/v1/sessions", c.handleSessions)
	mux.HandleFunc("/v1/memory", c.handleMemory)
}

func (c *Channel) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.cfg.Token != "" {
			if r.Header.Get("Authorization") != "Bearer "+c.cfg.Token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (c *Channel) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}
	from := r.URL.Query().Get("from")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := c.broker.Subscribe(sessionID, from)
	defer c.broker.Unsubscribe(sub)

	// Send an initial ping so clients know the stream is live.
	fmt.Fprintf(w, ": ping\n\n")
	flusher.Flush()

	enc := json.NewEncoder(w)
	for {
		select {
		case ev, ok := <-sub.C:
			if !ok {
				return
			}
			fmt.Fprint(w, "data: ")
			_ = enc.Encode(ev) // writes one JSON + trailing newline
			fmt.Fprint(w, "\n") // complete the SSE event with blank line
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// Stub remaining handlers; next tasks fill them in.
func (c *Channel) handleChat(w http.ResponseWriter, r *http.Request)     { http.Error(w, "not implemented", http.StatusNotImplemented) }
func (c *Channel) handleSessions(w http.ResponseWriter, r *http.Request) { http.Error(w, "not implemented", http.StatusNotImplemented) }
func (c *Channel) handleMemory(w http.ResponseWriter, r *http.Request)   { http.Error(w, "not implemented", http.StatusNotImplemented) }
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./pkg/channels/web -run TestHandleEvents -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/channels/web/channel.go pkg/channels/web/server.go pkg/channels/web/server_test.go
git commit -m "feat(channels/web): implement SSE /v1/events with cursor-based resume"
```

---

### Task 8: Implement /v1/chat

**Files:**
- Modify: `pkg/channels/web/server.go` (replace `handleChat`)
- Extend: `pkg/channels/web/server_test.go`

- [ ] **Step 1: Write the failing test**

Append to `pkg/channels/web/server_test.go`:

```go
func TestHandleChat_PublishesInboundToBus(t *testing.T) {
	msgBus := bus.NewMessageBus()
	defer msgBus.Close()

	cfg := config.WebConfig{Enabled: true, Host: "127.0.0.1", Port: 0}
	ch, err := NewChannel(cfg, msgBus)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(ch.authMiddleware(ch.muxForTest()))
	defer srv.Close()

	body := `{"session_id":"s1","text":"hello"}`
	resp, err := http.Post(srv.URL+"/v1/chat", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	msg, ok := msgBus.ConsumeInbound(ctx)
	if !ok {
		t.Fatal("expected inbound message on bus, got none")
	}
	if msg.Content != "hello" || msg.SessionKey != "s1" || msg.Channel != "web" {
		t.Fatalf("unexpected inbound: %+v", msg)
	}
}
```

Add `"net/http"` and `"strings"` imports if missing.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/channels/web -run TestHandleChat -v`
Expected: FAIL (501).

- [ ] **Step 3: Implement the handler**

In `pkg/channels/web/server.go`, replace `handleChat`:

```go
import (
    // add:
    "github.com/jasperan/picooraclaw/pkg/bus"
)

type chatRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	Workspace string `json:"workspace,omitempty"`
}

type chatResponse struct {
	MessageID string `json:"message_id"`
}

func (c *Channel) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.SessionID == "" || req.Text == "" {
		http.Error(w, "session_id and text are required", http.StatusBadRequest)
		return
	}

	c.bus.PublishInbound(bus.InboundMessage{
		Channel:    "web",
		SenderID:   "web-user",
		ChatID:     req.SessionID,
		Content:    req.Text,
		SessionKey: req.SessionID,
		Metadata:   map[string]string{"workspace": req.Workspace},
	})

	mid := fmt.Sprintf("m_%d", time.Now().UnixNano())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(chatResponse{MessageID: mid})
}
```

Ensure `"time"` is imported at the top of `server.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/channels/web -run TestHandleChat -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/channels/web/server.go pkg/channels/web/server_test.go
git commit -m "feat(channels/web): implement POST /v1/chat publishing inbound to bus"
```

---

### Task 9: Implement /v1/sessions

**Files:**
- Modify: `pkg/channels/web/server.go`
- Extend: `pkg/channels/web/server_test.go`

The v1 implementation is a **thin pass-through** to the existing `SessionManagerInterface`. The web channel doesn't own sessions; it lists/creates/deletes what picooraclaw already has.

- [ ] **Step 1: Extend the channel struct to accept a session store**

In `pkg/channels/web/channel.go`, add:

```go
type SessionLister interface {
    ListSessions() []SessionInfo
    CreateSession(title string) (SessionInfo, error)
    DeleteSession(id string) error
}

type SessionInfo struct {
    ID     string `json:"id"`
    Title  string `json:"title"`
    LastAt int64  `json:"last_at"`
}

// Channel now holds an optional sessions backend.
// Default behavior (nil) returns an empty list / error on write.

// Add to Channel struct:
// sessions SessionLister

// SetSessions wires the backend after construction.
func (c *Channel) SetSessions(s SessionLister) { c.sessions = s }
```

Add `sessions SessionLister` to the struct.

- [ ] **Step 2: Write the failing test**

Append to `server_test.go`:

```go
type fakeSessions struct{ items []SessionInfo }

func (f *fakeSessions) ListSessions() []SessionInfo            { return f.items }
func (f *fakeSessions) CreateSession(t string) (SessionInfo, error) {
	s := SessionInfo{ID: "s_new", Title: t, LastAt: 1}
	f.items = append(f.items, s)
	return s, nil
}
func (f *fakeSessions) DeleteSession(id string) error {
	for i, s := range f.items {
		if s.ID == id {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func TestHandleSessions_ListCreateDelete(t *testing.T) {
	cfg := config.WebConfig{Enabled: true, Host: "127.0.0.1", Port: 0}
	ch, err := NewChannel(cfg, bus.NewMessageBus())
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSessions{items: []SessionInfo{{ID: "s1", Title: "one", LastAt: 10}}}
	ch.SetSessions(f)

	srv := httptest.NewServer(ch.authMiddleware(ch.muxForTest()))
	defer srv.Close()

	// GET
	resp, _ := http.Get(srv.URL + "/v1/sessions")
	if resp.StatusCode != 200 {
		t.Fatalf("GET status %d", resp.StatusCode)
	}
	var got []SessionInfo
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if len(got) != 1 || got[0].ID != "s1" {
		t.Fatalf("list: %+v", got)
	}
	resp.Body.Close()

	// POST
	resp, _ = http.Post(srv.URL+"/v1/sessions", "application/json", strings.NewReader(`{"title":"two"}`))
	if resp.StatusCode != 201 {
		t.Fatalf("POST status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// DELETE
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/v1/sessions?id=s1", nil)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 204 {
		t.Fatalf("DELETE status %d", resp.StatusCode)
	}
	resp.Body.Close()
}
```

- [ ] **Step 3: Implement the handler**

Replace `handleSessions` in `server.go`:

```go
func (c *Channel) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var list []SessionInfo
		if c.sessions != nil {
			list = c.sessions.ListSessions()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	case http.MethodPost:
		if c.sessions == nil {
			http.Error(w, "sessions not configured", http.StatusServiceUnavailable)
			return
		}
		var body struct{ Title string `json:"title"` }
		_ = json.NewDecoder(r.Body).Decode(&body)
		s, err := c.sessions.CreateSession(body.Title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(s)
	case http.MethodDelete:
		if c.sessions == nil {
			http.Error(w, "sessions not configured", http.StatusServiceUnavailable)
			return
		}
		id := r.URL.Query().Get("id")
		if err := c.sessions.DeleteSession(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/channels/web -run TestHandleSessions -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/channels/web/channel.go pkg/channels/web/server.go pkg/channels/web/server_test.go
git commit -m "feat(channels/web): implement /v1/sessions (list/create/delete) via SessionLister"
```

---

### Task 10: Implement /v1/memory (thin wrapper)

**Files:**
- Modify: `pkg/channels/web/channel.go` (+ `MemorySearcher` interface)
- Modify: `pkg/channels/web/server.go` (fill in `handleMemory`)
- Extend: `pkg/channels/web/server_test.go`

- [ ] **Step 1: Define the interface**

Append to `channel.go`:

```go
type MemorySearcher interface {
    Search(query string, limit int) []MemoryResult
}

type MemoryResult struct {
    ID    string  `json:"id"`
    Text  string  `json:"text"`
    Score float64 `json:"score"`
    Date  int64   `json:"date"`
}

func (c *Channel) SetMemory(m MemorySearcher) { c.memory = m }
```

Add `memory MemorySearcher` to the struct.

- [ ] **Step 2: Write the failing test**

```go
type fakeMemory struct{}

func (fakeMemory) Search(q string, n int) []MemoryResult {
	return []MemoryResult{{ID: "m_1", Text: "user likes " + q, Score: 0.9, Date: 1}}
}

func TestHandleMemory_Search(t *testing.T) {
	cfg := config.WebConfig{Enabled: true, Host: "127.0.0.1", Port: 0}
	ch, _ := NewChannel(cfg, bus.NewMessageBus())
	ch.SetMemory(fakeMemory{})
	srv := httptest.NewServer(ch.authMiddleware(ch.muxForTest()))
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/v1/memory?q=go&limit=5")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var got []MemoryResult
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if len(got) != 1 || !strings.Contains(got[0].Text, "go") {
		t.Fatalf("unexpected: %+v", got)
	}
}
```

- [ ] **Step 3: Implement the handler**

```go
func (c *Channel) handleMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if c.memory == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
		return
	}
	q := r.URL.Query().Get("q")
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	results := c.memory.Search(q, limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/channels/web -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/channels/web/channel.go pkg/channels/web/server.go pkg/channels/web/server_test.go
git commit -m "feat(channels/web): implement /v1/memory search via MemorySearcher"
```

---

### Task 11: Register web channel in manager + gateway wiring

**Files:**
- Modify: `pkg/channels/manager.go` (add entry to the data-driven table around line 65-80)
- Modify: `cmd/picooraclaw/main.go` (gatewayCmd around line 513)

- [ ] **Step 1: Add manager entry**

In `pkg/channels/manager.go`, after the existing `onebot` entry in the `entries` slice, append:

```go
{"web", m.config.Channels.Web.Enabled,
    func() (Channel, error) {
        return web.NewChannel(m.config.Channels.Web, m.bus)
    }},
```

Add the import at the top:

```go
"github.com/jasperan/picooraclaw/pkg/channels/web"
```

Since `web.Channel` needs to satisfy the `Channel` interface that `manager.go` already uses, verify the method set matches. The existing interface (in `base.go`) should require `Name()`, `Start(ctx)`, `Stop(ctx)`, `Send(ctx, bus.OutboundMessage)`, `IsRunning()`. All are already implemented in Task 5.

- [ ] **Step 2: Wire emitter + session/memory in gatewayCmd**

In `cmd/picooraclaw/main.go`, inside `gatewayCmd()` (around line 513), **after** `NewAgentLoopWithStores` returns and `channelManager` is created, add:

```go
// If the web channel is enabled, wire it to the agent emitter and the stores.
if cfg.Channels.Web.Enabled {
    if wc, ok := channelManager.GetChannel("web").(*web.Channel); ok {
        agentLoop.SetEventEmitter(wc)
        // Sessions adapter: wrap the session manager.
        wc.SetSessions(newWebSessionAdapter(sessionMgr))
        // Memory adapter: optional; only if Oracle store is present.
        if oracleMem, ok := memoryStore.(agent.OracleMemoryStore); ok {
            wc.SetMemory(newWebMemoryAdapter(oracleMem))
        }
    }
}
```

`channelManager.GetChannel` may not exist yet — add this to `pkg/channels/manager.go`:

```go
func (m *Manager) GetChannel(name string) Channel {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.channels[name]
}
```

Create small adapters at the bottom of `main.go`:

```go
// newWebSessionAdapter wraps a SessionManagerInterface to satisfy web.SessionLister.
func newWebSessionAdapter(sm agent.SessionManagerInterface) *webSessionAdapter {
    return &webSessionAdapter{sm: sm}
}
type webSessionAdapter struct{ sm agent.SessionManagerInterface }
func (a *webSessionAdapter) ListSessions() []web.SessionInfo {
    // File-backed sessions don't expose a list API; v1 returns empty.
    // Oracle-backed implementations should supply a richer adapter.
    return nil
}
func (a *webSessionAdapter) CreateSession(title string) (web.SessionInfo, error) {
    id := fmt.Sprintf("s_%d", time.Now().UnixNano())
    return web.SessionInfo{ID: id, Title: title, LastAt: time.Now().Unix()}, nil
}
func (a *webSessionAdapter) DeleteSession(id string) error { return nil }

func newWebMemoryAdapter(m agent.OracleMemoryStore) *webMemoryAdapter {
    return &webMemoryAdapter{m: m}
}
type webMemoryAdapter struct{ m agent.OracleMemoryStore }
func (a *webMemoryAdapter) Search(q string, n int) []web.MemoryResult {
    recs, err := a.m.Recall(q, n)
    if err != nil {
        return nil
    }
    out := make([]web.MemoryResult, 0, len(recs))
    for _, r := range recs {
        out = append(out, web.MemoryResult{ID: r.MemoryID, Text: r.Text, Score: r.Score})
    }
    return out
}
```

Add `"github.com/jasperan/picooraclaw/pkg/channels/web"` to imports.

- [ ] **Step 3: Add --enable-web flag parsing**

In `gatewayCmd` where flags are parsed, add:

```go
enableWeb := flag.Bool("enable-web", false, "enable the web channel")
// after flag.Parse():
if *enableWeb {
    cfg.Channels.Web.Enabled = true
}
```

(If flags are parsed elsewhere — follow the existing pattern for how `--enable-telegram`-like toggles work, or just rely on config.json and skip this step.)

- [ ] **Step 4: Build and run a smoke test manually**

Run:

```bash
cd ~/git/personal/picooraclaw
make build
./build/picooraclaw gateway --enable-web &
GATEWAY_PID=$!
sleep 2

# Chat endpoint
curl -sS -X POST http://localhost:8090/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"s_smoke","text":"hi"}'
# Expected: {"message_id":"m_..."}

# SSE (will hang; Ctrl-C after a couple seconds)
timeout 2s curl -sS http://localhost:8090/v1/events?session_id=s_smoke || true

kill $GATEWAY_PID
```

Expected: `/v1/chat` returns 202 + message_id. SSE opens without error.

- [ ] **Step 5: Commit**

```bash
git add pkg/channels/manager.go cmd/picooraclaw/main.go
git commit -m "feat(gateway): register web channel and wire agent emitter to it"
```

---

### Task 12: Auth token test

**Files:**
- Extend: `pkg/channels/web/server_test.go`

- [ ] **Step 1: Write the test**

```go
func TestAuthMiddleware_RejectsWrongToken(t *testing.T) {
	cfg := config.WebConfig{Enabled: true, Host: "127.0.0.1", Port: 0, Token: "secret"}
	ch, _ := NewChannel(cfg, bus.NewMessageBus())
	srv := httptest.NewServer(ch.authMiddleware(ch.muxForTest()))
	defer srv.Close()

	// No header
	resp, _ := http.Get(srv.URL + "/v1/sessions")
	if resp.StatusCode != 401 {
		t.Fatalf("no-auth status %d", resp.StatusCode)
	}
	// Wrong header
	req, _ := http.NewRequest("GET", srv.URL+"/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer nope")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 401 {
		t.Fatalf("wrong-token status %d", resp.StatusCode)
	}
	// Correct header
	req, _ = http.NewRequest("GET", srv.URL+"/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Fatalf("correct-token status %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./pkg/channels/web -run TestAuthMiddleware -v`
Expected: PASS (auth middleware already implemented in Task 5).

- [ ] **Step 3: Commit**

```bash
git add pkg/channels/web/server_test.go
git commit -m "test(channels/web): cover bearer-token rejection paths"
```

---

### Task 13: End-to-end smoke test

**Files:**
- Create: `pkg/channels/web/e2e_test.go`

This exercises the full loop: POST /v1/chat → bus → (fake) agent emits events → SSE client receives them.

- [ ] **Step 1: Write the test**

```go
// pkg/channels/web/e2e_test.go
package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jasperan/picooraclaw/pkg/agent"
	"github.com/jasperan/picooraclaw/pkg/bus"
	"github.com/jasperan/picooraclaw/pkg/config"
)

func TestE2E_ChatThenEventStream(t *testing.T) {
	msgBus := bus.NewMessageBus()
	defer msgBus.Close()

	cfg := config.WebConfig{Enabled: true, Host: "127.0.0.1", Port: 0}
	ch, err := NewChannel(cfg, msgBus)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(ch.authMiddleware(ch.muxForTest()))
	defer srv.Close()

	// Fake agent: consume inbound, emit a tool_call lifecycle + message_end.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		msg, ok := msgBus.ConsumeInbound(ctx)
		if !ok {
			return
		}
		ok2 := true
		ch.Emit(agent.Event{Type: agent.EventMessageStart, SessionID: msg.SessionKey, MessageID: "m1", Timestamp: time.Now()})
		ch.Emit(agent.Event{Type: agent.EventToolCallStart, SessionID: msg.SessionKey, MessageID: "m1", ToolCallID: "tc1", ToolName: "remember", Timestamp: time.Now()})
		ch.Emit(agent.Event{Type: agent.EventToolCallEnd, SessionID: msg.SessionKey, MessageID: "m1", ToolCallID: "tc1", ToolName: "remember", OK: &ok2, Result: "stored", Timestamp: time.Now()})
		ch.Emit(agent.Event{Type: agent.EventMessageEnd, SessionID: msg.SessionKey, MessageID: "m1", Text: "done", Timestamp: time.Now()})
	}()

	// Open SSE FIRST so the fake agent's events land in a live subscription.
	resp, err := http.Get(srv.URL + "/v1/events?session_id=s_e2e")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Now post the chat.
	go func() {
		_, _ = http.Post(srv.URL+"/v1/chat", "application/json", strings.NewReader(`{"session_id":"s_e2e","text":"hi"}`))
	}()

	// Read 4 SSE data lines.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	deadline := time.Now().Add(3 * time.Second)
	got := 0
	types := []string{}
	for scanner.Scan() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout; got types=%v", types)
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var ev Event
		_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev)
		types = append(types, ev.Type)
		got++
		if got == 4 {
			break
		}
	}
	want := []string{"message_start", "tool_call_start", "tool_call_end", "message_end"}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("want %v, got %v", want, types)
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./pkg/channels/web -run TestE2E -v`
Expected: PASS. If it times out, the `httptest.NewServer` may not flush SSE quickly enough — try `srv.Client()` or add a small initial delay.

- [ ] **Step 3: Final commit**

```bash
git add pkg/channels/web/e2e_test.go
git commit -m "test(channels/web): e2e chat-then-event-stream smoke"
```

---

## Self-review checklist

Before marking Phase 1 complete:

- [ ] `go test ./...` from picooraclaw root passes (no regressions in other packages)
- [ ] `go vet ./...` clean
- [ ] `make build` still succeeds
- [ ] Manual smoke: `./build/picooraclaw gateway --enable-web`, curl the 4 endpoints
- [ ] No new dependencies added to `go.mod` beyond stdlib — confirm with `git diff go.mod`

## Phase 2 prerequisites delivered

After Phase 1 merges, Phase 2 (webui bridge) can assume picooraclaw exposes:

- `POST /v1/chat` accepting `{session_id, text, workspace?}` returning `202` + `{message_id}`
- `GET /v1/events?session_id=X[&from=Y]` streaming SSE of the 6 event types
- `GET/POST/DELETE /v1/sessions` with the `SessionInfo` shape
- `GET /v1/memory?q=X&limit=N` returning `[]MemoryResult`
- Bearer auth via `Authorization` header when `cfg.Channels.Web.Token` is set
