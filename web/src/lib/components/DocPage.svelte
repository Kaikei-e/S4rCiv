<script lang="ts">
	// Shared shell for the static policy / legal pages (about, terms, privacy, attribution).
	// Editorial ledger styling (DESIGN_LANGUAGE v1): serif display H1, sans section headers,
	// inset wells for source-credit and clause examples. Purely presentational — no data,
	// no third-party request (passive sentinel, 設計原則①). Body markup is passed as the
	// implicit children snippet, so its prose styles are reached via :global within .prose.
	import type { Snippet } from 'svelte';

	let {
		title,
		lead = undefined,
		updated = undefined,
		children
	}: {
		title: string;
		/** Optional one-line summary under the H1. */
		lead?: string;
		/** Last-updated date (YYYY-MM-DD), shown in the header. */
		updated?: string;
		children: Snippet;
	} = $props();
</script>

<main id="main" class="doc">
	<header class="dochead">
		<p class="label">S4RCIV</p>
		<h1>{title}</h1>
		{#if lead}<p class="lead">{lead}</p>{/if}
		{#if updated}<p class="updated mono">最終更新: {updated}</p>{/if}
	</header>
	<div class="prose">
		{@render children()}
	</div>
</main>

<style>
	.doc {
		max-width: 760px;
		margin: 0 auto;
		padding: 32px 24px 64px;
	}
	.dochead {
		padding-bottom: 16px;
		border-bottom: 1px solid var(--hairline-2);
		margin-bottom: 24px;
	}
	.dochead .label {
		margin: 0 0 8px;
	}
	h1 {
		margin: 0;
		font-size: var(--fs-display);
	}
	.lead {
		margin: 14px 0 0;
		color: var(--text-2);
		font-size: var(--fs-h3);
		line-height: 1.6;
	}
	.updated {
		margin: 12px 0 0;
		font-size: var(--fs-xs);
		color: var(--text-3);
	}

	/* Prose: section headers (h2) sans with a hairline rule; body 15px, generous leading.
	   The children come from the page component's scope, so target them via :global. */
	.prose :global(h2) {
		font-family: var(--font-sans);
		font-size: var(--fs-h2);
		font-weight: 600;
		color: var(--text-1);
		margin: 36px 0 10px;
		padding-top: 20px;
		border-top: 1px solid var(--hairline);
	}
	.prose :global(h2:first-child) {
		margin-top: 0;
		padding-top: 0;
		border-top: none;
	}
	.prose :global(h3) {
		font-family: var(--font-sans);
		font-size: var(--fs-h3);
		font-weight: 600;
		color: var(--text-1);
		margin: 22px 0 8px;
	}
	.prose :global(p) {
		margin: 10px 0;
		color: var(--text-2);
	}
	.prose :global(ul),
	.prose :global(ol) {
		margin: 10px 0;
		padding-left: 1.4em;
		color: var(--text-2);
	}
	.prose :global(li) {
		margin: 6px 0;
	}
	.prose :global(a) {
		color: var(--accent);
	}
	.prose :global(strong) {
		color: var(--text-1);
		font-weight: 600;
	}
	/* Aside / clarifying note — inset well. */
	.prose :global(.note) {
		margin: 16px 0;
		padding: 12px 14px;
		background: var(--surface-inset);
		border: 1px solid var(--hairline-2);
		border-radius: var(--r-lg);
		color: var(--text-2);
		font-size: var(--fs-sm);
	}
	/* Verbatim source-credit / clause example — monospace, accent rule. */
	.prose :global(.cite) {
		display: block;
		margin: 12px 0;
		padding: 10px 14px;
		background: var(--surface-inset);
		border-left: 2px solid var(--accent-line);
		border-radius: var(--r-sm);
		font-family: var(--font-mono);
		font-size: var(--fs-xs);
		line-height: 1.7;
		color: var(--text-2);
		white-space: pre-wrap;
	}
	@media (max-width: 30rem) {
		.doc {
			padding: 24px 16px 48px;
		}
		h1 {
			font-size: var(--fs-h1);
		}
	}
</style>
