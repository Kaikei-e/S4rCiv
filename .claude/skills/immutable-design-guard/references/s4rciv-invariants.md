# s4rCiv Immutable Data Model — Canonical Invariants

s4rCiv の append-only hash-chained event log（observation plane）とそこから派生する
projection / read model（interpretation plane）を持つすべての領域に通用する正規不変条件。
SKILL.md のコア原則 1 行リストの拡張定義。

> 一次出典: `docs/concepts/CORE_CONCEPT_0001.md` §9.1（event envelope schema）および
> `DISCIPLINE.md`（データ整合性・anti-doxxing・decontextualized diff 禁止）。
> envelope: `event_id`(uuidv7) / `stream_id` / `seq` / `content_hash` /
> `prev_content_hash` / `log_prev_hash` / `payload_ref` / `diff` / `confidence`。
> event 種別: `ResourceObserved` / `ResourceChanged` / `ResourceVanished` / `ResourceRestored`。

各項目は次の形で書く。

- **何を言っているか** (1-2 行)
- **典型的な違反**
- **満たす実装パターン**

## Contents

1. Append-first event log（+ hash-chain integrity）
2. Resource / Event separation
3. Event-time purity
4. Reproject-safe projector
5. Disposable projection
6. Versioned artifacts
7. Merge-safe upsert
8. Single emission
9. Dedupe is ingest-only
10. Why as first-class

---

## 1. Append-first event log（+ hash-chain integrity）

**何を言っているか**: 状態遷移は event の追記のみで表現する。event store
テーブルは INSERT only。事実は失われない。さらに s4rCiv の event log は
**hash-chained**（`content_hash` / `prev_content_hash` / `log_prev_hash`）であり、
過去を書き換えていないことを証明できる tamper-evident な台帳である。

**典型的な違反**:
- `UPDATE … events SET payload_ref = …` / `DELETE FROM … events WHERE …`
- 削除・消失を新 event (`ResourceVanished`) ではなく行削除で表現する
- soft-delete を `deleted_at` 列で event store に持つ
- event の payload を後から書き換える「修正」運用
- `prev_content_hash` / `log_prev_hash` を再計算してチェーンを「直す」操作
  （= 履歴の書き換え。観測の不可変性を破壊する）

**満たす実装パターン**:
- 取り消し / 訂正 / 復活は `ResourceChanged` / `ResourceRestored` などの新 event
- `content_hash` の連結（前 event の hash を参照）で改竄検知
- `seq`（uuidv7 + 単調連番）で ordering 担保
- 観測の欠落・消失も情報として `ResourceVanished` に記録（沈黙も情報）

## 2. Resource / Event separation

**何を言っているか**: kawasima 理論に従い、resource (日時を持たない名詞)
と event (日時を持つ動詞) を分離する。resource に `updated_at` を生やしたく
なったら、抽出されていない hidden event がある。

**典型的な違反**:
- 全テーブルへの機械的な `created_at` / `updated_at`
- `XxxUpdated` という generic event 名で意味の異なる変更を 1 つに押し込む
- resource 行を上書きして履歴を捨てる
- `status` 列だけで業務遷移を表現する

