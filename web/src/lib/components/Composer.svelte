<script lang="ts">
	type Props = {
		onSend: (text: string) => void;
		disabled?: boolean;
	};
	let { onSend, disabled = false }: Props = $props();

	let value = $state('');
	let textareaEl: HTMLTextAreaElement | undefined = $state();

	function submit() {
		const text = value.trim();
		if (!text || disabled) return;
		onSend(text);
		value = '';
		textareaEl?.focus();
	}

	function onKeydown(ev: KeyboardEvent) {
		if (ev.key === 'Enter' && !ev.shiftKey) {
			ev.preventDefault();
			submit();
		}
	}

	function onFormSubmit(ev: SubmitEvent) {
		ev.preventDefault();
		submit();
	}
</script>

<form class="composer" onsubmit={onFormSubmit}>
	<textarea
		bind:this={textareaEl}
		bind:value
		onkeydown={onKeydown}
		placeholder="Send a message…"
		rows="2"
		{disabled}
	></textarea>
	<button type="submit" disabled={disabled || !value.trim()}>Send</button>
</form>

<style>
	.composer {
		display: flex;
		gap: 8px;
		padding: 10px;
		border-top: 1px solid var(--border, #2a2a2a);
		background: var(--composer-bg, #0f0f0f);
	}
	textarea {
		flex: 1;
		resize: vertical;
		min-height: 42px;
		max-height: 200px;
		padding: 8px 10px;
		border-radius: 6px;
		border: 1px solid var(--border, #2a2a2a);
		background: var(--input-bg, #111);
		color: inherit;
		font: inherit;
		font-family: inherit;
	}
	textarea:focus {
		outline: 2px solid var(--accent, #3b82f6);
		outline-offset: -1px;
	}
	button {
		padding: 0 16px;
		border-radius: 6px;
		border: 0;
		background: var(--accent, #3b82f6);
		color: white;
		font-weight: 600;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
