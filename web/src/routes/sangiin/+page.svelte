<script lang="ts">
	import type { PageData } from './$types';
	let { data }: { data: PageData } = $props();
	const events = $derived(data.voteEvents);
</script>

<svelte:head><title>参議院 記名投票地図 — S4RCIV</title></svelte:head>

<main id="main" class="wrap">
	<a class="back" href="/">← タイムライン</a>
	<header class="phead">
		<span class="label">参議院 記名投票地図</span>
		<h1>参議院の記名投票を都道府県で見る</h1>
		<p class="meta">
			第{data.session}回国会の参議院本会議記名投票（押しボタン）を、議員の選挙区（都道府県）別に地図化します。1選挙区は複数議員なので、県は事実カテゴリ（全員賛成／全員反対／割れ／記録なし）で塗り、賛成・反対の内訳はクリックで表示します（賛同率の色分けはしません）。
		</p>
	</header>

	{#if events.length === 0}
		<p class="empty">記名投票がまだありません。</p>
	{:else}
		<ul class="list">
			{#each events as e (e.voteEventId)}
				<li>
					<a class="card" href="/sangiin/{e.voteEventId}">
						<div class="motion">{e.motion || '（件名なし）'}</div>
						<div class="sub mono">第{e.session}回 · {e.date ?? ''}</div>
						<div class="tally mono">賛成 {e.yesCount ?? 0} / 反対 {e.noCount ?? 0}</div>
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</main>

<style>
	.wrap {
		max-width: 820px;
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
	.phead {
		margin: 12px 0 20px;
		padding-bottom: 14px;
		border-bottom: 1px solid var(--hairline-2);
	}
	.phead h1 {
		font-size: 21px;
		margin: 6px 0;
	}
	.meta {
		font-size: 13px;
		line-height: 1.7;
		color: var(--text-2);
		margin: 0;
	}
	.empty {
		color: var(--text-3);
		font-size: 14px;
	}
	.list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.card {
		display: block;
		padding: 14px 16px;
		border: 1px solid var(--hairline-2);
		border-radius: 8px;
		text-decoration: none;
		color: inherit;
	}
	.card:hover {
		border-color: var(--accent);
	}
	.motion {
		font-weight: 600;
		font-size: 15px;
		color: var(--text-1);
		margin-bottom: 4px;
	}
	.sub {
		font-size: 12px;
		color: var(--text-3);
		margin-bottom: 6px;
	}
	.tally {
		font-size: 12px;
		color: var(--text-2);
	}
</style>
