# Postmortem template (s4rCiv)

この見出しと順序をそのまま使う。`<...>` を埋め、不要な補足コメント（`<!-- -->`）は消す。
blameless（主語は仕組み・条件、個人を責めない）・平易・事実ベースで書く。

## Contents
- 概要 / 影響 / タイムライン / 根本原因 / 検知 / 対応と復旧 /
  ふりかえり / 再発防止アクション / 再発確認 / 教訓 / 関連

---

```markdown
---
title: <一行サマリ（動詞含む。例: egov-law の reproject が重複 eId で停止した）>
date: <YYYY-MM-DD>
status: resolved        # resolved | monitoring | investigating
severity: <SEV1|SEV2|SEV3>
authors:
  - <name>
tags:
  - <例: reproject, egov-law, projection>
---

# Postmortem: <タイトル>

## 概要

<2–3 文。何が起きたか・なぜか・影響と継続時間・現状（解決済みか）。>

## 影響

- **observation 面（ground truth）**: <無事か／欠落・連鎖切れの有無。最優先で判定。>
- **interpretation 面（read model）**: <劣化内容と reproject で復旧可能かどうか。>
- **利用者・対象ソース**: <どの画面/ソース/期間が影響を受けたか。>
- **継続時間**: <発生〜復旧（JST）。不明なら「不明」。>

## タイムライン（JST）

<!-- ログが UTC なら JST に換算し、その旨を1行添える。 -->

- `YYYY-MM-DD HH:MM` <出来事（発生）>
- `YYYY-MM-DD HH:MM` <検知>
- `YYYY-MM-DD HH:MM` <原因特定>
- `YYYY-MM-DD HH:MM` <修正コミット <sha>>
- `YYYY-MM-DD HH:MM` <復旧確認>

## 根本原因

<Five Whys で寄与原因まで辿る。主語は仕組み・条件にする。観測面は不変で正しいのに
解釈面の derivation 等がどこで破れたか、を s4rCiv の語彙で書く。>

1. なぜ <事象> が起きたか → <…>
2. なぜ <…> → <…>
3. （根本原因に至るまで）

**根本原因**: <一文で。>

## 検知

<どう気づいたか（ユーザ報告 / アラート / テスト）。検知が遅れた/取りこぼした
仕組み上のギャップがあれば書く（例: 既存テストがこのケースを覆っていなかった）。>

## 対応と復旧

<実施した修正（コミット <sha>）。なぜその層で直したか（不変条件に照らした判断）。
復旧手順（reproject 等）。回避した代替案と却下理由があれば一行。>

## ふりかえり（blameless）

- **うまくいったこと**: <…>
- **まずかったこと（仕組みの話）**: <…>
- **幸運だったこと**: <…（例: 観測面が不変なのでデータ自体は無事だった）>

## 再発防止アクション

| 内容 | 担当 | 種別 | 状態 | 追跡 |
|---|---|---|---|---|
| <恒久対策の具体> | <name/TBD> | 恒久対策 | done/todo | <commit / issue> |
| <検知ギャップを埋める> | <name/TBD> | 改善 | todo | <…> |

## 再発確認

<同じ根本原因の別事象が無いか（他ソース・他 projector 等）。確認結果を1–2行。>

## 教訓

<次に同種を防ぐための一般化。該当する不変条件（reproject-safe projector,
disposable projection, append-first 等）や設計原則を名前で引く。>

## 関連

- [[000NNN]] <関連 ADR>
- 修正コミット: `<sha>`
- [DISCIPLINE.md](../../DISCIPLINE.md) / [CORE_CONCEPT](../../docs/concepts/CORE_CONCEPT_0001.md) <該当節>
```
