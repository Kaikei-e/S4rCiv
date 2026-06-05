<script lang="ts">
	import type { PageData } from './$types';
	import ProvenanceChip from '$lib/components/ProvenanceChip.svelte';
	import VerificationPanel from '$lib/components/VerificationPanel.svelte';

	let { data }: { data: PageData } = $props();
	const m = $derived(data.meeting);
	const subtitle = $derived(
		[m.session ? `第${m.session}回` : '', m.house ?? '', m.issue ?? '', m.date ?? '']
			.filter(Boolean)
			.join(' · ')
	);
	// Deep-link the provenance chip to this record's row in the verification panel.
	const verifyHref = $derived(
		m.attribution?.observationSeq ? `#verify-${m.attribution.observationSeq}` : undefined
	);
</script>

<svelte:head><title>{m.meetingName ?? m.issueId} — S4rCiv</title></svelte:head>

<main id="main" class="wrap">
	<a class="back" href="/">← タイムライン</a>

	<header class="mhead">
		<span class="label">国会会議録</span>
		<h1>{m.meetingName ?? m.issueId}</h1>
		<p class="meta mono">{subtitle}</p>
		<ProvenanceChip attr={m.attribution} {verifyHref} />
	</header>

	<!-- The full meeting is shown in order (§7-safe). We never compile one speaker's
	     remarks into a per-person anthology (ADR-000004); a speaker links instead to
	     their named-vote record (the only allowed per-person axis, ADR-000006). -->
	<section aria-label="発言" class="speeches">
		<h2 class="label">発言 <span class="cnt mono">{data.speeches.length}</span></h2>
		{#each data.speeches as s (s.speechId)}
			<article class="sp">
				<div class="who">
					{#if s.personId}
						<a class="speaker" href="/legislators/{s.personId}" title="記名投票の記録を見る"
							>{s.speaker || '—'}</a
						>
					{:else}
						<span class="speaker">{s.speaker || '—'}</span>
					{/if}
					{#if s.speakerPosition}<span class="pos">{s.speakerPosition}</span>{/if}
					{#if s.speakerGroup}<span class="grp">{s.speakerGroup}</span>{/if}
				</div>
				{#if s.speech}<p class="text">{s.speech}</p>{/if}
			</article>
		{/each}
	</section>

	{#if m.attribution?.permalink}
		<a class="ext" href={m.attribution.permalink} target="_blank" rel="noopener noreferrer external"
			>NDL 国会会議録検索システムで原文を見る ↗</a
		>
	{/if}

	{#if data.verification}
		<VerificationPanel data={data.verification} />
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
	.mhead {
		margin: 12px 0 20px;
		padding-bottom: 14px;
		border-bottom: 1px solid var(--hairline-2);
	}
	.mhead h1 {
		font-size: 21px;
		margin: 6px 0;
	}
	.meta {
		font-size: 13px;
		color: var(--text-2);
		margin: 0 0 8px;
	}
	h2.label {
		display: block;
		margin: 8px 0 14px;
	}
	.cnt {
		color: var(--text-3);
	}
	.sp {
		padding: 12px 0;
		border-bottom: 1px solid var(--hairline);
	}
	.who {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 10px;
		margin-bottom: 6px;
	}
	.speaker {
		font-weight: 600;
		color: var(--text-1);
		text-decoration: none;
	}
	a.speaker:hover {
		color: var(--accent);
		text-decoration: underline;
	}
	.pos,
	.grp {
		font-size: 12px;
		color: var(--text-3);
	}
	.text {
		margin: 0;
		font-size: 15px;
		line-height: 1.7;
		color: var(--text-1);
		white-space: pre-wrap;
	}
	.ext {
		display: inline-block;
		margin-top: 18px;
		font-size: 13px;
	}
</style>
