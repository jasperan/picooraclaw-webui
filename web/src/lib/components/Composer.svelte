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
		autogrow();
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

	function autogrow() {
		if (!textareaEl) return;
		textareaEl.style.height = 'auto';
		const next = Math.min(textareaEl.scrollHeight, 220);
		textareaEl.style.height = next + 'px';
	}
</script>

<form class="composer" onsubmit={onFormSubmit}>
	<div class="shell">
		<textarea
			bind:this={textareaEl}
			bind:value
			oninput={autogrow}
			onkeydown={onKeydown}
			placeholder="Message picooraclaw…"
			rows="1"
			{disabled}
			aria-label="Message"
		></textarea>
		<div class="controls">
			<div class="hint">
				<kbd>↵</kbd>
				<span>send</span>
				<span class="sep">·</span>
				<kbd>⇧↵</kbd>
				<span>newline</span>
			</div>
			<button type="submit" class="send" disabled={disabled || !value.trim()} aria-label="Send">
				<svg
					viewBox="0 0 16 16"
					width="14"
					height="14"
					fill="none"
					stroke="currentColor"
					stroke-width="1.75"
					aria-hidden="true"
				>
					<path d="M2 8h11M9 4l4 4-4 4" stroke-linecap="round" stroke-linejoin="round" />
				</svg>
			</button>
		</div>
	</div>
</form>

<style>
	.composer {
		padding: 14px 20px 18px;
		display: flex;
		justify-content: center;
	}

	.shell {
		position: relative;
		width: 100%;
		max-width: 880px;
		border-radius: var(--radius-xl);
		border: 1px solid var(--border);
		background:
			linear-gradient(180deg, rgba(255, 255, 255, 0.02), rgba(255, 255, 255, 0)),
			rgba(15, 15, 17, 0.72);
		backdrop-filter: blur(14px) saturate(140%);
		-webkit-backdrop-filter: blur(14px) saturate(140%);
		box-shadow:
			var(--shadow-diff),
			var(--shadow-inner-strong);
		padding: 14px 16px 10px;
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}

	.shell:focus-within {
		border-color: var(--border-strong);
		box-shadow:
			var(--shadow-diff),
			var(--shadow-inner-strong),
			0 0 0 3px var(--accent-ghost);
	}

	textarea {
		width: 100%;
		min-height: 28px;
		max-height: 220px;
		resize: none;
		padding: 4px 6px;
		border: 0;
		background: transparent;
		color: var(--fg);
		font: inherit;
		font-family: var(--font-sans);
		font-size: 0.95rem;
		line-height: 1.5;
		outline: none;
	}
	textarea::placeholder {
		color: var(--fg-faint);
	}
	textarea:disabled {
		opacity: 0.6;
	}

	.controls {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-top: 6px;
	}

	.hint {
		display: flex;
		align-items: center;
		gap: 6px;
		font-family: var(--font-mono);
		font-size: 0.68rem;
		color: var(--fg-faint);
		letter-spacing: 0.04em;
	}
	.hint .sep {
		opacity: 0.5;
	}
	kbd {
		display: inline-block;
		padding: 1px 5px;
		border-radius: 4px;
		border: 1px solid var(--border);
		background: var(--surface-2);
		color: var(--fg-subtle);
		font-family: var(--font-mono);
		font-size: 0.68rem;
	}

	.send {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 999px;
		background: var(--accent);
		color: var(--accent-fg);
		border: 0;
		cursor: pointer;
		transition: transform 0.12s ease, background 0.15s ease;
		box-shadow: 0 6px 18px -10px rgba(245, 158, 11, 0.6);
	}
	.send:hover:not(:disabled) {
		background: var(--accent-hover);
	}
	.send:active:not(:disabled) {
		transform: translateY(1px) scale(0.97);
	}
	.send:disabled {
		background: var(--surface-3);
		color: var(--fg-faint);
		cursor: not-allowed;
		box-shadow: none;
	}
</style>
