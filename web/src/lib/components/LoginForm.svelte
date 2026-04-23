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
	<form class="card" onsubmit={submit}>
		<h1>picooraclaw</h1>
		<label>
			<span>Password</span>
			<input
				type="password"
				autocomplete="current-password"
				bind:value={password}
				disabled={busy}
				required
			/>
		</label>
		{#if error}
			<div class="error" role="alert">{error}</div>
		{/if}
		<button type="submit" disabled={busy || !password}>{busy ? 'Signing in…' : 'Sign in'}</button>
	</form>
</div>

<style>
	.wrap {
		min-height: 100vh;
		display: grid;
		place-items: center;
		padding: 20px;
		background: var(--bg, #0a0a0a);
		color: var(--fg, #e7e7e7);
	}
	.card {
		width: 100%;
		max-width: 360px;
		background: var(--card-bg, #141414);
		border: 1px solid var(--border, #2a2a2a);
		border-radius: 10px;
		padding: 22px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	h1 {
		margin: 0 0 4px 0;
		font-size: 1.2rem;
		font-weight: 600;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 0.85rem;
	}
	input {
		padding: 10px;
		border-radius: 6px;
		border: 1px solid var(--border, #2a2a2a);
		background: var(--input-bg, #0f0f0f);
		color: inherit;
		font: inherit;
	}
	input:focus {
		outline: 2px solid var(--accent, #3b82f6);
		outline-offset: -1px;
	}
	button {
		padding: 10px;
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
	.error {
		color: #ff6b6b;
		font-size: 0.85rem;
	}
</style>
