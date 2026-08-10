# M4 実配信完了デモ

M4「配信を事前予約し、終了後に自動収集する」の実データ完了確認手順です。

## 対象配信

- 公開YouTubeライブ配信
- 配信終了後にアーカイブとチャットリプレイを利用できる
- 数時間規模の収集を確認できる

## 手順

1. 開始前または配信中のURLを解析予約へ登録する。
2. `scheduled / monitoring / live / waiting_for_archive` の状態遷移を確認する。
3. 監視Workerを一度再起動し、予約状態が復元されることを確認する。
4. アーカイブとチャットリプレイの準備完了後、自動でCollectionJobが1件だけ作成されることを確認する。
5. `metadata / chat_replay` の二工程が成功し、予約が `completed` になることを確認する。
6. 配信詳細でYouTube埋め込みプレーヤーとチャットを確認する。
7. チャット検索と、チャット項目または検索結果からの時刻ジャンプを確認する。
8. `m4-demo-report` を実行し、全完了条件がPASSになることを確認する。

## 証跡

- ReservationとCollectionJobのID
- 状態遷移と実行時刻
- `metadata / chat_replay` の工程状態とattempt
- CollectionJob件数とチャット保存件数
- Worker再起動、同期表示、検索、時刻ジャンプの確認結果

レポートにはCookie、APIキー、Authorization header、proxy認証情報、チャット本文を含めません。
