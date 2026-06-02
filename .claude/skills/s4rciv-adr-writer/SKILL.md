---
name: s4rciv-adr-writer
description: Writes an Architecture Decision Record for the s4rCiv project in Japanese after a decision or completed implementation. Authoring only — no deploy / no Pact / no container rebuild (s4rCiv has no deploy infra yet). Trigger when the user says "ADR書いて" / "ADRにまとめて" / "ADRに記録して" / "docs/ADR" 関連のまとめ依頼, or after a change that clearly warrants a decision record.
allowed-tools: Bash, Read, Glob, Grep, Edit, Write
---

# s4rCiv ADR Writer

s4rCiv の Architecture Decision Record を **日本語で執筆する**だけのスキル。
デプロイ・Pact・コンテナ再ビルドは **一切扱わない**（s4rCiv にデプロイ基盤がまだ無いため）。

実行は 2 ステップ:

1. **実装確認** (§1) — テスト可能なコードがあれば green を担保してから書く
2. **ADR 執筆** (§2) — `docs/ADR/NNNNNN.md` を日本語で追加する

---

## §1. 実装確認（コードがある場合のみ）

ADR が実装の決定記録なら、書く前に最低限のテストで動作を確認する。コードを伴わない
純粋な設計判断（例: イベントスキーマの採用方針）なら §1 はスキップして §2 へ。

| 変更の種類 | 最低限回すコマンド |
|---|---|
| Go component | `go test ./...` |
| Rust component | `cargo test` |
| TypeScript / Svelte (dashboard) | `pnpm run check && pnpm test` |
| Python (LLM 要約パイプライン等) | `uv run pytest`（型は `uv run pyrefly check`） |
| ドキュメント・scripts のみ | 該当テストだけ |

テストが落ちていたら ADR は書かず、原因を報告して止まる。ADR は「動いた実装の決定記録」
であり、憶測を書く場所ではない。

---

## §2. ADR 執筆

### 2.1 番号とテンプレート

```bash
ls docs/ADR/ | grep -E '^[0-9]{6}\.md$' | sort | tail -1   # 最新番号を確認
```

最新 +1 の 6 桁ゼロ埋め（最初の ADR は `000001`）をファイル名にする。
`docs/ADR/template.md` を Read で開き、そのセクション見出しをそのまま使う（勝手に増減しない）。

### 2.2 Frontmatter

| フィールド | 値の決め方 |
|---|---|
| `title` | 動詞始まりの行動指向の一文。ADR 番号は含めない |
| `date` | `YYYY-MM-DD`（当日） |
| `status` | 新規は `proposed`、合意済みなら `accepted`。過去 ADR を無効化する場合のみ `superseded` |
| `tags` | §2.4 の許可タグから最大 5 個 |
| `affected_services` | コンポーネント名と変更概要を 1 行/件で列挙（例: `kokkai-adapter (Rust)`） |
| `aliases` | `ADR-NNN` と `ADR-000NNN` の 2 形式を必ず両方入れる（wikilink 解決用） |

### 2.3 本文ルール

- **日本語で書く**。コンポーネント名 / コマンド / ライブラリ名 / ファイルパスは英語のまま。
- **セクション順は `template.md` を尊重**する。Status / Date / Affected Services / Context /
  Decision / Consequences (Pros, Cons/Tradeoffs) / Related ADRs の順が基本。
- **Context** は「なぜこの決定が必要だったか」を定量/定性の根拠とともに書く。計測結果が
  あれば数値を残す。
- **Decision** は採用した選択肢に加え、**検討した代替案と却下理由**を書く。これが後から
  読む人への最大の贈り物になる。s4rCiv の設計原則（passive / read-only、append-only
  hash-chain、plane 分離、AI は判定しない、source compliance）と矛盾しないことを確認する。
- **Consequences** は Pros と Cons/Tradeoffs を分けて列挙する。未解決の負債は Cons に書く。
- コードブロックは判断の根拠に必要な最小限にする。ロジックの羅列は git の diff で読める。
- **Related ADRs は wikilink `[[000NNN]] タイトル` 形式**で列挙する（`ADR-000NNN (タイトル)`
  形式は使わない）。`CORE_CONCEPT_0001.md` / `DISCIPLINE.md` への参照は通常リンクで良い。

### 2.4 許可タグ

```
architecture, clean-architecture, event-log, hash-chain, cqrs,
adapter, normalizer, diff, classification, projection, read-model,
akoma-ntoso, popolo, ocds, provenance, confidence,
llm-summary, dashboard, map, frontend, backend, api,
database, migration, security, compliance, rate-limit, robots-txt,
testing, ci-cd, observability, docker
```

この外のタグを増やしたくなったら、ADR ではなく先に `CLAUDE.md`（このスキルの §2.4）を更新する。

### 2.5 情報衛生（DISCIPLINE 準拠）

s4rCiv は AGPL-3.0 の公開プロジェクト。`DISCIPLINE.md` の anti-doxxing と整合させ、以下を含めない:

- 本番 IP / 本番ドメイン / 秘匿ポート
- 資格情報・API キー・シークレット類（機微情報は Docker secrets 側）
- 私的サーバー名
- **私人の個人情報**（s4rCiv の対象は常に説明責任を負う public actor。私人を profile しない）

`localhost:XXXX` と compose サービス名は OK。公的記録の主体（議員・省庁等）の固有名は
題材として当然 OK だが、私人を巻き込まない。

### 2.6 書き込み

Write ツールで `docs/ADR/NNNNNN.md` を作る。heredoc や `cat > ...` は使わない。
書き込み後に Read で自分の出力を読み返し、見出し / frontmatter / wikilink 形式を確認する。

---

## §3. 完了報告

ユーザに以下を伝える:

- 書いた ADR のパス（`docs/ADR/NNNNNN.md`）とタイトル
- §1 を実行したなら、緑だったテスト
- commit するかどうかはユーザに確認する（このスキルは commit / push を勝手に行わない）

---

## 参照

- `docs/ADR/template.md` — セクションと frontmatter のソース
- `docs/concepts/CORE_CONCEPT_0001.md` — 設計の単一の真実。ADR は本ドキュメントに従属する
- `DISCIPLINE.md` — 禁止事項（情報衛生・anti-doxxing の根拠）
