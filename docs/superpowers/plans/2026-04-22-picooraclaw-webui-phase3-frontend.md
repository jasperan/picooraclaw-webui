# Phase 3 — picooraclaw-webui SvelteKit Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Build the SvelteKit chat UI that talks to the Phase 2 bridge over WebSocket and renders streamed agent events as collapsible tool-call cards inside a familiar chat layout.

**Architecture:** SvelteKit static adapter (no Node server) — builds to a static bundle that the Go binary embeds. One route (`/`) with a login gate. Components: `MessageBubble`, `ToolCallCard`, `Sidebar`, `MemoryDrawer`, `LoginForm`, `Composer`. State in Svelte stores. Styling via taste-skill (one opinionated theme).

**Tech Stack:** SvelteKit (latest), `@sveltejs/adapter-static`, Vitest, `@testing-library/svelte`, Playwright for smoke.

**Pre-existing:** Phase 2 provides `POST /api/login`, `GET /api/sessions`, `GET /api/memory`, `WS /ws` with frame shapes documented in the Phase 2 plan.

---

### Task 1: Initialize SvelteKit with static adapter

**Files:** `web/` subtree

- [ ] **Step 1: Scaffold**

```bash
cd ~/git/personal/picooraclaw-webui/web
npm create svelte@latest . -- --template skeleton --types ts --no-git
npm i
npm i -D @sveltejs/adapter-static vitest @testing-library/svelte @playwright/test jsdom
```

- [ ] **Step 2: Configure static adapter**

Edit `web/svelte.config.js`:

```js
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({ fallback: 'index.html', strict: false }),
    paths: { base: '' }
  }
};
```

- [ ] **Step 3: Verify build**

```bash
npm run build
ls build/
# Expected: index.html + _app/ directory
```

- [ ] **Step 4: Commit**

```bash
cd ~/git/personal/picooraclaw-webui
git add web/package.json web/package-lock.json web/svelte.config.js web/.gitignore web/src/ web/static/
git commit -m "chore(web): scaffold SvelteKit with static adapter"
```

---

### Task 2: WebSocket store + message/session stores

**Files:**
- Create: `web/src/lib/stores/ws.ts`
- Create: `web/src/lib/stores/session.ts`
- Create: `web/src/lib/stores/messages.ts`

- [ ] **Step 1: `ws.ts`**

```ts
// web/src/lib/stores/ws.ts
import { writable } from 'svelte/store';

export type AgentEvent = {
  type: 'message_start' | 'message_end' | 'tool_call_start' | 'tool_call_end' | 'error' | 'agent_tick';
  session_id?: string;
  message_id?: string;
  id?: string;
  tool?: string;
  args?: Record<string, unknown>;
  result?: string;
  ok?: boolean;
  text?: string;
  error?: string;
  note?: string;
  ts?: string;
};

type OutgoingFrame =
  | { type: 'subscribe'; session_id: string; from?: string }
  | { type: 'send'; session_id: string; text: string };

export const wsConnected = writable(false);

let ws: WebSocket | null = null;
let eventHandlers: Array<(e: AgentEvent) => void> = [];
let reconnectDelay = 250;

export function connect() {
  const url = location.origin.replace(/^http/, 'ws') + '/ws';
  ws = new WebSocket(url);
  ws.onopen = () => {
    wsConnected.set(true);
    reconnectDelay = 250;
  };
  ws.onmessage = (ev) => {
    try {
      const frame = JSON.parse(ev.data);
      if (frame.type === 'event') {
        const event: AgentEvent = JSON.parse(new TextDecoder().decode(new Uint8Array(frame.payload.data ?? frame.payload)));
        eventHandlers.forEach((h) => h(event));
      }
    } catch (e) {
      // frame payload may already be a parsed object (server sends RawMessage as object in some Go configs)
      try {
        const frame = JSON.parse(ev.data);
        if (frame.type === 'event' && frame.payload) {
          eventHandlers.forEach((h) => h(frame.payload as AgentEvent));
        }
      } catch { /* swallow */ }
    }
  };
  ws.onclose = () => {
    wsConnected.set(false);
    setTimeout(connect, Math.min(reconnectDelay *= 2, 8000));
  };
  ws.onerror = () => ws?.close();
}

export function subscribe(sessionId: string, from?: string) {
  send({ type: 'subscribe', session_id: sessionId, from });
}

export function sendMessage(sessionId: string, text: string) {
  send({ type: 'send', session_id: sessionId, text });
}

export function onEvent(h: (e: AgentEvent) => void) {
  eventHandlers.push(h);
  return () => { eventHandlers = eventHandlers.filter((x) => x !== h); };
}

function send(frame: OutgoingFrame) {
  if (!ws || ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify(frame));
}
```

