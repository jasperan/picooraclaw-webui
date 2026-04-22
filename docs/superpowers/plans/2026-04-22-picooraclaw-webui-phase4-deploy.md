# Phase 4 — picooraclaw-webui Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Package picooraclaw-webui + picooraclaw + Oracle as a one-command `docker compose up` experience, with a README that gets a new user to first-message in under 5 minutes.

**Architecture:** Three-service compose file (oracle, picooraclaw, webui). Multi-stage Dockerfile for webui (Node → Go → alpine). Pre-built picooraclaw image pulled from Docker Hub (`jasperan/picooraclaw:latest`). Oracle uses the official `container-registry.oracle.com/database/free:latest` image. GitHub Actions pipeline: Go test, npm test, Playwright smoke against docker-compose, build + push images on tag.

---

### Task 1: Multi-stage Dockerfile

**Files:**
- Create: `Dockerfile`
- Create: `.dockerignore`

- [ ] **Step 1: Dockerfile**

```dockerfile
# Stage 1: build frontend
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: build Go binary
FROM golang:1.22-alpine AS go
WORKDIR /src
RUN apk add --no-cache make
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/build ./cmd/picooraclaw-webui/static
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /picooraclaw-webui ./cmd/picooraclaw-webui

# Stage 3: runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=go /picooraclaw-webui /usr/local/bin/picooraclaw-webui
EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/picooraclaw-webui"]
```

- [ ] **Step 2: .dockerignore**

```
.git
.gitignore
bin/
web/node_modules/
web/.svelte-kit/
web/build/
.superpowers/
docs/
*.md
!README.md
.agents/
.claude/
.crush/
.openhands/
.serena/
```

- [ ] **Step 3: Build manually**

```bash
docker build -t picooraclaw-webui:dev .
```

Expected: image builds in ~3 minutes, final size ~30-40 MB.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile .dockerignore
git commit -m "feat(docker): multi-stage Dockerfile (node→go→alpine, ~35 MB runtime)"
```

---

### Task 2: docker-compose.yml bundle

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`

- [ ] **Step 1: docker-compose.yml**

```yaml
services:
  oracle:
    image: container-registry.oracle.com/database/free:latest
    environment:
      ORACLE_PWD: ${ORACLE_PWD:-picooraclaw_admin}
      ORACLE_CHARACTERSET: AL32UTF8
    ports:
      - "1521:1521"
    volumes:
      - oracle-data:/opt/oracle/oradata
    healthcheck:
      test: ["CMD", "sqlplus", "-L", "sys/${ORACLE_PWD:-picooraclaw_admin}@localhost:1521/FREEPDB1 as sysdba", "@/dev/null"]
      interval: 30s
      timeout: 10s
      retries: 20

  picooraclaw:
    image: jasperan/picooraclaw:latest
    command: ["gateway", "--enable-web"]
    depends_on:
      oracle:
        condition: service_healthy
    environment:
      PICOORACLAW_WEB_TOKEN: ${PICOORACLAW_WEB_TOKEN:-}
      ORACLE_ENABLED: "true"
      ORACLE_DSN: "picooraclaw/${PICOORACLAW_DB_PWD:-picooraclaw}@oracle:1521/FREEPDB1"
    ports:
      - "8090:8090"
    volumes:
      - picooraclaw-workspace:/root/.picooraclaw

  webui:
    build: .
    image: jasperan/picooraclaw-webui:latest
    depends_on:
      - picooraclaw
    environment:
      PICOORACLAW_URL: http://picooraclaw:8090
      PICOORACLAW_WEB_TOKEN: ${PICOORACLAW_WEB_TOKEN:-}
      PICOORACLAW_WEBUI_PASSWORD: ${PICOORACLAW_WEBUI_PASSWORD:-}
    ports:
      - "3000:3000"

volumes:
  oracle-data:
  picooraclaw-workspace:
```

- [ ] **Step 2: .env.example**

```
# Oracle sys password (>= 12 chars recommended)
ORACLE_PWD=picooraclaw_admin_change_me

# picooraclaw DB user password (used by picooraclaw to connect to Oracle)
PICOORACLAW_DB_PWD=picooraclaw

# Shared bearer token between webui and picooraclaw. Leave empty for localhost dev.
PICOORACLAW_WEB_TOKEN=

# Optional: password gate for the webui. Leave empty for open access.
PICOORACLAW_WEBUI_PASSWORD=
```

- [ ] **Step 3: Smoke test the bundle**

```bash
cp .env.example .env
docker compose up -d
# wait for oracle to become healthy (~2-3 minutes on first run)
docker compose ps
# expect all three services running / healthy

curl -sS http://localhost:3000/ | head -5
# expect HTML including "picooraclaw"

curl -sS -X POST http://localhost:8090/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"smoke","text":"hi"}'
# expect {"message_id":"m_..."}

docker compose down
```

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml .env.example
git commit -m "feat(deploy): docker-compose bundle (oracle + picooraclaw + webui)"
```

---

### Task 3: README with 5-minute quickstart

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write the README**

```markdown
# picooraclaw-webui