**満たす実装パターン**:
- 業務的に複数の意味を持つ観測は別 event 種別に切り分け
  (例: `ResourceChanged`（内容変更）/ `ResourceVanished`（消失）/
  `ResourceRestored`（復活））
- resource（正規化エンティティ）は append-only event を集約した snapshot
- 更新が業務的に存在しない resource は「日時なし」のまま

## 3. Event-time purity

**何を言っているか**: business-fact を表す時刻は event 由来（観測時刻 /
ソース文書の発効時刻）から派生させる。projector / read model 内で
`time.Now()` / `Utc::now()` / `NOW()` を business fact に使わない。

**典型的な違反**:
- projector が `time.Now()` で「鮮度」フィールドを埋める
- `NOW()` を SQL の DEFAULT にし projector が業務時刻として読む
- 観測時刻が triggering event の時刻ではなく projector の wall-clock

**満たす実装パターン**:
- projection field: `observed_at = MAX(event.occurred_at)`,
  `source_event_seq`, `projection_seq_hiwater`, `projection_revision`
- wall-clock は **debug 専用の `projected_at`** のみ。API / public view /
  metrics label に出さない
- 各 record の fetch timestamp と attribution は envelope に保持（principle 7）

## 4. Reproject-safe projector

**何を言っているか**: projector は event payload と stable な versioned
resource (immutable な参照) のみから read model を再構築できなければならない。
latest state や active projection を読まない。

**典型的な違反**:
- projector 内で `GetLatestSummary(resource_id)` を呼ぶ
- projector 内で active projection 行を読み戻して merge する
- replay 順序が変わると結果が変わる
- swap 後に checkpoint をリセットせず gap を生む

**満たす実装パターン**:
- event payload に **stable な version id** を持たせる
  (例: `summary_version_id`, `classification_version_id`)
- projector は event の指す version をそのまま読む
- reproject の activation と checkpoint reset を不可分に
- replay 冪等性をテストする: 同一 event 列を任意順序で食わせても結果一致

## 5. Disposable projection

**何を言っているか**: interpretation plane の projection / read model は捨てて
event log から再構築できる。observation plane を source of truth とし、
projection を昇格させない。write path から projection を直接 mutate しない。

**典型的な違反**:
- write path (handler / usecase) が projection table を直接 UPDATE
- projection の値を event 復元せずに直接修正する管理スクリプト
- backfill 系 batch が event を経由せず projection を直接書く
- LLM 要約を event に残さず projection に直書きして再現不能にする

**満たす実装パターン**:
- 全変更は event → projector → projection の 1 方向
- projection rebuild の runbook が存在し、定期的に検証
- shadow projection を作って差分検証してから swap

## 6. Versioned artifacts

**何を言っているか**: LLM summary / classification など、後から内容が変わるもの
は version append のみで表現する。event は stable な version id を参照する。

**典型的な違反**:
- `summaries` テーブルを上書き更新
- 新 summary を作っても event は `resource_id` だけ参照
- "latest" を意味する flag 列を mutable に維持
- `summary_versions` の row を後から書き換える

**満たす実装パターン**:
- `summary_versions` / `classification_versions` 等が append-only
- event payload に `<artifact>_version_id` を必ず含める
- "latest" は projection 側の disposable view で表現
- prompt / model / mapping を変えるときは version を bump して full reproject

## 7. Merge-safe upsert

**何を言っているか**: projection の更新は monotonic + COALESCE 保持。
event 差分による増減算で表現し、SQL 内に business 判定を入れない。

**典型的な違反**:
- `UPDATE projection SET counter = current - 1` で負数発生
- SQL `CASE WHEN status='X' THEN …` で business logic を SQL に逃がす
- snapshot 上書きで COALESCE すべき値を NULL に戻す

**満たす実装パターン**:
- `GREATEST(0, current + delta)` で負数防止
- `COALESCE(EXCLUDED.x, projection.x)` で既存値を保持
- monotonicity を `seq_hiwater` guard で担保
  (`WHERE EXCLUDED.seq > projection.seq_hiwater`)
- business 判定は Go / Rust / Python 側で行い SQL は単純な merge に留める

## 8. Single emission

**何を言っているか**: 1 回の観測サイクルで同一 resource state に対し冗長な
event を出さない。内容が変わっていなければ `ResourceChanged` を出さない
（必要なら `ResourceObserved` のみ、あるいは何も出さない）。

**典型的な違反**:
- 同一 fetch で内容無変化なのに `ResourceChanged` を発行する
- retry / 再クロールで同じ変化 event が再発火し double count
- 1 つの変化を複数 event 種別に分けて二重計上する

**満たす実装パターン**:
- `content_hash` 比較で変化検出（hash 一致なら観測のみ or no-op）
- `(source, stream_id, content_hash)` を idempotency 単位にする
- fast-path dedupe + event store unique index で slow-path 拒否

## 9. Dedupe is ingest-only

**何を言っているか**: idempotency barrier (dedupe table) は event log の
**上流**。projection ではない。reproject で touch しない。

**典型的な違反**:
- reproject が dedupe table を含めて rebuild
- dedupe table を read model として join
- dedupe row を business signal として使う

**満たす実装パターン**:
- dedupe table は ingest layer のみが書く
- reproject は event log のみを scan、dedupe は無視
- 長期 idempotency は event store の `content_hash` unique 制約で担保

## 10. Why as first-class

**何を言っているか**: 分類 / 要約 / 抑制の理由を構造化 payload として
event / projection に保持する。「なぜそう分類・要約したか」を後から再現
できないものは event に書く。decontextualized な結論を単独で持たない。

**典型的な違反**:
- 自由 JSON `why_json` を許可し downstream が解釈不能になる
- classification（`administrative` / `substantive`）の根拠を保存しない
- 要約を source text / diff へのリンクや confidence なしで持つ
  （`DISCIPLINE.md`: decontextualized diff 禁止 / provenance 必須に違反）

**満たす実装パターン**:
- `WhyPayload { kind: enum, text, evidence_refs: [] }` のような構造化型
- `kind` は exhaustive enum、mapping 変更は version bump
- evidence_refs は event id / version id / source `payload_ref` への参照で保持
- 要約・分類は必ず `confidence` + provenance + 元テキスト/diff リンクを同梱

---

## 不変条件と s4rCiv event envelope の対応 (参考)

| Invariant | s4rCiv での具体形 |
|---|---|
| Append-first + hash-chain | observation 事件は INSERT only、`content_hash` / `prev_content_hash` / `log_prev_hash` 連結 |
| Resource / Event 分離 | `ResourceObserved` / `ResourceChanged` / `ResourceVanished` / `ResourceRestored` |
| Event-time purity | `observed_at = MAX(occurred_at)`、`projected_at` は debug only |
| Reproject-safe | `summary_version_id` / `classification_version_id` 経由で stable read |
| Versioned artifacts | `summary_versions` / `classification_versions`（append-only） |
| Merge-safe upsert | `GREATEST(0, …)` + `seq_hiwater` guard |
| Single emission | `content_hash` 比較 + `(source, stream_id, content_hash)` 単位 |
| Dedupe ingest-only | ingest 層の dedupe、reproject は event log のみ scan |
| Why as first-class | `WhyPayload { kind, text, evidence_refs }` + `confidence` + provenance |

固有テーブル名・分類コードの詳細は
[violation-examples.md](violation-examples.md) のケーススタディを参照。
