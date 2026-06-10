---
title: 公開タイムラインが read model の node_changes=null（JSON scalar）で SQLSTATE 22023 を起こし全断した
date: 2026-06-09
status: resolved
severity: SEV2
authors:
  - Kaikei-e
tags:
  - read-model
  - projection
  - diff
  - database
  - api
---

# Postmortem: 公開タイムラインが read model の node_changes=null で SQLSTATE 22023 を起こし全断した

## 概要

公開タイムラインが「⊘ 取得に失敗しました」だけを返し全断した。`interpretation.change` の 1 行で `node_changes` が JSON `null`（scalar）になっており、`ListTimeline` がそれを `jsonb_array_elements` で展開して `SQLSTATE 22023`（`cannot extract elements from a scalar`）を起こし、SELECT リスト内の式だったためクエリ全体が失敗していた。根本原因は egov-differ が構造差分 0 件のとき Go の nil スライスを `json.Marshal` し、`[]` ではなく `null` を直列化していたこと。観測面（ground truth）は無傷で、producer 修正 + consumer ガード + reproject で復旧済み。

## 影響

- **observation 面（ground truth）**: 無事。生スナップショットもハッシュ連鎖イベントも一切変更・欠落していない。問題は read 専用クエリと、観測面から導出された disposable read model の直列化の 1 行に閉じていた。
- **interpretation 面（read model）**: `interpretation.change` 9 行中 1 行（`observation_seq=2307`、egov-law `327R00000001003`）の `diff.node_changes` が `null`。reproject で再計算でき、実際に `[]` へ正規化して復旧した。
- **利用者・対象ソース**: 公開タイムライン（横断タイムライン画面 / Atom フィード）が全断。`ListTimeline` を叩く全リクエストが 502 を返していた。kokkai / egov-law の収集（collector）と観測面は影響なし。
- **継続時間**: 約 11〜12 時間（発生 2026-06-09 05:03 JST 頃 〜 復旧 16:47 JST 頃）。発生時刻は毒行の元イベント `observed_at`（2026-06-08 20:03 UTC = 06-09 05:03 JST）からの推定で、egov-differ は約 10 秒間隔で projection するため、その直後に毒行が書かれて以降 `ListTimeline` が失敗していたとみられる。

## タイムライン（JST）

ログは Go アプリが UTC で出力するため JST に換算。

- `2026-06-09 05:03` egov-law `327R00000001003` の `ResourceChanged`（seq 2307）を観測。直後に egov-differ が構造差分 0 件の change 行を `node_changes:null` で書き込み、以降 `ListTimeline` が 22023 で失敗（発生・推定）。
- `2026-06-09 12:19` 頃 api ログに `ListTimeline failed: internal: ERROR: cannot extract elements from a scalar (SQLSTATE 22023)` が約 10 秒間隔で継続記録（事後にログで確認できた最古の連続失敗。これ以前から発生していた可能性あり）。
- `2026-06-09 16:4x` 利用者が「⊘ 取得に失敗しました」を踏み報告（検知）。
- `2026-06-09 16:4x` 原因特定: web の汎用 502 文言の裏で api が 22023 を返していること、毒行が `node_changes:null` の 1 行であることを特定。
- `2026-06-09 16:45` migration `20260603000017`（UNIQUE + index 整理）を本番 DB に適用。
- `2026-06-09 16:47` 新イメージで api を再起動 → タイムライン 200 復帰（consumer ガードで毒行を無害化）。
- `2026-06-09 16:50` egov-differ デーモンを停止し egov-law を reproject（truncate-less + upsert）。seq 2307 を `[]` に正規化、デーモン再開。
- `2026-06-09 16:53` 検証完了: change 9 行・重複 0・null 0・タイムライン 200。

## 根本原因

