# M1 完了デモ

M1では、終了済みYouTubeライブ配信をURLから確認・登録し、再起動後も一覧と詳細から参照できることを確認する。

## 自動E2E

PostgreSQLを起動し、migrationを適用した状態で実行する。

```bash
cp .env.example .env
make setup
make db-up
make db-migrate
cd apps/web
npx playwright install chromium
npm run test:e2e
```

Playwrightは次のプロセスを自動起動する。

- Go製YouTube APIスタブ: `127.0.0.1:18080`
- Main API: `127.0.0.1:8080`
- Vite Web UI: `127.0.0.1:5173`

検証する主要導線は以下。

1. 0件の一覧から登録画面へ移動する
2. 終了済み配信URLを入力してプレビューする
3. タイトル、チャンネル、配信日時、動画時間を確認する
4. Streamを登録して詳細画面へ移動する
5. 詳細画面を再読込して永続化を確認する
6. 一覧へ戻り、登録済みStreamを再度開く
7. 同じURLを再登録しても既存Streamへ遷移する
8. 存在しないStream IDで404状態を確認する

失敗時は `apps/web/playwright-report/` と `apps/web/test-results/` を確認する。CIはPlaywright HTML reportを7日間artifactとして保存する。

## 実YouTube Data APIを使う手動デモ

`.env`へ以下を設定する。

```dotenv
YSA_DATABASE_URL=postgres://ysa:ysa@localhost:5432/youtube_stream_analyzer?sslmode=disable
YSA_YOUTUBE_API_KEY=<Google Cloudで発行したAPIキー>
YSA_YOUTUBE_API_BASE_URL=https://www.googleapis.com/youtube/v3
```

起動する。

```bash
make db-up
make db-migrate
make dev
```

ブラウザで `http://localhost:5173/streams` を開き、公開されている終了済みライブ配信URLを登録する。

## 完了判定

- URLから配信情報を取得できる
- 確認後に登録できる
- 同一動画を重複登録しない
- 一覧と詳細で保存済み情報を確認できる
- API・Web・PostgreSQLを再起動してもデータが残る
- `make check` と `npm run test:e2e` が成功する

M1ではデータ収集、動画プレーヤー、収集ジョブ、予約、リアルタイム解析を扱わない。
