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
</script>

<aside class="sidebar">
	<header>
		<button class="new-btn" onclick={newSession}>+ New chat</button>
	</header>
	<ul>
		{#each $sessions as s (s.id)}
			<li class:active={$currentSession === s.id}>
				<button class="session-btn" onclick={() => pick(s.id)}>
					<span class="title">{s.title || s.id}</span>
				</button>
			</li>
		{:else}
			<li class="empty">No sessions yet</li>
		{/each}
	</ul>
</aside>

<style>
	.sidebar {
		width: 240px;
		flex-shrink: 0;
		background: var(--sidebar-bg, #0b0b0b);
		border-right: 1px solid var(--border, #2a2a2a);
		display: flex;
		flex-direction: column;
		overflow-y: auto;
	}
	header {
		padding: 10px;
		border-bottom: 1px solid var(--border, #2a2a2a);
	}
	.new-btn {
		width: 100%;
		padding: 8px 10px;
		border-radius: 6px;
		background: var(--accent, #3b82f6);
		color: white;
		border: 0;
		font-weight: 600;
		cursor: pointer;
	}
	ul {
		list-style: none;
		margin: 0;
		padding: 4px;
	}
	li {
		border-radius: 6px;
		margin: 2px 0;
	}
	li.active {
		background: rgba(255, 255, 255, 0.06);
	}
	li.empty {
		padding: 10px;
		opacity: 0.6;
		font-size: 0.85rem;
	}
	.session-btn {
		width: 100%;
		padding: 8px 10px;
		text-align: left;
		background: transparent;
		border: 0;
		color: inherit;
		cursor: pointer;
		font: inherit;
	}
	.session-btn:hover {
		background: rgba(255, 255, 255, 0.04);
	}
	.title {
		display: block;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
