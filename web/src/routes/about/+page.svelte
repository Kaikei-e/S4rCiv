<script lang="ts">
	import DocPage from '$lib/components/DocPage.svelte';
</script>

<svelte:head>
	<title>S4RCIVについて — S4RCIV</title>
	<meta
		name="description"
		content="S4RCIV は公的一次記録の受動・読取専用フライトレコーダです。何を・どんな方針で記録するかを説明します。"
	/>
</svelte:head>

<DocPage
	title="S4RCIVについて"
	updated="2026-06-06"
	lead="公的一次記録の受動・読取専用フライトレコーダ。いつ・何が・どう変わったか（消されたか）を、出典つきで記録します。"
>
	<h2>このサービスは何か</h2>
	<p>
		S4RCIV（サーシヴ）は、日本の公開された一次記録（国会会議録、法令など）を継続的に収集し、その<strong
			>変化（削除を含む）</strong
		>を、追記のみのハッシュ連鎖ログに記録する、非営利の civic-tech インフラです。記録には常に出典・取得時刻が添えられ、差分は周辺の文脈と全文へのリンクとともに示されます。
	</p>
	<p class="note">
		本サービスは<strong>試験運用（ベータ）</strong
		>です。収集・表示する情報は検証中であり、予告なく変更・停止することがあります。内容の正確性・完全性・最新性は保証しません。詳しくは<a
			href="/terms">利用規約</a
		>をご覧ください。
	</p>

	<h2>運営の方針</h2>
	<p>S4RCIV は、次の方針のもとで運営される機械的なパイプラインです。いずれも例外なく適用します。</p>
	<ul>
		<li>
			<strong>受動・読取専用。</strong
			>公開エンドポイントへの HTTP GET のみで情報を取得します。認証の背後を取りに行かず、送信・書き込み・自動アクションを行いません。見張り（sentinel）であって行為者ではありません。
		</li>
		<li><strong>公的な一次ソースのみ</strong>を対象とします。</li>
		<li>
			<strong>非党派。</strong
			>特定の政党・思想を標的にせず、すべての主体へ同一の基準・同一のパイプラインを適用します。
		</li>
		<li>
			<strong>私人を晒さない。</strong
			>監視の矛先は、説明責任を負う公的主体（政治家・政党・政治団体・公職者）に固定します。私人（政治資金の少額寄附者など）の検索可能なプロファイルを作らず、横断的な名寄せをしません。
		</li>
		<li>
			<strong>AI は要約のみ・判断しない。</strong
			>スコアリングや論評をせず、すべての要約は原文・差分へリンクし、確信度と出所を併記します。
		</li>
		<li>
			<strong>出典・取得時刻・改ざん耐性を常時表示。</strong
			>各記録は出典と取得時刻を保持し、ログはハッシュ連鎖によって事後の書き換えを検出できます。
		</li>
	</ul>

	<h2>運営者</h2>
	<p>
		運営者：S4RCIV プロジェクト（個人による非営利運営）。本サービスは無償・非商用です。お問い合わせ・ご連絡は、当面 GitHub
		を窓口とします。
	</p>
	<ul>
		<li>リポジトリ / 連絡：<a href="https://github.com/Kaikei-e/S4rCiv" rel="external">github.com/Kaikei-e/S4rCiv</a></li>
	</ul>

	<h2>関連ページ</h2>
	<ul>
		<li><a href="/terms">利用規約</a></li>
		<li><a href="/privacy">プライバシーポリシー</a></li>
		<li><a href="/attribution">出典・著作権</a></li>
	</ul>
</DocPage>
