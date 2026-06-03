<script lang="ts">
	import type { Attribution } from '$lib/types';

	let { attr }: { attr?: Attribution } = $props();

	// Chain LINKAGE, not a verification result (ADR-000007 / E1): we show that the
	// record sits in the append-only hash chain (#seq ← prev) and a short log_hash,
	// labelled "(未検証)". A green "verified" state waits for a real verification job.
	const shortHash = (h?: string) => (h ? h.slice(0, 8) + '…' : '');
	const seqNum = (s?: string) => (s ? Number(s) : undefined);
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
	<span class="sep" aria-hidden="true">·</span>
	<span class="mono chain" title="append-only ハッシュ連鎖の連結。完全性検証は今後（ADR-000007）。">
		連鎖 #{seqNum(attr?.observationSeq) ?? '—'}{#if attr?.prevLogHash}
			← prev{/if}
		{#if attr?.logHash}<span class="hash">{shortHash(attr.logHash)}</span>{/if}
		<span class="unverified">(未検証)</span>
	</span>
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
	.fetched,
	.chain {
		font-size: 11px;
	}
	.sep {
		color: var(--hairline-2);
	}
	.hash {
		color: var(--text-3);
	}
	.unverified {
		color: var(--st-caution-t);
		opacity: 0.85;
	}
</style>
