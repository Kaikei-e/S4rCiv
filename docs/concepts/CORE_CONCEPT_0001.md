# S4rCiv — CONCEPT

> **S4rCiv**（サーシヴ / _sentinel for civic records_） 公的記録の受動・読取専用フライトレコーダ ＋ 市民のための司令室ダッシュボード。

|項目|内容|
|---|---|
|ステータス|Concept（v0）|
|系譜|g0v / Audrey Tang（Plurality） × EDGI Web Monitoring の現代版後継 × World Monitor 型UI|
|役割|立法・法令・公金・調達等の**公的一次データ**を継続収集し、**変化を不変ログに記録**して可視化する|
|立場|非党派・全方位。反権力ではなく、権力の**監視と市民の権利確保**のための情報インフラ|
|想定ライセンス|サーバ: AGPL-3.0（提案）／スキーマ・収集データ: CC0 or CC BY／クライアント lib: Apache-2.0（**未決**）|
|構成|Rust/Go ソース別アダプタ式コレクタ ＋ CQRS/イベントログ ＋ SvelteKit + MapLibre/WebGPU、単一自己ホストバイナリ|

---

## 1. 目的とビジョン

民主主義の透明性は「今どうなっているか」だけでなく「**いつ・何が・どう変わったか／消されたか**」を市民が辿れて初めて機能する。既存の監視系シビックテックは単機能サイト（情報公開／投票記録／予算）の縦割りで、しかも多くが「現在値」のスナップショットしか見せない。

S4rCiv は次の2つを同時に満たす：

1. **複数の公的ソースを横断する司令室ダッシュボード**（World Monitor の "作戦司令室" UI を地政学ではなく公共の権力に振り向ける）。
2. **変化の不変記録**（EDGI の Web Monitoring が示した「政府ページの差分監視」を、現代的・再展開可能・イベントソース化した形で実装）。

設計哲学は Audrey Tang / g0v の系譜を引く：ラディカルな透明性、権力の集中ではなく協働、サイロを作らない相互運用、そして「有益な情報流通（beneficial information flows）」。AIは判断者ではなく**要約者**であり、必ず一次ソースへ遡れる（Talk to the City 流）。

### なぜ今か（タイミングの追い風）

- **e-Gov 法令API v2**（2025-03 リリース、OpenAPI、法令XML/JSON、**更新法令一覧**あり）。
- **官報電子化**と**告示のベース・レジストリ**提供が**2026年度中目途**（法令XML等の機械可読構造化データ）。
- **政治資金収支報告書のオンライン提出・ネット公表が 2027-01 から義務化**（窓口公開のみだった県も対応必須）。ただし**国はDB化を先送り** ＝ 構造化・横断・時系列の余地が市民側に残る。
- EDGI の web-monitoring ツール群は現在休眠・再展開困難 ＝ 後継の空き地。
- 参加・熟議側（広聴AI / いどばた等）は活発に埋まりつつある ＝ **監視・記録側が相対的に空いている**。

---

## 2. 想定ユーザー

- 調査報道記者・データジャーナリスト（変化の検出 → 取材の起点）
- 研究者（政治学・行政学・公共政策の時系列分析）
- 市民監視団体・NPO・オンブズマン
- シビックハッカー（API/データの二次利用）
- 関心ある一般市民（議員/争点のウォッチ＆アラート）

---

## 3. スコープと非ゴール

### 監視対象（スコープ）

**制度・公金・公的行為**に限定する。

- 立法プロセス（本会議・委員会の議事、法案、採決、記名投票）
- 法令・告示・通達・規則の制定改廃
- 公金（予算、政治資金収支報告書、政党交付金）
- 公共調達・契約（入札・落札・随意契約）
- 公職者・政治団体の**公的な**発信・届出

### 非ゴール（明確に「やらない」こと）

- **私人の監視・プロファイリング・晒し（doxxing）をしない。** 監視の矛先は常に制度・権力であって個人ではない。
- **党派的でない。** 特定政党・思想を標的にしない。全主体に同一基準を適用する。
- **参加・熟議プラットフォームではない**（広聴AI/いどばた/Decidim 等の領域）。S4rCiv は「民意を入れる」のではなく「権力の出力を記録・可視化する」。両者は相補的で、S4rCiv の構造化記録は熟議の入力コンテキストになりうる。
- **自動的な"名指し糾弾"をしない。** 差分の自動SNS投稿で個人を狙い撃つ運用（DiffEngine 的な auto-post）は採らない。アラートは事実・出典リンク付き・opt-in。
- **意見・評価を data 層に持ち込まない。** スコア付けや論評はしない。出力は「観測された事実」と「出典付き要約」のみ。
- **書き込み・自動アクションをしない**（後述の受動性原則）。

