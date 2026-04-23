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
				!
			{:else}
				✓
			{/if}
		</span>
		<span class="tool-name">{summarize(call)}</span>
		<span class="chev">{open ? '▾' : '▸'}</span>
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
		border: 1px solid var(--border, #2a2a2a);
		border-radius: 6px;
		margin: 4px 0;
		font-size: 0.85rem;
		background: var(--tool-bg, #141414);
	}
	.tool-card[data-status='error'] {
		border-color: #8b1d1d;
	}
	.tool-card[data-status='done'] {
		border-color: #264d26;
	}
	.summary {
		width: 100%;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 8px;
		background: transparent;
		color: inherit;
		border: 0;
		cursor: pointer;
		text-align: left;
		font-family: inherit;
		font-size: inherit;
	}
	.summary:hover {
		background: rgba(255, 255, 255, 0.04);
	}
	.tool-name {
		flex: 1;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	}
	.indicator {
		width: 16px;
		display: inline-flex;
		justify-content: center;
	}
	.chev {
		opacity: 0.6;
	}
	.spinner {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		border: 2px solid #888;
		border-top-color: transparent;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	.detail {
		border-top: 1px solid var(--border, #2a2a2a);
		padding: 6px 8px;
	}
	.field {
		margin: 4px 0;
	}
	.label {
		font-size: 0.75rem;
		opacity: 0.6;
		display: block;
		margin-bottom: 2px;
	}
	pre {
		margin: 0;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 0.8rem;
	}
</style>
