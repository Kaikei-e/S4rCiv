<script lang="ts">
	import {
		verifyStream,
		type StreamVerificationJson,
		type StreamVerificationResult
	} from '$lib/verification/verifier';

	interface Props {
		// The GetStreamVerification payload from the SvelteKit load (ADR-000014).
		data: StreamVerificationJson;
	}
	const { data }: Props = $props();

	// The verifier recomputes on THIS device (WebCrypto is browser-only), so we run
	// it in an effect and hold the result in state — never on the server.
	let result = $state<StreamVerificationResult | null>(null);
	let failed = $state(false);

	$effect(() => {
		const payload = data;
		let cancelled = false;
		result = null;
		failed = false;
		(async () => {
			try {
				const r = await verifyStream(payload);
				if (!cancelled) result = r;
			} catch {
				if (!cancelled) failed = true;
			}
		})();
		return () => {
			cancelled = true;
		};
	});

	const allOk = $derived(!!result && result.allLogHashesOk && result.contentChainOk);
	const okCount = $derived(result ? result.events.filter((e) => e.logHashOk).length : 0);
	const shortHex = (h: string) => (h.length > 16 ? `${h.slice(0, 8)}…${h.slice(-4)}` : h);
</script>

<section class="verify" aria-label="記録の確かめ">
	<h2 class="label">この記録を確かめる</h2>
	<p class="lede">
		S4RCIV が記録した内容から、改ざん検知用の値を <strong>お使いのブラウザで計算し直し</strong>、
		記録済みの値と一致するか確かめます。S4RCIV の「確認済み」表示を信じる必要はありません — 計算はこの端末で行われます。
	</p>

	{#if failed}
		<p class="status warn">確かめを実行できませんでした。時間をおいて再度お試しください。</p>
	{:else if !result}
		<p class="status">確かめています…</p>
	{:else if result.events.length === 0}
		<p class="status">この事案にはまだ記録がありません。</p>
	{:else}
		{#if allOk}
			<p class="status ok">
				✓ この事案の {result.events.length} 件の記録すべてが、記録どおりに再計算できました（この端末で確認）。
			</p>
		{:else}
			<p class="status warn">
				⚠ 一致しない箇所があります（再計算が一致したのは {okCount} / {result.events.length} 件）。下の内訳をご確認ください。
			</p>
		{/if}

		<ol class="records">
			{#each result.events as ev (ev.seq)}
				<li id="verify-{ev.seq}" class="record">
					<span class="recno mono">記録 #{ev.seq}</span>
					{#if ev.logHashOk}
						<span class="mark ok">✓ 一致</span>
					{:else}
						<span class="mark ng">✗ 不一致</span>
					{/if}
					{#if ev.contentLinkOk === false}
						<span class="mark ng">✗ 前の記録とのつながりが切れています</span>
					{/if}
				</li>
			{/each}
		</ol>

		<p class="note">
			※ いつの時点の記録かを外部（Internet Archive 等）に固定する「外部アンカー」はまだ運用していません。これは
			S4RCIV 自身が過去をまるごと書き換えた場合に備えるもので、今後対応します。全件を通した計算は、公開エクスポートを使えば
			S4RCIV 以外の場所でも再現できます。
		</p>

		<details class="tech">
			<summary>技術的な内訳</summary>
			<dl class="kv mono">
				<dt>ハッシュ方式</dt>
				<dd>{result.algVersion || '—'}</dd>
				<dt>チェックポイント</dt>
				<dd>
					{#if result.checkpoint.present}
						あり（through_seq {result.checkpoint.throughSeq}）/ 署名{result.checkpoint.signed
							? 'あり'
							: 'なし'}
					{:else}
						未生成（この事案は連鎖内整合のみ確認）
					{/if}
				</dd>
			</dl>
			<table class="hashes mono">
				<thead>
					<tr><th>#seq</th><th>再計算した log_hash</th><th>記録の log_hash</th></tr>
				</thead>
				<tbody>
					{#each result.events as ev (ev.seq)}
						<tr class:ng={!ev.logHashOk}>
							<td>{ev.seq}</td>
							<td>{shortHex(ev.recomputedLogHash)}</td>
							<td>{shortHex(ev.storedLogHash)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</details>
	{/if}
</section>

<style>
	.verify {
		margin: 28px 0;
		padding: 16px;
		background: var(--surface-1);
		border: 1px solid var(--hairline-2);
		border-radius: var(--r);
	}
	h2.label {
		display: block;
		margin: 0 0 8px;
	}
	.lede {
		margin: 0 0 12px;
		font-size: 13px;
		line-height: 1.7;
		color: var(--text-2);
	}
	.status {
		font-size: 14px;
		margin: 8px 0;
		color: var(--text-2);
	}
	.status.ok {
		color: var(--st-safe-t, var(--accent));
	}
	.status.warn {
		color: var(--st-caution-t);
	}
	.records {
		list-style: none;
		margin: 8px 0;
		padding: 0;
	}
	.record {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 10px;
		padding: 6px 0;
		border-bottom: 1px solid var(--hairline);
		scroll-margin-top: 80px;
	}
	.record:target {
		background: var(--surface-2, rgba(127, 127, 127, 0.08));
	}
	.recno {
		font-size: 12px;
		color: var(--text-3);
	}
	.mark {
		font-size: 12px;
	}
	.mark.ok {
		color: var(--st-safe-t, var(--accent));
	}
	.mark.ng {
		color: var(--st-critical-t);
	}
	.note {
		margin: 12px 0 0;
		font-size: 12px;
		line-height: 1.7;
		color: var(--text-3);
	}
	.tech {
		margin-top: 12px;
		font-size: 11px;
		color: var(--text-3);
	}
	.tech summary {
		cursor: pointer;
		color: var(--text-2);
	}
	.kv {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 2px 12px;
		margin: 8px 0;
	}
	.kv dt {
		color: var(--text-3);
	}
	.kv dd {
		margin: 0;
		color: var(--text-2);
	}
	.hashes {
		width: 100%;
		border-collapse: collapse;
		font-size: 11px;
	}
	.hashes th,
	.hashes td {
		text-align: left;
		padding: 3px 8px 3px 0;
		color: var(--text-3);
	}
	.hashes tr.ng td {
		color: var(--st-critical-t);
	}
</style>
