<script lang="ts">
	import type { Message } from '$lib/stores/messages';
	import ToolCallCard from './ToolCallCard.svelte';

	let { msg }: { msg: Message } = $props();
</script>

<div class="bubble" data-role={msg.role} data-streaming={msg.streaming}>
	{#if msg.text}
		<div class="text">
			{msg.text}{#if msg.streaming}<span class="cursor" aria-hidden="true">▊</span>{/if}
		</div>
	{:else if msg.streaming && msg.toolCalls.length === 0}
		<div class="text streaming-placeholder">
			<span class="cursor">▊</span>
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
		<div class="error">{msg.error}</div>
	{/if}
</div>

<style>
	.bubble {
		padding: 10px 12px;
		border-radius: 8px;
		margin: 8px 0;
		max-width: 80ch;
	}
	.bubble[data-role='user'] {
		background: var(--user-bg, #1f2a44);
		align-self: flex-end;
		margin-left: auto;
	}
	.bubble[data-role='assistant'] {
		background: var(--assistant-bg, #161616);
		align-self: flex-start;
	}
	.text {
		white-space: pre-wrap;
		word-break: break-word;
		line-height: 1.5;
	}
	.cursor {
		display: inline-block;
		margin-left: 2px;
		animation: blink 1s steps(2) infinite;
		opacity: 0.7;
	}
	@keyframes blink {
		50% {
			opacity: 0;
		}
	}
	.tools {
		margin-top: 6px;
	}
	.error {
		margin-top: 6px;
		color: #ff6b6b;
		font-size: 0.85rem;
	}
	.streaming-placeholder {
		opacity: 0.7;
	}
</style>
