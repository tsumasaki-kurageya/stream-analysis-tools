# M2 収集ジョブ・チャットデータモデル

## collection.collection_jobs

| 列 | 型 | 制約・用途 |
|---|---|---|
| id | UUID | PK、`gen_random_uuid()` |
| stream_id | UUID | FK `stream.streams(id)`、NOT NULL |
| kind | TEXT | M2は`chat_replay` |
| status | TEXT | queued/running/succeeded/failed |
| attempt | INTEGER | 1以上 |
| worker_id | TEXT | claim中のみNULL不可 |
| lease_expires_at | TIMESTAMPTZ | claim中の期限 |
| heartbeat_at | TIMESTAMPTZ | 最終生存確認 |
| requested_at | TIMESTAMPTZ | NOT NULL |
| started_at | TIMESTAMPTZ | NULL可 |
| finished_at | TIMESTAMPTZ | NULL可 |
| error_code | TEXT | 利用者向け分類 |
| error_message | TEXT | 秘密情報を含まない要約 |
| created_at/updated_at | TIMESTAMPTZ | NOT NULL |

制約・index:

- `attempt >= 1`
- statusごとの必須時刻をCHECKで検証
- `(stream_id, kind)` に対し `status IN ('queued','running')` の部分UNIQUE index
- claim用index `(status, requested_at, id)`
- lease回復用index `(status, lease_expires_at)`

## collection.collection_steps

| 列 | 型 | 制約・用途 |
|---|---|---|
| id | UUID | PK |
| job_id | UUID | FK、CASCADE |
| name | TEXT | M2は`fetch_chat` |
| status | TEXT | pending/running/succeeded/failed |
| processed_count | BIGINT | 0以上 |
| cursor | TEXT | continuation。ログへ出さない |
| started_at/finished_at | TIMESTAMPTZ | NULL可 |
| error_code/error_message | TEXT | NULL可 |
| created_at/updated_at | TIMESTAMPTZ | NOT NULL |

- `(job_id, name)` UNIQUE
- UIレスポンスではcursorを公開しない

## chat.chat_messages

| 列 | 型 | 制約・用途 |
|---|---|---|
| id | UUID | PK |
| stream_id | UUID | FK、NOT NULL |
| collection_job_id | UUID | 初回保存元。FK、NOT NULL |
| source | TEXT | `youtube_chat_replay` |
| external_message_id | TEXT | NOT NULL |
| author_channel_id | TEXT | NULL可 |
| author_display_name | TEXT | NOT NULL |
| message_text | TEXT | NOT NULL |
| published_at | TIMESTAMPTZ | NOT NULL |
| offset_milliseconds | BIGINT | NOT NULL、負数可 |
| message_type | TEXT | M2は`text`を必須対象 |
| created_at | TIMESTAMPTZ | NOT NULL |

制約・index:

- `(stream_id, source, external_message_id)` UNIQUE
- 時系列取得index `(stream_id, offset_milliseconds, published_at, external_message_id)`
- `collection_job_id` index
- 生レスポンス、Cookie、認証情報は保存しない

## 再実行

再実行は新しいCollectionJobを作る。ChatMessageは正本キーでupsertし、既存行のcollection_job_idは変更しない。新規挿入件数と総保存件数を区別し、UIの取得件数は総保存件数を表示する。

## API表示モデル

CollectionJobレスポンスはjob ID、stream ID、kind、status、attempt、現在step、processedCount、requested/started/updated/finished日時、errorのみを返す。worker ID、lease、heartbeat、cursorは内部情報として公開しない。
