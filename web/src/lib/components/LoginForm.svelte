<script lang="ts">
	type Props = { onSuccess: () => void };
	let { onSuccess }: Props = $props();

	let password = $state('');
	let busy = $state(false);
	let error: string | null = $state(null);

	async function submit(ev?: SubmitEvent) {
		ev?.preventDefault();
		if (busy || !password) return;
		busy = true;
		error = null;
		try {
			const res = await fetch('/api/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ password })
			});
			if (res.ok) {
				password = '';
				onSuccess();
				return;
			}
			if (res.status === 429) {
				const retry = res.headers.get('Retry-After') ?? '';
				error = retry
					? `Too many attempts — try again in ${retry}s.`
					: 'Too many attempts — try again shortly.';
				return;
			}
			if (res.status === 401) {
				error = 'Invalid password.';
				return;
			}
			error = `Login failed (HTTP ${res.status}).`;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			busy = false;
		}
	}
</script>

<div class="wrap">
	<aside class="brand-panel" aria-hidden="true">
		<div class="orb"></div>
		<div class="grid"></div>
		<div class="brand-stack">
			<div class="mark">
				<span class="claw"></span>
				<span class="claw"></span>
				<span class="claw"></span>
			</div>
			<h1 class="brand">picooraclaw</h1>
			<p class="tag">Oracle-backed autonomous agent. Streamed reasoning, durable memory.</p>
		</div>
		<div class="foot">
			<span class="status-dot"></span>
			<span class="foot-text">v1 · oracle free 23ai</span>
		</div>
	</aside>

	<section class="form-panel">
		<form class="card" onsubmit={submit}>
			<header class="form-head">
				<span class="eyebrow">Access agent</span>
				<h2>Sign in</h2>
				<p>Enter the password configured on the bridge.</p>
			</header>
			<label>
				<span>Password</span>
				<div class="input-wrap">
					<input
						type="password"
						autocomplete="current-password"
						bind:value={password}
						disabled={busy}
						required
						placeholder="••••••••"
					/>
				</div>
			</label>
			{#if error}
				<div class="error" role="alert">{error}</div>
			{/if}
			<button type="submit" disabled={busy || !password}>
				{#if busy}
					<span class="btn-spinner"></span>
					<span>Signing in</span>
				{:else}
					<span>Sign in</span>
					<svg
						viewBox="0 0 16 16"
						width="14"
						height="14"
						fill="none"
						stroke="currentColor"
						stroke-width="1.5"
						aria-hidden="true"
					>
						<path d="M2 8h11M9 4l4 4-4 4" stroke-linecap="round" stroke-linejoin="round" />
					</svg>
				{/if}
			</button>
			<p class="hint">Default dev password is <code>demo</code>.</p>
		</form>
	</section>
</div>

<style>
	.wrap {
		min-height: 100dvh;
		display: grid;
		grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr);
	}

	.brand-panel {
		position: relative;
		overflow: hidden;
		padding: 48px;
		background: linear-gradient(180deg, #0a0a0c 0%, #111112 100%);
		border-right: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		justify-content: space-between;
	}

	.orb {
		position: absolute;
		width: 420px;
		height: 420px;
		border-radius: 50%;
		left: -120px;
		top: -80px;
		background: radial-gradient(
			circle,
			rgba(245, 158, 11, 0.35) 0%,
			rgba(245, 158, 11, 0.08) 35%,
			transparent 70%
		);
		filter: blur(12px);
		animation: float 18s ease-in-out infinite;
		pointer-events: none;
	}

	@keyframes float {
		0%,
		100% {
			transform: translate(0, 0) scale(1);
		}
		50% {
			transform: translate(40px, 60px) scale(1.08);
		}
	}

	.grid {
		position: absolute;
		inset: 0;
		background-image:
			linear-gradient(var(--border) 1px, transparent 1px),
			linear-gradient(90deg, var(--border) 1px, transparent 1px);
		background-size: 56px 56px;
		mask-image: radial-gradient(ellipse at 30% 40%, black 10%, transparent 70%);
		opacity: 0.35;
		pointer-events: none;
	}

	.brand-stack {
		position: relative;
		z-index: 1;
		margin-top: auto;
	}

	.mark {
		display: inline-flex;
		gap: 6px;
		margin-bottom: 28px;
	}
	.claw {
		width: 3px;
		height: 28px;
		background: var(--accent);
		border-radius: 2px;
		transform-origin: bottom center;
	}
	.claw:nth-child(1) {
		transform: rotate(-18deg);
	}
	.claw:nth-child(2) {
		height: 36px;
	}
	.claw:nth-child(3) {
		transform: rotate(18deg);
	}

	.brand {
		margin: 0;
		font-size: clamp(2.25rem, 3.5vw, 3rem);
		font-weight: 500;
		letter-spacing: -0.04em;
		line-height: 1;
	}

	.tag {
		margin-top: 16px;
		max-width: 34ch;
		color: var(--fg-muted);
		font-size: 0.95rem;
		line-height: 1.55;
	}

	.foot {
		position: relative;
		z-index: 1;
		display: flex;
		align-items: center;
		gap: 8px;
		font-family: var(--font-mono);
		font-size: 0.75rem;
		color: var(--fg-subtle);
		letter-spacing: 0.04em;
	}

	.status-dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--ok);
		box-shadow: 0 0 10px var(--ok);
		animation: pulse 2.5s ease-in-out infinite;
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 0.55;
			transform: scale(1);
		}
		50% {
			opacity: 1;
			transform: scale(1.25);
		}
	}

	.form-panel {
		display: grid;
		place-items: center;
		padding: 48px 32px;
	}

	.card {
		width: 100%;
		max-width: 380px;
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.form-head {
		margin-bottom: 8px;
	}

	.eyebrow {
		font-family: var(--font-mono);
		font-size: 0.72rem;
		text-transform: uppercase;
		letter-spacing: 0.14em;
		color: var(--accent);
	}

	h2 {
		margin: 8px 0 6px;
		font-size: 1.75rem;
		font-weight: 500;
		letter-spacing: -0.02em;
	}

	.form-head p {
		margin: 0;
		color: var(--fg-muted);
		font-size: 0.9rem;
	}

	label {
		display: flex;
		flex-direction: column;
		gap: 8px;
		font-size: 0.8rem;
		color: var(--fg-muted);
		font-weight: 500;
	}

	.input-wrap {
		position: relative;
	}

	input {
		width: 100%;
		padding: 12px 14px;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		background: var(--surface);
		color: var(--fg);
		font: inherit;
		font-size: 0.95rem;
		transition: border-color 0.15s ease, background 0.15s ease;
	}
	input::placeholder {
		color: var(--fg-faint);
	}
	input:hover:not(:disabled) {
		border-color: var(--border-strong);
	}
	input:focus {
		outline: none;
		border-color: var(--accent);
		background: var(--surface-2);
		box-shadow: 0 0 0 3px var(--accent-soft);
	}

	button[type='submit'] {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 12px 16px;
		border-radius: var(--radius);
		border: 0;
		background: var(--accent);
		color: var(--accent-fg);
		font-weight: 600;
		font-size: 0.95rem;
		cursor: pointer;
		transition:
			transform 0.12s ease,
			background 0.15s ease,
			box-shadow 0.15s ease;
		box-shadow: 0 8px 24px -12px rgba(245, 158, 11, 0.55);
	}
	button[type='submit']:hover:not(:disabled) {
		background: var(--accent-hover);
	}
	button[type='submit']:active:not(:disabled) {
		transform: translateY(1px);
	}
	button[type='submit']:disabled {
		opacity: 0.5;
		cursor: not-allowed;
		box-shadow: none;
	}

	.btn-spinner {
		width: 12px;
		height: 12px;
		border-radius: 50%;
		border: 2px solid currentColor;
		border-top-color: transparent;
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.error {
		padding: 10px 12px;
		border-radius: var(--radius-sm);
		background: var(--err-soft);
		color: var(--err);
		font-size: 0.85rem;
	}

	.hint {
		margin: 0;
		font-size: 0.8rem;
		color: var(--fg-subtle);
	}
	.hint code {
		font-family: var(--font-mono);
		color: var(--fg-muted);
		background: var(--surface-2);
		padding: 1px 6px;
		border-radius: 4px;
		font-size: 0.78rem;
	}

	@media (max-width: 820px) {
		.wrap {
			grid-template-columns: 1fr;
		}
		.brand-panel {
			padding: 32px 28px 40px;
			border-right: 0;
			border-bottom: 1px solid var(--border);
			min-height: 38vh;
		}
		.form-panel {
			padding: 32px 24px 48px;
		}
		.brand {
			font-size: 2rem;
		}
	}
</style>
