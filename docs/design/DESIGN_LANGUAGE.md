# S4rCiv — Design Language

> ビジュアルデザインの規定。`CONCEPT.md`（プロダクト設計）と対。
> 既定はダークテーマ。目標は WCAG 2.2 AA。

---

## 1. 原則

1. **無彩色を基調にし、色は状態のためだけに使う。** UIの大半はニュートラルなグレースケール。色は状態（観測・変化・注意・消失）の伝達にのみ用い、平常時は彩度を持たせない。情報密度が高くても疲れにくくするための方針（High-Performance HMI の考え方）。
2. **純黒・純白を使わない。** 背景は暗いグレー（`#0B0E13` 系）、本文はオフホワイト（`#E7ECF3` 系）。コントラストが極端だと長時間の可読性が落ちるため。
3. **階層は影でなく明度で表す。** 暗い背景では影が効かないので、`surface-1`→`surface-3` の明度差と細いヘアラインで層を作る。
4. **色だけに依存しない。** 状態は「色＋アイコン＋ラベル（必要に応じて形）」で多重に符号化する。差分は `+ / − / ~` の記号も併用（WCAG 1.4.1）。
5. **警告色は彩度を抑える。** 注意・消失などの色は、目立つが警報的になりすぎないトーンにする。実質的な変化だけを前面に出し、不要な通知で埋めない。
6. **出所と鮮度を常に表示する。** 各データに「最終取得時刻・出典・改ざん検証（ハッシュ連鎖）」を併記し、信頼性を可視化する。

---

## 2. デザイントークン

役割で命名し、テーマ切替を一括で行う。CSS カスタムプロパティとして実装する。

### Dark（既定）

```css
:root, [data-theme="dark"] {
  /* surfaces */
  --canvas:     #0B0E13;
  --surface-1:  #11161E;
  --surface-2:  #18202A;
  --surface-3:  #212B37;
  --hairline:   rgba(174,196,224,0.10);
  --hairline-2: rgba(174,196,224,0.18);

  /* text */
  --text-1: #E7ECF3;  /* 主要 */
  --text-2: #AEB9C9;  /* 副次 */
  --text-3: #8793A6;  /* 補助・ラベル */

  /* interactive */
  --accent:      #59C7E6;
  --accent-weak: rgba(89,199,230,0.14);
  --focus:       #8AD8F2;

  /* status: graphic(塗り/枠) / text(濃い背景上の文字) */
  --st-nominal:  #45B08A; --st-nominal-t:  #6FD3AC;
  --st-info:     #5A9FE0; --st-info-t:     #88BCF2;
  --st-changed:  #E0A845; --st-changed-t:  #F2C877;
  --st-caution:  #E08646; --st-caution-t:  #F2A974;
  --st-critical: #D86460; --st-critical-t: #F2908C;

  /* data-viz（Okabe-Ito をダーク調整。形状と併用） */
  --dv-1:#5AA9E6; --dv-2:#E0A845; --dv-3:#46C29A;
  --dv-4:#C98BDB; --dv-5:#E2796A; --dv-6:#9AD15B;

  /* shape */
  --r-sm:4px; --r:7px; --r-lg:11px;
}
```

### Light

```css
[data-theme="light"] {
  --canvas:#EEF1F5; --surface-1:#FFFFFF; --surface-2:#FFFFFF; --surface-3:#FFFFFF;
  --hairline:rgba(16,24,33,0.12); --hairline-2:rgba(16,24,33,0.20);
  --text-1:#16202C; --text-2:#465468; --text-3:#5C6B7E;
  --accent:#0C7FA3; --accent-weak:rgba(12,127,163,0.10); --focus:#0C7FA3;
  --st-nominal:#1E8E68; --st-nominal-t:#157054;
  --st-info:#1F6FBF;    --st-info-t:#175499;
  --st-changed:#B7791F; --st-changed-t:#8A5A12;
  --st-caution:#B45F1E; --st-caution-t:#8A4715;
  --st-critical:#C0413D; --st-critical-t:#993230;
  --dv-1:#1F6FBF; --dv-2:#B7791F; --dv-3:#138468;
  --dv-4:#8A4FB0; --dv-5:#C0413D; --dv-6:#4E8A1E;
}
```

---

## 3. カラー

### 3.1 サーフェス（明度の段階）

