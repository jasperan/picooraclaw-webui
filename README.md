# picooraclaw-webui

A browser UI for [picooraclaw](https://github.com/jasperan/picooraclaw), the Oracle-backed autonomous agent. Stream reasoning events, browse memory, and drive sessions from your laptop.

## What's inside

- **Go bridge** (`cmd/picooraclaw-webui`) — exposes picooraclaw's IPC socket over HTTP + WebSocket
- **SvelteKit frontend** (`web/`) — chat feed, tool-call cards, memory drawer, session sidebar
- **Oracle Database Free** — memory and session storage, bundled via docker-compose

## Quickstart (5 minutes)

Requirements: Docker 25+, Docker Compose v2.

```bash
git clone https://github.com/jasperan/picooraclaw-webui
cd picooraclaw-webui
cp .env.example .env
docker compose up -d
```

Oracle's first boot takes ~2 minutes. Once healthy, open http://localhost:3000 and log in (default: `demo/demo`).

### Override ports or the Oracle password

Edit `.env`:

```
ORACLE_PWD=your-secret
WEBUI_PORT=8080
ORACLE_PORT=1522
```

Then `docker compose up -d` again.

## Development

Local hot-reload (no Docker, uses Oracle in a container):

```bash
# Oracle only
docker compose up -d oracle

# Go bridge
go run ./cmd/picooraclaw-webui

# Frontend (separate terminal)
cd web && npm install && npm run dev
```

Frontend runs on http://localhost:5173 and proxies to the Go bridge on :3000.

## Testing

```bash
# Go unit tests
go test ./...

# Frontend unit tests
cd web && npm test

# Frontend smoke (requires running server)
cd web && npm run test:e2e
```

## Layout

```
cmd/picooraclaw-webui/  Go HTTP+WS server, embedded static assets
internal/               IPC client, session store, auth
web/                    SvelteKit frontend
docs/                   Architecture notes and Phase 3 status
Dockerfile              Multi-stage: node → go → alpine (~35 MB runtime)
docker-compose.yml      oracle + picooraclaw + webui bundle
```

## Full end-to-end (manual)

The compose-level smoke test drives the live stack (Oracle + picooraclaw + webui) through login, sessions, and memory search. It's gated behind `E2E_COMPOSE=1` so CI doesn't run it by default (Oracle boot is ~2 min).

```bash
# Start the full stack and wait for Oracle to become healthy
docker compose up -d
docker compose ps   # oracle must show (healthy) before continuing

# Run the compose-level Playwright smoke
cd web
E2E_COMPOSE=1 WEBUI_URL=http://localhost:3000 npx playwright test tests/e2e-compose.spec.ts
```

Set `WEBUI_URL` if you changed `WEBUI_PORT` in `.env`.

## License

MIT. See `LICENSE`.
