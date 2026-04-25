<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>picooraclaw</title>
</svelte:head>

{@render children()}

<style>
	:global(:root) {
		--bg: #09090b;
		--bg-deep: #050506;
		--surface: #0f0f11;
		--surface-2: #141417;
		--surface-3: #1b1b1f;
		--border: #27272a;
		--border-strong: #3f3f46;
		--fg: #fafafa;
		--fg-muted: #a1a1aa;
		--fg-subtle: #71717a;
		--fg-faint: #52525b;
		--accent: #f59e0b;
		--accent-hover: #fbbf24;
		--accent-soft: rgba(245, 158, 11, 0.14);
		--accent-ghost: rgba(245, 158, 11, 0.08);
		--accent-fg: #18181b;
		--ok: #34d399;
		--ok-soft: rgba(52, 211, 153, 0.16);
		--err: #f87171;
		--err-soft: rgba(248, 113, 113, 0.14);
		--radius-sm: 8px;
		--radius: 12px;
		--radius-lg: 18px;
		--radius-xl: 28px;
		--shadow-diff:
			0 30px 60px -30px rgb(0 0 0 / 0.8), 0 8px 20px -12px rgb(0 0 0 / 0.5);
		--shadow-inner: inset 0 1px 0 rgba(255, 255, 255, 0.04);
		--shadow-inner-strong: inset 0 1px 0 rgba(255, 255, 255, 0.06);
		--font-sans:
			'Geist', 'Satoshi', 'Inter var', system-ui, -apple-system, 'Segoe UI', sans-serif;
		--font-mono:
			'Geist Mono', 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace;

		color-scheme: dark;
	}

	:global(html, body) {
		margin: 0;
		padding: 0;
		background: var(--bg);
		color: var(--fg);
		font-family: var(--font-sans);
		font-size: 15px;
		line-height: 1.5;
		-webkit-font-smoothing: antialiased;
		-moz-osx-font-smoothing: grayscale;
		font-feature-settings: 'ss01', 'cv11';
		text-rendering: optimizeLegibility;
	}

	:global(body) {
		background:
			radial-gradient(
				1200px circle at 8% -10%,
				rgba(245, 158, 11, 0.08),
				transparent 55%
			),
			radial-gradient(
				900px circle at 95% 10%,
				rgba(59, 130, 246, 0.04),
				transparent 60%
			),
			var(--bg);
		min-height: 100dvh;
	}

	:global(*) {
		box-sizing: border-box;
	}

	:global(button) {
		font-family: inherit;
	}

	:global(::selection) {
		background: var(--accent-soft);
		color: var(--fg);
	}

	:global(:focus-visible) {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 4px;
	}

	:global(:focus:not(:focus-visible)) {
		outline: none;
	}

	:global(::-webkit-scrollbar) {
		width: 10px;
		height: 10px;
	}
	:global(::-webkit-scrollbar-track) {
		background: transparent;
	}
	:global(::-webkit-scrollbar-thumb) {
		background: var(--border);
		border-radius: 999px;
		border: 2px solid var(--bg);
	}
	:global(::-webkit-scrollbar-thumb:hover) {
		background: var(--border-strong);
	}
</style>
