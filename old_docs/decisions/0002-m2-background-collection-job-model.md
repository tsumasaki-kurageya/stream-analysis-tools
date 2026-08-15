# ADR-0002: M2のバックグラウンド収集ジョブモデル

- Status: Accepted
- Date: 2026-08-02
- Scope: M2 ライブチャット収集

## Context

ライブチャット収集はHTTP要求時間を超えるため、Main APIから独立したPythonワーカーで実行する必要がある。画面を閉じても処理を継続し、複数ワーカー、異常終了、再実行に対して安全でなければならない。

## Decision

### 責務

- Web UI: 収集開始、状態ポーリング、失敗理由表示、再実行、完了後の閲覧導線。
- Main API: Stream存在確認、ジョブ作成、二重開始防止、最新状態取得、再実行受付、チャット閲覧API。
- Pythonワーカー: DBからジョブをclaimし、チャットgatewayを実行し、ページ単位で保存・進捗更新する。
- PostgreSQL: ジョブ状態、工程状態、リース、チャット正本、重複排除の唯一の共有状態。

Main APIからワーカーへの直接RPCやインメモリキューは使用しない。

### 状態遷移

CollectionJobは `queued -> running -> succeeded|failed` を基本とする。

再実行は既存行をqueuedへ戻さず、新しいattemptのCollectionJobを作成する。配信ごとにqueued/runningの有効ジョブは最大1件とする。

許可遷移:

- queued -> running
- running -> succeeded
- running -> failed
- running -> queued（lease期限切れの回復処理のみ。attemptを増加）

禁止遷移:

- succeeded -> 任意状態
- failed -> running（retry APIは新規ジョブを作る）
- queued -> succeeded/failed

CollectionStepは `pending -> running -> succeeded|failed` とし、M2では `fetch_chat` の1工程から開始する。工程追加時もジョブ状態と分離する。

### Claimとリース

ワーカーは1トランザクションで次を行う。

1. queued、またはlease期限切れrunningの候補を `FOR UPDATE SKIP LOCKED` で1件取得する。
2. worker_id、lease_expires_at、heartbeat_at、started_at、attemptを更新する。
3. commit後に外部取得を開始する。

heartbeatは30秒間隔、leaseは120秒を既定値とする。処理中は60秒以内のポーリング遅延を許容する。期限切れジョブは別ワーカーが再claimできる。ページ保存と進捗更新は同一トランザクションで行う。

### 冪等性

ChatMessageの正本キーは `(stream_id, source, external_message_id)` とする。external_message_idが取得不能なfixtureは本番経路へ流さない。

再実行、ページ重複、ワーカー異常終了後の再claimでは `ON CONFLICT DO NOTHING` を使う。取得件数は受信件数ではなくDBに存在するメッセージ総数を表示する。

### 時刻

- `published_at`: YouTube上の投稿絶対時刻。
- `offset_milliseconds`: Stream.actual_start_atからの差。0未満は保存可能とし、UIで配信開始前と判別する。
- 一覧順序: `offset_milliseconds, published_at, external_message_id`。

### Gateway境界

ワーカーのchat gatewayはcontinuationページを正規化済みメッセージ列と次continuationへ変換する。HTTP、JSON、YouTube固有構造はgateway内部に閉じ込める。

fixture/stubは同じgateway契約を使い、正常複数ページ、重複ページ、一時失敗、恒久失敗、空チャットを提供する。秘密情報と生CookieはDB・ログ・artifactへ保存しない。

### ポーリング

Web UIは処理中のみ3秒間隔でlatest job APIをポーリングする。バックグラウンドタブではブラウザ制御を許容し、再表示時に即時取得する。M2ではSSE/WebSocketを導入しない。

## Consequences

- PostgreSQLだけでMain APIとワーカーを疎結合にできる。
- at-least-once実行となるため、保存処理の冪等性が必須になる。
- 外部取得の完全なexactly-onceは保証せず、DB正本で重複を防ぐ。
- 長時間処理でも画面やAPIプロセスの再起動に影響されない。
