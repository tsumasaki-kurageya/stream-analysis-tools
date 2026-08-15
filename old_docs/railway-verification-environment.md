# Railway検証環境の構築手順

## 1. 目的

Issue #76の実配信完了デモを実施するため、Railway上に次の検証環境を構築する。

```text
Internet
   ↓
web (public)
   ↓ private network
api ── Postgres
          ↑
       worker
          ↓ private network
youtube-data-gateway
          ↓
       YouTube
```

外部公開するのは`web`だけとする。`api`、`worker`、`youtube-data-gateway`、`Postgres`には公開DomainやTCP Proxyを設定しない。

## 2. 前提

- Railwayの有料プランを使用する
- GitHubリポジトリ`tsumasaki-kurageya/youtube-stream-analyzer`へRailwayからアクセスできる
- YouTube Data API v3のAPIキーを用意する
- Railway CLIを使用する場合はログインとProject linkを済ませる
- 秘密値をIssue、PR、ログ、スクリーンショットへ貼り付けない

## 3. ProjectとEnvironment

1. RailwayでEmpty Projectを作成する
2. Project名を`youtube-stream-analyzer-verification`など検証用と分かる名前にする
3. Environment名を`verification`とする
4. 全サービスを同じProject・Environment・リージョンへ配置する
5. 日本からの検証では`Southeast Asia / Singapore`を選択する

## 4. Shared Variables

ProjectのShared Variablesへ次を登録する。

```text
YSA_GATEWAY_BEARER_TOKEN=<32文字以上のランダム値>
YSA_GATEWAY_CONTINUATION_SECRET=<32文字以上の別のランダム値>
YSA_YOUTUBE_API_KEY=<YouTube Data API key>
```

Bearer tokenとcontinuation secretには同じ値を使わない。

## 5. サービス作成

同一Project内に次の5サービスを作成する。

| サービス名 | Source |
|---|---|
| `Postgres` | Railway PostgreSQL |
| `web` | GitHub Repository |
| `api` | GitHub Repository |
| `worker` | GitHub Repository |
| `youtube-data-gateway` | GitHub Repository |

GitHub Repositoryを使う4サービスは、すべて同じリポジトリと`main`ブランチへ接続する。

Root Directoryは`/`のままとし、各サービスのConfig File Pathを次のように設定する。

| サービス | Config File Path |
|---|---|
| `web` | `/deploy/railway/web.json` |
| `api` | `/deploy/railway/api.json` |
| `worker` | `/deploy/railway/worker.json` |
| `youtube-data-gateway` | `/deploy/railway/gateway.json` |

Config as CodeがDockerfile、watch path、health check、restart policy、API migrationを設定する。

## 6. Variables

### 6.1 web

```text
YSA_API_ORIGIN=http://${{api.RAILWAY_PRIVATE_DOMAIN}}:8080
```

`web`だけNetworkingからPublic Domainを生成する。

### 6.2 api

```text
YSA_DATABASE_URL=${{Postgres.DATABASE_URL}}
YSA_YOUTUBE_API_KEY=${{shared.YSA_YOUTUBE_API_KEY}}
YSA_YOUTUBE_API_BASE_URL=https://www.googleapis.com/youtube/v3
```

`api`のpre-deploy commandが`./migrate up`を実行する。`api`にはPublic Domainを生成しない。

### 6.3 worker

```text
YSA_DATABASE_URL=${{Postgres.DATABASE_URL}}
YSA_WORKER_ID=railway-verification-worker
YSA_WORKER_POLL_INTERVAL_SECONDS=3
YSA_WORKER_HEARTBEAT_INTERVAL_SECONDS=30
YSA_WORKER_LEASE_SECONDS=120
YSA_YOUTUBE_API_KEY=${{shared.YSA_YOUTUBE_API_KEY}}
YSA_YOUTUBE_API_BASE_URL=https://www.googleapis.com/youtube/v3
YSA_YOUTUBE_TIMEOUT_SECONDS=10
YSA_GATEWAY_BEARER_TOKEN=${{shared.YSA_GATEWAY_BEARER_TOKEN}}
YSA_CHAT_REPLAY_BASE_URL=http://${{youtube-data-gateway.RAILWAY_PRIVATE_DOMAIN}}:8080
YSA_CHAT_REPLAY_TIMEOUT_SECONDS=30
```

