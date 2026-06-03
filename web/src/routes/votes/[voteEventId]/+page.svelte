<script lang="ts">
	import type { PageData } from './$types';
	import type { Vote } from '$lib/types';
	import DistrictVoteMap from '$lib/components/DistrictVoteMap.svelte';
	import ProvenanceChip from '$lib/components/ProvenanceChip.svelte';

	let { data }: { data: PageData } = $props();
	const ev = $derived(data.voteEvent);
	const votes = $derived<Vote[]>(ev.votes ?? []);

	const RESULT_JA: Record<string, string> = {
		passed: '可決',
		rejected: '否決',
		unknown: '結果不明'
	};
	const OPT_JA: Record<string, string> = { yes: '賛成', no: '反対', abstain: '棄権' };

	// 比例選出議員は選挙区を持たず地図に乗らないため、常に併記して消さない (§5)。会派別に束ねる。
	const prByGroup = $derived(() => {
		const groups = new Map<string, Vote[]>();
		for (const v of votes) {
			if (!v.isPr) continue;
			const key = v.parliamentaryGroup || '（会派不明）';
			(groups.get(key) ?? groups.set(key, []).get(key)!).push(v);
		}
		return [...groups.entries()].sort((a, b) => b[1].length - a[1].length);
	});

	const districtCount = $derived(votes.filter((v) => !v.isPr && v.districtCode).length);
	const prCount = $derived(votes.filter((v) => v.isPr).length);
</script>

<svelte:head><title>{ev.motion || ev.voteEventId} — 選挙区投票地図 — S4rCiv</title></svelte:head>

<main id="main" class="wrap">
	<a class="back" href="/votes">← 記名投票一覧</a>

	<header class="vhead">
		<span class="label">選挙区投票地図 · 記名投票</span>
		<h1>{ev.motion || '（件名なし）'}</h1>
		<p class="meta mono">
			{RESULT_JA[ev.result ?? 'unknown'] ?? ev.result} · 賛成 {ev.yesCount ?? 0} / 反対 {ev.noCount ?? 0}
			{#if ev.abstainCount}/ 棄権 {ev.abstainCount}{/if}
			{#if ev.needsReview}<span class="review">· 自動抽出（未レビュー）</span>{/if}
		</p>
		<ProvenanceChip attr={ev.attribution} />
	</header>

	<div class="grid">
		<section class="mapcol" aria-label="選挙区別の投票">
			<DistrictVoteMap {votes} />
			<ul class="legend" aria-label="凡例">
				<li><span class="sw" style="background:#2e9e5b"></span>賛成</li>
				<li><span class="sw" style="background:#d2454a"></span>反対</li>
				<li><span class="sw" style="background:#e0a838"></span>棄権</li>
				<li><span class="sw" style="background:#d7d9dd"></span>記録なし</li>
			</ul>
			<!-- §5 / §7: be explicit about what the map does NOT show. -->
			<p class="note">
				記名投票のあった議案のみ地図化しています。色は「その区の現職がどう投じたか」の事実カテゴリで、集計スコアや賛同率ではありません。「記録なし」の区は、現職の記名投票記録が無い・未同定、または直近総選挙より前の構成です（地図は現会期のレンズ、履歴は不変ログ／タイムラインが保持）。
			</p>
		</section>

		<aside class="prcol" aria-label="比例選出議員">
			<h2 class="label">比例選出 <span class="cnt mono">{prCount}</span></h2>
			<p class="sub">比例代表は選挙区を持たず地図に乗らないため併記します（§5）。</p>
			{#each prByGroup() as [group, members] (group)}
				<div class="grp">
					<div class="grpname">{group} <span class="cnt mono">{members.length}</span></div>
					<ul>
						{#each members as m (m.personId || m.voterName)}
							<li>
								<span class="nm">{m.voterName || '—'}</span>
								<span class="opt opt-{m.option}">{OPT_JA[m.option ?? ''] ?? '—'}</span>
								{#if m.prBlock}<span class="blk">{m.prBlock}</span>{/if}
							</li>
						{/each}
					</ul>
				</div>
			{:else}
				<p class="sub">この議案に比例選出議員の記名投票記録はありません。</p>
			{/each}
		</aside>
	</div>

	<p class="counts mono">地図対象 {districtCount} 区 · 比例 {prCount} 名</p>

	{#if ev.issueId}
		<a class="ext" href="/meetings/{ev.issueId}">この採決の会議録を見る →</a>
	{/if}
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
	.review {
		color: var(--st-caution-t);
	}
	.grid {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 320px;
		gap: 22px;
		align-items: start;
	}
	@media (max-width: 880px) {
		.grid {
			grid-template-columns: 1fr;
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
		margin: 0 0 4px;
	}
	.sub {
		font-size: 12px;
		color: var(--text-3);
		margin: 0 0 12px;
	}
	.grp {
		margin-bottom: 14px;
	}
	.grpname {
		font-weight: 600;
		font-size: 14px;
		margin-bottom: 4px;
	}
	.cnt {
		color: var(--text-3);
		font-weight: 400;
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
		color: #e0a838;
	}
	.blk {
		font-size: 11px;
		color: var(--text-3);
	}
	.counts {
		font-size: 12px;
		color: var(--text-3);
		margin: 16px 0 0;
	}
	.ext {
		display: inline-block;
		margin-top: 14px;
		font-size: 13px;
	}
</style>
