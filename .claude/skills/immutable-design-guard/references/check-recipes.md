# Check Recipes — Language- and Layer-specific grep / ripgrep

違反候補を機械的に当てるためのレシピ集。SKILL.md ワークフロー Step 3 で使う。
**全部実行しない**。今回触れた言語 / 層だけ走らせる。

引数の `<root>` は対象コンポーネントのアプリルート (例: `adapters/<source>/`,
`crates/<adapter>/src`)。ripgrep (`rg`) があれば優先、無ければ `grep -rn`。

## 共通: append-first 違反

```bash
# event store への UPDATE / DELETE
rg -n 'UPDATE\s+\S*event\S*\s+SET|DELETE\s+FROM\s+\S*event\S*' <root>

# event store table への "soft delete" 列
rg -n 'deleted_at|is_deleted' <root>/migrations* <root>/*/migrations*
```

## 共通: hash-chain integrity 違反（s4rCiv 固有）

```bash
# 既存 event の hash 連結フィールドを再計算・上書きしていないか
rg -n 'UPDATE\s+\S*event\S*\s+SET\s+\S*(content_hash|prev_content_hash|log_prev_hash)' <root>
rg -n '(prev_content_hash|log_prev_hash)\s*[:=]' <root> -g '!*_test.*' -g '*migrat*' -g '*store*'
```

ヒットしたら **append 時の 1 回計算か、過去 event の書き換えか** を判定。
後者は履歴改竄であり high。

## 共通: 削除・消失を行削除で表現していないか（s4rCiv 固有）

```bash
# 消失を ResourceVanished event ではなく DELETE で表す疑い
rg -n 'DELETE\s+FROM\s+\S*(resource|event)\S*' <root>
rg -n 'ResourceVanished|ResourceRestored' <root>   # 逆に「使われているか」を確認
```

## 共通: time.Now() / NOW() による業務時刻汚染

```bash
# Go: projector / reproject / event handler 内の time.Now()
rg -n --type go '(time\.Now\(\)|time\.Since)' \
   <root> -g '*projector*' -g '*reproject*' -g '*event*handler*'

# Rust
rg -n --type rust '(Utc::now\(\)|SystemTime::now\(\))' \
   <root> -g '*projector*' -g '*reproject*'

# Python
rg -n --type py '(datetime\.now|datetime\.utcnow|time\.time\()' \
   <root> -g '*projector*' -g '*reproject*'

# SQL: projection 系 migration 内の DEFAULT NOW()
rg -n 'DEFAULT\s+NOW\(\)|DEFAULT\s+CURRENT_TIMESTAMP' <root>/migrations*
```

ヒットしたら **business fact か debug-only `projected_at` か** を判定。

## 共通: reproject-unsafe な「latest を読む」projector

```bash
# projector が latest を取りに行く呼び出し
rg -n '(GetLatest|FindCurrent|FindActive|SELECT.*FROM.*projection)' \
   <root> -g '*projector*' -g '*usecase*projector*'

# projector 内で active read model に SELECT
rg -n 'SELECT.*FROM\s+\S*projection\S*|SELECT.*FROM\s+\S*read_model\S*' \
   <root> -g '*projector*'
```

## 共通: write path が read model を mutate

```bash
# handler / usecase が projection table を直接 UPDATE/INSERT
rg -n 'UPDATE\s+\S*projection\S*\s+SET|INSERT\s+INTO\s+\S*projection\S*' \
   <root>/handler <root>/usecase
```

## SQL / migration: merge-safe 違反 / business logic in SQL

```bash
# SQL CASE WHEN で business 判定
rg -n 'CASE\s+WHEN' <root>/migrations* <root>/driver

# 引き算で負数になりうる UPDATE
rg -n 'SET\s+\w+\s*=\s*\w+\s*-\s*' <root>/migrations* <root>/driver

# COALESCE 漏れ (UPSERT で EXCLUDED 値を上書きしている)
rg -n 'ON CONFLICT.*DO UPDATE' <root>/migrations* | head -50
```

ヒットしたら `GREATEST(0, …)` / `COALESCE(EXCLUDED.x, table.x)` /
`WHERE EXCLUDED.seq > table.seq_hiwater` への置換を要求する。

## event payload / envelope schema

```bash
# event payload に複数の意味の異なる時刻が同居していないか
rg -n 'occurred_at|recorded_at|received_at|observed_at|fetched_at' <root> proto/

# generic な XxxUpdated / XxxChanged event (Property Sourcing 疑い)
rg -n 'message\s+\w+Updated\b|message\s+\w+Changed\b' proto/

# version_id / artifact_version_ref が event 側にあるか
rg -n 'version_id|artifact_version_ref|summary_version_id|classification_version_id' <root> proto/
```

## 共通: Versioned artifacts 違反

```bash
# *_versions テーブルへの UPDATE (append-only であるべき)
rg -n 'UPDATE\s+\S*_versions?\s+SET' <root> <root>/migrations*

# event payload が resource_id だけで version_id を持たない
rg -n 'resource_id\s*string' proto/ -A2 | rg -B1 -L 'version_id'
```

## 共通: Single emission / idempotency

```bash
# content_hash ベースの idempotency が効いているか
rg -n 'content_hash|idempotency_key|dedupe' <root>/handler <root>/usecase proto/

# 同一観測サイクルが複数 event を発行している箇所
rg -n 'PublishEvent|EmitEvent|AppendEvent' <root>/usecase | \
   awk -F: '{ print $1 }' | sort | uniq -c | sort -rn | head
```

2 連発以上を発行しているファイルは要検査。

## s4rCiv 固有 (例)

```bash
# 観測 event テーブル（例: observation_events）への UPDATE / DELETE
rg -n 'UPDATE\s+observation_events|DELETE\s+FROM\s+observation_events' <root>

# projector 内の wall-clock
rg -n '(time\.Now\(\)|Utc::now\(\))' <root> -g '*projector*' -g '*read_model*'

# summary / classification version テーブルへの UPDATE
rg -n 'UPDATE\s+(summary_versions|classification_versions)\s+SET' <root>

# 要約・diff が provenance / confidence / source link なしで保存されていないか
#   (DISCIPLINE: decontextualized diff 禁止 / provenance 必須)
rg -n 'summary|diff' <root>/usecase | rg -L 'confidence|provenance|payload_ref|source'
```

## レポートに含める情報

各ヒットについて:

- ファイル:行番号
- 当該行 (1-2 行)
- どの原則に該当しそうか (推定)
- false positive の可能性 (debug 用 / migration 一回限り 等)

最終判定 (high / medium / low) は SKILL.md の出力テンプレに従う。

[← back to SKILL.md](../SKILL.md)