| トークン | Hex (dark) | 用途 |
|---|---|---|
| `--canvas` | `#0B0E13` | ページ背景 |
| `--surface-1` | `#11161E` | パネル |
| `--surface-2` | `#18202A` | パネルヘッダ・一段上の面 |
| `--surface-3` | `#212B37` | オーバーレイ・ポップオーバー |

### 3.2 テキスト・アクセント（コントラスト）

コントラスト目標: 本文 ≥ 4.5:1、大きい文字/非テキスト ≥ 3:1（WCAG 2.2 / 1.4.3・1.4.11）。**値は概算。確定時に実測ツールで再検証する。**

| トークン | Hex (dark) | on `--canvas` | 判定 |
|---|---|---|---|
| `--text-1` | `#E7ECF3` | ≈ 15:1 | AAA |
| `--text-2` | `#AEB9C9` | ≈ 9:1 | AAA |
| `--text-3` | `#8793A6` | ≈ 5.5:1 | AA（大はAAA） |
| `--accent` | `#59C7E6` | ≈ 9:1 | リンク/選択/フォーカス |

### 3.3 状態色（色＝意味）

状態は**色＋グリフ＋ラベル**で必ず多重符号化する。

| 状態 | role | graphic | text | グリフ | 用途 |
|---|---|---|---|---|---|
| 安定 | nominal | `#45B08A` | `#6FD3AC` | `●` | 変化なし・正常 |
| 観測 | info | `#5A9FE0` | `#88BCF2` | `◉` | 新規取得・記録 |
| 変化 | changed | `#E0A845` | `#F2C877` | `Δ` | 実質的な変更（中心となる状態色） |
| 注意 | caution | `#E08646` | `#F2A974` | `▲` | 低信頼・要確認 |
| 消失 | critical | `#D86460` | `#F2908C` | `⊘` | 公開記録の削除 |

- 変更（changed）= 琥珀＋`Δ` を基準色とする。
- 消失（critical）はくすんだ赤。目立つが警報的にしない。
- pill 実装: 背景 = graphic を低 alpha、枠 = graphic ≈0.4、文字 = text 変種、＋グリフ＋ラベル。

### 3.4 データ可視化パレット

`--dv-1`〜`--dv-6`。Okabe-Ito を暗背景用に調整した色覚多様性配慮パレット。**グラフではマーカー形状・パターンを必ず併用**し、色だけに依存しない。暗背景は色の弁別にむしろ有利（IBM Carbon の知見）。色数は必要最小限に絞る。

| トークン | 形状（併用例） |
|---|---|
| `--dv-1` `#5AA9E6` | ● 円 |
| `--dv-2` `#E0A845` | ■ 四角 |
| `--dv-3` `#46C29A` | ▲ 三角 |
| `--dv-4` `#C98BDB` | ◆ 菱形 |
| `--dv-5` `#E2796A` | ✚ 十字 |
| `--dv-6` `#9AD15B` | ★ 星 |

---

## 4. タイポグラフィ

- **本文・UI（日本語対応）**: IBM Plex Sans JP
- **数値・ID・ハッシュ・ラベル**: IBM Plex Mono

```css
--font-sans: 'IBM Plex Sans JP','IBM Plex Sans',system-ui,sans-serif;
--font-mono: 'IBM Plex Mono',ui-monospace,SFMono-Regular,Menlo,monospace;
```

### スケールと規則

| 役割 | サイズ | 規則 |
|---|---|---|
| 見出し H1 | 21px / 700 | sans |
| 見出し H2 | 17px / 600 | sans |
| 本文 | 15px / 400 | sans, line-height 1.6 |
| 補助 | 13px | sans/mono |
| ラベル | 11px | mono, 大文字, letter-spacing 0.14em |
| 数値 | mono | `font-variant-numeric: tabular-nums`（桁揃え） |

- 数値・金額・日時・ID・ハッシュは必ず等幅（mono ＋ tabular-nums）。
- ラベルは mono・大文字・字間広めで、本文と視覚的に区別する。

---

## 5. スペーシング・角丸

- 基本グリッド: **4px**。`4 / 8 / 12 / 16 / 24 / 32`。
- 角丸: `--r-sm 4px`（小要素）, `--r 7px`（カード）, `--r-lg 11px`（パネル）。
- 罫線は 1px のヘアライン（`--hairline` / `--hairline-2`）。

---

## 6. コンポーネント仕様

機能要件のみ。スタイルはトークンに従う。

