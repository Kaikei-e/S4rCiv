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
データ源とその収集アダプタ（国会会議録 API、e-Gov 法令 API v2、両院公式議員名簿 など）。1 Source が多数の Stream を持つ。
_Avoid_: provider, origin

**現行（in-force text）**:
生きた文書（法令）の、ある時点で効力を持つ溶け込み済み本文。S4rCiv は法令 Stream の観測内容としてこの現行本文を取り、各改正は同 Stream の `ResourceChanged` 列として現れる（事後確定の固定記録である会議録と対比）。
_Avoid_: 沿革, 全版スナップショット

### 収集の運用（control 面）

**Watch（監視エントリ）**:
ある Stream を定期取得対象として control 面に登録した1件。全 Watch の集合＝**監視リスト**＝現在ポーリングしている Resource 集合。
_Avoid_: subscription, job

**Discover（発見）**:
Source の一覧／更新エンドポイントを走査し、新たな Resource を Watch として監視リストへ加える操作。**監視リストは Discover でのみ増える**（Poll は既存 Watch を取得するだけで監視対象を増やさない）。観測の網羅性は Discover の到達範囲が決める。
_Avoid_: crawl, scrape, index

**Poll（ポーリング）**:
既存 Watch を1件取得し、観測内容のハッシュから観測イベント型を判定・記録する操作。新しい Resource を発見する力は持たない。
_Avoid_: fetch（HTTP取得の意味に限定）, scan

### 観測面の記録物

**Observation Event（観測イベント）**:
ある Resource を1回観測した不変記録。型は `ResourceObserved` / `ResourceChanged` / `ResourceVanished` / `ResourceRestored`。観測面では単に Event。
_Avoid_: log entry, record, change

**消失（vanished）/ 本文未取得（content-unavailable）**:
`ResourceVanished` は Resource が**ソースの権威的な存在シグナルから消えた**ことだけを指す（e-Gov 法令なら法令一覧メタデータから当該 LawId が消える）。これと、ある時点の観測内容（現行本文スナップショット）が一時的に取得できない**本文未取得**とを峻別する。Resource がメタデータ上は存在し続けるのに本文ファイルだけ取れない場合（改正直後に新リビジョン本文の公開が遅れる等）は消失ではなく、観測イベントを生まない（間もなく再取得）。**存在（メタデータ）と観測内容（スナップショット）は別シグナル**であり、消失は前者の消滅のみを根拠とする。
_Avoid_: 本文未取得を消失・404 と同一視すること

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

**連結（chain linkage）**:
あるイベントが log chain の中に座っている事実そのもの（`seq`・`log_hash`・`log_prev_hash` を持つ）。連結の「表示」は鎖の中にあることを示すだけで、ハッシュを再計算した完全性検証とは別物。Provenance chip が担うのは出典・取得時刻と、記録を一意に指す安定した引用ハンドル（記録番号）であって、連結ハッシュの掲示や検証結果ではない。連結ハッシュの提示と再計算は完全性検証として別に扱い、レコード単位のバッジにはしない（ADR-000014）。
_Avoid_: 検証（再計算による確認と峻別）, レコード単位の検証済み/未検証バッジ

**完全性検証（integrity verification）**:
log chain のハッシュを正規形（`HashableEvent`、ADR-000003）から再計算し、`seq` 順に `log_prev_hash` の連続を確かめてログ改ざんが無いことを示す行為。**レコード単位ではなく連鎖（直近チェックポイントまで）単位の性質**。主たる受け手は S4rCiv を信用しない第三者（§5「自ログ改ざん疑義」）であり、S4rCiv 自身が緑の「検証済み」を名乗ることではない。
_Avoid_: 連結（鎖内にある事実と峻別）, レコード単位の「検証済み」状態

**チェックポイント（checkpoint）**:
log chain のある `seq` 時点の状態に対するコミットメント。完全性検証を「genesis から全件再計算」ではなく「直近チェックポイントから当該区間だけ再計算」に**有界化**し、各ユーザーの端末内検証を実用域に収める装置（全履歴の通し再計算は export 経由の第三者ミラーに委ねる）。§5 は定期・署名つきを要請する。`observation.checkpoint`（`alg_version` を保持、ADR-000003）。
_Avoid_: スナップショット, セーブポイント

**Provenance（来歴）**:
解釈面の各値が**どの観測イベントに由来するか**を指す参照。解釈面の全フィールドが provenance と confidence を必ず持つ。
_Avoid_: lineage, origin

### 解釈面の記録物

**Change（変化）**:
解釈面の派生物。連続する2スナップショット間の diff ＋ 分類 ＋ confidence をまとめた1件。観測面の `ResourceChanged` イベントに provenance で紐づく recomputable な projection。diff そのものを観測面に焼かない。
_Avoid_: diff（データ形式・機構の呼称としては可、面の記録物としては避ける）

**eId（要素識別子）**:
ある法令 Work 内の規範ノード（条・項・号・号の細分〔イ・ロ・(1)(2)… の枝〕）を版を跨いで一意に指す安定識別子（AKN 由来）。Change はこの eId をキーに、ノード単位（追加・削除・改変）で構造差分を対応づける。号が用語とその意義を併記する定義号は、用語と意義を1ノードのテキストとして併せ持つ。
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

**Timeline（タイムライン）**:
全 Source を横断する、時系列・人間可読な観測ログの projection。1項目はちょうど1つの Observation Event に対応し（1:1）、状態（observed/changed/vanished/restored）・観測時刻・出所はその Event 由来、「何が動いたか」の人間可読な中身は解釈面の各 projection 由来。Change の上位概念であり、`ResourceChanged` 由来の項目は Change を内包するが、`ResourceObserved`/`ResourceVanished`/`ResourceRestored` 由来の項目は Change ではない（diff を持たない）。市民が「いつ・何が・どう変わったか」を辿る主動線。
_Avoid_: feed, activity log, change log（Change と紛れる）, 単なる「変更履歴」

### 選挙地理（electoral geography）

**選挙区（electoral district）**:
議員が選出される地理単位。衆＝小選挙区（各1名＝議員と 1:1）、参＝選挙区（都道府県単位、複数名＝1:N）。S4rCiv は議員→選挙区の帰属を両院公式議員名簿（公的一次）の観測から取り、**現職のみ**を扱う。境界ポリゴンは観測する事実ではなく地図の下地（外部の地理参照、出典明記して用いる）。
_Avoid_: 選挙地盤, 地元, 地盤

**比例選出（proportional / PR）**:
選挙区を持たず比例代表で選出される議員（衆＝11ブロック、参＝全国区）。選挙区地図に乗らないため、非党派（§5）の要請として常に別パネルで併記し、地図から消さない。
_Avoid_: 比例区（衆ブロックと参全国区の混同を避け文脈を明示）

**選挙区投票地図（district vote map）**:
ある1件の記名投票（VoteEvent）を、各選挙区の現職議員の賛否という**事実カテゴリ**で塗る解釈面の projection。Timeline と同じく観測ログ由来の派生物だが、Timeline が person 軸を持たないのに対しこれは議員単位の賛否を持つ — 記名投票が公職者の事実記録だから（speech との非対称、Timeline／ADR-000004・000006 と同じ線）。集計スコア・賛同率は持たない（評価は data 層に置かない）。歴史は不変ログ／Timeline が保持し、地図は**現会期の現在のレンズ**に徹する。
_Avoid_: 議員地図, 勢力図, 忠誠度マップ, 賛同率ヒートマップ
