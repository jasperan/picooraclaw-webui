<script lang="ts">
	import { onMount } from 'svelte';
	import { sessions, currentSession, loadSessions, type Session } from '$lib/stores/session';

	onMount(() => {
		loadSessions().catch(() => {
			// Non-fatal — sidebar can be empty until first load.
		});
	});

	function newSession() {
		const id = `s-${Date.now().toString(36)}`;
		currentSession.set(id);
		const entry: Session = { id, title: 'New chat', last_at: Date.now() };
		sessions.update((list) => [entry, ...list.filter((s) => s.id !== id)]);
	}

	function pick(id: string) {
		currentSession.set(id);
	}

	function relTime(ms: number): string {
		if (!ms) return '';
		const diff = Date.now() - ms;
		if (diff < 60_000) return 'now';
		if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m`;
		if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h`;
		return `${Math.floor(diff / 86_400_000)}d`;
	}
</script>

<aside class="sidebar">
	<header class="brand-row">
		<div class="mark" aria-hidden="true">
			<span class="claw"></span>
			<span class="claw"></span>
			<span class="claw"></span>
		</div>
		<div class="brand">
			<span class="name">picooraclaw</span>
			<span class="sub">bridge · ws</span>
		</div>
	</header>

	<div class="new-row">
		<button class="new-btn" onclick={newSession}>
			<svg
				viewBox="0 0 16 16"
				width="14"
				height="14"
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
				aria-hidden="true"
			>
				<path d="M8 3v10M3 8h10" stroke-linecap="round" />
			</svg>
			<span>New chat</span>
		</button>
	</div>

	<div class="list-head">
		<span>Sessions</span>
		<span class="count">{$sessions.length}</span>
	</div>

	<ul>
		{#each $sessions as s (s.id)}
			<li class:active={$currentSession === s.id}>
				<button class="session-btn" onclick={() => pick(s.id)}>
					<span class="accent-bar" aria-hidden="true"></span>
					<span class="row-main">
						<span class="title">{s.title || s.id}</span>
						<span class="id">{s.id}</span>
					</span>
					<span class="time">{relTime(s.last_at)}</span>
				</button>
			</li>
		{:else}
			<li class="empty">
				<span class="empty-dot" aria-hidden="true"></span>
				<span>No sessions yet</span>
				<span class="empty-sub">Start one above</span>
			</li>
		{/each}
	</ul>

	<footer>
		<span class="foot-dot" aria-hidden="true"></span>
		<span class="foot-text">oracle free · 23ai</span>
		<span class="foot-ver">v1</span>
	</footer>
</aside>

<style>
	.sidebar {
		width: 272px;
		flex-shrink: 0;
		background: linear-gradient(180deg, var(--surface) 0%, var(--bg) 100%);
		border-right: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		overflow-y: auto;
	}

	.brand-row {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 18px 18px 14px;
		border-bottom: 1px solid var(--border);
	}

	.mark {
		display: inline-flex;
		gap: 3px;
		align-items: flex-end;
	}
	.claw {
		width: 2px;
		background: var(--accent);
		border-radius: 1px;
	}
	.claw:nth-child(1) {
		height: 12px;
		transform: rotate(-18deg) translateY(1px);
	}
	.claw:nth-child(2) {
		height: 16px;
	}
	.claw:nth-child(3) {
		height: 12px;
		transform: rotate(18deg) translateY(1px);
	}

	.brand {
		display: flex;
		flex-direction: column;
		line-height: 1.15;
	}
	.name {
		font-weight: 600;
		font-size: 0.95rem;
		letter-spacing: -0.01em;
	}
	.sub {
		font-family: var(--font-mono);
		font-size: 0.68rem;
		color: var(--fg-subtle);
		letter-spacing: 0.08em;
		text-transform: uppercase;
		margin-top: 2px;
	}

	.new-row {
		padding: 14px 12px 6px;
	}
	.new-btn {
		width: 100%;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 10px 12px;
		border-radius: var(--radius);
		background: var(--accent);
		color: var(--accent-fg);
		border: 0;
		font-weight: 600;
		font-size: 0.88rem;
		cursor: pointer;
		box-shadow: 0 8px 22px -14px rgba(245, 158, 11, 0.55);
		transition: transform 0.12s ease, background 0.15s ease;
	}
	.new-btn:hover {
		background: var(--accent-hover);
	}
	.new-btn:active {
		transform: translateY(1px);
	}

	.list-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 16px 18px 6px;
		font-family: var(--font-mono);
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.12em;
		color: var(--fg-subtle);
	}
	.count {
		color: var(--fg-faint);
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 2px 8px;
		flex: 1;
	}

	li {
		border-radius: var(--radius-sm);
		margin: 2px 0;
	}

	.session-btn {
		width: 100%;
		display: grid;
		grid-template-columns: 3px 1fr auto;
		align-items: center;
		gap: 10px;
		padding: 9px 10px 9px 7px;
		text-align: left;
		background: transparent;
		border: 0;
		color: inherit;
		cursor: pointer;
		border-radius: var(--radius-sm);
		transition: background 0.12s ease;
	}
	.session-btn:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.accent-bar {
		width: 3px;
		height: 22px;
		border-radius: 2px;
		background: transparent;
		transition: background 0.15s ease;
	}

	li.active .session-btn {
		background: rgba(255, 255, 255, 0.04);
	}
	li.active .accent-bar {
		background: var(--accent);
		box-shadow: 0 0 10px var(--accent-soft);
	}

	.row-main {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.title {
		font-size: 0.88rem;
		font-weight: 500;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.id {
		font-family: var(--font-mono);
		font-size: 0.68rem;
		color: var(--fg-faint);
		margin-top: 1px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.time {
		font-family: var(--font-mono);
		font-size: 0.7rem;
		color: var(--fg-subtle);
		padding-right: 4px;
	}

	li.empty {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 22px 16px;
		color: var(--fg-subtle);
		font-size: 0.85rem;
	}
	.empty-dot {
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: var(--border-strong);
	}
	.empty-sub {
		font-size: 0.75rem;
		color: var(--fg-faint);
	}

	footer {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 12px 18px;
		border-top: 1px solid var(--border);
		font-family: var(--font-mono);
		font-size: 0.7rem;
		color: var(--fg-subtle);
		letter-spacing: 0.06em;
	}
	.foot-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--ok);
		box-shadow: 0 0 8px var(--ok);
		animation: dot-pulse 3s ease-in-out infinite;
	}
	.foot-ver {
		margin-left: auto;
		color: var(--fg-faint);
	}

	@keyframes dot-pulse {
		0%,
		100% {
			opacity: 0.6;
		}
		50% {
			opacity: 1;
		}
	}

	@media (max-width: 820px) {
		.sidebar {
			width: 240px;
		}
	}
</style>
