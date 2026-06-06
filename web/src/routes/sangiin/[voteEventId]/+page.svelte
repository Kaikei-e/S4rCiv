<script lang="ts">
	import type { PageData } from './$types';
	import type { SangiinPrVote } from '$lib/types';
	import SangiinVoteMap from '$lib/components/SangiinVoteMap.svelte';
	import ProvenanceChip from '$lib/components/ProvenanceChip.svelte';

	let { data }: { data: PageData } = $props();
	const m = $derived(data.map);
	const OPT_JA: Record<string, string> = { yes: '賛成', no: '反対', abstain: '棄権・欠席' };

	// 比例 (全国区) は選挙区を持たず地図に乗らないため会派別に併記する（§5）。
	const prByGroup = $derived(() => {
		const groups = new Map<string, SangiinPrVote[]>();
		for (const v of m.prVotes ?? []) {
			const key = v.parliamentaryGroup || '（会派不明）';
			(groups.get(key) ?? groups.set(key, []).get(key)!).push(v);
		}
		return [...groups.entries()].sort((a, b) => b[1].length - a[1].length);
	});

	const unmatched = $derived((m.totalVotes ?? 0) - (m.matchedVotes ?? 0));
	const prCount = $derived((m.prVotes ?? []).length);
</script>

<svelte:head><title>{m.motion || m.voteEventId} — 参議院記名投票地図 — S4rCiv</title></svelte:head>

<main id="main" class="wrap">
	<a class="back" href="/sangiin">← 参議院 記名投票一覧</a>

	<header class="vhead">
		<span class="label">参議院 記名投票地図 · 都道府県</span>
		<h1>{m.motion || '（件名なし）'}</h1>
		<p class="meta mono">第{m.session}回国会 · {m.date ?? ''} · 賛成 {m.yesCount ?? 0} / 反対 {m.noCount ?? 0}</p>
		<ProvenanceChip attr={m.attribution} />
	</header>

	<div class="grid">
		<section class="mapcol" aria-label="都道府県別の投票">
			<SangiinVoteMap prefectures={m.prefectures ?? []} />
			<ul class="legend" aria-label="凡例">
				<li><span class="sw" style="background:#2e9e5b"></span>全員賛成</li>
				<li><span class="sw" style="background:#d2454a"></span>全員反対</li>
				<li><span class="sw" style="background:#e0a838"></span>割れ</li>
				<li><span class="sw" style="background:#d7d9dd"></span>記録なし</li>
			</ul>
			<!-- Narrow layout stacks 比例（全国区）below the map; keep it in view (§5). -->
			{#if prCount > 0}
				<a class="pr-jump" href="#pr-panel">比例（全国区） {prCount}名 ↓</a>
			{/if}
			<p class="note">
				色は「その県選出議員の賛否が割れたか／揃ったか」という事実カテゴリで、賛同率の色分けではありません（§3/§5-C）。賛成・反対の内訳は県をクリックで表示します。比例（全国区）は選挙区を持たないため右に併記します（§5）。
				{#if unmatched > 0}
					氏名照合できた {m.matchedVotes} / {m.totalVotes} 名のみ集計（{unmatched} 名は氏名表記差で未集計）。
				{/if}
			</p>
		</section>

		<aside class="prcol" id="pr-panel" aria-label="比例選出議員">
			<h2 class="label">比例（全国区） <span class="cnt mono">{prCount}</span></h2>
			{#each prByGroup() as [group, members] (group)}
				<div class="grp">
					<div class="grpname">{group} <span class="cnt mono">{members.length}</span></div>
					<ul>
						{#each members as v (v.voterName)}
							<li>
								<span class="nm">{v.voterName || '—'}</span>
								<span class="opt opt-{v.option}">{OPT_JA[v.option ?? ''] ?? '—'}</span>
							</li>
						{/each}
					</ul>
				</div>
			{:else}
				<p class="sub">比例選出議員の記名投票記録はありません。</p>
			{/each}
		</aside>
	</div>
</main>

<style>
	.wrap {
		max-width: 1100px;
		margin: 0 auto;
		padding: 24px;
	}
	.back {
		font-size: 13px;
		text-decoration: none;
	}
	.vhead {
		margin: 12px 0 18px;
		padding-bottom: 14px;
		border-bottom: 1px solid var(--hairline-2);
	}
	.vhead h1 {
		font-size: 20px;
		margin: 6px 0;
	}
	.meta {
		font-size: 13px;
		color: var(--text-2);
		margin: 0 0 8px;
	}
	.grid {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 320px;
		gap: 22px;
		align-items: start;
	}
	.pr-jump {
		display: none;
	}
	/* Below --bp-lg (55rem) the 比例 panel stacks under the map; surface the jump. */
	@media (max-width: 55rem) {
		.grid {
			grid-template-columns: 1fr;
		}
		.pr-jump {
			display: inline-flex;
			align-items: center;
			gap: 6px;
			margin-top: 12px;
			font-size: 13px;
			padding: 6px 10px;
			border: 1px solid var(--hairline-2);
			border-radius: var(--r-sm);
			text-decoration: none;
			color: var(--text-2);
		}
	}
	@media (max-width: 30rem) {
		.wrap {
			padding: 16px;
		}
	}
	.legend {
		list-style: none;
		display: flex;
		flex-wrap: wrap;
		gap: 14px;
		padding: 0;
		margin: 10px 0 0;
		font-size: 12px;
		color: var(--text-2);
	}
	.legend li {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.sw {
		width: 13px;
		height: 13px;
		border-radius: 3px;
		display: inline-block;
	}
	.note {
		font-size: 12px;
		line-height: 1.7;
		color: var(--text-3);
		margin: 12px 0 0;
	}
	.prcol h2 {
		margin: 0 0 10px;
	}
	.cnt {
		color: var(--text-3);
		font-weight: 400;
	}
	.sub {
		font-size: 12px;
		color: var(--text-3);
	}
	.grp {
		margin-bottom: 14px;
	}
	.grpname {
		font-weight: 600;
		font-size: 14px;
		margin-bottom: 4px;
	}
	.prcol ul {
		list-style: none;
		padding: 0;
		margin: 0;
	}
	.prcol li {
		display: flex;
		align-items: baseline;
		gap: 8px;
		padding: 3px 0;
		font-size: 13px;
		border-bottom: 1px solid var(--hairline);
	}
	.nm {
		flex: 1;
		color: var(--text-1);
	}
	.opt {
		font-size: 12px;
	}
	.opt-yes {
		color: #2e9e5b;
	}
	.opt-no {
		color: #d2454a;
	}
	.opt-abstain {
		color: var(--text-3);
	}
</style>
