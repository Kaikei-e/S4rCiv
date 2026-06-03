<script lang="ts">
	import type { PageData } from './$types';
	import ProvenanceChip from '$lib/components/ProvenanceChip.svelte';

	let { data }: { data: PageData } = $props();

	const compiled = $derived(data.identityConfidence === 'high');
	const OPT: Record<string, string> = { yes: '賛成', no: '反対', abstain: '棄権' };
</script>

<svelte:head><title>{data.personName || data.personId} の記名投票 — S4rCiv</title></svelte:head>

<main id="main" class="wrap">
	<a class="back" href="/">← タイムライン</a>

	<header class="phead">
		<span class="label">議員 · 記名投票の記録</span>
		<h1>{data.personName || data.personId}</h1>
		<p class="note mono">
			同定確信度: {data.identityConfidence || '不明'} · 出典に公開された記名投票のみ（発言は集成しません）
		</p>
	</header>

	{#if !compiled}
		<!-- ADR-000006: a possible homonym is never merged into one profile. -->
		<p class="guard">
			▲ 同名異人の可能性があるため（同定確信度が「high」でない）、この識別子の投票記録は集成していません。これは反プロファイリングのための意図的な制約です（ADR-000006）。
		</p>
	{:else if data.votes.length === 0}
		<p class="empty">記名投票の記録がまだありません。</p>
	{:else}
		<ul class="votes">
			{#each data.votes as v (v.voteEventId)}
				<li class="vote">
					<span class="opt opt-{v.option}">{OPT[v.option ?? ''] ?? v.option}</span>
					<div class="body">
						<a class="motion" href="/meetings/{v.issueId}">{v.motion || '(議案名なし)'}</a>
						<p class="ctx">
							{v.meetingName ?? ''}{#if v.house} · {v.house}{/if}{#if v.date} · {v.date}{/if}
							· 結果: {v.result ?? '—'}
						</p>
						<ProvenanceChip attr={v.attribution} />
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</main>

<style>
	.wrap {
		max-width: 760px;
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
	.note {
		font-size: 12px;
		color: var(--text-3);
		margin: 0;
	}
	.guard {
		color: var(--st-caution-t);
		background: color-mix(in srgb, var(--st-caution) 10%, transparent);
		border: 1px solid color-mix(in srgb, var(--st-caution) 35%, transparent);
		border-radius: var(--r);
		padding: 16px;
		font-size: 14px;
	}
	.empty {
		color: var(--text-2);
	}
	.votes {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.vote {
		display: flex;
		gap: 12px;
		padding: 12px 0;
		border-bottom: 1px solid var(--hairline);
	}
	/* Neutral chips — votes are facts, not judgments (§5-C): no good/bad coloring. */
	.opt {
		flex: none;
		align-self: start;
		font-size: 12px;
		padding: 3px 10px;
		border-radius: var(--r-sm);
		border: 1px solid var(--hairline-2);
		background: var(--surface-2);
		color: var(--text-2);
	}
	.body {
		min-width: 0;
	}
	.motion {
		font-size: 15px;
		font-weight: 600;
		color: var(--text-1);
		text-decoration: none;
	}
	.motion:hover {
		color: var(--accent);
	}
	.ctx {
		margin: 4px 0 6px;
		font-size: 13px;
		color: var(--text-2);
	}
</style>
