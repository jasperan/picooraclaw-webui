# picooraclaw-webui — Design Document

**Date:** 2026-04-22
**Status:** Approved (brainstorm phase complete)
**Repository:** `jasperan/picooraclaw-webui` (new, public)
**Related:** `jasperan/picooraclaw`

## Purpose

A standalone web chat UI that lets a human interact with a running picooraclaw instance through the browser. The UI replicates the CLI experience (`picooraclaw agent`) but adds live visibility into agent internals — tool calls, memory writes, cron fires, heartbeats — as structured cards in the conversation feed.

The project is public. End users self-host: run picooraclaw + picooraclaw-webui + (optional) Oracle, open the browser, start chatting.

## Success criteria

1. User can send a message from the browser and see the agent's response stream back, token by token.
2. Every tool call the agent issues appears as a collapsible card in the conversation — name, arguments, result — rendered as it happens, not after the fact.
3. User can browse prior sessions in a left sidebar; Oracle-backed history when available, file-backed history when not.
4. User can open a right-side drawer to search memories stored by `remember`/`recall`.
5. `docker compose up` from a fresh clone yields a working install (picooraclaw + webui + Oracle), first message in under 5 minutes.
6. No multi-user complexity: single-user password gate, nothing more.

## Architecture

Two processes, two transports.

```
┌──────────────┐    WebSocket    ┌────────────────────┐  HTTP + SSE   ┌────────────────┐
│   Browser    │ ◄─────────────► │ picooraclaw-webui  │ ◄──────────► │  picooraclaw   │
│ (SvelteKit)  │  ws://.../stream│   (Go bridge)      │ /v1/chat POST│  gateway mode  │
└──────────────┘                 └────────────────────┘ /v1/events SSE└────────────────┘
                                         ▲                                    │
                                         │ static assets embedded             ▼
                                         │ via //go:embed                Oracle DB
                                                                        (optional)
```

The **bridge** is a single Go binary. It serves a SvelteKit bundle (embedded at build time), speaks WebSocket to the browser, and speaks HTTP + SSE to picooraclaw. It owns no agent state — picooraclaw is the source of truth for sessions, messages, memory.

The **upstream contract** (picooraclaw side) is a new channel `pkg/channels/web/` that plugs into the existing channel manager. When `picooraclaw gateway --enable-web` runs, the channel exposes an HTTP server on a dedicated port (default 8090, separate from `health`).

### Why this shape

- **Separate repo** matches the user's stated constraint and keeps picooraclaw lean.
- **Minimal IPC** means picooraclaw only grows the endpoints strictly needed; everything else stays inside picooraclaw.
- **SSE upstream** is simpler than WebSocket for one-way streams, debuggable with curl, aligned with picooraclaw's existing `health` HTTP pattern.
- **WebSocket downstream** is idiomatic for browser chat and avoids SSE's one-direction limitation (need to push user messages back).
- **Embedded frontend** means one binary to ship. No "install Node, build frontend, serve static files" dance for end users.

## IPC contract (picooraclaw side)

The new `web` channel exposes exactly these endpoints. Nothing more. All paths are under the channel's configured base path (default `/`).

| Endpoint | Method | Purpose |
|---|---|---|
| `/v1/chat` | POST | Submit a user message. Body `{session_id, text, workspace?}`. Response 202 `{message_id}`. |
| `/v1/events` | GET (SSE) | Subscribe to agent events for a session. Query `?session_id=...&from=<message_id>` (resume cursor). |
| `/v1/sessions` | GET / POST / DELETE | List / create / delete sessions. GET returns list with `{id, title, last_at}`. Reads Oracle when enabled, file-backed store otherwise. |
| `/v1/memory` | GET | Search memories. Query `?q=<text>&limit=20`. Returns memory rows for the memory-tab drawer. Read-only. |
| `/v1/health` | GET | Reuses existing `pkg/health` — not a new endpoint, just documented here for completeness. |

### Event stream schema

Each SSE `data:` line is a single JSON object. Every event carries `type` and `session_id`; most carry `message_id` (the response message being produced).

```jsonc
{"type":"message_start","message_id":"m_123","session_id":"s_9"}
{"type":"token","message_id":"m_123","text":"Hel"}
{"type":"token","message_id":"m_123","text":"lo"}
{"type":"tool_call_start","message_id":"m_123","id":"tc_1","tool":"remember","args":{"text":"user likes Go"}}
{"type":"tool_call_end","message_id":"m_123","id":"tc_1","ok":true,"result":"stored"}
{"type":"agent_tick","message_id":"m_123","note":"cron:daily-recap fired"}
{"type":"message_end","message_id":"m_123"}
{"type":"error","message_id":"m_123","error":"upstream LLM timeout"}
```

Extensibility: new event types (new `type` values) are forward-compatible — the bridge forwards unknown types verbatim to the browser; the frontend ignores types it doesn't recognize.

### Upstream auth

If env var `PICOORACLAW_WEB_TOKEN` is set on the picooraclaw process, the `web` channel requires `Authorization: Bearer $TOKEN` on every request. The bridge sends the same token when its own env var is set. If unset on both sides, no auth — localhost dev mode.

