# ADR-0004: YouTube実データ取得Gatewayを独立内部サービスとする

- Status: Accepted
- Date: 2026-08-03
- Issue: #87
- Parent: #86

## Context

M2・M3では、チャットリプレイと字幕の外部仕様をPython Worker内のGateway境界へ閉じ込める設計を採用した。現在はWorker側のHTTPクライアント、正規化・保存処理、fixture/stubまで存在するが、実際のYouTubeからチャットリプレイと字幕を取得して契約済みJSONへ変換する本番用の接続先が存在しない。

そのため、自動E2Eは成功していても、M3親Issue #51、M4親Issue #68、実配信完了デモ #76を実データで完了できない。また、現在のチャットクライアントはYouTube内部レスポンスに近いJSONを解釈しており、外部仕様をWorkerから隔離するという当初の境界を完全には満たしていない。

## Decision

### 1. デプロイ単位

モノレポ内に `youtube-data-gateway` を追加し、Web、Main API、Python Workerとは別プロセス・別コンテナとしてデプロイする。

初版ではチャットリプレイと字幕を1サービスにまとめる。次の条件が実測で発生した場合だけ分割を再検討する。

- チャットと字幕で必要なスケール特性が大きく異なる
- 認証情報またはアクセス権限を物理的に分離する必要がある
- リリース頻度・障害境界・担当責任が明確に分かれる

### 2. 技術スタック

初版はPython 3.12で実装する。

- FastAPI
- Pydantic v2
- Uvicorn
- HTTPX
- yt-dlp等のYouTube取得ライブラリは#88・#89で選定する
- pytest

取得方式はこのADRでは固定しない。Workerとの外部契約を固定し、YouTube側の取得実装を交換可能にする。

### 3. 責務分担

#### youtube-data-gateway

- YouTubeへアクセスする
- YouTube固有のレスポンスを解釈する
- チャットページ、字幕トラック、字幕セグメントを契約済みJSONへ正規化する
- データなし、アクセス拒否、一時障害、外部仕様変更を分類する
- Cookie等の取得用認証情報を保持する
- health / readinessを提供する

#### Python Worker

- CollectionStepのclaim、lease、heartbeatを管理する
- Gatewayをページ単位で呼び出す
- continuationループと全体ページ上限を管理する
- 配信開始時刻からの経過時刻を算出する
- PostgreSQLへ冪等保存する
- 進捗、失敗、再実行を管理する

#### PostgreSQL

収集ジョブ、状態、正規化済みチャット・字幕の唯一の正本とする。Gatewayは状態を保持せず、収集結果を永続化しない。

### 4. HTTP API

契約の正本は `contracts/youtube-data-gateway.yaml` とする。

- `GET /healthz`
- `GET /readyz`
- `GET /v1/chat-replay/pages?videoId=&continuation=`
- `GET /v1/transcripts/tracks?videoId=`
- `GET /v1/transcripts/segments?videoId=&trackId=&continuation=`

Gatewayは1リクエストにつき1ページだけ返す。continuationは不透明な文字列として扱い、Workerも内容を解釈しない。

チャットレスポンスはYouTubeのrendererやactionを返さず、次だけを返す。

- 外部メッセージID
- 投稿者外部ID
- 投稿者名
- 本文
- 投稿日時
- continuation

字幕レスポンスはトラック情報、開始・終了ミリ秒、本文、continuationだけを返す。

### 5. 認証とネットワーク

`/v1/*` はPrivate Network内でのみ到達可能とし、さらにBearer tokenを必須とする。

```http
Authorization: Bearer <shared-secret>
```

Gatewayは次の環境変数を受け付ける。

- `YSA_GATEWAY_AUTH_TOKEN`: 現行token
- `YSA_GATEWAY_AUTH_TOKEN_PREVIOUS`: ローテーション中だけ受理する旧token。省略可能

Workerは現行tokenだけを送信する。ローテーションは次の順序で行う。

1. Gatewayへ新tokenと旧tokenを設定する
2. Workerを新tokenへ切り替える
3. 旧tokenの利用がないことを確認する
4. Gatewayから旧tokenを削除する

