<script lang="ts">
	// Self-hosted fonts (DESIGN_LANGUAGE §11 / ADR-000020): no third-party request, so no
	// visitor-IP leak (passive sentinel, principle #1). Weight- and subset-controlled to keep
	// the CJK payload down; @fontsource ships font-display:swap so first paint never blocks.
	// IBM Plex Mono — numbers / IDs / hashes / labels (latin subset, 400/500/600).
	import '@fontsource/ibm-plex-mono/latin-400.css';
	import '@fontsource/ibm-plex-mono/latin-500.css';
	import '@fontsource/ibm-plex-mono/latin-600.css';
	// IBM Plex Sans — latin runs inside sans body (latin subset, 400/500/600/700).
	import '@fontsource/ibm-plex-sans/latin-400.css';
	import '@fontsource/ibm-plex-sans/latin-500.css';
	import '@fontsource/ibm-plex-sans/latin-600.css';
	import '@fontsource/ibm-plex-sans/latin-700.css';
	// IBM Plex Sans JP — Japanese body / UI (japanese subset, 400/500/600/700).
	import '@fontsource/ibm-plex-sans-jp/japanese-400.css';
	import '@fontsource/ibm-plex-sans-jp/japanese-500.css';
	import '@fontsource/ibm-plex-sans-jp/japanese-600.css';
	import '@fontsource/ibm-plex-sans-jp/japanese-700.css';
	// Zen Old Mincho — editorial titles ONLY: Display / page H1 / record titles (600/700,
	// japanese + latin subsets). System mincho is the fallback until this loads (--font-serif).
	import '@fontsource/zen-old-mincho/japanese-600.css';
	import '@fontsource/zen-old-mincho/japanese-700.css';
	import '@fontsource/zen-old-mincho/latin-600.css';
	import '@fontsource/zen-old-mincho/latin-700.css';

	import '$lib/styles/tokens.css';
	import favicon from '$lib/assets/favicon.png';
	import Masthead from '$lib/components/Masthead.svelte';
	import type { Snippet } from 'svelte';
	import type { LayoutData } from './$types';

	let { children, data }: { children: Snippet; data: LayoutData } = $props();

	// Map the masthead provenance into the component's props. Coverage is always real
	// (control.watch count); the checkpoint lights up only once the generator (ADR-000019)
	// has written one. ▸検証 points at the public signed-checkpoint feed (passive exposure).
	const m = $derived(data?.masthead ?? null);
	const coverage = $derived(m?.watchCount != null ? Number(m.watchCount) : undefined);
	const checkpoint = $derived(
		m?.hasCheckpoint && m.checkpoint
			? {
					seq: Number(m.checkpoint.throughSeq ?? 0),
					observedAt: m.checkpoint.recordedAt,
					verifyHref: '/checkpoints'
				}
			: undefined
	);
</script>

<svelte:head>
	<link rel="icon" type="image/png" href={favicon} />
</svelte:head>

<a class="skip" href="#main">本文へスキップ</a>
<Masthead {coverage} {checkpoint} />
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
