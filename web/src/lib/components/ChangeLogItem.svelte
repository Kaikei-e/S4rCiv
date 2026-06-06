<script lang="ts">
	import type { TimelineItem } from '$lib/types';
	import ProvenanceChip from './ProvenanceChip.svelte';
	import { toJstMinute } from '$lib/time';

	let { item }: { item: TimelineItem } = $props();

	// event_type → state node (color + glyph + label), per DESIGN_LANGUAGE §3.3.
	// Color is for STATE only; never a value judgment (§1, §5-C).
	type Status = { glyph: string; label: string; color: string; text: string };
	const STATUS: Record<string, Status> = {
		ResourceObserved: { glyph: '◉', label: '観測', color: 'var(--st-info)', text: 'var(--st-info-t)' },
		ResourceChanged: { glyph: 'Δ', label: '変化', color: 'var(--st-changed)', text: 'var(--st-changed-t)' },
		ResourceVanished: { glyph: '⊘', label: '消失', color: 'var(--st-critical)', text: 'var(--st-critical-t)' },
		ResourceRestored: { glyph: '●', label: '復活', color: 'var(--st-nominal)', text: 'var(--st-nominal-t)' }
	};
	const status = $derived(STATUS[item.eventType ?? ''] ?? STATUS.ResourceObserved);

	// Internal detail route (built next); external 出典 lives in the ProvenanceChip.
	const detailHref = $derived(
		item.lawId ? `/laws/${item.lawId}` : item.issueId ? `/meetings/${item.issueId}` : undefined
	);

	// observedAt is RFC3339 UTC; show it in JST (ADR-000018). The <time datetime> below
	// keeps the raw UTC ISO for machines; the visible text is the JST-converted value.
	const observedDay = $derived(toJstMinute(item.observedAt));
	const hasCounts = $derived(
		(item.nodesAdded ?? 0) + (item.nodesDeleted ?? 0) + (item.nodesModified ?? 0) > 0
	);
	const isSubstantive = $derived(item.classification === 'substantive');
</script>

<article class="item">
	<time class="mono time" datetime={item.observedAt}>{observedDay}</time>

	<div class="node" style="--c: {status.color}; --t: {status.text}" aria-hidden="true">
		<span class="dot">{status.glyph}</span>
	</div>

	<div class="body">
		<div class="headline">
			<span class="label state" style="color: {status.text}">{status.label}</span>
			{#if detailHref}
				<a class="title" href={detailHref}>{item.title || item.streamId}</a>
			{:else}
				<span class="title">{item.title || item.streamId}</span>
			{/if}
		</div>

		{#if item.subtitle}
			<p class="subtitle">{item.subtitle}</p>
		{/if}

		<div class="chips">
			{#if item.classification}
				<!-- Classification is a heuristic, shown as an UNREVIEWED auto-result (E4 / §6):
				     never presented as an established judgment. -->
				<span class="chip" class:substantive={isSubstantive}>
					{isSubstantive ? '実質的変更' : '事務的変更'} · 自動分類(未レビュー)
					{#if item.classConfidence}<span class="conf">確信度 {item.classConfidence}</span>{/if}
				</span>
			{/if}

			{#if hasCounts}
				<span class="chip counts mono" title="構造差分の件数。本文は詳細ビューで文脈付き表示（§7）。">
					{#if item.nodesAdded}+{item.nodesAdded}{/if}
					{#if item.nodesDeleted}−{item.nodesDeleted}{/if}
					{#if item.nodesModified}~{item.nodesModified}{/if}
				</span>
			{/if}

			{#if item.featuredVoteEventId}
				<span class="chip">採決を含む</span>
			{/if}

			{#if item.wasOcr}
				<!-- Low-confidence / OCR: marked, not hidden (E3 / §6); links to source for check. -->
				<span class="chip caution">▲ OCR由来・要確認</span>
			{/if}
		</div>

		<ProvenanceChip attr={item.attribution} />
	</div>
</article>

<style>
	.item {
		display: grid;
		grid-template-columns: 8.5em 1.5em 1fr;
		grid-template-areas: 'time node body';
		gap: 12px;
		padding: 14px 0;
		border-bottom: 1px solid var(--hairline);
	}
	.time {
		grid-area: time;
		font-size: 12px;
		color: var(--text-3);
		padding-top: 2px;
		white-space: nowrap;
	}
	.node {
		grid-area: node;
		display: flex;
		justify-content: center;
		padding-top: 1px;
	}
	/* Narrow container (mirrors --bp-sm 30rem): the status node + timestamp collapse
	   to a top meta row; the body spans full width below. Multi-encoding (colour +
	   glyph + label) is preserved — DESIGN_LANGUAGE §9.2. */
	@container timeline (max-width: 30rem) {
		.item {
			grid-template-columns: auto 1fr;
			grid-template-areas:
				'node time'
				'body body';
			gap: 4px 8px;
			align-items: center;
		}
		.node {
			justify-content: flex-start;
			padding-top: 0;
		}
		.time {
			padding-top: 0;
		}
	}
	.dot {
		color: var(--t);
		font-size: 14px;
		line-height: 1.2;
	}
	.body {
		grid-area: body;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.headline {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 8px;
	}
	.state {
		font-size: 11px;
	}
	/* The row title IS the record title (法令名・会議録名), so it carries the serif display
	   face — the editorial accent of the ledger (DESIGN_LANGUAGE §4.1). */
	.title {
		font-family: var(--font-display);
		font-size: 16px;
		font-weight: 700;
		color: var(--text-1);
		text-decoration: none;
	}
	a.title:hover {
		color: var(--accent);
		text-decoration: underline;
	}
	.subtitle {
		margin: 0;
		font-size: 13px;
		color: var(--text-2);
	}
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 11px;
		padding: 2px 8px;
		border-radius: var(--r-sm);
		border: 1px solid var(--hairline-2);
		color: var(--text-2);
		background: var(--surface-2);
	}
	/* Substantive changes are the central status color (amber), NOT an alarm (§1-5). */
	.chip.substantive {
		color: var(--st-changed-t);
		border-color: color-mix(in srgb, var(--st-changed) 40%, transparent);
		background: color-mix(in srgb, var(--st-changed) 12%, transparent);
	}
	.chip.caution {
		color: var(--st-caution-t);
		border-color: color-mix(in srgb, var(--st-caution) 40%, transparent);
		background: color-mix(in srgb, var(--st-caution) 12%, transparent);
	}
	.chip.counts {
		letter-spacing: 0.04em;
	}
	.conf {
		color: var(--text-3);
	}
</style>
