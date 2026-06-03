# S4rCiv

公的一次記録の受動・読取専用フライトレコーダ。公的ソースの変化（削除を含む）を不変・ハッシュ連鎖のログに記録し、市民が「いつ・何が・どう変わったか」を辿れるようにする civic-tech インフラ。

このファイルは用語集（ubiquitous language）であり、仕様でも実装メモでもない。実装詳細は持ち込まない。

## Language

### 二面分離（dual-plane）

**Observation Plane（観測面）**:
改ざん耐性のある ground truth。生スナップショットとハッシュ連鎖された変化イベントだけが住む。不変・append-only。
_Avoid_: raw layer, ingest layer

**Interpretation Plane（解釈面）**:
観測面から再計算可能な projection。正規化エンティティ・変化分類・要約が住む。可変だが必ず provenance と confidence を伴う。
_Avoid_: projection layer（機構名としては可、面の呼称としては避ける）, derived layer

### 観測の対象と単位

**Resource（資源）**:
S4rCiv が監視する**外部の**公的一次ソース文書／エンドポイント1件（ある法令、ある会議録、ある契約公告）。観測される対象であって、S4rCiv の外に存在する。
_Avoid_: page, target, document（document は AKN 等の正規化エンティティと紛れるため避ける）

**Stream（ストリーム）**:
ちょうど1つの Resource についての観測イベントの append-only・ハッシュ連鎖列。監視の単位。`stream_id` で識別する。Resource と Stream は 1:1。
_Avoid_: feed, channel, source（source はアダプタ／データ源を指す）

**Source（ソース）**:
データ源とその収集アダプタ（国会会議録 API、e-Gov 法令 API v2 など）。1 Source が多数の Stream を持つ。
_Avoid_: provider, origin

**現行（in-force text）**:
生きた文書（法令）の、ある時点で効力を持つ溶け込み済み本文。S4rCiv は法令 Stream の観測内容としてこの現行本文を取り、各改正は同 Stream の `ResourceChanged` 列として現れる（事後確定の固定記録である会議録と対比）。
_Avoid_: 沿革, 全版スナップショット

### 観測面の記録物

**Observation Event（観測イベント）**:
ある Resource を1回観測した不変記録。型は `ResourceObserved` / `ResourceChanged` / `ResourceVanished` / `ResourceRestored`。観測面では単に Event。
_Avoid_: log entry, record, change

**Snapshot（スナップショット）**:
ある時点で Resource から取得した生バイト列。content hash で content-addressed され、Event が参照する ground-truth ペイロード。
_Avoid_: capture, dump, blob（blob は格納機構の呼称）

### 改ざん耐性（tamper-evidence）

**Log chain（ログ連鎖）**:
全 Stream を挿入順に貫く**単一 global** ハッシュ連鎖。「S4rCiv が自分の台帳を後から書き換えていない」ことを示す。
_Avoid_: audit chain

**Content chain（コンテンツ連鎖）**:
1つの Stream 内で前スナップショットの hash を次イベントが持つ**stream 別**連鎖。「ある Resource の観測内容の履歴が連続している」ことを示す。
_Avoid_: snapshot chain

**Provenance（来歴）**:
解釈面の各値が**どの観測イベントに由来するか**を指す参照。解釈面の全フィールドが provenance と confidence を必ず持つ。
_Avoid_: lineage, origin

### 解釈面の記録物

**Change（変化）**:
解釈面の派生物。連続する2スナップショット間の diff ＋ 分類 ＋ confidence をまとめた1件。観測面の `ResourceChanged` イベントに provenance で紐づく recomputable な projection。diff そのものを観測面に焼かない。
_Avoid_: diff（データ形式・機構の呼称としては可、面の記録物としては避ける）

**eId（要素識別子）**:
ある法令 Work 内の規範ノード（条・項・号）を版を跨いで一意に指す安定識別子（AKN 由来）。Change はこの eId をキーに、ノード単位（追加・削除・改変）で構造差分を対応づける。
_Avoid_: XPath, 行番号, ノードindex

**Administrative change / Substantive change（変化分類）**:
Change の分類。administrative = ナビ・日付・体裁など実質を伴わない変化。substantive = 条文・金額・採決・契約額など実質的変化。substantive は人手レビューキューへ送られ、レビュー結果自体も解釈面の記録になる。
_Avoid_: minor/major, trivial/important

**Interpretation event（解釈イベント）**:
解釈面の append-only な durable 事実。人手レビューの verdict・分類 override・手動訂正など、観測ログから**再計算できない**新情報。観測イベントへ provenance を持つ。read model とは別物で、rebuild で消えてはならない。
_Avoid_: review record, annotation

**Read model（リードモデル）**:
解釈面の disposable な projection。観測イベント＋解釈イベントを projector が fold して再生成する。いつでも truncate→replay できる前提で、ここに durable な事実を直接書かない。
_Avoid_: view, cache, materialized table（機構名としては可）