Web chat UI for [picooraclaw](https://github.com/jasperan/picooraclaw). Shows agent tool calls as they happen, keeps session history in Oracle, runs in Docker.

## Quickstart (docker-compose, recommended)

```bash
git clone https://github.com/jasperan/picooraclaw-webui
cd picooraclaw-webui
cp .env.example .env
# edit .env if you want a password gate or auth token
docker compose up
```

Wait ~2-3 minutes for Oracle to initialize on first run, then open **http://localhost:3000**. First message works immediately.

## Quickstart (binary, no docker)

Requires: Go 1.22+, Node 22+, a running picooraclaw gateway.

```bash
# In picooraclaw repo:
./build/picooraclaw gateway --enable-web

# In this repo:
cd web && npm install && npm run build && cd ..
make build
./bin/picooraclaw-webui --picooraclaw-url http://localhost:8090
```

Open **http://localhost:3000**.

## What you'll see

- Chat feed with user messages right-aligned, agent responses left-aligned.
- **Tool calls appear inline as collapsible cards** — one line per tool, click to see full args and result.
- Left sidebar lists past sessions.
- Right drawer (toggle from the header) searches Oracle-backed memories.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `PICOORACLAW_URL` | `http://localhost:8090` | Upstream picooraclaw gateway URL |
| `PICOORACLAW_WEBUI_LISTEN` | `:3000` | Webui listen address |
| `PICOORACLAW_WEBUI_PASSWORD` | unset | If set, gates the UI behind a login page |
| `PICOORACLAW_WEB_TOKEN` | unset | Shared bearer token between webui and picooraclaw |
| `PICOORACLAW_WEBUI_SECRET` | auto | Cookie signing key (auto-generated if unset) |

## Architecture

Bridge (Go) mediates between the browser (WebSocket) and picooraclaw (HTTP + SSE). One binary serves the embedded SvelteKit bundle. picooraclaw exposes 5 endpoints via a new `web` channel: `/v1/chat`, `/v1/events` (SSE), `/v1/sessions`, `/v1/memory`, `/v1/health`.

See [`docs/superpowers/specs/2026-04-22-picooraclaw-webui-design.md`](docs/superpowers/specs/2026-04-22-picooraclaw-webui-design.md) for the full design.

## Development

```bash
make test     # Go tests
cd web && npm test  # Svelte + store tests
make lint     # go vet
```

## Contributing

PRs welcome. Keep the 5 upstream endpoints as the minimum contract — feature creep in picooraclaw should land in picooraclaw, not here.

## License

MIT.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: README with 5-minute quickstart"
```

---

### Task 4: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

```yaml
name: CI
on:
  push: { branches: [main] }
  pull_request:

jobs:
  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: go vet ./...
      - run: go test -race ./...

  web:
    runs-on: ubuntu-latest
    defaults: { run: { working-directory: web } }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '22', cache: npm, cache-dependency-path: web/package-lock.json }
      - run: npm ci
      - run: npm test
      - run: npm run build

  docker:
    runs-on: ubuntu-latest
    needs: [go, web]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - run: docker build -t picooraclaw-webui:ci .

  playwright:
    runs-on: ubuntu-latest
    needs: [docker]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '22' }
      - name: Install Playwright
        working-directory: web
        run: npm ci && npx playwright install --with-deps chromium
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - name: Build binary with embedded static
        run: |
          cd web && npm run build && cd ..
          mkdir -p cmd/picooraclaw-webui/static
          cp -r web/build/* cmd/picooraclaw-webui/static/
          go build -o bin/picooraclaw-webui ./cmd/picooraclaw-webui
      - name: Run Playwright smoke
        working-directory: web
        run: npx playwright test
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: go test, npm test, docker build, playwright smoke"
```

---

### Task 5: Full e2e smoke (docker-compose + Playwright)

**Files:**
- Modify: `web/playwright.config.ts`
- Create: `web/tests/e2e-compose.spec.ts` (optional, manual only)

This variant spins up the full compose stack and drives the UI. Kept manual (not in CI) because Oracle's first-run init is slow.

- [ ] **Step 1: Write the spec**

```ts
// web/tests/e2e-compose.spec.ts
import { test, expect } from '@playwright/test';

// Run with: docker compose up -d && npx playwright test tests/e2e-compose.spec.ts
test.skip(process.env.E2E_COMPOSE !== '1', 'set E2E_COMPOSE=1 with docker compose running');

test('send message, see response', async ({ page }) => {
  await page.goto('/');
  // If password is set, skip — this smoke assumes open access
  const input = page.locator('textarea');
  await input.waitFor({ timeout: 10000 });
  await input.fill('hello');
  await page.keyboard.press('Enter');
  // Agent response bubble appears within 30s
  await expect(page.locator('.bubble.assistant')).toBeVisible({ timeout: 30000 });
});
```

- [ ] **Step 2: Document in README**

Add to the README under a "Testing" section:

```markdown
### Full end-to-end (manual)

```bash
docker compose up -d
cd web && E2E_COMPOSE=1 npx playwright test tests/e2e-compose.spec.ts
docker compose down
```
```

- [ ] **Step 3: Commit**

```bash
git add web/tests/e2e-compose.spec.ts README.md
git commit -m "test(e2e): compose-level smoke gated on E2E_COMPOSE=1"
```

---

## Self-review checklist

- [ ] `docker build .` succeeds
- [ ] `docker compose up` starts all three services; webui reachable on :3000
- [ ] `curl http://localhost:3000/` returns the SPA HTML
- [ ] `curl http://localhost:8090/v1/chat` accepts a POST
- [ ] README quickstart works from a fresh checkout on a fresh machine
- [ ] CI: all jobs green on a PR

## Release

Once Phases 1-4 are green on main:

1. Tag `v0.1.0` in both repos (picooraclaw with the web-channel change, picooraclaw-webui).
2. GitHub Actions builds and pushes images `jasperan/picooraclaw:v0.1.0` and `jasperan/picooraclaw-webui:v0.1.0`.
3. Update `docker-compose.yml` to pin specific tags instead of `:latest` for reproducibility.
4. Write a short release blog post (Medium/DevPortal via jasperSOCIAL) that walks through the 5-minute quickstart.
