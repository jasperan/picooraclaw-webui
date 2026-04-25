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
	let sidebarOpen = $state(false);

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

		const onKey = (e: KeyboardEvent) => {
			if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
				e.preventDefault();
				memoryOpen = true;
			}
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
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
	const isEmpty = $derived(messages.length === 0);
</script>

{#if needsLogin}
	<LoginForm onSuccess={onLoginSuccess} />
{:else if ready}
	<div class="app" data-sidebar-open={sidebarOpen}>
		<Sidebar />
		<main>
			<header class="topbar">
				<button
					class="menu-btn"
					onclick={() => (sidebarOpen = !sidebarOpen)}
					aria-label="Toggle sidebar"
				>
					<svg
						viewBox="0 0 16 16"
						width="14"
						height="14"
						fill="none"
						stroke="currentColor"
						stroke-width="1.5"
						aria-hidden="true"
					>
						<path d="M2.5 4.5h11M2.5 8h11M2.5 11.5h11" stroke-linecap="round" />
					</svg>
				</button>

				<div class="left">
					<span class="conn-pill" data-ok={$wsConnected}>
						<span class="conn-dot"></span>
						<span class="conn-label">{$wsConnected ? 'live' : 'offline'}</span>
					</span>
					<span class="divider" aria-hidden="true"></span>
					<span class="session-label">session</span>
					<span class="session-id">{$currentSession}</span>
				</div>

				<div class="right">
					<button class="mem-btn" onclick={() => (memoryOpen = true)}>
						<svg
							viewBox="0 0 16 16"
							width="13"
							height="13"
							fill="none"
							stroke="currentColor"
							stroke-width="1.5"
							aria-hidden="true"
						>
							<circle cx="7" cy="7" r="4.25" />
							<path d="M10.25 10.25L13 13" stroke-linecap="round" />
						</svg>
						<span>Memory</span>
						<kbd>⌘K</kbd>
					</button>
				</div>
			</header>

			<section class="feed-wrap">
				<div class="feed" aria-live="polite">
					{#if isEmpty}
						<div class="welcome">
							<div class="welcome-mark" aria-hidden="true">
								<span class="claw"></span>
								<span class="claw"></span>
								<span class="claw"></span>
							</div>
							<h1>Ready when you are.</h1>
							<p>
								Ask picooraclaw anything. Memory is pulled from Oracle AI Vector Search, tool
								calls stream inline, and every reasoning step is auditable.
							</p>
							<div class="suggestions">
								<div class="suggestion">
									<span class="s-tag">explore</span>
									<span>Summarize recent memory about “database migrations”</span>
								</div>
								<div class="suggestion">
									<span class="s-tag">analyze</span>
									<span>Compare today's Oracle ADW usage to last week</span>
								</div>
								<div class="suggestion">
									<span class="s-tag">plan</span>
									<span>Draft a rollout for the new vector index</span>
								</div>
							</div>
						</div>
					{:else}
						{#each messages as msg (msg.id)}
							<MessageBubble {msg} />
						{/each}
					{/if}
				</div>
			</section>

			<Composer onSend={handleSend} />
		</main>
		<MemoryDrawer open={memoryOpen} onClose={() => (memoryOpen = false)} />
	</div>
{/if}

<style>
	.app {
		display: grid;
		grid-template-columns: auto 1fr;
		min-height: 100dvh;
		height: 100dvh;
	}

	main {
		display: flex;
		flex-direction: column;
		min-width: 0;
		min-height: 0;
		background: transparent;
	}

	.topbar {
		display: flex;
		align-items: center;
		gap: 14px;
		padding: 14px 20px;
		border-bottom: 1px solid var(--border);
		background: rgba(9, 9, 11, 0.6);
		backdrop-filter: blur(14px);
		-webkit-backdrop-filter: blur(14px);
		position: sticky;
		top: 0;
		z-index: 20;
	}

	.menu-btn {
		display: none;
		width: 32px;
		height: 32px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--fg-muted);
		cursor: pointer;
		align-items: center;
		justify-content: center;
	}

	.left {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 0.82rem;
	}
	.conn-pill {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px;
		border-radius: 999px;
		background: var(--err-soft);
		color: var(--err);
		font-family: var(--font-mono);
		font-size: 0.7rem;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}
	.conn-pill[data-ok='true'] {
		background: var(--ok-soft);
		color: var(--ok);
	}
	.conn-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: currentColor;
		box-shadow: 0 0 6px currentColor;
		animation: breathe 2.4s ease-in-out infinite;
	}
	@keyframes breathe {
		0%,
		100% {
			opacity: 0.55;
			transform: scale(1);
		}
		50% {
			opacity: 1;
			transform: scale(1.2);
		}
	}
	.divider {
		width: 1px;
		height: 14px;
		background: var(--border);
	}
	.session-label {
		font-family: var(--font-mono);
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: var(--fg-subtle);
	}
	.session-id {
		font-family: var(--font-mono);
		font-size: 0.82rem;
		color: var(--fg);
	}

	.right {
		margin-left: auto;
	}
	.mem-btn {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 7px 12px;
		border-radius: 999px;
		background: transparent;
		color: var(--fg);
		border: 1px solid var(--border);
		cursor: pointer;
		font-size: 0.85rem;
		transition: border-color 0.15s ease, background 0.15s ease;
	}
	.mem-btn:hover {
		background: var(--surface-2);
		border-color: var(--border-strong);
	}
	.mem-btn kbd {
		font-family: var(--font-mono);
		font-size: 0.68rem;
		color: var(--fg-subtle);
		padding: 1px 5px;
		border-radius: 4px;
		background: var(--surface-3);
		border: 1px solid var(--border);
	}

	.feed-wrap {
		flex: 1;
		overflow-y: auto;
		min-height: 0;
	}
	.feed {
		padding: 24px 20px 40px;
		display: flex;
		flex-direction: column;
		min-height: 100%;
	}

	.welcome {
		margin: auto;
		max-width: 640px;
		text-align: left;
		padding: 32px 4px;
	}
	.welcome-mark {
		display: inline-flex;
		gap: 6px;
		align-items: flex-end;
		margin-bottom: 24px;
	}
	.welcome-mark .claw {
		width: 4px;
		background: var(--accent);
		border-radius: 2px;
	}
	.welcome-mark .claw:nth-child(1) {
		height: 24px;
		transform: rotate(-18deg) translateY(2px);
	}
	.welcome-mark .claw:nth-child(2) {
		height: 34px;
	}
	.welcome-mark .claw:nth-child(3) {
		height: 24px;
		transform: rotate(18deg) translateY(2px);
	}

	.welcome h1 {
		margin: 0 0 12px;
		font-size: clamp(1.75rem, 3vw, 2.25rem);
		font-weight: 500;
		letter-spacing: -0.025em;
		line-height: 1.15;
	}
	.welcome p {
		margin: 0 0 24px;
		color: var(--fg-muted);
		max-width: 58ch;
		line-height: 1.6;
	}

	.suggestions {
		display: grid;
		grid-template-columns: 1fr;
		gap: 8px;
		max-width: 580px;
	}
	.suggestion {
		display: flex;
		align-items: center;
		gap: 14px;
		padding: 12px 14px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		background: var(--surface);
		color: var(--fg-muted);
		font-size: 0.9rem;
		transition: border-color 0.15s ease, background 0.15s ease, transform 0.12s ease;
		cursor: default;
	}
	.suggestion:hover {
		border-color: var(--border-strong);
		background: var(--surface-2);
		color: var(--fg);
	}
	.s-tag {
		font-family: var(--font-mono);
		font-size: 0.65rem;
		text-transform: uppercase;
		letter-spacing: 0.12em;
		color: var(--accent);
		padding: 2px 8px;
		border-radius: 999px;
		background: var(--accent-soft);
		flex-shrink: 0;
	}

	@media (max-width: 820px) {
		.app {
			grid-template-columns: 1fr;
		}
		.app :global(.sidebar) {
			position: fixed;
			top: 0;
			bottom: 0;
			left: 0;
			z-index: 30;
			transform: translateX(-100%);
			transition: transform 0.25s cubic-bezier(0.16, 1, 0.3, 1);
		}
		.app[data-sidebar-open='true'] :global(.sidebar) {
			transform: translateX(0);
			box-shadow: 20px 0 40px -20px rgba(0, 0, 0, 0.6);
		}
		.menu-btn {
			display: inline-flex;
		}
		.mem-btn kbd {
			display: none;
		}
	}
</style>
