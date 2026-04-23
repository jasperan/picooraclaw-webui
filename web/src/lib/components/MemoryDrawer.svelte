<script lang="ts">
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

	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	function scheduleSearch() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(runSearch, 250);
	}

	async function runSearch() {
		const query = q.trim();
		if (!query) {
			results = [];
			error = null;
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
		}
	}

	function hitText(h: MemoryHit): string {
		return h.content ?? h.text ?? JSON.stringify(h);
	}
</script>

{#if open}
	<div
		class="backdrop"
		onclick={onClose}
		onkeydown={(e) => e.key === 'Escape' && onClose()}
		role="button"
		tabindex="-1"
		aria-label="Close memory drawer"
	></div>
	<section class="drawer" aria-label="Memory search">
		<header>
			<h2>Memory</h2>
			<button class="close" onclick={onClose} aria-label="Close">×</button>
		</header>
		<input
			type="search"
			placeholder="Search memory…"
			bind:value={q}
			oninput={scheduleSearch}
			autocomplete="off"
		/>
		<div class="body">
			{#if loading}
				<div class="status">Searching…</div>
			{:else if error}
				<div class="status err">{error}</div>
			{:else if results.length === 0 && q.trim()}
				<div class="status">No results</div>
			{:else}
				<ul>
					{#each results as h, i (h.id ?? i)}
						<li>
							<div class="text">{hitText(h)}</div>
							{#if h.score !== undefined}
								<div class="meta">score: {h.score.toFixed(3)}</div>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</section>
{/if}

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.4);
		z-index: 10;
		border: 0;
	}
	.drawer {
		position: fixed;
		top: 0;
		right: 0;
		bottom: 0;
		width: min(420px, 100%);
		background: var(--drawer-bg, #0e0e0e);
		border-left: 1px solid var(--border, #2a2a2a);
		z-index: 11;
		display: flex;
		flex-direction: column;
		padding: 12px;
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 8px;
	}
	h2 {
		margin: 0;
		font-size: 1rem;
	}
	.close {
		background: transparent;
		color: inherit;
		border: 0;
		font-size: 1.5rem;
		cursor: pointer;
		line-height: 1;
		padding: 0 4px;
	}
	input {
		padding: 8px 10px;
		border-radius: 6px;
		border: 1px solid var(--border, #2a2a2a);
		background: var(--input-bg, #111);
		color: inherit;
		font: inherit;
	}
	.body {
		margin-top: 10px;
		overflow-y: auto;
		flex: 1;
	}
	.status {
		padding: 8px;
		opacity: 0.6;
		font-size: 0.85rem;
	}
	.status.err {
		color: #ff6b6b;
	}
	ul {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	li {
		padding: 8px;
		border-radius: 6px;
		margin-bottom: 6px;
		background: rgba(255, 255, 255, 0.03);
	}
	.text {
		white-space: pre-wrap;
		word-break: break-word;
		font-size: 0.9rem;
	}
	.meta {
		margin-top: 4px;
		font-size: 0.75rem;
		opacity: 0.6;
	}
</style>