1. なぜタイムラインが全断したか → `ListTimeline` が `interpretation.change` の 1 行で `SQLSTATE 22023` を起こし、それが SELECT リスト内の式だったためクエリ全体が失敗したから。
2. なぜ 22023 が起きたか → `jsonb_array_elements(c.diff->'node_changes')` を、配列でなく JSON `null`（scalar）の値に対して呼んだから。Postgres は scalar の要素展開を拒否する。
3. なぜ `node_changes` が `null` だったか → egov-differ が構造差分 0 件（article/clause レベルの変化が無い administrative な内容変更）のとき、Go のスライスが `nil` のまま `json.Marshal` され、nil スライスが `[]` ではなく `null` に直列化されたから。
4. なぜ nil スライスのまま直列化されたか → `diffJSON.NodeChanges` を空配列で初期化しておらず、かつ「差分 0 件」のケースを覆うテストが無かったから（既存テストは差分 1 件のケースのみを検証していた）。
5. なぜ 1 行の不正で全体が落ちる構造だったか → 差分カウントが SELECT リストの相関サブクエリにあり、read model の不正行に対する型ガード（scalar 耐性）が無かったから。

**根本原因**: egov-differ が「構造差分 0 件」を JSON `null`（scalar）で直列化し、それを read 時に `jsonb_array_elements` で展開する `ListTimeline` が scalar 非耐性だったため、disposable read model の 1 行がタイムライン全体を落とした。

## 検知

利用者が公開タイムラインで「⊘ 取得に失敗しました：一時的に取得できませんでした。時間をおいて再度お試しください。」を踏み、報告したことで気付いた。これは `web/src/lib/server/errors.ts` が NotFound 以外の RPC 失敗を一律 502 + 汎用文言に潰す設計（上流メッセージを漏らさない CWE-209 対策）どおりの挙動で、利用者には原因が見えない。一方で api 側は実エラー（22023）をサーバログに記録していたが、**そのエラー率を監視するアラートが無く**、毒行が書かれてから人が踏むまで約 11 時間、無検知のまま全断が続いた。検知ギャップは「監視の不在」であって「ログの不在」ではない。

## 対応と復旧

3 層を一体で修正した（[[000024]]）。

- **producer（根本原因）**: egov-differ が `node_changes` を常に配列で直列化するよう、空でも `[]nodeChangeJSON{}` で初期化（`internal/usecase/diff/diff.go`）。
- **consumer（多重防御）**: `ListTimeline` の 3 本の相関サブクエリを 1 本の `LEFT JOIN LATERAL` + `count(*) FILTER` に集約し、`jsonb_typeof(...)='array'` でない `node_changes` を空配列扱いするガードを 1 箇所に集約（`internal/driver/postgres/timeline.go`）。read model の 1 行が壊れてもクエリ全体は落とさない。
- **データ正規化 + 構造の堅牢化**: migration `20260603000017` で `interpretation.change` に `UNIQUE(observation_seq)` を追加、`ApplyChange` を upsert 化、`BeginRebuild` を truncate-less 化（[[000022]] の読者原子的 reproject に egov 差分 projector を揃える）。その上でデーモン停止下で egov-law を reproject し、seq 2307 を `null` → `[]` に再計算した。

復旧は consumer ガードを載せた api 再起動の時点で完了し（毒行が無害化されタイムライン 200）、reproject はデータを正しい表現に揃える後始末として行った。**手書き SQL で該当行を直接 UPDATE する案は却下**した。disposable read model を観測ログ経由せず直接 mutate することになり、根本原因（producer の nil→null）も残るため、producer 修正 + reproject の方が plane 分離の原則に沿う。

## ふりかえり（blameless）

