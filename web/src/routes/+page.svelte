<script lang="ts">
	import ChangeLogItem from '$lib/components/ChangeLogItem.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	// Build a querystring from the active filters, with optional overrides
	// (used for the next-page link and the Atom feed URL).
	function qs(extra: Record<string, string> = {}) {
		const f = data.filters;
		const p = new URLSearchParams();
		if (f.source) p.set('source', f.source);
		if (f.eventType) p.set('event_type', f.eventType);
		if (f.classification) p.set('classification', f.classification);
		if (f.keyword) p.set('q', f.keyword);
		for (const [k, v] of Object.entries(extra)) if (v) p.set(k, v);
		const s = p.toString();
		return s ? `?${s}` : '';
	}

	const feedHref = $derived(`/timeline.atom${qs()}`);
	const nextHref = $derived(data.nextPageToken ? qs({ page: data.nextPageToken }) || '?' : '');
</script>

<svelte:head>
	<title>S4rCiv — 市民のための公的記録可視化ダッシュボード</title>
	<meta
		name="description"
		content="公的一次記録の変化を時系列で辿る。いつ・何が・どう変わったか／消されたか。"
	/>
</svelte:head>

<header class="topbar">
	<div class="brand">
		<span class="dot" aria-hidden="true">◉</span>
		<h1>S4rCiv <span class="label">公的記録の司令室</span></h1>
	</div>
	<a class="feed" href={feedHref} title="この絞り込みの Atom フィードを購読（ウォッチ）">📡 RSS</a>
</header>

<main id="main" class="wrap">
	<form class="filters" method="GET" action="/">
		<label>
			<span class="label">ソース</span>
			<select name="source" value={data.filters.source}>
				<option value="">すべて</option>
				<option value="kokkai">国会会議録</option>
				<option value="egov-law">法令</option>
			</select>
		</label>
		<label>
			<span class="label">種別</span>
			<select name="event_type" value={data.filters.eventType}>
				<option value="">すべて</option>
				<option value="ResourceObserved">観測</option>
				<option value="ResourceChanged">変化</option>
				<option value="ResourceVanished">消失</option>
				<option value="ResourceRestored">復活</option>
			</select>
		</label>
		<label>
			<span class="label">分類</span>
			<select name="classification" value={data.filters.classification}>
				<option value="">すべて</option>
				<option value="substantive">実質的変更</option>
				<option value="administrative">事務的変更</option>
			</select>
		</label>
		<label class="grow">
			<span class="label">キーワード（法令名・議案・会議名）</span>
			<input name="q" type="search" value={data.filters.keyword} placeholder="例: 刑法 / 予算" />
		</label>
		<button type="submit">絞り込む</button>
	</form>

	<section class="panel" aria-label="横断タイムライン">
		<div class="panel-head">
			<span class="label">横断タイムライン</span>
			<span class="count mono">{data.items.length} 件</span>
		</div>

		{#if data.error}
			<p class="state error">⊘ 取得に失敗しました: <span class="mono">{data.error}</span></p>
		{:else if data.items.length === 0}
			<p class="state">該当する記録がありません。</p>
		{:else}
			<div class="list">
				{#each data.items as item (item.seq)}
					<ChangeLogItem {item} />
				{/each}
			</div>
			{#if nextHref}
				<a class="more" href={nextHref}>古い記録を読み込む →</a>
			{/if}
		{/if}
	</section>

	<footer class="foot">
		<p>
			S4rCiv は公的一次記録の受動・読取専用フライトレコーダです。各記録は出典・取得時刻・ハッシュ連鎖の連結とともに表示されます（完全性の検証ツールは今後提供）。
		</p>
	</footer>
</main>

<style>
	.topbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 12px 24px;
		border-bottom: 1px solid var(--hairline-2);
		background: var(--surface-1);
	}
	.brand {
		display: flex;
		align-items: baseline;
		gap: 10px;
	}
	.brand .dot {
		color: var(--st-info-t);
	}
	.brand h1 {
		font-size: 21px;
		font-weight: 700;
		margin: 0;
		display: flex;
		align-items: baseline;
		gap: 10px;
	}
	.feed {
		font-size: 12px;
		padding: 4px 10px;
		border: 1px solid var(--hairline-2);
		border-radius: var(--r-sm);
		text-decoration: none;
		color: var(--text-2);
	}
	.feed:hover {
		color: var(--accent);
		border-color: var(--accent);
	}
	.wrap {
		max-width: 880px;
		margin: 0 auto;
		padding: 24px;
	}
	.filters {
		display: flex;
		flex-wrap: wrap;
		align-items: end;
		gap: 12px;
		margin-bottom: 20px;
	}
	.filters label {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.filters .grow {
		flex: 1;
		min-width: 200px;
	}
	select,
	input,
	button {
		font: inherit;
		color: var(--text-1);
		background: var(--surface-2);
		border: 1px solid var(--hairline-2);
		border-radius: var(--r-sm);
		padding: 7px 10px;
	}
	button {
		background: var(--accent);
		color: #04222c;
		font-weight: 600;
		border-color: transparent;
		cursor: pointer;
	}
	.panel {
		background: var(--surface-1);
		border: 1px solid var(--hairline-2);
		border-radius: var(--r-lg);
		padding: 4px 18px 18px;
	}
	.panel-head {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		padding: 14px 0 6px;
		border-bottom: 1px solid var(--hairline-2);
	}
	.count {
		font-size: 12px;
		color: var(--text-3);
	}
	.state {
		color: var(--text-2);
		padding: 24px 0;
	}
	.state.error {
		color: var(--st-critical-t);
	}
	.more {
		display: inline-block;
		margin-top: 14px;
		font-size: 13px;
	}
	.foot {
		margin-top: 20px;
		font-size: 12px;
		color: var(--text-3);
	}
</style>
