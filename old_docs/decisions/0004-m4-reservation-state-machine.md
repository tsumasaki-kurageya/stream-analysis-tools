# ADR 0004: M4解析予約の状態機械と責務境界

- Status: Accepted
- Date: 2026-08-02
- Issue: #69

## Context

M4では、開始前または配信中のYouTube URLを登録し、配信終了後に利用者の操作なしでM3のfull収集を開始する。

解析予約は長時間存続し、監視プロセスの再起動、多重ワーカー、YouTube側の一時障害、アーカイブ準備遅延、M3収集の部分失敗を扱う必要がある。Reservation、Stream、CollectionJobの責務を混在させると、監視失敗と収集失敗の区別、重複ジョブ防止、再開処理が不明確になる。

## Decision

### 1. 責務境界

- `Reservation`は、YouTube動画の将来状態を監視し、M3収集を一度だけ起動するための長寿命オーケストレーション状態を保持する。
- `Stream`は、M1/M3と同じく、終了済み配信として確定したメタデータを保持する。予約作成時点ではStreamを必須としない。
- `CollectionJob`は、M3の`metadata / chat_replay / transcript`工程を実行する。Reservationは収集内部状態を複製せず、`collection_job_id`を参照する。
- 予約監視ワーカーはYouTube状態・アーカイブ利用可否を判定する。配信データ収集ワーカーの責務は変更しない。

### 2. Reservation状態

| 状態 | 意味 | terminal |
|---|---|---|
| `scheduled` | 予定開始時刻が未来で、その時刻付近まで待機中 | no |
| `monitoring` | 予定時刻不明、開始時刻接近、または配信開始確認中 | no |
| `live` | 配信中であることを確認済み | no |
| `waiting_for_archive` | 配信終了後、動画アーカイブとチャットリプレイの準備待ち | no |
| `collecting` | M3 full CollectionJobを作成済み。Jobの完了待ち | no |
| `completed` | 参照先CollectionJobが`succeeded` | yes |
| `cancelled` | 利用者が監視・自動収集を停止 | yes |
| `failed` | 監視または自動起動の恒久的失敗 | yes |

M3 CollectionJobが`partial`または`failed`の場合、Reservationは`collecting`を維持する。Reservationレスポンスの`collectionStatus`、`collectionErrorCode`、`collectionErrorMessage`で収集側の失敗を表示する。これにより、Reservationの`failed`は監視・起動失敗だけを意味する。

### 3. 許可する状態遷移

```text
scheduled ───────────────┐
  │ 予定時刻接近         │ 配信開始確認
  ▼                      ▼
monitoring ────────────> live
  │ 終了済み確認          │ 終了確認
  └──────────────┬───────┘
                 ▼
      waiting_for_archive
                 │ archive + chat replay ready
                 ▼
             collecting
                 │ CollectionJob succeeded
                 ▼
             completed

scheduled / monitoring / live / waiting_for_archive ──cancel──> cancelled
scheduled / monitoring / live / waiting_for_archive ──permanent monitoring error──> failed
waiting_for_archive ──permanent unavailability──> failed
```

禁止事項:

- terminal状態から他状態へ遷移しない。
- `collecting`以降はキャンセルしない。収集停止機能はM4対象外。
- `scheduled`から直接`collecting`へ遷移しない。
- `completed`はCollectionJobの`succeeded`確認なしに設定しない。
- `partial`または`failed`のCollectionJobをReservationの監視失敗へ変換しない。

### 4. 状態遷移の入力事実

監視Gatewayは最低限、次を返す。

- `videoExists`
- `broadcastState`: `scheduled | live | ended | unknown`
- `scheduledStartAt`
- `actualStartAt`
- `actualEndAt`
- `archiveState`: `processing | ready | unavailable | unknown`
- `chatReplayState`: `processing | ready | unavailable | unknown`

字幕有無は自動収集開始条件に含めない。M3の`transcript=no_data`規則を再利用する。

### 5. 再確認時刻と障害分類

Reservationは`next_check_at`を持ち、期限到来した行だけをclaimする。

推奨初期値:

- `scheduled`: 予定開始の15分前までは最大15分間隔。以後1分間隔。
- `monitoring`: 1分間隔。
- `live`: 2分間隔。
- `waiting_for_archive`: 最初の30分は2分、以後指数バックオフし最大30分。
- 一時障害: 1分から指数バックオフし最大30分。

一時障害では状態を維持し、`monitor_attempt`、`last_checked_at`、`last_error_*`を更新する。恒久障害だけ`failed`へ遷移する。

恒久障害例:

- 動画が削除・非公開で、再試行しても利用不能と判定された。
- アーカイブまたはチャットリプレイが恒久的に利用不能。
- 内部整合性違反により自動CollectionJobを作成できない。

### 6. Claim / lease契約

監視ワーカーは次の条件でReservationをclaimする。

```sql
WHERE state IN ('scheduled','monitoring','live','waiting_for_archive','collecting')
  AND next_check_at <= now()
  AND (lease_expires_at IS NULL OR lease_expires_at < now())
ORDER BY next_check_at, created_at, id
FOR UPDATE SKIP LOCKED
LIMIT 1
```

claim時に`worker_id`、`lease_expires_at`、`heartbeat_at`を設定する。処理中はheartbeatでleaseを延長する。期限切れleaseは別のワーカーが再claimできる。外部通信をDBトランザクション内で行わない。

状態更新は`id + worker_id + revision`を条件にした楽観ロックで行う。lease所有権またはrevisionが失われた更新は破棄する。

### 7. 重複防止

- 同じ`youtube_video_id`について、terminalでないReservationは1件だけ許可する部分一意インデックスを設ける。
- `reservation_id`を持つCollectionJobは1件だけ許可する一意制約を設ける。
- 自動CollectionJob作成、Reservationへの`collection_job_id`設定、`collecting`遷移は同一DBトランザクションで行う。
- 既に同じYouTube動画のStreamが存在する場合は再利用する。存在しない場合は終了済みメタデータ確定時にStreamをupsertする。

### 8. キャンセル

キャンセル可能状態は`scheduled / monitoring / live / waiting_for_archive`のみ。キャンセルAPIは原子的に`cancelled`へ遷移し、lease情報を消去する。処理中ワーカーは更新時のrevision不一致によりキャンセル後の状態を上書きできない。

### 9. API

利用者向け:

- `POST /reservations`
- `GET /reservations`
- `GET /reservations/{reservationId}`
- `POST /reservations/{reservationId}/cancel`

予約作成は開始前・配信中・終了直後のURLを許可する。既に終了済みで収集可能な場合も予約として受理し、`waiting_for_archive`または`collecting`から開始できる。

内部claim APIは公開HTTP APIにしない。監視ワーカーはPostgreSQL repositoryを直接使用する。

## Consequences

- M3の収集状態機械を再利用しつつ、長寿命監視の状態を独立管理できる。
- 監視失敗と収集失敗が利用者・運用者の両方に明確になる。
- 多重ワーカー、再起動、キャンセル競合、重複ジョブをDB制約とleaseで防止できる。
- Reservationが`collecting`のまま長期間残る可能性があるため、UIは参照先CollectionJobの工程状態と再実行導線を表示する必要がある。
