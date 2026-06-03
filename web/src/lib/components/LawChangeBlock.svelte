<script lang="ts">
	import type { LawChange, LawNode, LawNodeChange } from '$lib/types';

	let { change, nodes }: { change: LawChange; nodes: LawNode[] } = $props();

	const byEid = $derived(new Map(nodes.map((n) => [n.eid ?? '', n])));

	// Walk parentEid up to the enclosing article so a change is shown WITHIN its
	// article, never as a bare snippet (§7 / E2).
	function articleOf(eid?: string): LawNode | undefined {
		let cur = eid ? byEid.get(eid) : undefined;
		const seen = new Set<string>();
		while (cur && cur.nodeType !== 'article') {
			if (cur.parentEid && !seen.has(cur.parentEid)) {
				seen.add(cur.parentEid);
				cur = byEid.get(cur.parentEid);
			} else {
				return undefined;
			}
		}
		return cur;
	}

	type Group = { articleEid: string; article?: LawNode; changes: LawNodeChange[] };

	// Group this change's node-changes by their enclosing article.
	const groups = $derived.by(() => {
		const m = new Map<string, Group>();
		for (const nc of change.nodeChanges ?? []) {
			const art = articleOf(nc.eid);
			const key = art?.eid ?? nc.eid ?? '?';
			if (!m.has(key)) m.set(key, { articleEid: key, article: art, changes: [] });
			m.get(key)!.changes.push(nc);
		}
		return [...m.values()];
	});

	// Current nodes of an article, in document order, for dimmed context.
	function articleNodes(articleEid: string): LawNode[] {
		return nodes
			.filter((n) => articleOf(n.eid)?.eid === articleEid || n.eid === articleEid)
			.sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0));
	}

	const OP = {
		added: { mark: '+', label: '追加', cls: 'add' },
		modified: { mark: '~', label: '改変', cls: 'mod' },
		deleted: { mark: '−', label: '削除', cls: 'del' },
		moved: { mark: '→', label: '移動', cls: 'mod' }
	} as const;
	const op = (o?: string) => OP[(o ?? '') as keyof typeof OP] ?? OP.modified;

	const isSubstantive = $derived(change.classification === 'substantive');
	function changeFor(nodes_eid: string | undefined): LawNodeChange | undefined {
		return (change.nodeChanges ?? []).find((c) => c.eid === nodes_eid);
	}
	const nodeLabel = (n: LawNode) =>
		(n.nodeType === 'article' ? '第' + (n.num ?? '') + '条' : n.num ? '(' + n.num + ')' : '') +
		(n.caption ? ' ' + n.caption : '');
</script>

<article class="change">
	<header class="chead">
		<span class="chip" class:substantive={isSubstantive}>
			{isSubstantive ? '実質的変更' : '事務的変更'} · 自動分類(未レビュー)
		</span>
		{#if change.classConfidence}<span class="label">確信度 {change.classConfidence}</span>{/if}
		<span class="mono when">観測 #{change.observationSeq} · {(change.observedAt ?? '').slice(0, 10)}</span>
	</header>

	{#each groups as g (g.articleEid)}
		<div class="article">
			{#if g.article}
				<h4 class="art-h">{nodeLabel(g.article)}</h4>
				<!-- The whole article is rendered; changed nodes are marked, the rest is
				     dimmed context so the diff is never decontextualized (§7). -->
				{#each articleNodes(g.articleEid) as n (n.eid)}
					{@const nc = changeFor(n.eid)}
					{#if nc}
						{@const o = op(nc.op)}
						<div class="line {o.cls}">
							<span class="mark mono">{o.mark}</span>
							<div class="texts">
								<span class="nlabel">{nodeLabel(n) || nc.num || nc.eid} <span class="op-label">{o.label}</span></span>
								{#if nc.prevText}<p class="prev"><span class="sign">−</span>{nc.prevText}</p>{/if}
								{#if nc.currText}<p class="curr"><span class="sign">+</span>{nc.currText}</p>{/if}
							</div>
						</div>
					{:else}
						<p class="ctx">{n.sentenceText || nodeLabel(n)}</p>
					{/if}
				{/each}
			{:else}
				<!-- Article not in the current tree (e.g. a deleted node): show before/after. -->
				{#each g.changes as nc (nc.eid)}
					{@const o = op(nc.op)}
					<div class="line {o.cls}">
						<span class="mark mono">{o.mark}</span>
						<div class="texts">
							<span class="nlabel">{nc.num || nc.eid} <span class="op-label">{o.label}</span></span>
							{#if nc.prevText}<p class="prev"><span class="sign">−</span>{nc.prevText}</p>{/if}
							{#if nc.currText}<p class="curr"><span class="sign">+</span>{nc.currText}</p>{/if}
						</div>
					</div>
				{/each}
			{/if}
		</div>
	{/each}
</article>

<style>
	.change {
		border: 1px solid var(--hairline-2);
		border-radius: var(--r);
		padding: 14px 16px;
		margin-bottom: 14px;
		background: var(--surface-1);
	}
	.chead {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 10px;
		margin-bottom: 12px;
	}
	.chip {
		font-size: 11px;
		padding: 2px 8px;
		border-radius: var(--r-sm);
		border: 1px solid var(--hairline-2);
		color: var(--text-2);
		background: var(--surface-2);
	}
	.chip.substantive {
		color: var(--st-changed-t);
		border-color: color-mix(in srgb, var(--st-changed) 40%, transparent);
		background: color-mix(in srgb, var(--st-changed) 12%, transparent);
	}
	.when {
		font-size: 11px;
		color: var(--text-3);
		margin-left: auto;
	}
	.art-h {
		font-size: 14px;
		margin: 0 0 8px;
		color: var(--text-1);
	}
	.ctx {
		margin: 0 0 6px;
		padding-left: 20px;
		color: var(--text-3);
		font-size: 13px;
	}
	.line {
		display: flex;
		gap: 8px;
		padding: 6px 8px;
		margin: 4px 0;
		border-left: 3px solid var(--hairline-2);
		border-radius: 0 var(--r-sm) var(--r-sm) 0;
	}
	.line.mod {
		border-left-color: var(--st-changed);
		background: color-mix(in srgb, var(--st-changed) 8%, transparent);
	}
	.line.add {
		border-left-color: var(--st-nominal);
		background: color-mix(in srgb, var(--st-nominal) 8%, transparent);
	}
	.line.del {
		border-left-color: var(--st-critical);
		background: color-mix(in srgb, var(--st-critical) 8%, transparent);
	}
	.mark {
		font-weight: 700;
		color: var(--text-2);
	}
	.texts {
		min-width: 0;
	}
	.nlabel {
		font-size: 12px;
		color: var(--text-2);
	}
	.op-label {
		color: var(--text-3);
	}
	.prev,
	.curr {
		margin: 4px 0 0;
		font-size: 14px;
		line-height: 1.6;
	}
	.prev {
		color: var(--st-critical-t);
	}
	.curr {
		color: var(--st-nominal-t);
	}
	.sign {
		font-family: var(--font-mono);
		margin-right: 6px;
		opacity: 0.8;
	}
</style>