`/healthz`と`/readyz`はプラットフォームprobeのため認証不要とする。ただし外部公開せず、応答には設定値や外部接続情報を含めない。

### 6. health / readiness

- `/healthz`: プロセスがリクエストを処理できる場合に200
- `/readyz`: 必須設定、認証token、取得providerの初期化が完了している場合に200

YouTubeへの到達確認はreadinessで毎回行わない。外部障害でGateway自体を再起動ループさせないためである。

### 7. エラー契約

エラーは `application/problem+json` で返し、次の拡張フィールドを必須とする。

- `code`: 安定した機械判定用コード
- `retryable`: Workerが工程再実行可否を判断する値
- `requestId`: ログ照合用ID

代表的な対応:

| HTTP | code | retryable | 意味 |
|---|---|---:|---|
| 400 | `INVALID_REQUEST` | false | videoId等の入力不正 |
| 401 | `GATEWAY_UNAUTHORIZED` | false | 内部token不正 |
| 403 | `YOUTUBE_ACCESS_DENIED` | false | YouTube側アクセス拒否 |
| 404 | `CHAT_REPLAY_NOT_AVAILABLE` / `TRANSCRIPT_NOT_AVAILABLE` | false | データが存在しない、または恒久取得不能 |
| 409 | `SOURCE_NOT_READY` | true | アーカイブ等がまだ処理中 |
| 429 | `YOUTUBE_RATE_LIMITED` | true | 上流rate limit |
| 502 | `YOUTUBE_SOURCE_CHANGED` | false | 上流仕様変更または解釈不能 |
| 503 | `YOUTUBE_TEMPORARILY_UNAVAILABLE` | true | 一時障害 |
| 504 | `YOUTUBE_TIMEOUT` | true | 上流timeout |

字幕トラックが存在しない場合はエラーにせず、200と空の`tracks`を返す。チャットが利用可能だがメッセージ0件の場合も200と空の`messages`を返す。

### 8. タイムアウトとリトライ

- Gatewayの1上流リクエストの既定timeoutは20秒
- WorkerからGatewayへの既定timeoutは30秒
- Gatewayはジョブ単位のリトライを行わない
- 429、5xx、timeoutはProblem Detailsとして返し、CollectionStepの再実行責務はWorkerに残す
- 同一HTTPリクエスト内の隠れた長時間バックオフは行わない

### 9. ログと秘密情報

構造化ログにはrequestId、endpoint、videoId、結果code、処理時間を記録してよい。

次は記録しない。

- Authorization header
- Cookie、生Cookie、Cookieファイル内容
- API key
- continuation全文
- 字幕・チャット本文
- YouTube生レスポンス

continuationを識別する必要がある場合は、不可逆hashの先頭だけを記録する。

### 10. 契約互換性

- URL prefixのmajor versionを互換性境界とする
- v1内では必須フィールドの削除・型変更を行わない
- optional field追加は許可する
- fixture/stubと実Gatewayは同じOpenAPI契約テストを通す
- 現在のWorker側チャットクライアントは#90で正規化済み`messages`契約へ移行する

## Consequences

- YouTube側の取得方式をWorker・DB・UIから独立して変更できる
- RailwayではGatewayだけをPrivate Serviceとして追加できる
- 共有tokenの設定・ローテーションが必要になる
- サービス間HTTP、timeout、契約バージョニング、可観測性を運用対象として持つ
- 初版はサービス数が1つ増えるが、M3・M4の実データ完了条件を満たせる

## Rejected alternatives

### Workerへ実取得処理を直接組み込む

デプロイは単純になるが、YouTube固有仕様、Cookie管理、取得ライブラリ変更がジョブ管理・DB保存と同じリリース単位になる。既存のHTTP Gatewayクライアント境界も活かせないため採用しない。

### チャットと字幕を最初から別サービスにする

現時点では利用者、認証情報、デプロイ先、負荷実測が共通であり、運用単位を2つに分ける根拠が不足しているため採用しない。

### Private Networkだけで認証しない

設定誤りや将来のネットワーク変更時に無認証アクセスとなるため採用しない。
