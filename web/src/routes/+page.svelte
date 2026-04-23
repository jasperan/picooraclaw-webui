<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import MessageBubble from '$lib/components/MessageBubble.svelte';
	import Composer from '$lib/components/Composer.svelte';
	import MemoryDrawer from '$lib/components/MemoryDrawer.svelte';
	import LoginForm from '$lib/components/LoginForm.svelte';
	import { currentSession } from '$lib/stores/session';
	import { messagesBySession, appendUserMessage, applyEvent } from '$lib/stores/messages';
	import {
		connect,
		subscribe as wsSubscribe,
		sendMessage,
		onEvent,
		wsConnected
	} from '$lib/stores/ws';

	let needsLogin = $state(false);
	let memoryOpen = $state(false);
	let ready = $state(false);

	onMount(async () => {
		try {
			const probe = await fetch('/api/sessions');
			if (probe.status === 401) {
				needsLogin = true;
				return;
			}
		} catch {
			// Network error — still try to init; ws reconnect will retry.
		}
		initApp();
	});

	function initApp() {
		ready = true;
		connect();
		onEvent((e) => {
			if (e.session_id) applyEvent(e.session_id, e);
		});
		currentSession.subscribe((sid) => {
			if (sid) wsSubscribe(sid);
		});
	}

	function onLoginSuccess() {
		needsLogin = false;
		initApp();
	}

	function handleSend(text: string) {
		const sid = $currentSession;
		appendUserMessage(sid, text);
		sendMessage(sid, text);
	}

	const messages = $derived($messagesBySession[$currentSession] ?? []);
</script>

{#if needsLogin}
	<LoginForm onSuccess={onLoginSuccess} />
{:else if ready}
	<div class="app">
		<Sidebar />
		<main>
			<header class="topbar">
				<div class="left">
					<span class="conn" data-ok={$wsConnected}>
						{$wsConnected ? 'connected' : 'offline'}
					</span>
					<span class="session-id">{$currentSession}</span>
				</div>
				<div class="right">
					<button class="mem-btn" onclick={() => (memoryOpen = true)}>Memory</button>
				</div>
			</header>
			<section class="feed" aria-live="polite">
				{#each messages as msg (msg.id)}
					<MessageBubble {msg} />
				{/each}
			</section>
			<Composer onSend={handleSend} />
		</main>
		<MemoryDrawer open={memoryOpen} onClose={() => (memoryOpen = false)} />
	</div>
{/if}

<style>
	:global(body) {
		margin: 0;
		background: var(--bg, #0a0a0a);
		color: var(--fg, #e7e7e7);
		font-family:
			system-ui,
			-apple-system,
			Segoe UI,
			Roboto,
			sans-serif;
	}
	.app {
		display: flex;
		height: 100vh;
	}
	main {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.topbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 12px;
		border-bottom: 1px solid var(--border, #2a2a2a);
		background: var(--topbar-bg, #0b0b0b);
	}
	.left {
		display: flex;
		align-items: center;
		gap: 12px;
		font-size: 0.85rem;
	}
	.conn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 2px 8px;
		border-radius: 999px;
		background: #3a1f1f;
		color: #ff9a9a;
	}
	.conn[data-ok='true'] {
		background: #1f3a24;
		color: #9fe3ab;
	}
	.session-id {
		opacity: 0.6;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	}
	.mem-btn {
		padding: 6px 12px;
		border-radius: 6px;
		background: transparent;
		color: inherit;
		border: 1px solid var(--border, #2a2a2a);
		cursor: pointer;
	}
	.mem-btn:hover {
		background: rgba(255, 255, 255, 0.04);
	}
	.feed {
		flex: 1;
		overflow-y: auto;
		padding: 16px;
		display: flex;
		flex-direction: column;
	}
</style>