## Bridge internals (picooraclaw-webui)

```
picooraclaw-webui/
├── cmd/picooraclaw-webui/main.go      # entrypoint, flag/env parsing
├── internal/
│   ├── bridge/                        # SSE consumer + HTTP client for picooraclaw
│   ├── ws/                            # WebSocket hub, per-session fan-out
│   ├── auth/                          # password gate, signed cookie session
│   ├── config/                        # flag > env > ~/.picooraclaw-webui/config.json > default
│   └── server/                        # HTTP handlers, static serving
├── web/                               # SvelteKit app
│   ├── src/routes/+page.svelte        # main chat UI
│   ├── src/lib/components/
│   │   ├── MessageBubble.svelte
│   │   ├── ToolCallCard.svelte
│   │   ├── Sidebar.svelte
│   │   ├── MemoryDrawer.svelte
│   │   └── LoginForm.svelte
│   ├── src/lib/stores/                # session store, messages store, ws store
│   └── build/                         # static output, embedded via //go:embed
├── docker-compose.yml                 # oracle + picooraclaw + webui bundle
├── Dockerfile                         # multi-stage: node build → go build → alpine
├── README.md
└── LICENSE                            # MIT
```

### Flows

**Boot:**
1. Bridge parses config, resolves `--picooraclaw-url` and `--listen`.
2. Bridge opens SSE connection to picooraclaw's `/v1/events` with the current session_id cursor.
3. Bridge serves the embedded SvelteKit bundle on its listen port.
4. Browser loads the page, establishes WebSocket to `/ws`, passes cookie if password gate is enabled.

**User sends message:**
1. Browser → bridge (WS frame `{type:"send", session_id, text}`).
2. Bridge → picooraclaw POST `/v1/chat` with the body.
3. picooraclaw kicks off the agent loop; emits SSE events.
4. Bridge's SSE consumer receives each event, fans out to all WS clients subscribed to the same session_id.
5. Browser renders tokens/tool-cards/messages as events arrive.

**Session switch:**
1. Browser fetches `/api/sessions` (bridge proxies to picooraclaw `GET /v1/sessions`).
2. User clicks a session in the sidebar.
3. Browser sends WS frame `{type:"subscribe", session_id}`.
4. Bridge's WS hub updates subscription; if the SSE cursor needs to change, re-subscribes upstream.

**Reconnect:**
1. WS drops (browser sleep, network blip).
2. Frontend reconnects with jitter (250ms → 8s cap, 10 attempts).
3. On reconnect, frontend sends its last-seen `message_id`.
4. Bridge replays events from its in-memory cursor buffer (last 1000 events per session) or re-subscribes upstream SSE from that `message_id`. No lost tool calls.

### Deliberate non-concerns

- **Bridge does not touch Oracle directly.** All Oracle reads go through picooraclaw. This keeps DB credentials in one place and keeps the bridge dependency-light (pure Go, no `go-ora`).
- **Bridge does not persist messages or sessions.** It keeps a small in-memory ring buffer (last 1000 events per session) for WS reconnect replay, but no disk or DB state. Fresh process = empty buffers, refetch from picooraclaw.
- **No WebSocket protocol negotiation.** Plain JSON frames, one message type per frame.

## Frontend (SvelteKit)

Styled with taste-skill (no dark/light toggle in v1, one strong theme).

### Layout

- **Left sidebar (~240px)**: session list (active highlighted), "+ New session" button, settings cog at bottom.
- **Main pane**: scrollable message feed. User bubbles right-aligned, agent bubbles left-aligned. Tool calls rendered inline with the agent bubble they belong to.
- **Composer** (fixed bottom): textarea, send button, keyboard shortcuts (Enter = send, Shift+Enter = newline).
- **Right drawer (slide-in, ~380px)**: memory search panel, toggled by an icon in the header.

### Tool call rendering

Default: one collapsed line per tool call.

```
🔧 remember("user likes Go") → ok
```

Click to expand: shows full args as pretty-printed JSON and the full result. Red border if `ok:false`. State persists across re-renders; collapsing is a UI-only toggle, not server state.

### Token streaming

Agent's current message appends tokens as they arrive. A blinking cursor renders at the end until `message_end`. Tool call cards appear at the position in the message where the LLM emitted them (not batched at the top/bottom).

### Memory drawer

- Search input at the top (debounced 300ms).
- Results as a scrollable list: each row shows the memory text, embedding similarity score (if Oracle), and the date it was stored.
- v1 is read-only. Editing/deleting is post-v1.

## Auth, config, deploy

### Auth (v1)

**Browser → bridge:**
- If `PICOORACLAW_WEBUI_PASSWORD` is set, `/login` page prompts for it.
- On success, bridge issues a signed cookie (HMAC-SHA256 with `PICOORACLAW_WEBUI_SECRET`, 30-day expiry).
- WebSocket upgrade rejects if cookie is missing/invalid.
- 3 wrong attempts → 30s cooldown per source IP. No account system.
- If password env is unset, no login page — open access. Suitable for localhost dev.

