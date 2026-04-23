# Testing picooraclaw-webui

## Levels

| Level | Command | What it proves |
|---|---|---|
| Unit | `go test ./...` + `cd web && npm test` | Functions and components work in isolation. |
| Integration | `go test ./internal/server -run TestE2E` | The bridge fans out upstream events to a WS client. |
| Smoke (UI load) | `cd web && npx playwright test` | The page loads without JS errors. |
| **Dogfood (preferred)** | See below | Every agent event renders correctly in the real UI against a real agent. |

Unit + integration + smoke tell you the code is structurally fine. Only the **dogfood** level confirms users get what the spec promises: live tool-call cards, memory tab, session sidebar — all driven by a real picooraclaw agent through the real Oracle backend.

## Dogfood test (the one you should run before shipping)

### 1. Bring up the full stack

```bash
# Oracle (first run ~2-3 min to become healthy)
cd ~/git/personal/picooraclaw
docker compose --profile oracle up -d oracle-db

# OCI GenAI proxy (default LLM backend)
cd oci-genai && python proxy.py &
cd ..

# picooraclaw gateway with the web channel
./build/picooraclaw gateway --enable-web &

# picooraclaw-webui bridge
cd ~/git/personal/picooraclaw-webui
./bin/picooraclaw-webui --picooraclaw-url http://localhost:8090 &
```

Once Phase 4 ships, `docker compose up` from the webui repo replaces all four steps.

### 2. Open the UI

Navigate to `http://localhost:3000` in a real browser (or via Playwright MCP). Log in if you set `PICOORACLAW_WEBUI_PASSWORD`.

### 3. Exercise every event type

Pick prompts that force the underlying behavior. Don't paraphrase — the specific wording matters for `remember`/`recall` tool activation.

| Event type | Prompt to trigger it | Expected UI |
|---|---|---|
| `message_start` + `message_end` | `hi` | One assistant bubble appears |
| `tool_call_start` / `tool_call_end` (ok) | `Remember that I like Go` | Inline collapsed card `🔧 remember("...") → ok`; click expands |
| `tool_call_end` (error) | `Read the file /nope/nowhere.txt` | Card with red border, error in result |
| `error` | Stop the OCI proxy, then any prompt | Red status, retry button, no partial assistant bubble |
| Session switch | New session from sidebar, ask a different thing | Feed resets; old session history intact when you switch back |
| Memory drawer | Previous `remember` → open drawer, search `go` | One result with similarity score |
| Reconnect replay | Close and reopen the browser tab mid-stream | Feed restored, no duplicate cards |

### 4. Fix what you find

Don't file a bug and move on — fix it. The likely suspects per symptom:

- Tool card won't expand → `web/src/lib/components/ToolCallCard.svelte`
- Card appears twice → messages store `applyEvent` dedup on `id` in `web/src/lib/stores/messages.ts`
- Empty memory drawer → Oracle ONNX model not loaded, run `picooraclaw setup-oracle` again
- Session switch loses history → `currentSession` subscription in `+page.svelte` not re-sending the WS `subscribe` frame
- Tokens arrive in one chunk, not streaming → **this is expected in v1**. Providers don't stream yet; tracked as a post-v1 improvement.

### 5. Record evidence

```bash
mkdir -p docs/dogfood/$(date +%Y-%m-%d)
# save screenshots, session logs, the prompts you used
```

`docs/dogfood/` is in `.gitignore`. Keep it local; attach the interesting bits to any bug write-ups.
