# M2 完了デモ手順

M2「ライブチャットをバックグラウンド収集し、時系列で閲覧する」の完了確認手順です。

## 自動E2E

```bash
make db-migrate
cd apps/worker && python -m pip install -e '.[dev]'
cd ../web && npm install && npx playwright install chromium
npm run test:e2e -- m2-completion.spec.ts
```

このシナリオはPostgreSQL、Main API、Pythonワーカー、Web UIを実際に起動し、チャット取得先だけを決定的なfixtureへ差し替えます。

確認対象:

- UIからジョブ作成
- queued/running表示
- ページ再読込後の状態復元
- ワーカーによる複数ページ取得
- ChatMessageのDB保存
- 完了後の時系列閲覧
- 再収集時の重複防止
- 一時障害、失敗理由表示、再実行

## 数時間規模の実配信デモ

1. `YSA_YOUTUBE_API_KEY`を設定する。
2. `YSA_CHAT_REPLAY_BASE_URL`へ実チャット取得GatewayのURLを設定する。
3. PostgreSQL、Main API、Web、Pythonワーカーを起動する。
4. 数時間規模の終了済みライブ配信を登録する。
5. 配信詳細で「チャット収集を開始」を押す。
6. 画面を再読込し、同じジョブID・状態・取得件数が復元されることを確認する。
7. 完了後、チャット時系列を開き、先頭・中間・末尾の経過時刻と投稿順を確認する。
8. 同じ配信で再度収集を開始し、`chat.chat_messages`の件数が増えないことを確認する。

## 結果記録

実データ確認時は以下だけを記録し、APIキー、Cookie、continuation token、チャット本文の大量コピーは保存しません。

- 実施日
- YouTube video ID
- 動画時間
- CollectionJob IDと所要時間
- 保存件数
- 再収集後の保存件数
- 先頭・中間・末尾の経過時刻
- 成功／失敗と補足

実配信の結果は環境・認証情報が必要なため、CIではfixtureによる同等経路を検証します。
