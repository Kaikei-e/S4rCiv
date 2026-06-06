---
title: egov-law の reproject が重複 eId で停止し read model が再生成できなかった
date: 2026-06-06
status: monitoring
severity: SEV2
authors:
  - Kaikei-e
tags:
  - reproject
  - egov-law
  - projection
  - eid
---

# Postmortem: egov-law の reproject が重複 eId で停止し read model が再生成できなかった

## 概要

egov-law の `collector reproject` が、法令 508CO0000000106 の投影中に
`law_node UNIQUE(law_id, eid)` 違反（SQLSTATE 23505）で停止し、replay が完走
しなかった。法令パーサが 1 つの版の中で同じ eId を二度生成し、read model への
batch INSERT が重複キーに当たったのが原因。観測面（ground truth）は不変で無事。
解釈面の egov-law read model は失敗した reproject で truncate されたまま残り、
パーサ修正のコミット後、本番での reproject 再実行による復旧を待っている。

## 影響

- **observation 面（ground truth）**: 無事。スナップショットもハッシュ連鎖も触れていない（reproject は解釈面のみを再生成する）。
- **interpretation 面（read model）**: `interpretation.law_node` / `legislative_work` が、失敗した reproject の冒頭 truncate により空のまま。egov-law の法令ページ・タイムラインの法令項目が劣化。reproject の再実行で復旧可能（read model は disposable）。
- **対象**: egov-law ソースのみ。国会会議録・投票など他ソースの read model は無関係。
- **継続時間**: 最初の失敗（ユーザ報告）から、修正コミット `d61ad90`（2026-06-06 23:17 JST）まで。本番 read model の復旧は reproject 再実行まで継続中。

## タイムライン（JST）

<!-- collector のコンテナログは UTC。JST に換算して記載。 -->

- `2026-06-06`（時刻不明） reproject 実行が重複キーで停止 → ユーザが検知・報告
- `2026-06-06 23:01` 原因再現のため reproject を実行（`project seq 1742: ... law_node_eid_unique (SQLSTATE 23505)`）。この再現で read model が再度 truncate された（観測面は不変なので復旧可能）
- `2026-06-06`（同セッション内） 根本原因を特定（パーサが 1 版内で同一 eId を生成）
- `2026-06-06 23:17` 修正コミット `d61ad90`（パーサ側で eId を一意化）。`go test ./...` 全 green
- `（未実施）` 本番で reproject 再実行 → read model 復旧の確認

## 根本原因

1. なぜ reproject が止まったか → ある法令版の `b.Nodes` に同じ `(law_id, eid)` が二つあり、`replaceLawNodes` の batch 内 INSERT が `UNIQUE(law_id, eid)` に当たった。
2. なぜ同じ eId が二つできたか → `ParseLawXML` は eId を構造の番号から組み立てる（`art_<Num>__para_<Num>__item_<Num>…`）が、入力 XML が同じ番号を二度持つケース（重複した Article Num、同番号の号など）があり、一意性を保証していなかった。
3. なぜそれが投影まで素通りしたか → eId の一意性は `ADR-000013` の eId 契約で**仕様としては**定められていたが、パーサの**実装が enforce していなかった**。`law_node UNIQUE(law_id, eid)` 制約は最後の防波堤として正しく働いたが、それは投影時の失敗としてしか現れなかった。
4. なぜ reproject 特有に見えたか → `replaceLawNodes` は版ごとに DELETE→INSERT する（current-tree-only）。重複は版間ではなく**1 版の batch 内**。reproject は全版を replay するため、重複版を持つ法令に到達した時点で必ず停止した。

**根本原因**: 法令パーサが 1 つの版の中で eId の一意性を保証しておらず、観測スナップショットは正しいのに解釈面の derivation が total（全入力を決定的に投影しきる）でなかった。

## 検知

ユーザが `reproject` を実行した際の異常終了で検知。アラートではなく手動実行で判明した。検知ギャップ: reproject-safety の integration テスト（`d222fdb`）は存在したが、**重複 eId を持つ法令スナップショットを covering していなかった**ため、この class を事前に捕まえられなかった。

