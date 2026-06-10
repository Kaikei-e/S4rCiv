---
title: Cloudflare Tunnel が edge への外向き TCP/7844 全滅で約12分 Error 1033 になった
date: 2026-06-07
status: resolved
severity: SEV3
authors:
  - Kaikei-e
tags:
  - cloudflare-tunnel
  - cloudflared
  - availability
  - error-1033
---

# Postmortem: Cloudflare Tunnel が edge への外向き TCP/7844 全滅で約12分 Error 1033 になった

## 概要

公開 Web（s4rciv.com、Cloudflare Tunnel 経由で配信）が約12分間 Error 1033（Cloudflare
Tunnel error）を返した。`cloudflared` の HA 4 接続すべてが、Cloudflare edge レンジへの
外向き TCP/7844 で `i/o timeout` になり、edge に登録された healthy な接続が 0 になった
のが原因。これは前回対処した QUIC フラッピングの再発ではなく別系統で、`http2` 固定は
健在だった。`cloudflared` はプロセス無再起動のまま自己復旧した。観測面（ground truth）と
収集（collector）は配信経路と独立で、一切影響を受けていない。再発時の復旧を速めるため
`TUNNEL_RETRIES=3` を適用済み。

## 影響

- **observation 面（ground truth）**: 無事。Cloudflare Tunnel は公開 Web の**配信経路**で
  あり、観測スナップショットもハッシュ連鎖イベントも触れない。収集は外向き HTTP GET の
  別経路で、トンネルとは無関係。
- **interpretation 面（read model）**: 無事。DB・read model は健全。劣化したのは
  「公開 Web の到達性」だけで、データではない。
- **利用者・対象**: 約12分、s4rciv.com への閲覧アクセスが 1033。API/collector の動作は無関係。
- **継続時間**: 2026-06-07 17:19:36–17:31:26 JST（約12分）。

## タイムライン（JST）

<!-- cloudflared のコンテナログは UTC。JST（＝UTC+9）に換算して記載。 -->

- `2026-06-07 15:51:01` `cloudflared` 起動。`Initial protocol http2`、HA 4 接続が即 Registered（全 `protocol=http2`）。以後約88分正常。
  <!-- この直前（UTC 06:50–06:51）に overlay が編集され http2 固定が適用されていた。 -->
- `2026-06-07 17:19:36`〜`17:27` 4 接続すべてが `DialContext error: dial tcp <edge>:7844: i/o timeout` を反復。登録接続 0 本。
- `2026-06-07 17:29:42` ユーザが s4rciv.com で Error 1033 を観測（断絶窓の中）。「過去に対応した問題の再発」と認識。
- `2026-06-07 17:31:14`〜`17:31:26` 4 接続とも `http2` で再 Registered。**プロセス無再起動**（RestartCount=0）で自己復旧。
- `2026-06-07`（同セッション） ログ精読で「http2 固定は健在・別系統の一過性ネットワーク断」と特定。
- `2026-06-07 17:49:20` `TUNNEL_RETRIES=3` を適用（`cloudflared` のみ再生成）。`http2` で 4 接続再登録を確認。

## 根本原因

1. なぜユーザは 1033 を見たか → 断絶窓中、edge に登録された healthy な `cloudflared` 接続が 0 で、edge はルーティング先を持てなかった。1033 の公式定義は「edge が healthy な `cloudflared` を見つけられない」。DNS はトンネルを指したままなので、全接続 unregistered の間ユーザに 1033 が出る。
2. なぜ接続が 0 になったか → HA 4 接続すべてが Cloudflare edge レンジ（region1=198.41.192.0/24・region2=198.41.200.0/24）への外向き TCP/7844 で `i/o timeout` になった。`http2` 固定ゆえ経路は TCP/7844 のみ。
3. なぜ全 edge へ同時に TCP timeout したか → ホストから両 edge レンジへの外向きパケットが約12分間ドロップされた。公式トラブルシュートが挙げる原因カテゴリは firewall/上流制御によるドロップ・経路上のパケットドロップ・Linux conntrack の established/related 追い出し。**具体的 root cause はホスト/上流側の事象で、コンテナログ上は特定不能（不明）**。`cloudflared` 内部の不具合ではなく一過性のネットワーク事象。
4. なぜ復旧に約12分かかったか（うち約4分は回線でなく `cloudflared` 側）→ `cloudflared` の再接続は指数 backoff（base 1s を `1<<retries` で倍々、full-jitter、`RetryForever`）。既定 `--retries=5` で天井が実 `1<<5=32s`／ログ表示 `1<<6=64s` まで伸び、**スリープ中はプローブしない**。最後の timeout（17:27:12）から次の再接続試行（17:31:14）まで約4分の空白があり、回線が窓の途中で戻っても次のタイマーまで気づかない。HA 4 接続が各々別 backoff でずれて積み上がる。
5. なぜ「前回の再発」に見えたか → 約88分前に同じ overlay を編集して `http2` 固定を適用した直後だったため、別原因の 1033 が「直したはずの問題のぶり返し」に見えた。実際にはログ上 `Initial protocol http2`・全接続 `protocol=http2`・実 QUIC 接続 0 で、http2 固定は健在だった（ログの `suggested_protocol=quic` は precheck の提案で実接続ではない）。

