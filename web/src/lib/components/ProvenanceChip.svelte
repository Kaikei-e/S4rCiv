<script lang="ts">
	import type { Attribution } from '$lib/types';

	interface Props {
		attr?: Attribution;
		// Deep-link to the per-case verification panel, anchored at this record
		// (#verify-<seq>). When set, 記録#seq becomes a link into the panel; otherwise
		// it is a plain citation handle. ADR-000014 §2: the chip is provenance ONLY —
		// the truncated hash and the "(未検証)" wording moved to the panel, because
		// integrity is a per-chain/checkpoint property, not a per-record badge. #seq is
		// a stable citation handle into the panel, never a per-record verdict.
		verifyHref?: string;
	}
	const { attr, verifyHref }: Props = $props();

	const seq = $derived(attr?.observationSeq ? Number(attr.observationSeq) : undefined);
</script>

<div class="prov">
	{#if attr?.permalink}
		<a class="src" href={attr.permalink} target="_blank" rel="noopener noreferrer external"
			>出典 {attr.source ?? ''} ↗</a
		>
	{:else}
		<span class="src">出典 {attr?.source ?? '—'}</span>
	{/if}
	<span class="sep" aria-hidden="true">·</span>
	<span class="mono fetched">最終取得 {attr?.fetchedAt ?? '—'}</span>
	{#if seq !== undefined}
		<span class="sep" aria-hidden="true">·</span>
		{#if verifyHref}
			<a class="rec" href={verifyHref} title="この記録が記録どおりか、お使いの端末で確かめられます"
				>記録 #{seq} ↗</a
			>
		{:else}
			<span class="rec mono" title="この記録の通し番号">記録 #{seq}</span>
		{/if}
	{/if}
</div>

<style>
	.prov {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0 8px;
		font-size: 12px;
		color: var(--text-3);
	}
	.src {
		color: var(--text-2);
	}
	.fetched {
		font-size: 11px;
	}
	.sep {
		color: var(--hairline-2);
	}
	.rec {
		font-size: 11px;
		color: var(--text-2);
	}
	a.rec:hover {
		color: var(--accent);
		text-decoration: underline;
	}
</style>