- [ ] **Step 2: `session.ts` and `messages.ts`**

```ts
// web/src/lib/stores/session.ts
import { writable } from 'svelte/store';

export type Session = { id: string; title: string; last_at: number };

export const currentSession = writable<string>('default');
export const sessions = writable<Session[]>([]);

export async function loadSessions() {
  const res = await fetch('/api/sessions');
  if (res.ok) sessions.set(await res.json());
}
```

```ts
// web/src/lib/stores/messages.ts
import { writable } from 'svelte/store';
import type { AgentEvent } from './ws';

export type ToolCall = {
  id: string;
  tool: string;
  args?: Record<string, unknown>;
  result?: string;
  ok?: boolean;
  done: boolean;
};

export type Message = {
  id: string;
  role: 'user' | 'assistant';
  text: string;
  toolCalls: ToolCall[];
  streaming: boolean;
  error?: string;
};

type BySession = Record<string, Message[]>;

export const messagesBySession = writable<BySession>({});

export function appendUserMessage(sessionId: string, text: string) {
  messagesBySession.update((state) => {
    const list = state[sessionId] ?? [];
    return { ...state, [sessionId]: [...list, { id: `u_${Date.now()}`, role: 'user', text, toolCalls: [], streaming: false }] };
  });
}

export function applyEvent(sessionId: string, e: AgentEvent) {
  messagesBySession.update((state) => {
    const list = [...(state[sessionId] ?? [])];
    switch (e.type) {
      case 'message_start':
        list.push({ id: e.message_id!, role: 'assistant', text: '', toolCalls: [], streaming: true });
        break;
      case 'tool_call_start': {
        const m = findAssistant(list, e.message_id);
        if (m) m.toolCalls.push({ id: e.id!, tool: e.tool ?? '?', args: e.args, done: false });
        break;
      }
      case 'tool_call_end': {
        const m = findAssistant(list, e.message_id);
        if (m) {
          const tc = m.toolCalls.find((x) => x.id === e.id);
          if (tc) { tc.result = e.result; tc.ok = e.ok; tc.done = true; }
        }
        break;
      }
      case 'message_end': {
        const m = findAssistant(list, e.message_id);
        if (m) { m.text = e.text ?? m.text; m.streaming = false; }
        break;
      }
      case 'error': {
        const m = findAssistant(list, e.message_id);
        if (m) { m.error = e.error; m.streaming = false; }
        break;
      }
    }
    return { ...state, [sessionId]: list };
  });
}

function findAssistant(list: Message[], id?: string) {
  if (!id) return null;
  for (let i = list.length - 1; i >= 0; i--) if (list[i].id === id) return list[i];
  return null;
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/stores/
git commit -m "feat(web): stores for ws, sessions, messages with event folding"
```

---

### Task 3: Core components (MessageBubble, ToolCallCard, Composer)

**Files:**
- Create: `web/src/lib/components/MessageBubble.svelte`
- Create: `web/src/lib/components/ToolCallCard.svelte`
- Create: `web/src/lib/components/Composer.svelte`

- [ ] **Step 1: ToolCallCard**

