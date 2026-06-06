---
name: s4rciv-postmortem-writer
description: Writes a blameless incident postmortem for the s4rCiv project in Japanese, grounded in the real timeline, commits, and logs (never speculation). Use when the user says "ポストモーテム書いて" / "障害報告書いて" / "事後検証" / "incident report" / "RCA" / "根本原因", or after an incident, outage, data-integrity scare, or a committed bug fix worth a durable record. Authoring only — no deploy.
allowed-tools: Bash, Read, Glob, Grep, Edit, Write
---

# s4rCiv Postmortem Writer

s4rCiv のインシデント・ポストモーテムを **日本語で・blameless に・事実に基づいて**執筆するスキル。
デプロイやインフラ操作は扱わない（執筆のみ）。

**Blameless の原則**（Google SRE / Atlassian）: 個人やチームを責めず、**寄与原因（contributing causes）**に集中する。関係者は手元の情報で善意に基づき最善を尽くしたと仮定する。ポストモーテムは罰ではなく学習の記録。

実行は 3 ステップ。途中はこのチェックリストを応答にコピーして埋める。

```
Postmortem Progress:
- [ ] 1. 事実確認（git / ログ / コードから timeline と根本原因を裏取り）
- [ ] 2. 不変条件レンズで影響を分類（observation 面 / interpretation 面）
- [ ] 3. テンプレに沿って docs/postmortem/ に執筆
```

---

## §1. 事実確認（先に裏取り。推測で書かない）

ポストモーテムは「実際に起きたこと」の記録。書く前に一次情報から timeline・影響・根本原因・修正を確定する。**確認できない点は「不明」と明記**し、創作しない。

| 知りたいこと | 取り方 |
|---|---|
| 修正コミット・関連変更 | `git log --oneline`, `git show --stat <sha>`, `git log -S<symbol>` |
| エラー・再現 | ユーザ提供のログ、`make` / `go test` / `collector` 等の出力（手元にあるもの） |
| 影響範囲のコード | `Grep` / `Read` で該当の projector / driver / migration / handler を確認 |
| 設計上の位置づけ | 関連 ADR（`docs/ADR/`）、`CONTEXT.md`、`DISCIPLINE.md`、`CORE_CONCEPT` |

本番 DB への直接クエリや本番への副作用は**このスキルでは行わない**。必要なら手順をユーザに渡す（実行は分類器の承認を要する本番操作）。

---

## §2. 不変条件レンズで影響を分類（s4rCiv 固有）

s4rCiv は二面分離が中核。影響評価は必ずこの軸で書く。

- **observation 面（観測面）= 不変 ground truth**: 生スナップショット＋ハッシュ連鎖イベント。「**ground truth は無事か**（改ざん・欠落・連鎖切れが無いか）」を最優先で判定。ここが壊れていれば最重大。
- **interpretation 面（解釈面）= 再計算可能な projection**: read model は disposable。「reproject で復旧可能か」を書く（多くの read-model 障害は復旧可能＝重大度が下がる）。
- **設計原則との関係**: 受動/読取専用、append-only、AI は判定しない、反ドキシング、ソース遵守。違反やニアミスがあれば明記。

### Severity（基準は「再投影できなくなるか / ground truth が壊れるか」）

| SEV | 目安 |
|---|---|
| SEV1 | observation 面の完全性が危機（データ欠落・連鎖切れ・誤った事実を提示）。要即時 |
| SEV2 | interpretation 面の read model 劣化（reproject で復旧可）、または 1 ソースの収集停止 |
| SEV3 | 軽微・一過性・自動復旧。記録はするが影響小 |

---

## §3. 執筆

### 3.1 ファイル

`docs/postmortem/YYYY-MM-DD-<slug>.md`（無ければディレクトリごと作る）。`<slug>` は英小文字・ハイフン（例: `egov-law-reproject-eid-collision`）。日付は当日（YYYY-MM-DD）。

### 3.2 テンプレート

[references/template.md](references/template.md) の見出しと順序をそのまま使う（勝手に増減しない）。Five Whys で根本原因まで辿り、action item には **担当・状態・種別（恒久対策 / 改善）・追跡先** を必ず付ける。

### 3.3 書き方の規律

- **Blameless**: 主語を人でなく**仕組み・条件**にする（「X が Y を許していた」）。個人名・属人的な非難を書かない。
- **平易な語**: 誇張・芝居がかった語を避け、淡々と書く（DESIGN_LANGUAGE §10 / プロジェクト方針）。
- **事実と推測を分ける**: 確証は断定、推測は「推測」と明記、不明は「不明」。
- **時刻は JST**（観測/取得時刻の表記と揃える。ログが UTC なら換算して JST と明記）。
- **再発確認**: 同じ根本原因の別事象が無いか（Atlassian recurrence check）を 1 行で。

---

## §4. 情報衛生（DISCIPLINE 準拠）

s4rCiv は AGPL-3.0 の公開プロジェクト。ポストモーテムにも以下を**含めない**:

- 本番 IP / 本番ドメイン / 秘匿ポート / 資格情報・鍵・シークレット（`localhost:PORT` と compose サービス名は可）
- **私人の個人情報**（対象は常に説明責任を負う public actor。題材の議員・省庁等の固有名は可だが私人を巻き込まない）
- 個人への非難（blameless）

---

## §5. 完了報告

ユーザに次を伝える:

- 書いたパス（`docs/postmortem/YYYY-MM-DD-<slug>.md`）と一行サマリ・Severity
- §1 で裏取りに使った事実（コミット sha・ログ・該当ファイル）
- action item の一覧（担当が未定なら「TBD」と明示）
- commit するかはユーザに確認する（このスキルは commit / push を勝手に行わない）

---

## 参照

- [references/template.md](references/template.md) — ポストモーテムの正規テンプレート（見出し順のソース）
- `docs/ADR/` — 関連 ADR（wikilink `[[000NNN]]` 形式で引く）
- `DISCIPLINE.md` — 禁止事項（情報衛生・反ドキシングの根拠）
- `docs/concepts/CORE_CONCEPT_0001.md` — 二面分離・設計原則の単一の真実