## 対応と復旧

- **修正（`d61ad90`）**: パーサ側で一意化。`builder.uniq` が衝突時に決定的な `~N` 接尾辞を付け、一意化した eId を**子の prefix と ParentEID に thread** するので親子連結は保たれる。同一スナップショットは常に同一 eId を生み、reproject は決定的。
- **層の選択（不変条件に照らした判断）**: projector 側で `ON CONFLICT (law_id, eid) DO UPDATE` する逃げは採らなかった。eId は版を跨ぐ安定キー（Change の diff キー）であり、別ノードを黙って 1 つに潰すとデータ欠落と diff 破壊を招き、正しいガードである UNIQUE 制約を無効化するため。一意化は anti-corruption 層（パーサ）の責務とした。
- **回帰テスト**: 重複 Article Num＋同番号の号を持つ XML で、eId の一意性と親子連結の保全を固定（`law_test.go`）。`go test ./...` 全 green。
- **本番復旧（ユーザ操作・未実施）**:
  ```bash
  docker compose -f compose.yaml -f compose.tunnel.yaml build collector
  docker compose -f compose.yaml -f compose.tunnel.yaml run --rm --no-deps collector --source egov-law reproject
  ```

## ふりかえり（blameless）

- **うまくいったこと**: `UNIQUE(law_id, eid)` 制約が最後の防波堤として機能し、誤ったデータを黙って投影せず**早く失敗**した。観測面と解釈面の分離により、ground truth は一切損なわれなかった。
- **まずかったこと（仕組みの話）**: eId 契約（ADR-000013）が仕様にあったのに実装で enforce されておらず、reproject-safety テストもこの入力 class を覆っていなかった。
- **幸運だったこと**: read model は disposable で、観測面が不変だったため、復旧は reproject の再実行のみで済む（データ復元作業は不要）。

## 再発防止アクション

| 内容 | 担当 | 種別 | 状態 | 追跡 |
|---|---|---|---|---|
| パーサで eId を決定的に一意化（`builder.uniq`） | Kaikei-e | 恒久対策 | done | `d61ad90` |
| 重複 eId の回帰テスト追加 | Kaikei-e | 恒久対策 | done | `d61ad90`（`law_test.go`） |
| 本番で egov-law を reproject し read model を復旧 | Kaikei-e | 恒久対策 | todo | 上記コマンド |
| 重複 eId スナップショットを seed する projector の integration テスト追加（検知ギャップを埋める） | TBD | 改善 | todo | — |

## 再発確認

同型（版ごとに DELETE→INSERT し、source 由来のキーで一意性を仮定する）の projector は他にもある（speech・vote・roster・sangiin・change）。それらは source 提供のキー（issue/seq, person, vote_event 等）に依存し、source が同一キーを重複させれば同じ 23505 class が起こり得る。eId のように**構造から derive するキー**を持つのは法令のみで、そこは本修正で閉じた。他 projector で 23505 が出た場合は同じ視点（batch 内重複か）で見る。

## 教訓

- **reproject-safe projector**: projector は不変スナップショットから**全入力を決定的に**投影しきれねばならない。一意性などの read-model 制約は、それを供給する側（パーサ/gateway = anti-corruption 層）で保証する。制約違反を投影時の逃げ（silent upsert）で回避すると、別の不変条件（データ保全・diff の意味）を壊す。
- **契約は実装で enforce する**: ADR で定めた契約（eId の一意性）は、コードでも保証し、回帰テストで固定して初めて守られる。

## 関連

- [[000005]] e-Gov 法令 API v2 adapter / AKN
- [[000013]] 用語定義・号細分の eId・文連結契約
- 修正コミット: `d61ad90`
- [DISCIPLINE.md](../../DISCIPLINE.md) / [CORE_CONCEPT](../concepts/CORE_CONCEPT_0001.md) §4（二面分離・append-only）
