<script lang="ts">
	import type { ToolCall } from '$lib/stores/messages';

	let { call }: { call: ToolCall } = $props();
	let open = $state(false);

	function shorten(s: string, n = 120): string {
		if (!s) return '';
		return s.length <= n ? s : s.slice(0, n) + '…';
	}

	function summarize(tc: ToolCall): string {
		if (!tc.args) return tc.tool;
		const keys = Object.keys(tc.args);
		if (keys.length === 0) return tc.tool;
		const kv = keys
			.slice(0, 3)
			.map((k) => `${k}=${shorten(String((tc.args as Record<string, unknown>)[k]), 32)}`)
			.join(', ');
		return `${tc.tool}(${kv})`;
	}

	const status = $derived(call.running ? 'running' : call.ok === false ? 'error' : 'done');
</script>

<div class="tool-card" data-status={status}>
	<button class="summary" onclick={() => (open = !open)} aria-expanded={open}>
		<span class="indicator" aria-hidden="true">
			{#if call.running}
				<span class="spinner"></span>
			{:else if call.ok === false}
				<svg viewBox="0 0 12 12" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.6">
					<circle cx="6" cy="6" r="4.25" />
					<path d="M6 4v2.5M6 7.75v.25" stroke-linecap="round" />
				</svg>
			{:else}
				<svg viewBox="0 0 12 12" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.6">
					<path d="M2.5 6.5l2.25 2L9.5 4" stroke-linecap="round" stroke-linejoin="round" />
				</svg>
			{/if}
		</span>
		<span class="tool-name">{summarize(call)}</span>
		<span class="chev" data-open={open} aria-hidden="true">
			<svg viewBox="0 0 10 10" width="9" height="9" fill="none" stroke="currentColor" stroke-width="1.6">
				<path d="M3 3l3.5 2L3 7" stroke-linecap="round" stroke-linejoin="round" />
			</svg>
		</span>
	</button>

	{#if open}
		<div class="detail">
			{#if call.args}
				<div class="field">
					<span class="label">args</span>
					<pre>{JSON.stringify(call.args, null, 2)}</pre>
				</div>
			{/if}
			{#if call.result !== undefined}
				<div class="field">
					<span class="label">result</span>
					<pre>{call.result}</pre>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.tool-card {
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface);
		font-size: 0.85rem;
		overflow: hidden;
	}
	.tool-card[data-status='error'] {
		border-color: rgba(248, 113, 113, 0.35);
		background: linear-gradient(90deg, var(--err-soft), transparent 80%);
	}
	.tool-card[data-status='done'] {
		border-color: rgba(52, 211, 153, 0.25);
	}
	.tool-card[data-status='running'] {
		border-color: var(--accent-soft);
	}

	.summary {
		width: 100%;
		display: grid;
		grid-template-columns: 18px 1fr 16px;
		align-items: center;
		gap: 10px;
		padding: 8px 12px;
		background: transparent;
		color: inherit;
		border: 0;
		cursor: pointer;
		text-align: left;
		font-family: inherit;
		font-size: inherit;
		transition: background 0.12s ease;
	}
	.summary:hover {
		background: rgba(255, 255, 255, 0.025);
	}

	.indicator {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		border-radius: 50%;
	}
	.tool-card[data-status='done'] .indicator {
		color: var(--ok);
		background: var(--ok-soft);
	}
	.tool-card[data-status='error'] .indicator {
		color: var(--err);
		background: var(--err-soft);
	}
	.tool-card[data-status='running'] .indicator {
		color: var(--accent);
		background: var(--accent-soft);
	}

	.tool-name {
		flex: 1;
		font-family: var(--font-mono);
		font-size: 0.82rem;
		color: var(--fg-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.chev {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: var(--fg-faint);
		transition: transform 0.18s ease;
	}
	.chev[data-open='true'] {
		transform: rotate(90deg);
	}

	.spinner {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		border: 2px solid currentColor;
		border-top-color: transparent;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.detail {
		border-top: 1px solid var(--border);
		padding: 10px 12px;
		background: var(--bg-deep);
	}
	.field {
		margin: 6px 0;
	}
	.label {
		font-family: var(--font-mono);
		font-size: 0.68rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--fg-subtle);
		display: block;
		margin-bottom: 4px;
	}
	pre {
		margin: 0;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: var(--font-mono);
		font-size: 0.78rem;
		color: var(--fg-muted);
		line-height: 1.5;
	}
</style>