---

## 4. 設計原則

1. **受動・読取専用（passive / read-only）** — 公開エンドポイントへの HTTP GET のみ。認証・送信・ログイン背後のスクレイピングをしない。"sentinel"（見張り）であって行為者ではない。
2. **公的一次データのみ** — 出所が公的に公開済みの一次情報だけを扱う。
3. **append-only 不変ログ** — 削除・改変を含めてすべて残す（フライトレコーダ）。S4rCiv 自身の記録も**ハッシュ連鎖で改ざん耐性**を持たせる（transparency-log 方式）。「記録の記録」である以上、S4rCiv が自分で歴史を書き換えていない証跡を提供する。
4. **観測面 / 解釈面の分離（dual-plane）** — BGP-GARM の設計言語を継承。
    - _観測面（Observation Plane）_: 生スナップショット＋ハッシュ連鎖の変化イベント。改ざん耐性のある ground truth。
    - _解釈面（Interpretation Plane）_: 正規化エンティティ（AKN/Popolo/OCDS）＋人手による変化分類＋LLM要約。可変・出典付き・confidence 付き。
5. **標準準拠でサイロを作らない** — Akoma Ntoso（法令/議事）、Popolo（人/役職）、OCDS（調達）。Wikidata 連携余地。
6. **AIは要約のみ・必ず原文と差分にリンク** — 判断は人間。ハルシネーション対策と Tang 流の追跡可能性。
7. **出典明記＋利用規約の技術的内蔵** — ソース別のレート制御・robots 遵守・出典表示を仕組みとして組み込む。

---

## 5. 脅威・濫用モデル（Threat & Abuse Model）

### A. S4rCiv の完全性・可用性への脅威

|脅威|対策|
|---|---|
|データ源の仕様変更・URL変更で取りこぼし|アダプタを疎結合・バージョン管理。観測欠損も `ResourceVanished` として記録（沈黙も情報）。|
|過度なアクセスによる BAN|ソース別レート制御（例: 国会会議録は逐次・数秒間隔）。積極的キャッシュ。可能なら Internet Archive(Memento) 経由で負荷分散＆第三者証跡化。|
|OCR 誤字（議事録末尾・PDF系）|extraction 由来データは confidence を下げて flag。一次PDF/画像へのリンクを必ず併記。|
|S4rCiv 自身のログ改ざん疑義|ハッシュ連鎖＋定期署名チェックポイント。ログは append-only。|
|アーカイブの取りこぼし（消された後に気づく）|監視対象URLは高頻度ポーリング＋IA への push。|

### B. S4rCiv が**害をなす／濫用される**リスク（最重要）

|リスク|対策|
|---|---|
|私人（特に政治資金の少額寄附者等）への嫌がらせ・プロファイリングに使われる|**監視の矛先は説明責任を負う主体（政治家・政党・政治団体・公職者）に固定**。私人寄附者の検索可能なプロファイルは作らない。政治資金は「報告書に公開されている範囲・粒度」をそのまま出典付きで提示し、私人を横断名寄せしない。閾値・集計で個人特定性を抑制。|
|差分の切り取り（decontextualization）で "gotcha" 化|差分は**必ず周辺コンテキスト＋全文へのリンク**とともに表示。スニペット単体を結論として出さない。|
|党派的な選択監視|監視対象の選定基準を公開・機械的に。全会派・全主体へ同一パイプラインを適用。|
|自動投稿による個人攻撃|auto-post は採らない。アラートは opt-in・事実・出典リンク付き。|
|「AIが断定した」と誤認される|要約は判断を含まず、必ず一次ソース/差分にリンク。confidence と provenance を明示。|

### C. 法的・倫理的リスク

- 各データ源の利用規約逸脱 → §7 で技術的に内蔵。
- 名誉毀損リスク → data 層で論評・評価をしない方針が一次防御。事実＋出典のみ。
- 中立性の構造的担保 → 運営体制を非党派に。参考: デジタル民主主義2030 は中立性維持のため発起人がボードを退任する構造をとった。