```svelte
<!-- web/src/lib/components/ToolCallCard.svelte -->
<script lang="ts">
  import type { ToolCall } from '$lib/stores/messages';
  export let tc: ToolCall;
  let expanded = false;
</script>

<div class="tc" class:error={tc.done && tc.ok === false} class:pending={!tc.done}>
  <button class="summary" on:click={() => (expanded = !expanded)}>
    <span class="icon">🔧</span>
    <span class="name">{tc.tool}</span>
    <span class="args-inline">({summarize(tc.args)})</span>
    {#if tc.done}
      <span class="arrow">→</span>
      <span class="result-inline">{tc.ok === false ? 'error' : shorten(tc.result ?? 'ok')}</span>
    {:else}
      <span class="running">running…</span>
    {/if}
  </button>
  {#if expanded}
    <div class="detail">
      <div class="label">args</div>
      <pre>{JSON.stringify(tc.args ?? {}, null, 2)}</pre>
      {#if tc.done}
        <div class="label">result</div>
        <pre>{tc.result ?? ''}</pre>
      {/if}
    </div>
  {/if}
</div>

<script lang="ts" context="module">
  function summarize(args: Record<string, unknown> | undefined): string {
    if (!args) return '';
    const first = Object.values(args)[0];
    if (typeof first === 'string') return first.length > 40 ? first.slice(0, 37) + '…' : first;
    return JSON.stringify(first ?? '').slice(0, 40);
  }
  function shorten(s: string): string {
    return s.length > 40 ? s.slice(0, 37) + '…' : s;
  }
</script>

<style>
  .tc { border: 1px solid #2a2a2a; border-radius: 8px; margin: 4px 0; background: #121212; }
  .tc.error { border-color: #b33; }
  .tc.pending .summary { opacity: 0.7; }
  .summary { width: 100%; text-align: left; padding: 8px 12px; background: none; border: none; color: inherit; cursor: pointer; display: flex; gap: 6px; align-items: baseline; font-family: inherit; font-size: 13px; }
  .icon { font-size: 12px; }
  .name { font-weight: 600; }
  .args-inline, .result-inline { color: #a0a0a0; }
  .arrow { color: #555; }
  .running { color: #8a8; font-style: italic; }
  .detail { padding: 0 12px 12px 12px; }
  .label { font-size: 11px; text-transform: uppercase; color: #888; margin-top: 8px; }
  pre { background: #000; padding: 8px; border-radius: 4px; overflow-x: auto; font-size: 12px; margin: 2px 0; white-space: pre-wrap; word-break: break-word; }
</style>
```

- [ ] **Step 2: MessageBubble**

```svelte
<!-- web/src/lib/components/MessageBubble.svelte -->
<script lang="ts">
  import type { Message } from '$lib/stores/messages';
  import ToolCallCard from './ToolCallCard.svelte';
  export let msg: Message;
</script>

<div class="bubble" class:user={msg.role === 'user'} class:assistant={msg.role === 'assistant'}>
  {#if msg.text}
    <div class="text">{msg.text}{#if msg.streaming}<span class="cursor">▋</span>{/if}</div>
  {/if}
  {#each msg.toolCalls as tc (tc.id)}
    <ToolCallCard {tc} />
  {/each}
  {#if msg.error}
    <div class="error">error: {msg.error}</div>
  {/if}
</div>

<style>
  .bubble { max-width: 720px; padding: 10px 14px; border-radius: 10px; margin: 6px 0; white-space: pre-wrap; }
  .bubble.user { background: #1e3a5f; align-self: flex-end; color: #d8e6f5; }
  .bubble.assistant { background: #1a1a1a; align-self: flex-start; color: #e8e8e8; }
  .text { font-size: 14px; line-height: 1.5; }
  .cursor { animation: blink 1s step-end infinite; }
  @keyframes blink { 50% { opacity: 0; } }
  .error { margin-top: 6px; color: #f88; font-size: 12px; }
</style>
```

- [ ] **Step 3: Composer**

```svelte
<!-- web/src/lib/components/Composer.svelte -->
<script lang="ts">
  export let onSend: (text: string) => void;
  let value = '';
  function submit() {
    const t = value.trim();
    if (!t) return;
    onSend(t);
    value = '';
  }
  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); }
  }
</script>

<form class="composer" on:submit|preventDefault={submit}>
  <textarea bind:value rows="2" on:keydown={onKey} placeholder="Ask picooraclaw..."></textarea>
  <button type="submit" disabled={!value.trim()}>Send</button>
</form>

<style>
  .composer { display: flex; gap: 8px; padding: 12px; border-top: 1px solid #222; background: #0d0d0d; }
  textarea { flex: 1; resize: none; background: #111; color: #e8e8e8; border: 1px solid #2a2a2a; border-radius: 6px; padding: 8px 10px; font-family: inherit; font-size: 14px; }
  textarea:focus { outline: none; border-color: #4a7a9f; }
  button { padding: 0 18px; background: #4a7a9f; color: white; border: none; border-radius: 6px; cursor: pointer; font-weight: 600; }
  button:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
```

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/components/
git commit -m "feat(web): MessageBubble, ToolCallCard, Composer components"
```

---

### Task 4: Sidebar and MemoryDrawer

**Files:**
- Create: `web/src/lib/components/Sidebar.svelte`
- Create: `web/src/lib/components/MemoryDrawer.svelte`

- [ ] **Step 1: Sidebar**

```svelte
<!-- web/src/lib/components/Sidebar.svelte -->
<script lang="ts">
  import { sessions, currentSession, loadSessions } from '$lib/stores/session';
  import { onMount } from 'svelte';
  onMount(loadSessions);
