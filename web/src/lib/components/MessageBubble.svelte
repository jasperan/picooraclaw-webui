<script lang="ts">
	import type { Message } from '$lib/stores/messages';
	import ToolCallCard from './ToolCallCard.svelte';

	let { msg }: { msg: Message } = $props();

	function timeStr(ts: number): string {
		if (!ts) return '';
		const d = new Date(ts);
		return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}
</script>

<div class="row" data-role={msg.role}>
	<div class="avatar" aria-hidden="true">
		{#if msg.role === 'user'}
			<span class="initial">U</span>
		{:else}
			<span class="mark">
				<span class="claw"></span>
				<span class="claw"></span>
				<span class="claw"></span>
			</span>
		{/if}
	</div>
	<div class="bubble" data-role={msg.role} data-streaming={msg.streaming}>
		<div class="meta">
			<span class="author">{msg.role === 'user' ? 'You' : 'picooraclaw'}</span>
			{#if msg.streaming}
				<span class="tag">thinking</span>
			{/if}
			<span class="time">{timeStr(msg.ts)}</span>
		</div>

		{#if msg.text}
			<div class="text">
				{msg.text}{#if msg.streaming}<span class="cursor" aria-hidden="true"></span>{/if}
			</div>
		{:else if msg.streaming && msg.toolCalls.length === 0}
			<div class="text streaming-placeholder">
				<span class="dots" aria-label="thinking">
					<span></span>
					<span></span>
					<span></span>
				</span>
			</div>
		{/if}

		{#if msg.toolCalls.length > 0}
			<div class="tools">
				{#each msg.toolCalls as call (call.id)}
					<ToolCallCard {call} />
				{/each}
			</div>
		{/if}

		{#if msg.error}
			<div class="error" role="alert">
				<svg
					viewBox="0 0 16 16"
					width="12"
					height="12"
					fill="none"
					stroke="currentColor"
					stroke-width="1.75"
					aria-hidden="true"
				>
					<circle cx="8" cy="8" r="6.25" />
					<path d="M8 5v3.5M8 10.75v.25" stroke-linecap="round" />
				</svg>
				<span>{msg.error}</span>
			</div>
		{/if}
	</div>
</div>

<style>
	.row {
		display: grid;
		grid-template-columns: 32px 1fr;
		gap: 14px;
		padding: 18px 0;
		max-width: 860px;
		width: 100%;
		margin: 0 auto;
	}

	.avatar {
		width: 32px;
		height: 32px;
		border-radius: 10px;
		display: grid;
		place-items: center;
		border: 1px solid var(--border);
		background: var(--surface-2);
		color: var(--fg-muted);
		font-family: var(--font-mono);
		font-size: 0.78rem;
		font-weight: 500;
		flex-shrink: 0;
	}

	.row[data-role='user'] .avatar {
		background: var(--surface-3);
		color: var(--fg);
	}

	.mark {
		display: inline-flex;
		gap: 2px;
		align-items: flex-end;
	}
	.claw {
		width: 2px;
		background: var(--accent);
		border-radius: 1px;
	}
	.claw:nth-child(1) {
		height: 10px;
		transform: rotate(-18deg) translateY(1px);
	}
	.claw:nth-child(2) {
		height: 13px;
	}
	.claw:nth-child(3) {
		height: 10px;
		transform: rotate(18deg) translateY(1px);
	}

	.bubble {
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.meta {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: 0.8rem;
		color: var(--fg-subtle);
	}
	.author {
		color: var(--fg);
		font-weight: 600;
		letter-spacing: -0.005em;
	}
	.time {
		font-family: var(--font-mono);
		font-size: 0.7rem;
		color: var(--fg-faint);
		margin-left: auto;
	}

	.tag {
		font-family: var(--font-mono);
		font-size: 0.65rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--accent);
		background: var(--accent-soft);
		padding: 1px 7px;
		border-radius: 999px;
	}

	.text {
		white-space: pre-wrap;
		word-break: break-word;
		line-height: 1.65;
		color: var(--fg);
		font-size: 0.97rem;
	}

	.row[data-role='user'] .text {
		color: var(--fg);
	}

	.cursor {
		display: inline-block;
		width: 7px;
		height: 1.05em;
		margin-left: 2px;
		vertical-align: text-bottom;
		background: var(--accent);
		border-radius: 1px;
		animation: blink 1.1s steps(2) infinite;
	}

	@keyframes blink {
		50% {
			opacity: 0;
		}
	}

	.streaming-placeholder {
		opacity: 0.85;
	}

	.dots {
		display: inline-flex;
		gap: 4px;
		align-items: center;
	}
	.dots span {
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: var(--fg-subtle);
		animation: dot 1.2s ease-in-out infinite;
	}
	.dots span:nth-child(2) {
		animation-delay: 0.15s;
	}
	.dots span:nth-child(3) {
		animation-delay: 0.3s;
	}
	@keyframes dot {
		0%,
		100% {
			opacity: 0.35;
			transform: translateY(0);
		}
		50% {
			opacity: 1;
			transform: translateY(-2px);
		}
	}

	.tools {
		margin-top: 6px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.error {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		border-radius: var(--radius-sm);
		background: var(--err-soft);
		color: var(--err);
		font-size: 0.85rem;
		width: fit-content;
	}

	@media (max-width: 680px) {
		.row {
			grid-template-columns: 28px 1fr;
			gap: 10px;
			padding: 14px 0;
		}
		.avatar {
			width: 28px;
			height: 28px;
		}
	}
</style>
