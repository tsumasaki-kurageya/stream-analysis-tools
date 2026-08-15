# M1 ローカル構成・コード生成方針

Related issue: #13

## ローカル構成

リポジトリルートのComposeをローカル実行の正本とする。

```text
Host / Dev Container
  ├─ Web UI (Vite)
  ├─ Main API (Go)
  ├─ PostgreSQL (Compose service)
  └─ YouTube API stub (Go test server; test時のみ)
```

- PostgreSQLはルートComposeのサービスとして提供する
- Dev Containerからも同じComposeサービス名で接続する
- WebとAPIは開発時のホットリロードを優先し、ホストまたはDev Container内プロセスとして起動する
- YouTube APIスタブは独立起動可能なGoプロセスとし、API統合テストとPlaywright E2Eで共用する
- 実YouTube APIは完了デモ時のみ使用し、CIでは使用しない

## 設定値

M1で使用する主な環境変数:

```text
YSA_DATABASE_URL
YSA_HTTP_ADDRESS
YSA_YOUTUBE_API_KEY
YSA_YOUTUBE_API_BASE_URL
VITE_API_BASE_URL
```

`YSA_YOUTUBE_API_BASE_URL` は本番相当ではYouTube Data API、テストではスタブを指す。

## OpenAPIの正本

`contracts/openapi.yaml` をHTTP契約の正本とする。

変更順序:

1. OpenAPIを更新する
2. Goサーバー型・インターフェースを生成する
3. TypeScript API型・クライアントを生成する
4. 実装とテストを更新する
5. 生成差分が残っていないことをCIで検証する

## Goコード生成

- `oapi-codegen` を使用する
- リクエスト・レスポンス型とサーバーインターフェースを生成対象とする
- 業務ロジック、HTTPエラー変換、DBモデルは生成コードへ置かない
- 生成物は `apps/api/internal/generated/openapi` 配下を想定する

## Webコード生成

- OpenAPIからTypeScriptの型付きクライアントを生成する
- APIレスポンス型を画面側で重複定義しない
- 生成物は `apps/web/src/api/generated` 配下を想定する
- TanStack Queryのquery/mutation hook自体は画面要件に合わせて手書きし、生成クライアントを利用する

具体的な生成ツールは #14 で導入時に固定する。選定条件はOpenAPI 3.1対応、安定したTypeScript出力、CIでの再生成容易性とする。

## エラー契約

HTTPエラーはRFC 9457 Problem Details形式を使用し、`code` を追加する。

- 外部APIの生メッセージ、APIキー、内部スタックトレースを返さない
- UIはHTTP statusだけでなく`code`を使って利用者向け表示を決定する
- 未知の内部障害は`INTERNAL_ERROR`へ正規化する

## M1で導入しないもの

- Pythonワーカー、ジョブテーブル、リトライキュー
- 認証、クラウドデプロイ
- M2以降の収集API
