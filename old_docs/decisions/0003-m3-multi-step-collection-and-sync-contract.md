# ADR 0003: M3の複数工程収集と同期契約

- Status: Accepted
- Date: 2026-08-02
- Issue: #52

## Context

M2ではライブチャット収集だけを扱い、CollectionJobは単一工程の成功・失敗を表していた。M3ではメタデータ、チャットリプレイ、字幕を収集し、字幕なし、字幕取得失敗、チャット成功などを同時に表現する必要がある。

また、成功済み工程をやり直さず、失敗工程だけを再実行できなければならない。Web UIではYouTubeプレーヤー、チャット、字幕を動画開始からの経過時刻で同期し、同じ検索結果契約から時刻ジャンプする。

## Decision

### 1. JobとStepの責務

- CollectionJobは利用者が開始した「配信データ収集要求」を表す。
- CollectionStepは独立して取得、失敗判定、再実行できる工程を表す。
- M3の標準工程は `metadata`、`chat_replay`、`transcript` とする。
- ワーカーが排他取得する単位はStepとする。JobはStep状態から集約する。
- 同一Job内で同じStep名は1件だけとする。再実行はStepを上書きせず、step_attemptsへ試行履歴を追加する。

### 2. Step状態

Step状態は次の6種類とする。

- `queued`: 実行待ち
- `running`: ワーカーがleaseを保持して実行中
- `succeeded`: データ取得・保存が完了
- `no_data`: 正常に確認した結果、利用可能データが存在しない
- `failed`: エラーで完了していない
- `cancelled`: 親要求の取消等で実行しない

`transcript`の字幕なしは`no_data`であり、失敗ではない。アクセス拒否、外部仕様変更、タイムアウト等は`failed`とする。

### 3. Job集約状態

Job状態は次の規則でStepから算出する。

- `queued`: 全Stepが未開始で、少なくとも1件がqueued
- `running`: 1件以上がrunning、または完了していないqueuedが存在
- `succeeded`: 全Stepが`succeeded`または`no_data`
- `partial`: 1件以上が`succeeded`または`no_data`、かつ1件以上がfailed
- `failed`: 全ての実行対象Stepがfailed
- `cancelled`: 全Stepがcancelled、またはJob自体が取消済み

`partial`は利用可能データを閲覧可能とし、失敗工程の再実行導線を表示する。

### 4. 工程単位再実行

- `POST /collection-jobs/{jobId}/steps/{stepName}/retry`を使用する。
- 対象Stepが`failed`の場合だけ受理する。
- 成功済み・`no_data`・実行中・queuedのStepは再実行できない。
- 新しいstep_attemptを作成し、Stepをqueuedへ戻す。
- 他Stepの状態・保存データ・attemptは変更しない。
- 同一Stepのactive attemptは1件だけとする。

### 5. TranscriptSegment

TranscriptSegmentは外部字幕レスポンスではなく、同期表示用の正規化モデルとして保存する。

必須属性:

- stream_id
- external_track_id
- language_code
- is_auto_generated
- start_offset_milliseconds
- end_offset_milliseconds
- text
- source_segment_id

一意性は `(stream_id, external_track_id, source_segment_id)` を基本とする。source_segment_idが取得元に存在しない場合は、track、開始、終了、正規化本文から決定的IDを生成する。

同一トラックの再収集は、1回の保存トランザクション内でupsertし、取得完了後に今回観測されなかった旧セグメントを削除する。途中失敗時には旧データを残す。

### 6. 時刻規約

- API・DBの同期基準は動画開始からの経過ミリ秒とする。
- 開始時刻は0以上のint64。
- 字幕区間は半開区間 `[start, end)` とする。
- チャットは点時刻として扱う。
- プレーヤー秒は表示・操作境界でミリ秒へ変換し、内部では整数ミリ秒を使用する。
- 時刻ジャンプは対象のstart/offsetへseekする。

### 7. 取得・同期API

- `POST /streams/{streamId}/collections`: M3標準工程を含む収集開始
- `GET /streams/{streamId}/collections/latest`: 最新Jobと全Step状態
- `POST /collection-jobs/{jobId}/steps/{stepName}/retry`: 失敗工程だけ再実行
- `GET /streams/{streamId}/transcript-segments`: 字幕一覧・カーソルページング
- `GET /streams/{streamId}/timeline?fromMs=&toMs=`: 指定範囲のチャット・字幕を統一応答
- `GET /streams/{streamId}/search?q=&types=`: チャット・字幕の統一検索

既存M2のchat専用APIは互換のためM3期間中も残す。新規UIは汎用collection APIを利用する。

### 8. 検索

M3はPostgreSQLの部分一致検索を使う。検索結果は`chat`または`transcript`、経過時刻、本文、表示用主体情報を統一形式で返す。並び順は `(offsetMilliseconds, type, id)` とする。

ベクトル検索、意味検索、外部検索エンジンは導入しない。

## Error classification

字幕gatewayは次を区別する。

- `TRANSCRIPT_NOT_AVAILABLE`: 字幕なし。エラーではなくno_dataへ変換
- `TRANSCRIPT_ACCESS_DENIED`: 恒久失敗
- `TRANSCRIPT_SOURCE_CHANGED`: 外部仕様変更。恒久失敗
- `TRANSCRIPT_TEMPORARILY_UNAVAILABLE`: 一時失敗、再実行可能
- `TRANSCRIPT_TIMEOUT`: 一時失敗、再実行可能

Stepのerrorにはcode、message、retryableを保存する。

## Consequences

- JobとStepの状態集約ロジックが必要になる。
- Step単位のlease・attempt履歴が必要になる。
- 字幕なしを利用者へ明示しつつ、Job全体を成功扱いにできる。
- 部分成功時もチャット等の利用可能データを閲覧できる。
- 将来、字幕以外の工程を同じ仕組みへ追加できる。

## Rejected alternatives

### Jobを工程ごとに分割する

工程間の利用者要求、全体進捗、部分成功をまとめて表示しにくいため採用しない。

### 字幕なしをfailedにする

正常な配信特性と障害を区別できず、不要な再実行を誘発するため採用しない。

### 再実行でJob全体を複製する

成功済みチャット等の再取得が発生し、工程単位再実行の要件を満たさないため採用しない。
