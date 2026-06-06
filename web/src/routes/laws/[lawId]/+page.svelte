<script lang="ts">
	import type { PageData } from './$types';
	import ProvenanceChip from '$lib/components/ProvenanceChip.svelte';
	import LawChangeBlock from '$lib/components/LawChangeBlock.svelte';
	import VerificationPanel from '$lib/components/VerificationPanel.svelte';

	let { data }: { data: PageData } = $props();
	const law = $derived(data.law);
	// Deep-link the provenance chip to this record's row in the verification panel.
	const verifyHref = $derived(
		law.attribution?.observationSeq ? `#verify-${law.attribution.observationSeq}` : undefined
	);

	// Indent depth. 号の細分 (subitem) share one flat node_type, so the イ→(1)→(ア) nesting
	// depth is read from the eId (each level adds one __subitem segment).
	const indent = (n: { nodeType?: string; eid?: string }) => {
		if (n.nodeType === 'article') return 0;
		if (n.nodeType === 'paragraph') return 1;
		if (n.nodeType === 'item') return 2;
		if (n.nodeType === 'subitem') return 2 + (n.eid?.match(/__subitem/g)?.length ?? 1);
		return 2;
	};
	const sortedNodes = $derived([...data.nodes].sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0)));
	const repealed = $derived(law.repealStatus && law.repealStatus !== 'None');
</script>

<svelte:head><title>{law.lawTitle ?? law.lawId} — S4RCIV</title></svelte:head>

<main id="main" class="wrap">
	<a class="back" href="/">← タイムライン</a>

	<header class="lhead">
		<span class="label">法令 {law.lawType ?? ''}</span>
		<h1>{law.lawTitle ?? law.lawId}</h1>
		<p class="meta mono">
			{law.lawNum ?? ''}{#if law.promulgationDate} · 公布 {law.promulgationDate}{/if}{#if law.amendmentEnforcementDate}
				· 施行 {law.amendmentEnforcementDate}{/if}
		</p>
		{#if repealed}<p class="repeal">⊘ {law.repealStatus} {law.repealDate ?? ''}</p>{/if}
		<ProvenanceChip attr={law.attribution} {verifyHref} />
	</header>

	<section aria-label="変更履歴">
		<h2 class="label">変更履歴</h2>
		{#if data.changes.length === 0}
			<p class="empty">記録された変更はまだありません（初回観測のみ）。法令を2回以上観測すると、構造差分がここに条文の文脈付きで表示されます。</p>
		{:else}
			{#each data.changes as change (change.observationSeq)}
				<LawChangeBlock {change} nodes={data.nodes} />
			{/each}
		{/if}
	</section>

	<section aria-label="現行全文">
		<h2 class="label">現行全文 <span class="cnt mono">{sortedNodes.length} ノード</span></h2>
		<div class="tree">
			{#each sortedNodes as n (n.eid)}
				<div class="node" style="--lv: {indent(n)}" class:suppl={n.isSuppl}>
					{#if n.num || n.caption}
						<span class="num mono"
							>{n.nodeType === 'article' ? '第' + n.num + '条' : n.num ?? ''}</span
						>{#if n.caption}<span class="cap">{n.caption}</span>{/if}
					{/if}
					{#if n.sentenceText}<span class="txt">{n.sentenceText}</span>{/if}
				</div>
			{/each}
		</div>
		{#if law.attribution?.permalink}
			<a class="ext" href={law.attribution.permalink} target="_blank" rel="noopener noreferrer external"
				>e-Gov で原文を見る ↗</a
			>
		{/if}
	</section>

	{#if data.verification}
		<VerificationPanel data={data.verification} />
	{/if}
</main>

<style>
	.wrap {
		max-width: 880px;
		margin: 0 auto;
		padding: 24px;
	}
	@media (max-width: 30rem) {
		.wrap {
			padding: 16px;
		}
	}
	.back {
		font-size: 13px;
		text-decoration: none;
	}
	.lhead {
		margin: 12px 0 24px;
		padding-bottom: 16px;
		border-bottom: 1px solid var(--hairline-2);
	}
	.lhead h1 {
		font-size: 21px;
		margin: 6px 0;
	}
	.meta {
		font-size: 13px;
		color: var(--text-2);
		margin: 0 0 8px;
	}
	.repeal {
		color: var(--st-critical-t);
		font-size: 13px;
		margin: 0 0 8px;
	}
	h2.label {
		display: block;
		margin: 28px 0 12px;
	}
	.cnt,
	.cnt {
		color: var(--text-3);
	}
	.empty {
		color: var(--text-2);
		background: var(--surface-1);
		border: 1px solid var(--hairline);
		border-radius: var(--r);
		padding: 16px;
		font-size: 14px;
	}
	.tree {
		background: var(--surface-1);
		border: 1px solid var(--hairline-2);
		border-radius: var(--r);
		padding: 12px 16px;
	}
	.node {
		padding: 4px 0;
		padding-left: calc(var(--lv, 0) * 1.5em);
		font-size: 14px;
		line-height: 1.7;
	}
	.node.suppl {
		border-left: 2px solid var(--hairline-2);
	}
	.num {
		color: var(--accent);
		margin-right: 8px;
	}
	.cap {
		color: var(--text-2);
		margin-right: 8px;
	}
	.ext {
		display: inline-block;
		margin-top: 14px;
		font-size: 13px;
	}
</style>