---

## 6. データ源カタログ

優先度順。各ソースはアダプタ（収集・正規化）として実装する。

### 6.1 立法（議事）— 国会会議録検索API ★MVP

- エンドポイント: `https://kokkai.ndl.go.jp/api/{meeting_list|meeting|speech}`（会議単位簡易／会議単位／発言単位）
- 形式: XML / JSON、HTTP GET、**申請不要**、ページネーション（`startRecord` / `nextRecordPosition`）
- 範囲: 第1回(1947)〜現在の衆参本会議・委員会。**末尾部分に法案・質問主意書/答弁書・議案の投票者氏名・委員会報告書**（※機械生成テキストはOCR誤字あり）
- 利用条件: 国立国会図書館ウェブサイトのコンテンツ利用規約、出典明記、多重リクエスト回避
- 差分検知: 会議録は事後確定だが、確定版差し替え・追録の追跡に有効
- 仕様: https://kokkai.ndl.go.jp/api.html

### 6.2 法令 — e-Gov 法令API v2 ★MVP

- エンドポイント: `https://laws.e-gov.go.jp/api/2/`（OpenAPI / Swagger UI: `/api/2/swagger-ui`）
- 形式: 法令XML / JSON。憲法・法律・政令・省令・規則・告示等
- **差分起点: 更新法令一覧** `https://laws.e-gov.go.jp/update/`（法令種別・更新日）
- 利用条件: 政府標準利用規約／CC BY 互換（**要・規約確認**）

### 6.3 官報・告示（移行期）

- 官報電子化が進行中。**告示のベース・レジストリ提供が2026年度中目途**（法令XML等）。
- 当面は官報PDF/HTMLの監視 → ベース・レジストリ提供開始後にAPI移行。

### 6.4 公金（政治資金）

- 総務省: 政治資金収支報告書（現状PDF中心）`https://www.soumu.go.jp/senkyo/seiji_s/seijishikin/`
- **2027-01 からオンライン提出・ネット公表が義務化**（機械可読化が進む）
- 既存の市民側DB（**棲み分け対象**）: 政治資金センター `https://search.openpolitics.or.jp/`、political-finance-database.com
- S4rCiv の差別化: **時系列差分**（訂正・差し替えの追跡）と横断（議事↔法令↔予算↔調達との相関）

### 6.5 公共調達・契約

- 調達ポータル（落札実績オープンデータ）`https://www.p-portal.go.jp/`
- 省庁別契約情報は月次PDF混在 → OCDS 整形
- 標準: OCDS（Open Contracting Data Standard）

### 6.6 横断カタログ

- e-Gov データポータル `https://www.data.go.jp/`、Awesome Japan Open Data

---

## 7. データ源の合法性と利用規約

- **公的文書の自由利用**: 著作権法13条により、(1)憲法その他の法令、(2)国・地方公共団体等が発する告示・訓令・通達等、(3)裁判所の判決等 は「権利の目的とならない著作物」。→ 法令・告示・通達の収集・差分表示・再配布は法的足場が堅い。
- **国会会議録**: 議員の公開の場での演述は自由利用に近いが、**NDL職員の発言およびデータベース自体の著作権はNDLに帰属**する点に注意。出典明記必須。
- **e-Gov 法令データ**: 出典明記で利用可（政府標準利用規約／CC BY 互換と理解。**正式な規約文面を要確認、TODO**）。
- **運用上の遵守（技術的内蔵）**:
    - ソース別レート制御（serial + interval、設定可能）
    - robots.txt 遵守、User-Agent に S4rCiv とコンタクト先を明記
    - 出典・取得日時を全レコードに保持し、UI/API で常に表示
    - 可能な場面では Internet Archive(Memento) 経由で取得し、第三者アーカイブとの二重化

---

## 8. アーキテクチャ