**Bridge → picooraclaw:**
- `PICOORACLAW_WEB_TOKEN` env var on both sides. Bridge adds `Authorization: Bearer` header.
- No token rotation in v1.

### Config

Precedence: flag > env var > `~/.picooraclaw-webui/config.json` > built-in default.

```
picooraclaw-webui \
  --picooraclaw-url http://localhost:8090 \   # upstream gateway
  --listen :3000 \
  --password "$PICOORACLAW_WEBUI_PASSWORD" \  # optional
  --upstream-token "$PICOORACLAW_WEB_TOKEN" \ # optional
  --secret "$PICOORACLAW_WEBUI_SECRET"        # optional, auto-generated if unset
```

The config file holds per-user preferences (last open session, etc.) — no secrets.

### Deploy

**Single binary:**
```
make build
./picooraclaw-webui
```
Opens `http://localhost:3000`, connects to `http://localhost:8090`.

**Docker Compose bundle** (the "expected" path):

```yaml
services:
  oracle:
    image: container-registry.oracle.com/database/free:latest
    environment:
      ORACLE_PWD: ${ORACLE_PWD}
    ports: ["1521:1521"]
    volumes: ["oracle-data:/opt/oracle/oradata"]

  picooraclaw:
    image: jasperan/picooraclaw:latest
    command: ["gateway", "--enable-web"]
    depends_on: [oracle]
    environment:
      PICOORACLAW_WEB_TOKEN: ${PICOORACLAW_WEB_TOKEN}
      ORACLE_DSN: "picooraclaw/picooraclaw@oracle:1521/FREEPDB1"
    ports: ["8090:8090"]

  webui:
    image: jasperan/picooraclaw-webui:latest
    depends_on: [picooraclaw]
    environment:
      PICOORACLAW_URL: http://picooraclaw:8090
      PICOORACLAW_WEB_TOKEN: ${PICOORACLAW_WEB_TOKEN}
      PICOORACLAW_WEBUI_PASSWORD: ${PICOORACLAW_WEBUI_PASSWORD}
    ports: ["3000:3000"]

volumes:
  oracle-data:
```

`docker compose up` is the default documented path. README quickstart targets 5 minutes to first message.

## Error handling

| Failure | Behavior |
|---|---|
| picooraclaw unreachable at boot | Bridge starts, UI shows "Agent offline — retrying". SSE retry backoff 1s → 30s cap. |
| picooraclaw disconnects mid-stream | In-flight message flagged interrupted. Red dot on partial bubble. "Retry" button resubmits prompt. |
| Tool call errors | picooraclaw sends `tool_call_end` with `ok:false`. UI renders card with red border. |
| WS disconnect (browser) | Frontend reconnects with jitter. Bridge replays events from last-seen `message_id` (1000-event in-memory buffer per session). |
| Upstream auth 401 | Bridge logs, UI shows "Bridge can't reach agent — check PICOORACLAW_WEB_TOKEN". |
| Password gate failure | 3 wrong attempts → 30s cooldown per IP. No lockout beyond that. |
| Malformed SSE event | Bridge logs at WARN, skips the event. Doesn't kill the stream. |

**XSS surface:** tool call args/results rendered as JSON via `<pre>` + text nodes, never `innerHTML`. LLM message text rendered as plain text (post-v1: opt-in markdown mode).

## Testing

**picooraclaw side** (in the picooraclaw repo):
- `pkg/channels/web/*_test.go` — handler tests with `httptest`, event fan-out tests with a fake bus.
- Follows existing pattern: go-sqlmock for Oracle paths, table-driven tests.

**Bridge:**
- Unit tests for SSE consumer (replay an SSE stream, assert events forwarded).
- Unit tests for WS hub fan-out (N subscribers, one event, all receive).
- Unit tests for auth cookie round-trip, password cooldown logic.
- Integration test: fake picooraclaw SSE source + real bridge + test WS client end-to-end.

**Frontend:**
- Vitest + `@testing-library/svelte` for components: `MessageBubble`, `ToolCallCard`, `Sidebar`, `MemoryDrawer`.
- Playwright smoke test: `docker compose up`, page loads, send one message, assert token stream and tool-call card appear.

**CI:**
- GitHub Actions matrix: `go test ./...`, `npm test`, `playwright test` on every PR.
- Playwright runs against the docker-compose bundle. Expect ~2 min added per PR.

## Out of scope for v1

Listed explicitly so we don't drift:

- OAuth or multi-user accounts
- Voice input/output (even though picooraclaw has Groq Whisper)
- File/image upload to the agent
- Message editing/deletion
- Markdown rendering in agent responses
- Dark/light theme toggle
- Cron/skills/heartbeat admin panels
- Mobile-specific layouts (desktop-first, responsive later)
- Conversation export (markdown/PDF)
- Plugin system for custom tool-card renderers

All of these are fair game for v1.x or v2.

## Open questions

None blocking v1. First implementation question (for the plan phase): pick the Go WebSocket library — `gorilla/websocket` (battle-tested, bigger footprint) vs `nhooyr/websocket` (smaller, context-native). Recommendation: `nhooyr/websocket` for new code.