</script>

<aside>
  <header><h2>picooraclaw</h2></header>
  <button class="new" on:click={() => currentSession.set('s_' + Date.now())}>+ New session</button>
  <ul>
    {#each $sessions as s}
      <li class:active={$currentSession === s.id}>
        <button on:click={() => currentSession.set(s.id)}>{s.title || s.id}</button>
      </li>
    {/each}
  </ul>
</aside>

<style>
  aside { width: 240px; background: #0a0a0a; border-right: 1px solid #1f1f1f; display: flex; flex-direction: column; }
  header { padding: 16px; border-bottom: 1px solid #1f1f1f; }
  h2 { margin: 0; font-size: 14px; letter-spacing: 0.5px; color: #aaa; }
  .new { margin: 12px; padding: 8px; background: none; color: #bbb; border: 1px dashed #333; border-radius: 6px; cursor: pointer; }
  .new:hover { background: #151515; }
  ul { list-style: none; padding: 0; margin: 0; overflow-y: auto; }
  li button { width: 100%; text-align: left; padding: 10px 16px; background: none; border: none; color: #ccc; cursor: pointer; font-family: inherit; font-size: 13px; }
  li.active button { background: #1a1a1a; color: #fff; }
  li button:hover { background: #141414; }
</style>
```

- [ ] **Step 2: MemoryDrawer**

```svelte
<!-- web/src/lib/components/MemoryDrawer.svelte -->
<script lang="ts">
  export let open = false;
  let q = '';
  let results: Array<{ id: string; text: string; score: number }> = [];
  let debounceId: ReturnType<typeof setTimeout> | null = null;
  $: if (q !== undefined) scheduleSearch(q);

  function scheduleSearch(query: string) {
    if (debounceId) clearTimeout(debounceId);
    debounceId = setTimeout(async () => {
      const res = await fetch('/api/memory?q=' + encodeURIComponent(query));
      if (res.ok) results = await res.json();
    }, 300);
  }
</script>

{#if open}
  <div class="drawer">
    <header>
      <h3>Memories</h3>
      <button on:click={() => (open = false)}>✕</button>
    </header>
    <input bind:value={q} placeholder="Search..." />
    <ul>
      {#each results as r}
        <li>
          <div class="text">{r.text}</div>
          <div class="meta">score {r.score.toFixed(2)}</div>
        </li>
      {/each}
      {#if results.length === 0 && q}
        <li class="empty">no matches</li>
      {/if}
    </ul>
  </div>
{/if}

<style>
  .drawer { position: fixed; right: 0; top: 0; bottom: 0; width: 380px; background: #0d0d0d; border-left: 1px solid #1f1f1f; display: flex; flex-direction: column; z-index: 10; }
  header { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid #1f1f1f; }
  h3 { margin: 0; font-size: 13px; color: #aaa; }
  header button { background: none; border: none; color: #888; font-size: 16px; cursor: pointer; }
  input { margin: 12px; padding: 8px 10px; background: #111; color: #e8e8e8; border: 1px solid #2a2a2a; border-radius: 6px; font-family: inherit; }
  ul { list-style: none; padding: 0; margin: 0; overflow-y: auto; flex: 1; }
  li { padding: 10px 16px; border-bottom: 1px solid #151515; }
  .text { color: #ddd; font-size: 13px; }
  .meta { color: #666; font-size: 11px; margin-top: 4px; }
  .empty { color: #666; font-style: italic; }
</style>
```

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/components/Sidebar.svelte web/src/lib/components/MemoryDrawer.svelte
git commit -m "feat(web): Sidebar (sessions) and MemoryDrawer (search)"
```

---

### Task 5: Login page + gate

**Files:**
- Create: `web/src/lib/components/LoginForm.svelte`
- Modify: `web/src/routes/+page.svelte`

- [ ] **Step 1: LoginForm**

```svelte
<!-- web/src/lib/components/LoginForm.svelte -->
<script lang="ts">
  export let onSuccess: () => void;
  let password = '';
  let error = '';
  async function submit() {
    error = '';
    const res = await fetch('/api/login', { method: 'POST', body: JSON.stringify({ password }) });
    if (res.ok) onSuccess();
    else if (res.status === 429) error = 'too many attempts, wait 30 seconds';
    else error = 'invalid password';
  }
</script>

<div class="wrap">
  <form on:submit|preventDefault={submit}>
    <h1>picooraclaw</h1>
    <input type="password" bind:value={password} placeholder="Password" autofocus />
    <button type="submit">Unlock</button>
    {#if error}<p class="err">{error}</p>{/if}
  </form>
</div>

<style>
  .wrap { min-height: 100vh; display: grid; place-items: center; background: #0a0a0a; color: #e8e8e8; }
  form { display: flex; flex-direction: column; gap: 10px; width: 280px; }
  h1 { margin: 0 0 12px 0; text-align: center; font-weight: 500; }
  input { padding: 10px; background: #111; color: #e8e8e8; border: 1px solid #2a2a2a; border-radius: 6px; font-family: inherit; font-size: 14px; }
  button { padding: 10px; background: #4a7a9f; color: white; border: none; border-radius: 6px; font-weight: 600; cursor: pointer; }
  .err { color: #f88; font-size: 12px; margin: 0; text-align: center; }
</style>
```

- [ ] **Step 2: Main route**

```svelte
<!-- web/src/routes/+page.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from '$lib/components/Sidebar.svelte';
  import MessageBubble from '$lib/components/MessageBubble.svelte';
  import Composer from '$lib/components/Composer.svelte';
  import MemoryDrawer from '$lib/components/MemoryDrawer.svelte';
  import LoginForm from '$lib/components/LoginForm.svelte';
  import { currentSession } from '$lib/stores/session';
  import { messagesBySession, appendUserMessage, applyEvent } from '$lib/stores/messages';
  import { connect, subscribe as wsSubscribe, sendMessage, onEvent, wsConnected } from '$lib/stores/ws';

  let needsLogin = false;
  let memoryOpen = false;

  onMount(async () => {
    // Probe auth status
    const probe = await fetch('/api/sessions');
    if (probe.status === 401) { needsLogin = true; return; }
    initApp();
  });

  function initApp() {
    connect();
    onEvent((e) => {
      if (e.session_id) applyEvent(e.session_id, e);
    });
    currentSession.subscribe((sid) => {
      if (sid) wsSubscribe(sid);
    });
  }

  function handleSend(text: string) {
    const sid = $currentSession;
    appendUserMessage(sid, text);
    sendMessage(sid, text);
  }

  $: messages = $messagesBySession[$currentSession] ?? [];
</script>

{#if needsLogin}
  <LoginForm onSuccess={() => { needsLogin = false; initApp(); }} />
{:else}
  <div class="app">
    <Sidebar />
    <main>
      <header>
        <span class="status" class:on={$wsConnected}>{$wsConnected ? '● connected' : '○ offline'}</span>
        <button class="mem" on:click={() => (memoryOpen = !memoryOpen)}>Memories</button>
      </header>
      <div class="feed">
        {#each messages as m (m.id)}
          <MessageBubble msg={m} />
        {/each}
      </div>
      <Composer onSend={handleSend} />
    </main>
    <MemoryDrawer bind:open={memoryOpen} />
  </div>
{/if}

<style>
  :global(body) { margin: 0; font-family: system-ui, -apple-system, "SF Pro Text", sans-serif; background: #0a0a0a; color: #e8e8e8; }
  .app { display: flex; min-height: 100vh; }
  main { flex: 1; display: flex; flex-direction: column; }
  header { display: flex; justify-content: space-between; align-items: center; padding: 10px 16px; border-bottom: 1px solid #1f1f1f; font-size: 12px; }
  .status { color: #888; }
  .status.on { color: #7a7; }
  .mem { background: none; color: #888; border: 1px solid #2a2a2a; padding: 4px 10px; border-radius: 6px; cursor: pointer; }
  .feed { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; }
</style>
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build && cd ..
git add web/src/
git commit -m "feat(web): main route with auth gate, chat feed, memory drawer"
```

---

### Task 6: Unit tests for ToolCallCard + messages store

**Files:**
- Create: `web/src/lib/components/ToolCallCard.test.ts`
- Create: `web/src/lib/stores/messages.test.ts`

- [ ] **Step 1: Component test**

```ts
// web/src/lib/components/ToolCallCard.test.ts
import { render, fireEvent } from '@testing-library/svelte';
import ToolCallCard from './ToolCallCard.svelte';
import { describe, it, expect } from 'vitest';

describe('ToolCallCard', () => {
  it('renders collapsed by default', () => {
    const tc = { id: '1', tool: 'remember', args: { text: 'hi' }, done: true, ok: true, result: 'ok' };
    const { queryByText, getByRole } = render(ToolCallCard, { props: { tc } });
    expect(getByRole('button').textContent).toContain('remember');
    expect(queryByText('ok', { exact: false })).not.toBeNull();
  });

  it('expands on click', async () => {
    const tc = { id: '1', tool: 'remember', args: { text: 'hi' }, done: true, ok: true, result: 'ok' };
    const { getByRole, findByText } = render(ToolCallCard, { props: { tc } });
    await fireEvent.click(getByRole('button'));
    expect(await findByText(/"text": "hi"/)).toBeTruthy();
  });

  it('shows error border when ok=false', () => {
    const tc = { id: '1', tool: 'remember', done: true, ok: false, result: 'boom' };
    const { container } = render(ToolCallCard, { props: { tc } });
    expect(container.querySelector('.tc.error')).toBeTruthy();
  });
});
```

- [ ] **Step 2: Store test**

```ts
// web/src/lib/stores/messages.test.ts
import { get } from 'svelte/store';
import { describe, it, expect } from 'vitest';
import { messagesBySession, appendUserMessage, applyEvent } from './messages';

describe('messages store', () => {
  it('appends user then folds message_start → tool_call → message_end', () => {
    messagesBySession.set({});
    appendUserMessage('s1', 'hello');
    applyEvent('s1', { type: 'message_start', session_id: 's1', message_id: 'm1' });
    applyEvent('s1', { type: 'tool_call_start', session_id: 's1', message_id: 'm1', id: 'tc1', tool: 'remember', args: { text: 'hi' } });
    applyEvent('s1', { type: 'tool_call_end', session_id: 's1', message_id: 'm1', id: 'tc1', ok: true, result: 'ok' });
    applyEvent('s1', { type: 'message_end', session_id: 's1', message_id: 'm1', text: 'done' });

    const list = get(messagesBySession).s1;
    expect(list).toHaveLength(2);
    expect(list[1].text).toBe('done');
    expect(list[1].toolCalls[0].done).toBe(true);
    expect(list[1].streaming).toBe(false);
  });
});
```

- [ ] **Step 3: Configure Vitest**

Create `web/vitest.config.ts`:

```ts
import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte({ hot: !process.env.VITEST })],
  test: { environment: 'jsdom', globals: true }
});
```

Add to `web/package.json` scripts: `"test": "vitest run"`.

- [ ] **Step 4: Run tests**

```bash
cd web && npm test
```

Expected: both test files pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/components/ToolCallCard.test.ts web/src/lib/stores/messages.test.ts web/vitest.config.ts web/package.json
git commit -m "test(web): ToolCallCard rendering + messages-store event folding"
```

---

### Task 7: Playwright smoke test

**Files:**
- Create: `web/playwright.config.ts`
- Create: `web/tests/smoke.spec.ts`

- [ ] **Step 1: Config**

```ts
// web/playwright.config.ts
import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  use: { baseURL: 'http://localhost:3000', trace: 'on-first-retry' },
  webServer: {
    command: '../bin/picooraclaw-webui --listen :3000 --picooraclaw-url http://localhost:8090',
    url: 'http://localhost:3000',
    reuseExistingServer: true
  }
});
```

- [ ] **Step 2: Smoke spec**

```ts
// web/tests/smoke.spec.ts
import { test, expect } from '@playwright/test';

test('loads and shows login or chat', async ({ page }) => {
  await page.goto('/');
  const hasLogin = await page.locator('input[type=password]').isVisible().catch(() => false);
  const hasFeed = await page.locator('.feed').isVisible().catch(() => false);
  expect(hasLogin || hasFeed).toBe(true);
});
```

(The fuller smoke — send message, see tool call — is in Phase 4 after docker-compose is ready.)

- [ ] **Step 3: Run**

```bash
cd web && npx playwright install && npm run build && cd .. && make build && cd web && npx playwright test
```

- [ ] **Step 4: Commit**

```bash
git add web/playwright.config.ts web/tests/ web/package.json
git commit -m "test(web): playwright smoke for login/feed visibility"
```

---

## Self-review checklist

- [ ] `cd web && npm test` passes
- [ ] `cd web && npm run build` produces `web/build/index.html`
- [ ] `make sync-static && make build` produces a working binary (runs, serves UI)
- [ ] Opening browser shows login page (if password set) or chat feed
- [ ] No hydration errors in browser console
- [ ] Send a message end-to-end: appears in feed, tool-call card renders when agent uses a tool

## Phase 4 prerequisites delivered

- SvelteKit bundle builds to `web/build/`
- `make sync-static && make build` produces embedded binary
- All frontend routes functional assuming bridge and picooraclaw are running
