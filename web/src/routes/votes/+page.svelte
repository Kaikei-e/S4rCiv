<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	const events = $derived(data.voteEvents);

	const RESULT_JA: Record<string, string> = {
		passed: '可決',
		rejected: '否決',
		unknown: '結果不明'
	};
</script>

<svelte:head><title>選挙区投票地図 — S4rCiv</title></svelte:head>

<main id="main" class="wrap">
	<a class="back" href="/">← タイムライン</a>

	<header class="phead">
		<span class="label">選挙区投票地図</span>
		<h1>記名投票を地図で見る</h1>
		<p class="meta">
			現会期（第{data.session}回国会）の記名投票のうち、選挙区別に地図化できるものです。色は事実カテゴリ（賛成／反対／棄権）のみで、集計スコアではありません。比例選出議員は各地図の横に併記します。
		</p>
	</header>

	{#if events.length === 0}
		<p class="empty">地図化できる記名投票はまだありません。</p>
	{:else}
		<ul class="list">
			{#each events as e (e.voteEventId)}
				<li>
					<a class="card" href="/votes/{e.voteEventId}">
						<div class="motion">{e.motion || '（件名なし）'}</div>
						<div class="sub mono">
							{e.house ?? ''} · {e.meetingName ?? ''} · {e.date ?? ''}
						</div>
						<div class="tally mono">
							<span class="res res-{e.result}">{RESULT_JA[e.result ?? 'unknown'] ?? e.result}</span>
							賛成 {e.yesCount ?? 0} / 反対 {e.noCount ?? 0}{#if e.abstainCount}
								/ 棄権 {e.abstainCount}{/if}
						</div>
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
	.res {
		margin-right: 8px;
	}
	.res-passed {
		color: #2e9e5b;
	}
	.res-rejected {
		color: #d2454a;
	}
	.res-unknown {
		color: var(--text-3);
	}
</style>