- **Status pill** — 状態の表示。`色（低alpha背景＋枠）＋グリフ＋ラベル`。アイコンのみは不可（必ずラベルを伴う）。
- **Panel（パネル）** — 情報の最小単位。ヘッダ（左ドット＋mono大文字タイトル）＋本文。`--surface-1` ＋ `--hairline-2` ＋角丸 `--r-lg`。複数を並べて1画面に統合する。
- **Metric tile（指標）** — ラベル（mono大文字）＋大きな数値（mono, tabular-nums）＋増減表示。増加は `changed` 色＋`Δ`、それ以外は `--text-3`。
- **Change log item（変更タイムライン項目）** — 時刻（mono）＋状態ノード（色付き）＋本文＋ソース行（`SRC=…`・confidence・出典リンク）。状態ノードの枠色で changed/critical を区別。
- **Diff（差分）** — 行頭記号 `+ / − / ~` ＋背景色 ＋左罫の三重符号化。法令XMLは条・項・号レベルの構造差分を優先（テキスト差分より高精度）。
- **Data table（テーブル）** — 高密度。ヘッダは mono大文字、数値列は右寄せ・tabular-nums。行ホバーは `--accent-weak`。
- **Alert（通知）** — アイコン（角丸の小バッジ）＋見出し＋本文＋左罫（caution/critical）。彩度を抑え、落ち着いたトーン。
- **Provenance chip（出所・鮮度）** — `最終取得時刻 / 出典 / ハッシュ連鎖検証` を区切って表示。検証済みは `nominal` 色。
- **Map（地図）** — 選挙区レイヤー等の空間表示。マーカーは状態色＋形＋ラベル。`role="img"` ＋ `aria-label` を付す。
- **Button** — primary（`--accent` 背景）/ secondary（surface＋枠）/ ghost（透明）。
- **Focus** — `:focus-visible` で 2px 実線＋2px offset、`--focus`、≥3:1。

---

## 7. モーション

- 使ってよい: ロード時の段階的フェード（`animation-delay`）、ホバーの控えめな明度変化、状態遷移の短いフェード（120–200ms）。
- 避ける: 点滅（WCAG 2.3.1）、自動スクロール、点滅するグロー、視線を奪う動き。
- `prefers-reduced-motion: reduce` で全アニメーション・トランジションを停止する。

```css
@media (prefers-reduced-motion: reduce) {
  * { animation: none !important; transition: none !important; }
}
```

---

## 8. アクセシビリティ（WCAG 2.2 AA 目標）

- [ ] **コントラスト** — 本文 ≥ 4.5:1、大きい文字/非テキスト（UI要素・グラフ・フォーカス）≥ 3:1（1.4.3 / 1.4.11）。全色ペアを再計算する（ダークは自動で安全ではない）。
- [ ] **色だけに依存しない**（1.4.1）— 状態は色＋グリフ＋ラベル、差分は記号併用。
- [ ] **フォーカス可視**（2.4.7 / 2.4.11）— `:focus-visible` 2px＋offset、≥3:1。
- [ ] **OS設定の尊重** — `prefers-color-scheme` / `prefers-contrast` を尊重し、ユーザ選択を上書きしない。既定はダークだが、テーマ切替も提供する。
- [ ] **動きの抑制** — `prefers-reduced-motion` 対応。点滅不使用。
- [ ] **キーボード操作**（2.1.1）— 全機能をキーボードで操作可能に。スキップリンク、論理的な見出し階層、適切な ARIA。
- [ ] **無効状態** — 色だけで示さない（opacity ＋ cursor ＋ 表示の併用）。
- [ ] **将来対応** — APCA（WCAG 3 草案、暗背景を非対称評価。本文 Lc 60+ 目安）を追加指標として併用。WCAG 2.x は引き続き遵守。

検証ツール: axe / WAVE / Lighthouse / Stark / APCA Contrast Calculator。

---

## 9. 参考

- ダークモード設計（純黒回避・脱彩度・明度で階層）: onething.design, LogRocket
- High-Performance HMI / 運用画面（無彩色基調・色＝状態・通知過多の回避）: Activu, EEMUA 201 / ISA-101
- WCAG 2.2 コントラスト・色依存回避・フォーカス・APCA: WebAIM, W3C
- データ可視化の配色（ダークが弁別に有利・色数最小化）: IBM Carbon／カテゴリ配色は Okabe-Ito を基に調整

---

*Design Language v0 — ダーク既定・WCAG 2.2 AA 目標。トークン値は確定時に実測ツールで再検証すること。*