```
┌─────────────┐   ┌──────────────┐   ┌──────────────────┐
│ Source       │   │ Normalizer    │   │ Event Log (CQRS) │
│ Adapters     │──▶│ → AKN/Popolo/ │──▶│ append-only,     │
│ (Rust/Go)    │   │   OCDS         │   │ hash-chained     │
│ kokkai/egov/ │   │ + diff/        │   │ (observation     │
│ kanpo/fund/  │   │   classify     │   │  plane)          │
│ procurement  │   └──────────────┘   └────────┬─────────┘
└─────────────┘                                 │ projections
        ▲ HTTP GET only (passive)                ▼
   public APIs / pages              ┌──────────────────────────┐
   (+ Internet Archive)             │ Read Models               │
                                    │ timeline / entity / vote / │
                                    │ contract / funding         │
                                    │ + LLM summaries(linked)    │
                                    └────────────┬──────────────┘
                                                 ▼
                                    ┌──────────────────────────┐
                                    │ Web (SvelteKit +          │
                                    │ MapLibre/WebGPU)          │
                                    │ situation-room dashboard  │
                                    └──────────────────────────┘
```

- **単一自己ホストバイナリ**（BGP-GARM / c2quay と同じ流儀）。組み込みストレージ（イベントログ）でゼロ依存に寄せる。
- **アダプタ式**：`bgpkit` 的に「ソース別アダプタ」を pluggable に。新ソース追加＝アダプタ追加。
- **観測面と解釈面を物理的に分離**：観測面は不変・改ざん耐性、解釈面は再計算可能な projection。

---

## 9. イベントスキーマ案（AKN + Popolo + OCDS）

### 9.1 観測面：イベントエンベロープ（append-only, hash-chained）

```jsonc
{
  "event_id":            "uuidv7",
  "stream_id":           "source-resource-uri",   // 監視単位（例: ある法令/会議録/契約）
  "seq":                 1234,                      // stream内連番
  "type":                "ResourceChanged",        // 後述
  "source":              "ndl-kokkai",             // アダプタ識別
  "fetcher_version":     "S4rCiv-collect/0.1.0",
  "observed_at":         "2026-06-02T09:00:00Z",   // 取得時刻
  "source_published_at": "2026-05-30T00:00:00Z",   // ソース主張の公開/更新時刻
  "content_hash":        "sha256:…",               // 取得コンテンツのハッシュ
  "prev_content_hash":   "sha256:…",               // 直前スナップショットのハッシュ
  "archive_ref":         "ia://web/2026…/…",       // IAスナップショット(任意)
  "log_prev_hash":       "sha256:…",               // ★ログのハッシュ連鎖
  "payload_ref":         "blob://…",               // 生スナップショット本体への参照
  "diff":                { /* §9.3 */ },
  "confidence":          "high|medium|low"          // OCR系は低
}
```

**イベント型（観測面）**

- `ResourceObserved` — 初観測
- `ResourceChanged` — ハッシュ差分（diff と classification を伴う）
- `ResourceVanished` — 在ったものが消えた（404/削除。**沈黙も記録**）
- `ResourceRestored` — 復活

### 9.2 解釈面：エンティティ projection（標準準拠）

- **LegislativeWork / Bill**（Akoma Ntoso 準拠）— 法案・法令。AKN の time-versioning と手続きワークフロー（委員会→修正→審議→採決）をメタデータで保持。
- **Person / Membership / Organization**（Popolo 準拠）— 議員・会派・政治団体。
- **VoteEvent / Vote**（Popolo voting）— 記名投票（賛否・棄権）。会議録末尾の投票者名から構築。
- **Contract / Award / Tender**（OCDS 準拠）— 調達。
- **FundingReport / Transaction** — 政治資金（公開範囲・粒度のまま、私人横断名寄せをしない）。

各 projection の各フィールドは **provenance（どの観測イベントに由来するか）と confidence** を必ず持つ。

### 9.3 差分（diff）と変化分類

- **構造化ソース（法令XML/AKN）**: 構造差分（条・項・号レベル）。HTMLテキスト差分より遥かに高精度。
- **半構造/PDF（政治資金・一部調達）**: extraction → テキスト差分、confidence=low、一次PDFへリンク。
- **変化分類（EDGI に倣う）**: `administrative`（ナビ・日付・体裁）と `substantive`（条文・金額・採決・契約額）を heuristic で一次分類 → `substantive` は**人手レビュー・キュー**へ。レビュー結果も解釈面イベントとして記録。

---

## 10. 要約レイヤー（Talk to the City 流）

- LLM は**要約・クラスタリングのみ**。判断・評価・スコアリングをしない。
- すべての要約文は**元の発言／条文／差分へリンク**（クリックで原文に着地）。
- confidence と provenance を併記。OCR由来は要約に使わないか、明示的に低信頼として扱う。
- 多言語（日↔英）対応は将来。

---

## 11. MVP / マイルストーン

