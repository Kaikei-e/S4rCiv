<script lang="ts">
	// "Back to top" control (NN/g guidelines): one per page, fixed bottom-right,
	// labelled (not icon-only), revealed only after the reader is well past the fold.
	// It is a plain in-page anchor to #main — the same target as the skip link — so it
	// works without JS and lands keyboard focus correctly. JS only toggles visibility on
	// scroll; the smooth scroll + reduced-motion handling live globally in tokens.css.
	let shown = $state(false);

	$effect(() => {
		const onScroll = () => {
			// Reveal once the reader is ~1.5 viewports down, so the control never
			// clutters short pages (NN/g: only worthwhile well past the fold).
			shown = window.scrollY > window.innerHeight * 1.5;
		};
		onScroll();
		window.addEventListener('scroll', onScroll, { passive: true });
		return () => window.removeEventListener('scroll', onScroll);
	});
</script>

<a
	class="btt"
	class:shown
	href="#main"
	aria-hidden={!shown}
	tabindex={shown ? undefined : -1}
>
	<span class="arrow" aria-hidden="true">↑</span> 先頭へ
</a>

<style>
	/* Ledger translation of the conventional floating button: a bordered chip, no
	   shadow (hierarchy by hairline + surface lift), sharp corners, mono label, and
	   colour+glyph+label triple-coding — per DESIGN_LANGUAGE (not a round Material FAB). */
	.btt {
		position: fixed;
		right: var(--s4);
		bottom: var(--s4);
		z-index: 50;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		min-height: 44px; /* WCAG 2.5.5 touch target on both pointers */
		padding: 0 14px;
		font-family: var(--font-mono);
		font-size: var(--fs-sm);
		color: var(--text-1);
		background: var(--surface-3);
		border: 1px solid var(--hairline-2);
		border-radius: var(--r);
		text-decoration: none;
		/* fade in on reveal; disabled for reduced-motion users by the global rule */
		opacity: 0;
		visibility: hidden;
		transition:
			opacity 200ms ease,
			visibility 200ms ease;
	}
	.btt.shown {
		opacity: 1;
		visibility: visible;
	}
	.btt:hover {
		color: var(--accent);
		border-color: var(--hairline-3);
		background: var(--surface-raise);
	}
	.arrow {
		font-size: 15px;
		line-height: 1;
	}
</style>
