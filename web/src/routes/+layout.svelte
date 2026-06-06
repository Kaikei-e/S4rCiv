<script lang="ts">
	import '$lib/styles/tokens.css';
	import favicon from '$lib/assets/favicon.png';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" type="image/png" href={favicon} />
</svelte:head>

<a class="skip" href="#main">本文へスキップ</a>
{@render children()}

<style>
	/* Keyboard skip link (WCAG 2.4.1 / DESIGN_LANGUAGE §8). */
	.skip {
		position: absolute;
		left: -9999px;
		top: 0;
		z-index: 100;
		padding: 8px 12px;
		background: var(--surface-3);
		color: var(--text-1);
		border-radius: var(--r-sm);
	}
	.skip:focus {
		left: 8px;
		top: 8px;
	}
</style>