- **M0 — 骨格**: 単一バイナリ、イベントログ（append-only + hash-chain）、アダプタ interface、観測面/解釈面の分離。
- **M1 — 立法アダプタ**: 国会会議録API から会議・発言を取得 → スナップショット → `ResourceObserved/Changed`。Popolo で議員/会派を projection。記名投票の VoteEvent 化。
- **M2 — 法令アダプタ**: e-Gov「更新法令一覧」をポーリング → 変更法令を取得 → AKN 構造差分。
- **M3 — ダッシュボード v0**: タイムライン＋議員/キーワードのウォッチ＆アラート（apprise 的通知）。
- **M4 — マップ**: MapLibre に選挙区/自治体レイヤー。
- **M5 — 要約 v0**: T3C 流の要約（原文リンク必須）を薄く。
- **M6 — 公開**: ライセンス確定、ドキュメント、自己ホスト手順、監視対象URLの選定基準を公開。

### 将来拡張

政治資金（2027 義務化に合わせ）／調達 OCDS 化／官報・告示ベースレジストリ（2026 提供後）／地方議会会議録／予算書データ／パブリックコメント／審議中継の字幕。

---

## 12. 既存プロジェクトとの関係（協調 ＞ 競合）

- **デジタル民主主義2030 / チームみらい**（広聴AI・いどばた、OSS、非党派）: 参加・熟議側。**相補的**。S4rCiv の構造化記録・差分が熟議の入力になりうる。
- **Code for Japan / Code for 選挙**（Popolo、TheyWorkForYou 志向）: 立法トラッカーで**協調**（Popolo 相互運用）。
- **政治資金センター / political-finance-database**: カネ側。**時系列差分**で補完、データ相互利用を検討。
- **mySociety スタック / EDGI**: 海外の系譜。標準（AKN/Popolo/OCDS）準拠で接続可能性を残す。

---

## 13. オープン性・ガバナンス

- **ライセンス（提案・未決）**: サーバ本体は **AGPL-3.0**（mySociety 等の civic-tech 慣行に倣い、SaaS フォークもオープンに保つ）。収集データ・スキーマは CC0 または CC BY。クライアント lib は Apache-2.0/MIT。
- **中立性の構造的担保**: 運営を非党派に。監視対象選定基準・収集ロジックを公開。
- **再現性**: 観測面のハッシュ連鎖により、誰でも記録の完全性を検証可能。

---

## 14. オープンクエスチョン（要決定）

1. ライセンス（AGPL-3.0 サーバ + CC0/CC BY データ で確定可？）。
2. ホスティングモデル（単一インスタンス公開 / 自己ホスト前提 / 将来フェデレーション）。
3. 一次文書の扱い（自前ミラー vs Internet Archive へのリンク/プッシュ中心）。
4. 変化分類の heuristic（administrative/substantive）の初期ルールと閾値。
5. OCR ノイズの抑制方針（低信頼データの UI 上の扱い）。
6. 政治資金における私人寄附者の表示粒度（公開範囲の遵守と個人特定性抑制のバランス）。
7. アラートの配信境界（opt-in、レート、個人を狙い撃たない設計の具体）。

---

## 付録: 主要参考リンク

- 国会会議録検索API 仕様 — https://kokkai.ndl.go.jp/api.html
- e-Gov 法令API v2（Swagger UI） — https://laws.e-gov.go.jp/api/2/swagger-ui ／ 更新法令一覧 — https://laws.e-gov.go.jp/update/
- 政治資金収支報告書（総務省） — https://www.soumu.go.jp/senkyo/seiji_s/seijishikin/
- 調達ポータル — https://www.p-portal.go.jp/
- e-Gov データポータル — https://www.data.go.jp/
- EDGI Web Monitoring（先行事例 / 休眠） — https://envirodatagov.org/ ／ awesome-website-change-monitoring — https://github.com/edgi-govdata-archiving/awesome-website-change-monitoring
- changedetection.io（汎用OSS・要差別化） — https://github.com/dgtlmoon/changedetection.io
- Akoma Ntoso / LegalDocML（OASIS） — https://www.oasis-open.org/committees/tc_home.php?wg_abbrev=legaldocml
- Popolo（人/役職標準） / OCDS（調達標準）
- Talk to the City（broad listening, OSS） — https://www.talktothe.city/
- デジタル民主主義2030 — https://dd2030.org/