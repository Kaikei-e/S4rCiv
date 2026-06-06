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
	// Keyset pager over the immutable seq spine: prev = newer (seq >), next = older
	// (seq <). An empty token means that end has no further page. total_count + page
	// are orientation only — keyset has no random page jump (no clickable numbers).
	const prevHref = $derived(data.prevPageToken ? qs({ page: data.prevPageToken }) || '?' : '');
	const nextHref = $derived(data.nextPageToken ? qs({ page: data.nextPageToken }) || '?' : '');
	const totalPages = $derived(Math.max(1, Math.ceil((data.totalCount || 0) / (data.pageSize || 50))));

	// On narrow screens the filter form collapses into a <details> disclosure
	// (DESIGN_LANGUAGE §9.2); the summary surfaces which filters are active so the
	// collapsed state never hides that the timeline is being narrowed.
	const activeFilterCount = $derived(
		[data.filters.source, data.filters.eventType, data.filters.classification, data.filters.keyword]
			.filter(Boolean).length
	);
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
		<h1>S4rCiv： サーシヴ <span class="label">公的記録の可視化・追跡</span></h1>
	</div>
	<nav class="topnav">
		<a class="navlink" href="/votes" title="衆院の記名投票を小選挙区別に地図で見る">🗺 衆院</a>
		<a class="navlink" href="/sangiin" title="参院の記名投票を都道府県別に地図で見る">🗺 参院</a>
		<a class="feed" href={feedHref} title="この絞り込みの Atom フィードを購読（ウォッチ）">📡 RSS</a>
	</nav>
</header>

<main id="main" class="wrap">
	<details class="filterbox">
		<summary>
			<span class="label">絞り込み</span>
			{#if activeFilterCount > 0}
				<span class="active mono">適用中 {activeFilterCount}</span>
			{/if}
		</summary>
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
	</details>

	<section class="panel" aria-label="横断タイムライン">
		<div class="panel-head">
			<span class="label">横断タイムライン</span>
			<span class="count mono">全 {data.totalCount.toLocaleString()} 件</span>
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
			{#if prevHref || nextHref}
				<nav class="pager" aria-label="ページ送り">
					{#if prevHref}
						<a class="pg" href={prevHref} rel="prev">← 新しい記録</a>
					{:else}
						<span class="pg disabled" aria-disabled="true">← 新しい記録</span>
					{/if}
					<span class="pg-count mono">{data.page} / {totalPages} ページ</span>
					{#if nextHref}
						<a class="pg" href={nextHref} rel="next">古い記録 →</a>
					{:else}
						<span class="pg disabled" aria-disabled="true">古い記録 →</span>
					{/if}
				</nav>
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
		flex-wrap: wrap;
		gap: 12px 16px;
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
	.topnav {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.feed,
	.navlink {
		font-size: 12px;
		padding: 4px 10px;
		border: 1px solid var(--hairline-2);
		border-radius: var(--r-sm);
		text-decoration: none;
		color: var(--text-2);
		white-space: nowrap;
	}
	.feed:hover,
	.navlink:hover {
		color: var(--accent);
		border-color: var(--accent);
	}
	.wrap {
		max-width: 880px;
		margin: 0 auto;
		padding: 24px;
	}
	/* Filter disclosure: collapsed on narrow (native <details>), always-open inline
	   on wide (DESIGN_LANGUAGE §9.2). */
	.filterbox {
		margin-bottom: 20px;
	}
	.filterbox > summary {
		display: flex;
		align-items: center;
		gap: 10px;
		cursor: pointer;
		padding: 8px 0;
		list-style: none;
	}
	.filterbox > summary::-webkit-details-marker {
		display: none;
	}
	.filterbox > summary::before {
		content: '▸';
		color: var(--text-3);
	}
	.filterbox[open] > summary::before {
		content: '▾';
	}
	.filterbox .active {
		font-size: 12px;
		color: var(--st-changed-t);
	}
	.filters {
		display: flex;
		flex-wrap: wrap;
		align-items: end;
		gap: 12px;
	}
	/* Wide: no disclosure — filters are always visible inline. (mirrors --bp-lg 55rem) */
	@media (min-width: 55rem) {
		.filterbox > summary {
			display: none;
		}
		.filterbox > .filters {
			display: flex !important;
		}
	}
	/* Narrow: stack each control full-width. (mirrors --bp-sm 30rem) */
	@media (max-width: 30rem) {
		.wrap {
			padding: 16px;
		}
		.filters {
			flex-direction: column;
			align-items: stretch;
		}
		.filters .grow {
			min-width: 0;
		}
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
	.pager {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		margin-top: 16px;
		padding-top: 14px;
		border-top: 1px solid var(--hairline-2);
	}
	.pg {
		font-size: 13px;
		padding: 6px 12px;
		border: 1px solid var(--hairline-2);
		border-radius: var(--r-sm);
		text-decoration: none;
		color: var(--text-2);
		white-space: nowrap;
	}
	.pg:hover {
		color: var(--accent);
		border-color: var(--accent);
	}
	.pg.disabled {
		color: var(--text-3);
		opacity: 0.4;
		cursor: default;
	}
	.pg-count {
		font-size: 12px;
		color: var(--text-3);
	}
	.foot {
		margin-top: 20px;
		font-size: 12px;
		color: var(--text-3);
	}
	/* Establish a query container so ChangeLogItem reflows to its own width,
	   not the viewport's (DESIGN_LANGUAGE §9.2). */
	.list {
		container-type: inline-size;
		container-name: timeline;
	}
	/* Narrow skeleton: stack brand over nav, drop the decorative tagline. */
	@media (max-width: 30rem) {
		.topbar {
			flex-direction: column;
			align-items: flex-start;
			padding: 12px 16px;
		}
		.brand h1 .label {
			display: none;
		}
		.topnav {
			width: 100%;
		}
	}
	/* Touch: enlarge link targets to ≥44px (DESIGN_LANGUAGE §9.3 / WCAG 2.5.5). */
	@media (pointer: coarse) {
		.navlink,
		.feed,
		.pg {
			min-height: 44px;
			display: inline-flex;
			align-items: center;
		}
		.filterbox > summary {
			min-height: 44px;
		}
	}
</style>
