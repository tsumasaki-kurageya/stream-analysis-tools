# YouTube Data Gateway 契約

YouTube Data Gateway は、YouTube固有のチャットリプレイ取得方式を配信収集Workerから隔離する、private network専用のステートレスHTTPサービスです。

## 責務

- 実YouTubeチャットリプレイをページ単位で取得する
- YouTube固有レスポンスを契約済みJSONへ正規化する
- continuationをopaque tokenとして発行・検証する
- データなし、準備中、アクセス拒否、一時障害、外部仕様変更を分類する
- Cookieやproxy等の取得用認証情報をGateway内に閉じ込める

GatewayはJob状態や収集結果を永続化しません。正本はPostgreSQLです。

## Endpoint

| Endpoint | 用途 |
|---|---|
| `GET /healthz` | process liveness |
| `GET /readyz` | 設定とchat providerのreadiness |
| `GET /v1/chat-replay/pages` | 正規化済みチャットリプレイ1ページ |

完全なHTTP契約は `contracts/youtube-data-gateway.yaml` を正本とします。private endpointはBearer tokenを要求し、token rotation中は現行値と直前値を受け付けます。

## 運用境界

GatewayログへCookie、Authorization header、proxy認証情報、チャット本文を出力しません。YouTubeの外部仕様変更は `YOUTUBE_SOURCE_CHANGED`、一時障害はretryableなエラーとしてWorkerへ返します。

設計判断は `docs/decisions/0004-youtube-data-gateway-service.md`、字幕等の廃止判断は `docs/decisions/0005-discontinue-media-and-transcript-collection.md` を参照します。