- **うまくいったこと**: plane 分離が効いた。観測面が不変だったため、毒データはあくまで「再計算できる導出物の直列化ミス」に閉じ、ground truth を疑う必要が無かった。consumer ガードだけで再ビルド + 再起動から数十秒で全断を止められた。
- **まずかったこと（仕組みの話）**: (1) Go の nil スライスが JSON `null` になる挙動に対し、差分 0 件のケースを覆うテストが無かった。(2) read model の 1 行の不正がクエリ全体を落とす構造（SELECT リスト内 set-returning 関数 + 型ガード無し）だった。(3) api の内部エラー率にアラートが無く、検知が利用者報告任せだった。
- **幸運だったこと**: 毒行が「差分 0 件」という、表示上は `[]` と等価な無害な内容だった。観測面が不変なので、データ自体は最初から正しく、失われたのは可読性だけだった。

## 再発防止アクション

| 内容 | 担当 | 種別 | 状態 | 追跡 |
|---|---|---|---|---|
| egov-differ が `node_changes` を空でも `[]` で直列化（producer 修正） | Kaikei-e | 恒久対策 | done | diff.go + 回帰テスト（[[000024]]） |
| `ListTimeline` を非配列 `node_changes` に耐性化（LATERAL + CASE ガード） | Kaikei-e | 恒久対策 | done | timeline.go + postgres 統合テスト |
| `interpretation.change` に UNIQUE(observation_seq) + upsert + truncate-less reproject | Kaikei-e | 恒久対策 | done | migration 20260603000017 / change.go |
| api の内部エラー率（5xx / Connect internal）にアラートを入れ、検知を利用者報告に依存しない | TBD | 改善 | todo | （監視基盤の整備と一体・未着手） |
| read model 由来の SQL で set-returning 関数を使う他の箇所に同種の scalar 非耐性が無いか棚卸し | TBD | 改善 | todo | 下記「再発確認」で初回確認済み |

## 再発確認

同じ「read model の JSON を `jsonb_array_elements` 等で展開する」パターンを棚卸しした。`node_changes` を SQL で配列展開しているのは `ListTimeline`（timeline.go）のみで、ここは本対応でガード済み。law 詳細側（lawquery.go）は `node_changes` を Go の `json.Unmarshal` で読むため、JSON `null` は nil スライスになりエラーにならない。他の projector（kokkai meeting / law / roster / vote）に nil スライス→`null` の直列化が無いかは未棚卸しで、上表の改善アクションに残す。

## 教訓

- **disposable projection は「捨てて再計算できる」が、read 経路はその projection の不正に耐えねばならない。** 1 行の不正データがクエリ全体（=画面全体）を落とす構造は、それ自体が可用性の脆さ。set-returning 関数を read model に当てるときは型ガードを既定にする。
- **言語の「空」と JSON の「空」を取り違えない。** Go の nil スライスは `[]` ではなく `null`。projection の直列化は「空でも配列」を明示初期化し、空ケースをテストで固定する。
- **plane 分離は障害の重大度を下げる。** observation 面が不変な限り、interpretation 面の事故は reproject 可能性に帰着し、ground truth を疑わずに済む（[[000002]]）。
- **汎用エラー文言（CWE-209 対策）は利用者から原因を隠すが、その分サーバ側の検知（エラー率アラート）が無いと無検知の窓が伸びる。** 文言の秘匿と検知は別々に手当てする。

## 関連

- [[000024]] egov-law 差分 read model の空 node_changes を [] で直列化し timeline を scalar 耐性化・reproject を読者原子的にする（本インシデントの恒久対策）
- [[000022]] 解釈面 read model の reproject を TRUNCATE 廃止の upsert 上書き replay にして読者原子的にする
- [[000002]] 解釈面を二層化し diff・分類を観測面に焼かず reproject-safe にする
- [[000006]] 横断タイムライン（落ちた `ListTimeline` の出自）
- 修正コミット: 本 postmortem と同じ変更群（[[000024]] 参照。sha はコミット時に確定）
- [DISCIPLINE.md](../../DISCIPLINE.md) / [CORE_CONCEPT](../concepts/CORE_CONCEPT_0001.md) — 設計原則④（plane 分離）・データ整合性
