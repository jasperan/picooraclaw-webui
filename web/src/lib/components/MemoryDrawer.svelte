<script lang="ts">
	import { fly, fade } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';

	type MemoryHit = {
		id?: string | number;
		content?: string;
		text?: string;
		score?: number;
		metadata?: Record<string, unknown>;
	};

	type Props = {
		open: boolean;
		onClose: () => void;
	};
	let { open, onClose }: Props = $props();

	let q = $state('');
	let results: MemoryHit[] = $state([]);
	let loading = $state(false);
	let error: string | null = $state(null);
	let searched = $state(false);

	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	function scheduleSearch() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(runSearch, 220);
	}

	async function runSearch() {
		const query = q.trim();
		if (!query) {
			results = [];
			error = null;
			searched = false;
			return;
		}
		loading = true;
		error = null;
		try {
			const res = await fetch(`/api/memory?q=${encodeURIComponent(query)}`);
			if (!res.ok) {
				throw new Error(`HTTP ${res.status}`);
			}
			const data = await res.json();
			results = Array.isArray(data) ? data : (data?.results ?? []);
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
			results = [];
		} finally {
			loading = false;
			searched = true;
		}
	}

	function hitText(h: MemoryHit): string {
		return h.content ?? h.text ?? JSON.stringify(h);
	}

	function onKeydown(ev: KeyboardEvent) {
		if (ev.key === 'Escape') onClose();
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	<button
		class="backdrop"
		onclick={onClose}
		aria-label="Close memory drawer"
		transition:fade={{ duration: 160 }}
	></button>
	<section
		class="drawer"
		aria-label="Memory search"
		transition:fly={{ x: 420, duration: 260, easing: cubicOut }}
	>
		<header>
			<div class="title-row">
				<span class="eyebrow">Oracle AI · vector</span>
				<h2>Memory</h2>
			</div>
			<button class="close" onclick={onClose} aria-label="Close">
				<svg
					viewBox="0 0 16 16"
					width="14"
					height="14"
					fill="none"
					stroke="currentColor"
					stroke-width="1.6"
					aria-hidden="true"
				>
					<path d="M4 4l8 8M12 4l-8 8" stroke-linecap="round" />
				</svg>
			</button>
		</header>

		<div class="search">
			<svg
				class="search-icon"
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
			<input
				type="search"
				placeholder="Semantic search…"
				bind:value={q}
				oninput={scheduleSearch}
				autocomplete="off"
				aria-label="Search memory"
			/>
			{#if loading}
				<span class="live-dot" aria-hidden="true"></span>
			{/if}
		</div>

		<div class="body">
			{#if loading && !searched}
				<div class="skeleton-list">
					<div class="skel"></div>
					<div class="skel"></div>
					<div class="skel short"></div>
				</div>
			{:else if error}
				<div class="status err">
					<span>Couldn't reach memory:</span>
					<code>{error}</code>
				</div>
			{:else if searched && results.length === 0}
				<div class="status">
					<span class="empty-icon" aria-hidden="true"></span>
					<span>No hits for</span>
					<code>"{q}"</code>
				</div>
			{:else if !searched && !q.trim()}
				<div class="status idle">
					<span>Search embeddings, scores, and raw traces.</span>
				</div>
			{:else}
				<ul>
					{#each results as h, i (h.id ?? i)}
						<li>
							<div class="hit-head">
								<span class="hit-id">#{h.id ?? i}</span>
								{#if h.score !== undefined}
									<span class="score">{h.score.toFixed(3)}</span>
								{/if}
							</div>
							<div class="text">{hitText(h)}</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<footer class="foot">
			<span class="foot-label">results</span>
			<span class="foot-count">{results.length}</span>
		</footer>
	</section>
{/if}

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		background:
			radial-gradient(600px circle at 70% 30%, rgba(245, 158, 11, 0.05), transparent 60%),
			rgba(5, 5, 6, 0.55);
		backdrop-filter: blur(4px);
		-webkit-backdrop-filter: blur(4px);
		z-index: 40;
		border: 0;
		cursor: default;
	}

	.drawer {
		position: fixed;
		top: 0;
		right: 0;
		bottom: 0;
		width: min(440px, 100%);
		background:
			linear-gradient(180deg, rgba(255, 255, 255, 0.015), rgba(255, 255, 255, 0)),
			rgba(14, 14, 16, 0.92);
		backdrop-filter: blur(18px) saturate(140%);
		-webkit-backdrop-filter: blur(18px) saturate(140%);
		border-left: 1px solid var(--border);
		z-index: 41;
		display: flex;
		flex-direction: column;
		box-shadow: -24px 0 60px -30px rgba(0, 0, 0, 0.8);
	}

	header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		padding: 20px 22px 16px;
		border-bottom: 1px solid var(--border);
	}
	.title-row {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.eyebrow {
		font-family: var(--font-mono);
		font-size: 0.68rem;
		text-transform: uppercase;
		letter-spacing: 0.14em;
		color: var(--accent);
	}
	h2 {
		margin: 0;
		font-size: 1.15rem;
		font-weight: 500;
		letter-spacing: -0.01em;
	}
	.close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 30px;
		height: 30px;
		border-radius: 8px;
		background: transparent;
		color: var(--fg-muted);
		border: 1px solid var(--border);
		cursor: pointer;
		transition: background 0.15s ease, border-color 0.15s ease;
	}
	.close:hover {
		background: var(--surface-2);
		border-color: var(--border-strong);
		color: var(--fg);
	}

	.search {
		position: relative;
		margin: 16px 20px 12px;
	}
	.search-icon {
		position: absolute;
		left: 12px;
		top: 50%;
		transform: translateY(-50%);
		color: var(--fg-faint);
	}
	input {
		width: 100%;
		padding: 10px 34px 10px 32px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		background: var(--surface);
		color: var(--fg);
		font: inherit;
		font-size: 0.88rem;
		transition: border-color 0.15s ease;
	}
	input::placeholder {
		color: var(--fg-faint);
	}
	input:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 3px var(--accent-ghost);
	}
	.live-dot {
		position: absolute;
		right: 12px;
		top: 50%;
		transform: translateY(-50%);
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--accent);
		box-shadow: 0 0 8px var(--accent);
		animation: pulse-fast 0.9s ease-in-out infinite;
	}
	@keyframes pulse-fast {
		0%,
		100% {
			opacity: 0.4;
		}
		50% {
			opacity: 1;
		}
	}

	.body {
		margin: 0 20px;
		overflow-y: auto;
		flex: 1;
		padding-bottom: 12px;
	}

	.status {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 6px;
		padding: 14px 12px;
		font-size: 0.85rem;
		color: var(--fg-subtle);
		border-radius: var(--radius-sm);
		background: var(--surface);
		border: 1px dashed var(--border);
	}
	.status.idle {
		border-style: solid;
	}
	.status.err {
		background: var(--err-soft);
		color: var(--err);
		border: 0;
	}
	.status code {
		font-family: var(--font-mono);
		font-size: 0.78rem;
		background: rgba(255, 255, 255, 0.04);
		padding: 1px 6px;
		border-radius: 4px;
		color: var(--fg-muted);
	}
	.empty-icon {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--fg-faint);
	}

	.skeleton-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 4px 0;
	}
	.skel {
		height: 56px;
		border-radius: var(--radius-sm);
		background: linear-gradient(
			90deg,
			var(--surface) 0%,
			var(--surface-2) 50%,
			var(--surface) 100%
		);
		background-size: 200% 100%;
		animation: shimmer 1.4s linear infinite;
	}
	.skel.short {
		width: 70%;
	}
	@keyframes shimmer {
		from {
			background-position: 200% 0;
		}
		to {
			background-position: -200% 0;
		}
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	li {
		padding: 12px 14px;
		border-radius: var(--radius-sm);
		background: var(--surface);
		border: 1px solid var(--border);
		transition: border-color 0.15s ease, background 0.15s ease;
	}
	li:hover {
		border-color: var(--border-strong);
		background: var(--surface-2);
	}

	.hit-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 6px;
	}
	.hit-id {
		font-family: var(--font-mono);
		font-size: 0.7rem;
		color: var(--fg-faint);
	}
	.score {
		font-family: var(--font-mono);
		font-size: 0.72rem;
		color: var(--accent);
		background: var(--accent-soft);
		padding: 1px 8px;
		border-radius: 999px;
	}

	.text {
		white-space: pre-wrap;
		word-break: break-word;
		font-size: 0.88rem;
		line-height: 1.55;
		color: var(--fg);
	}

	.foot {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px 22px;
		border-top: 1px solid var(--border);
		font-family: var(--font-mono);
		font-size: 0.7rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--fg-subtle);
	}
	.foot-count {
		margin-left: auto;
		color: var(--fg);
	}
</style>