**根本原因**: ホスト→Cloudflare edge への外向き TCP/7844 が一過性に全滅したネットワーク事象。`cloudflared` は設計どおり自己復旧したが、既定の保守的な再接続 backoff が回線復帰後も数分の空回りを生み、断絶窓を必要以上に伸ばした。

## 検知

ユーザが s4rciv.com で 1033 を踏んで報告。アラートではなく人手で判明した。検知ギャップ:
`cloudflared` の `/ready`（0 接続で HTTP 503）を監視する仕組みが無く、「人間が 1033 を踏む」
以外にトンネル断を知る手段が無い。加えて、前回の http2 対策が gitignore された overlay の
コメントにしか残っておらず記録が無かったため、再発時に「同じ問題か別か」の切り分けに
ログ精読を要した。

## 対応と復旧

- **自己復旧**: `cloudflared` がプロセス無再起動のまま 17:31 に再登録し回復。
- **恒久寄りの調整（適用済）**: overlay（`compose.tunnel.yaml`）に `TUNNEL_RETRIES=3` を追加し、backoff 天井を実 `32s→8s`（表示 `64s→16s`）に縮小。回線復帰後の空回りを約 1/4 に。17:49:20 JST 適用、`http2` で 4 接続再登録を確認。
- **層の選択（公式仕様に照らした判断）**: 設定で backoff 天井を縮められるのは `--retries`（`TUNNEL_RETRIES`）のみで、即時リセット（retries=0）はプロセス再起動でしか起きない。`/ready` 監視＋自動再起動（autoheal）は `docker.sock` マウント＝ホスト root 相当で、`cloudflared` の `read_only`/`cap_drop: ALL`/`no-new-privileges` ハードニングを崩す。単発・自己復旧の事象に対しては過剰として却下し、設定一個で済む `--retries` 低減に留めた。**再発するなら autoheal と `/ready` 検知をまとめて再評価する**。

## ふりかえり（blameless）

- **うまくいったこと**: ログ精読で「http2 固定は健在・QUIC フラッピングは再発していない」と即断定でき、別系統と切り分けられた。`cloudflared` が無再起動で自己復旧した。観測面・収集が配信経路と独立だったため ground truth は無傷。
- **まずかったこと（仕組みの話）**: トンネル断の検知手段が「人間が 1033 を踏む」しかない。前回の http2 対策が gitignore された overlay にしか残らず、決定の記録が無かったため切り分けコストが上がった。
- **幸運だったこと**: 配信面のみの一過性で、観測面が不変・収集が独立だったためデータは無事。回線が自力で戻り、`cloudflared` が「再起動必須で stuck」（cloudflared #724/#917 の反例）にはならなかった。

## 再発防止アクション

| 内容 | 担当 | 種別 | 状態 | 追跡 |
|---|---|---|---|---|
| `TUNNEL_RETRIES=3` で backoff 天井を縮小（復旧の空回りを短縮） | Kaikei-e | 恒久対策 | done | `compose.tunnel.yaml`（gitignore）/ 本 postmortem・[[000023]] |
| 対策の経緯と却下案を ADR 化（http2 固定の未記録分も backfill） | Kaikei-e | 改善 | done | [[000023]] |
| `/ready`（503）検知の導入を、autoheal の是非と一体で判断 | TBD | 改善 | todo | — |
| 再発時にホスト/上流のネットワーク証跡（router log・`dmesg`・conntrack）を採取する手順を用意 | TBD | 改善 | todo | — |

## 再発確認

同根（ホスト→edge の外向き TCP/7844 全滅）の別事象は、現在のコンテナログ範囲では他に無い。
前回の事象は QUIC（UDP/7844）の "no recent network activity" フラッピングで別系統であり、
`http2` 固定で対処済み。今回はそれと独立した一過性のネットワーク断で、再接続機構は
protocol 非依存のため http2 固定が不利に働いた事実も無い。

## 教訓

- **ハードニングを崩さずに可用性を上げる**: `docker.sock` を要する自動再起動より、設定一個（`--retries`）で済む調整を優先する。受動・読取専用（設計原則①）とコンテナのハードニング姿勢を、可用性のために安易に妥協しない。
- **配信面の障害と観測面を混同しない**: トンネルは公開 Web の配信経路にすぎず、observation 面の ground truth・収集とは独立。配信が落ちてもデータは無傷、という二面分離の効きを再確認した。
- **gitignore された運用設定の決定は ADR/postmortem にしか durable に残らない**: 前回の http2 固定が記録されず、再発時に経緯が失われた反省を踏まえ、本対応は記録を伴わせる。

## 関連

- [[000019]] 署名チェックポイント（Cloudflare Tunnel を設計制約として参照）
- [[000023]] 本対応の ADR（`--retries` 低減と autoheal 却下の理由）
- [DISCIPLINE.md](../../DISCIPLINE.md) / [CORE_CONCEPT](../concepts/CORE_CONCEPT_0001.md) 設計原則①（受動・読取専用）・⑦（ソース遵守・ハードニング）
