<script module lang="ts">
	// An entry in the desktop reading outline (目次). Pure navigation over the
	// interpretation-plane read model — never a new fact, never a per-person axis.
	// For 議事録 the items stay in 発言順 (会議軸, ADR-000004): a flat ordered list
	// labelled by speaker, NOT a deduplicated speaker roster.
	export interface OutlineItem {
		/** id of the target element to scroll to (an in-page anchor, e.g. "sp-3"). */
		id: string;
		/** human-readable primary label (第N条 / 発言者名). Never an internal id. */
		label: string;
		/** optional secondary line (caption / 会派), shown dimmed. */
		sub?: string;
		/** 0 = top-level section, 1 = nested entry. Defaults to 0. */
		level?: number;
	}
</script>

<script lang="ts">
	interface Props {
		items: OutlineItem[];
		/** nav landmark label and visible heading. */
		heading?: string;
		/** count chip beside the heading (e.g. number of 条 / 発言). */
		count?: number | string;
	}
	const { items, heading = '目次', count }: Props = $props();

	// Scroll-spy: the section currently in view. Navigation itself is plain anchor
	// links (works without JS, shareable); this only drives the active highlight.
	let activeId = $state<string | null>(null);

	$effect(() => {
		// IntersectionObserver is the efficient way to follow the visible section
		// (no scroll-event thrash). Absent under SSR / jsdom — guard and no-op there.
		if (typeof IntersectionObserver === 'undefined') return;
		const targets = items
			.map((it) => document.getElementById(it.id))
			.filter((el): el is HTMLElement => el != null);
		if (targets.length === 0) return;

		const visible = new Set<string>();
		const io = new IntersectionObserver(
			(entries) => {
				for (const e of entries) {
					if (e.isIntersecting) visible.add(e.target.id);
					else visible.delete(e.target.id);
				}
				// Highlight the first item (document order) currently in the band.
				const first = items.find((it) => visible.has(it.id));
				if (first) activeId = first.id;
			},
			// Bias the active band to the upper viewport so the heading you are
			// reading is the highlighted one (and clear of the anchor offset).
			{ rootMargin: '-12% 0px -70% 0px', threshold: 0 }
		);
		for (const el of targets) io.observe(el);
		return () => io.disconnect();
	});
</script>

<nav class="outline" aria-label={heading}>
	<p class="head label">
		{heading}{#if count != null}<span class="cnt mono">{count}</span>{/if}
	</p>
	<ul>
		{#each items as it (it.id)}
			<li class="lv{it.level ?? 0}">
				<a
					href="#{it.id}"
					class="link"
					class:active={activeId === it.id}
					title={it.sub ?? it.label}
					aria-current={activeId === it.id ? 'location' : undefined}
				>
					<span class="lbl">{it.label}</span>
					{#if it.sub}<span class="sub">{it.sub}</span>{/if}
				</a>
			</li>
		{/each}
	</ul>
</nav>

<style>
	/* Desktop-only reading aid: it lives in the left gutter and never disturbs the
	   centred reading column. Hidden below --bp-xl (1200px) and on mobile, where the
	   back-to-top control is the only scroll affordance (per the design decision). */
	.outline {
		display: none;
	}
	@media (min-width: 75rem) {
		.outline {
			display: block;
			position: sticky;
			top: var(--s4);
			align-self: start;
			width: min(220px, 100%);
			max-height: calc(100vh - var(--s6));
			overflow-y: auto;
			padding-right: var(--s3);
			border-right: 1px solid var(--hairline);
			font-size: var(--fs-sm);
		}
	}
	.head {
		margin: 0 0 var(--s3);
	}
	.head .cnt {
		margin-left: 6px;
		color: var(--text-3);
	}
	ul {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	li {
		margin: 0;
	}
	.lv1 {
		padding-left: var(--s3);
	}
	.link {
		display: block;
		padding: 4px 8px;
		color: var(--text-2);
		text-decoration: none;
		line-height: 1.4;
		/* shape cue (left bar) carries the active state alongside colour — multi-coded
		   state per DESIGN_LANGUAGE §3; the bar is transparent until active. */
		border-left: 2px solid transparent;
		border-radius: 0 var(--r-sm) var(--r-sm) 0;
	}
	.link:hover {
		color: var(--text-1);
		background: var(--surface-raise);
	}
	.link.active {
		color: var(--accent);
		border-left-color: var(--accent);
		background: var(--surface-1);
	}
	.lbl {
		display: block;
	}
	.sub {
		display: block;
		color: var(--text-3);
		font-size: var(--fs-xs);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
