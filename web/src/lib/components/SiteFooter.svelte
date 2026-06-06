<script lang="ts">
	// Site-wide footer (DESIGN_LANGUAGE / ledger). Carries the copyright line, the AGPL-3.0
	// source offer, and links to the policy pages. The source link satisfies AGPL-3.0 §13
	// (prominently offer the running version's Corresponding Source to all network users);
	// it points at the public repository. Static — no visitor data, no third-party request.
	const SOURCE_URL = 'https://github.com/Kaikei-e/S4rCiv';
</script>

<footer class="site-foot">
	<nav class="links" aria-label="サイト情報">
		<a href="/about">S4RCIVについて</a>
		<a href="/terms">利用規約</a>
		<a href="/privacy">プライバシー</a>
		<a href="/attribution">出典・著作権</a>
		<a href={SOURCE_URL} rel="external">ソース (AGPL-3.0)</a>
	</nav>
	<p class="meta mono">© 2026 S4RCIV プロジェクト · 公的記録の受動・読取専用フライトレコーダ（試験運用）</p>
</footer>

<style>
	.site-foot {
		max-width: 880px;
		margin: 40px auto 0;
		padding: 18px 24px 28px;
		border-top: 1px solid var(--hairline-2);
	}
	.links {
		display: flex;
		flex-wrap: wrap;
		gap: 6px 18px;
	}
	.links a {
		font-size: var(--fs-sm);
		color: var(--text-2);
		text-decoration: none;
		white-space: nowrap;
	}
	.links a:hover {
		color: var(--accent);
	}
	.meta {
		margin: 12px 0 0;
		font-size: var(--fs-xs);
		color: var(--text-3);
		line-height: 1.6;
	}
	/* Touch: enlarge link targets to ≥44px (DESIGN_LANGUAGE §9.3 / WCAG 2.5.5). */
	@media (pointer: coarse) {
		.links a {
			min-height: 44px;
			display: inline-flex;
			align-items: center;
		}
	}
	@media (max-width: 30rem) {
		.site-foot {
			padding: 18px 16px 24px;
		}
	}
</style>