`worker`は1 replicaとし、Serverlessによる自動停止を使用しない。Restart Policyは`Always`とする。

### 6.4 youtube-data-gateway

```text
YSA_GATEWAY_TOKENS=${{shared.YSA_GATEWAY_BEARER_TOKEN}}
YSA_GATEWAY_CONTINUATION_SECRET=${{shared.YSA_GATEWAY_CONTINUATION_SECRET}}
YSA_GATEWAY_REQUEST_TIMEOUT_SECONDS=30
YSA_GATEWAY_CHAT_PAGE_SIZE=500
YSA_GATEWAY_TRANSCRIPT_PAGE_SIZE=1000
```

必要な場合だけ、認証情報を含むURLをIssue等へ貼らずに次を追加する。

```text
YSA_GATEWAY_PROXY_URL=<HTTPSまたはSOCKS proxy URL>
YSA_GATEWAY_COOKIE_FILE=<コンテナ内cookie file path>
```

初回はproxy・Cookieなしで検証し、YouTubeからアクセス拒否やIP blockが返った場合に限り追加する。

`youtube-data-gateway`にはPublic Domainを生成しない。

## 7. 初回デプロイ確認

VariablesとConfig File Pathを保存し、staged changesをDeployする。

次を確認する。

- `Postgres`が稼働している
- `api`のpre-deploy migrationが成功している
- `api`のDeploymentが`SUCCESS`
- `youtube-data-gateway`の`/readyz` health checkが成功している
- `worker`のDeploymentが`SUCCESS`
- `worker`ログに`worker ready`が出ている
- `web`のDeploymentが`SUCCESS`
- `web`のPublic Domainを開ける
- Web経由の`/api/health`と`/api/ready`が200になる

内部サービスにPublic Domainが作成されていないことも確認する。

## 8. Gateway実データ事前確認

実配信予約の前に、チャットリプレイを持つ終了済み配信IDを使い、Gatewayから応答できることを確認する。

Railway Dashboardで`youtube-data-gateway`を右クリックしてSSH commandを取得するか、Railway CLIで対象サービスへSSHする。

```bash
railway ssh --service youtube-data-gateway
```

コンテナ内ではtokenを画面出力せず、Worker経由で小さな収集ジョブを実行して次を確認する。

- チャットリプレイが1件以上保存される
- GatewayログへCookie、Authorization header、チャット本文が出ない

クラウドIPが拒否された場合は、GatewayのProblem Detailsとログに基づいてproxyまたはCookieを設定する。認証情報をGitHubへコミットしない。

## 9. Worker再起動確認

Issue #76の予約が`scheduled`、`monitoring`、または`live`の間に`worker`サービスをRestartする。

- Redeployではなく既存DeploymentのRestartを使用する
- 再起動時刻と予約状態を記録する
- 再起動後に同じReservation IDの監視が再開されることを確認する
- CollectionJobが重複しないことを確認する

Gateway再起動テストでは`youtube-data-gateway`もRestartし、一時障害後にWorkerの工程単位再実行が可能であることを確認する。

## 10. 証跡レポート

予約が`completed`になった後、APIサービスへSSHする。

```bash
railway ssh --service api
```

APIコンテナ内で次を実行する。

```bash
./m4-demo-report \
  -reservation-id '<reservation-uuid>' \
  -strict \
  -worker-restart-confirmed \
  -m3-sync-confirmed \
  -m3-search-confirmed \
  -m3-seek-confirmed
```

レポートが`PASS`であることを確認し、Issue #76へ貼り付ける。APIキー、token、Cookie、チャット本文は貼り付けない。

## 11. 検証終了後

環境を継続利用しない場合は、次を実施する。

1. `worker`と`youtube-data-gateway`を停止または削除する
2. `web`と`api`を停止または削除する
3. 必要な証跡だけを保存する
4. 不要ならPostgresを削除する
5. YouTube API key、Gateway bearer token、continuation secretを失効・更新する
6. proxyやCookieを設定した場合は、それらも失効・削除する
