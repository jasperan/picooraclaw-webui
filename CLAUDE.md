# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Browser UI for **picooraclaw** (Oracle-backed autonomous agent). Go HTTP+WS bridge with a SvelteKit frontend, bundled via docker-compose with Oracle Database Free.

## Commands

```bash
# Build Go binary (syncs web/build → cmd/picooraclaw-webui/static first)
make build

# Go tests (CI runs with -race -count=1)
make test
go test ./internal/server -run TestLogin -v   # single test

# Go vet
make lint

# Run built binary against a local upstream
make run   # = ./bin/picooraclaw-webui --picooraclaw-url http://localhost:8090 --listen :3000

# Full stack (Oracle ~2 min cold start)
docker compose up -d

# Dev loop: Oracle in docker, Go + Svelte on host
docker compose up -d oracle
go run ./cmd/picooraclaw-webui
cd web && npm run dev          # :5173, proxies to :3000

# Frontend
cd web && npm test             # vitest
cd web && npm run check        # svelte-check
cd web && npm run test:e2e     # Playwright (needs running server)

# Compose-level end-to-end (gated, Oracle must be healthy)
cd web && E2E_COMPOSE=1 WEBUI_URL=http://localhost:3000 npx playwright test tests/e2e-compose.spec.ts
```

## Architecture

Three-layer system: **browser ↔ Go bridge ↔ picooraclaw core ↔ Oracle**.

**Go bridge** (`cmd/picooraclaw-webui`, `internal/`) is a thin HTTP+WS proxy in front of the upstream picooraclaw gateway. It does not own domain data — all sessions/memory live in Oracle via picooraclaw.

Key flow:
1. `main.go` wires `config.Load` → `bridge.Client` → `ws.Hub` → `auth.Gate` → `server.NewMux`.
2. A single **SSE pump goroutine** (`runSSEPump`) subscribes to upstream `/v1/events` for one session (`"default"` in v1), demuxes events by `SessionID`, and fans them into the Hub. Reconnects with 1s backoff on upstream close. The TODO in `main.go` notes post-v1 work: track active sessions from the Hub and run one pump per session.
3. `server.NewMux` routes: `/api/login`, `/api/sessions`, `/api/memory`, `/ws` (auth-gated), and `/` → embedded static SvelteKit build. Static handler is registered **last** so it doesn't shadow `/api/` or `/ws`.
4. `ws.Hub` is a `sessionID → {Conn}` fanout map. `handleWS` reads frames: `subscribe` (joins a session, auto-unsubscribing from prior one) and `send` (posts chat to upstream, pinned to the subscribed session — do NOT trust `f.SessionID` on send, that would be session spoofing).
5. `bridge.Client` hits upstream REST: `POST /v1/chat`, `GET /v1/sessions`, `GET /v1/memory?q=`, `GET /v1/events` (SSE, in `bridge/sse.go`). Bearer token via `--upstream-token` / `PICOORACLAW_WEB_TOKEN` is optional.
6. `auth.Gate` — optional password mode. HMAC-SHA256 signed cookie `pwac_session` (expiry-Unix + signature), 30-day TTL, `HttpOnly`+`SameSite=Lax`. 3 failed attempts / 30s / IP triggers 429 cooldown; sweeper goroutine GCs the attempts map every 60s. When `PICOORACLAW_WEBUI_PASSWORD` is empty, all routes are open (dev mode).

**Static assets**: `make sync-static` copies `web/build/*` into `cmd/picooraclaw-webui/static/` where `static.go` embeds them via `go:embed`. The Dockerfile bypasses Make and injects the `web/build` output directly in stage 2. Keep `.gitkeep` or a real build in `cmd/picooraclaw-webui/static/` — missing dir breaks `go:embed`.

**Frontend** (`web/`) is SvelteKit 2 + Svelte 5 with `adapter-static` (the Go binary serves the SPA). `src/lib/` holds `components/`, `stores/`, and `index.ts`. Routing is a single `+page.svelte` — state lives in stores, not routes.

**Session/event reactivity**: recent commits show the fragile area. When WS reconnects, the client must replay `subscribe` and the server side re-registers; frames arriving during connect must be buffered and flushed (see `45e3eea`, `7b47af9`, `5b6a4e9`). When adding event-driven UI, reconstruct `Message` objects on apply so Svelte 5 reactivity triggers — reusing references will silently miss renders.

## Config

All flags accept env var fallback:

| Flag | Env | Default |
|---|---|---|
| `--picooraclaw-url` | `PICOORACLAW_URL` | `http://localhost:8090` |
| `--listen` | `PICOORACLAW_WEBUI_LISTEN` | `:3000` |
| `--password` | `PICOORACLAW_WEBUI_PASSWORD` | *(empty = open)* |
| `--upstream-token` | `PICOORACLAW_WEB_TOKEN` | *(empty)* |
| `--secret` | `PICOORACLAW_WEBUI_SECRET` | *(32-byte random per process)* |

**`--secret` must be set in multi-replica deploys** — otherwise each replica signs cookies with its own random key and sessions break on LB rotation.

## Conventions

- Go module is `github.com/jasperan/picooraclaw-webui`, Go 1.24. Only dep: `nhooyr.io/websocket`.
- No domain persistence in this repo. Don't add SQL or memory tables here — picooraclaw owns that.
- Upstream errors: Go bridge returns `502 Bad Gateway` with upstream body passthrough.
- Route order matters in `server.NewMux` (static last). Adding new `/api/*` endpoints: register before `m.Handle("/", Static)`.
- Session pinning: any handler that writes on behalf of a connected client must use the *server-tracked* session, never a client-supplied one.

## Sync to oracle-ai-developer-hub

This project is one of the two registered projects (`agent-reasoning`, `picooraclaw`) for hub sync via `~/bin/sync-to-hub.sh`. If a post-commit hook is added here, follow the convention in `~/CLAUDE.md`: pass commit message via `--msg`, sync only on real diff. Do not add new projects to the hub script without explicit approval.
