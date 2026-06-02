# Violation Examples — Case Studies and Fixes

違反パターンの **症状 → 該当原則 → 是正** テンプレ集。

s4rCiv はまだ concept stage で実ケースが無いため、以下は event schema
（`CORE_CONCEPT_0001.md` §9.1）と `DISCIPLINE.md` から導いた **概念例**。
実装が進んだら実在のレビュー / ポストモーテムから anonymized したケースで
差し替え・追加していく（末尾「育て方」参照）。

## Case 1: Reproject-unsafe projector — latest を引いて投影してしまう

### 症状

要約 projector が、event payload の `summary_version_id` を使わず、
`resource_id` から `GetLatestSummaryVersion()` を呼んでいる。replay 時に
「古い event が新しい summary 内容で投影される」。

### 該当原則

- **Reproject-safe projector** — projector が latest state に依存
- **Versioned artifacts** — event が stable な version id を参照していない

### 是正

1. event payload に `summary_version_id` を含めることを契約として固定
2. summary version port に `GetSummaryVersionByID(id)` を追加
3. projector を `event.summary_version_id` で読むよう変更
4. `GetLatestSummaryVersion()` 依存を projector から外す
5. test に「旧 version event を replay しても旧 excerpt が使われる」を追加

### 一般化

「projector に `Get<Latest>` / `Find<Active>` を呼ぶ箇所があれば、ほぼ常に
reproject-unsafe」。event payload に stable id を持たせ、projector はそれを直接読む。

---

## Case 2: Decontextualized diff — 周辺文脈 / provenance なしで diff を保存

### 症状

`ResourceChanged` の projection（timeline read model）が diff 断片だけを保持し、
周辺文脈・元文書への `payload_ref`・`confidence` を持っていない。UI はその断片を
単独で表示し、何がどの条文の話か追跡できない。

### 該当原則

- **Why as first-class** — 変更の根拠（前後文脈・出典）が構造化保持されていない
- `DISCIPLINE.md` — **decontextualized diff を提示しない** / provenance 必須に違反

### 是正

1. event / projection に `payload_ref`（元 snapshot）と周辺文脈範囲を必須化
2. diff は必ず full-text link + `confidence`（PDF/OCR 由来は `low`）と同梱
3. UI は diff 単独を出さず、周辺文脈と出典リンクを併記する契約にする
4. contract test で「文脈・出典なしの diff は projection に入らない」を固定

### 一般化

「結論（diff / 要約 / 分類）だけ」を保持する設計は s4rCiv では原則違反。
常に **出典 + 周辺文脈 + confidence** を payload first-class に持たせる。

---

## Case 3: 消失を行削除で表現してしまう

### 症状

ソースから資料が消えたとき、resource 行を `DELETE` して「消えた」を表現。
いつ消えたか・後で復活したかが追えず、hash-chain にも観測事実が残らない。

### 該当原則

- **Append-first event log** — 事実（消失）が失われ、INSERT-only が破れている
- **Resource / Event 分離** — 消失という event を行削除で隠している

### 是正

1. 消失は `ResourceVanished` event として append（沈黙も情報）
2. 復活は `ResourceRestored` event として append
3. resource snapshot は event の集約として再構築（直接削除しない）
4. test に「消失 → 復活」の replay で履歴が完全復元されることを追加

### 一般化

「無くなった」「戻った」を行の有無で表すのは hidden event。すべて event 種別で表現する。

---

## Case 4: Merge-unsafe upsert — snapshot 上書きで負数 / loss

### 症状

projection の集計列を event 差分 (delta) ではなく snapshot 上書きで更新。
並行 event があると COALESCE すべき列が NULL に戻り、`current - 1` で
counter が負数になりうる。

### 該当原則

- **Merge-safe upsert** — monotonic + COALESCE が守られていない
- **No business logic in SQL** — 集計判定が SQL CASE に依存

### 是正

1. UPSERT を `GREATEST(0, current + delta)` 形式に変える
2. 既存の他列は `COALESCE(EXCLUDED.x, table.x)` で保持
3. monotonicity を `WHERE EXCLUDED.seq > table.seq_hiwater` で担保
4. business 判定は Go / Rust 側で行い、SQL は単純な merge に留める

### 一般化

「同じ projection 行に複数 projector が触る」「event 順序が前後しうる」場合は、
必ず `GREATEST(0, current + delta)` + `seq_hiwater` ガード。

---

## レビュー Findings の書き方サンプル

SKILL.md の出力テンプレを埋めると次のようになる:

```markdown
## Immutable Design Findings

### 1. [high] reproject 時に latest summary が読まれて古い event が新しい内容で投影される
- 該当箇所: `<adapter>/app/job/summary_projector.go:128`
- 破っている原則: Reproject-safe projector / Versioned artifacts
- なぜ危険か: replay 順序が変わると read model 結果が変わる。
- 代替案:
  1. event payload に `summary_version_id` を必須化
  2. projector を `summary_version_id` 直読に変更
  3. `GetLatestSummaryVersion()` 依存を projector から外す
- 既知の類例: violation-examples.md Case 1
```

---

## このリストの育て方

新しい違反パターンを発見したら、上のテンプレに沿って追加する:

- 症状 (実例から anonymized)
- 該当原則 (`s4rciv-invariants.md` の名前で参照)
- 是正手順
- 一般化 (他コンポーネントでも適用できる教訓)
- 一次出典 (s4rCiv の review / postmortem / ADR への参照)

[← back to SKILL.md](../SKILL.md)
