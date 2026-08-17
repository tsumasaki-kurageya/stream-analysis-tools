import { FormEvent, ReactNode, useState } from "react";
import type {
  CollectionJob,
  StreamListItem,
  StreamPreview,
} from "./api/client";

export function StreamListPage({
  streams,
  isLoading,
  error,
  url,
  onURLChange,
  onPreview,
  isPreviewing,
  preview,
  previewNode,
  onOpenStream,
}: {
  streams: StreamListItem[];
  isLoading: boolean;
  error: string | null;
  url: string;
  onURLChange: (value: string) => void;
  onPreview: (event: FormEvent<HTMLFormElement>) => void;
  isPreviewing: boolean;
  preview: StreamPreview | null;
  previewNode: ReactNode;
  onOpenStream: (stream: StreamListItem) => void;
}) {
  const [addExpanded, setAddExpanded] = useState(false);

  return (
    <section className="stream-list-page" aria-labelledby="stream-list-title">
      <div className="page-heading-row">
        <h1 id="stream-list-title">配信一覧</h1>
        <button
          type="button"
          className="secondary-button"
          aria-expanded={addExpanded}
          aria-controls="stream-add-panel"
          onClick={() => setAddExpanded((expanded) => !expanded)}
        >
          {addExpanded ? "配信追加を閉じる" : "配信を追加"}
        </button>
      </div>

      {addExpanded ? (
        <section
          id="stream-add-panel"
          className="stream-add-panel"
          aria-labelledby="stream-add-title"
        >
          <h2 id="stream-add-title">新しい配信</h2>
          <form
            className="registration-form stream-add-form"
            onSubmit={onPreview}
          >
            <label htmlFor="youtube-url">YouTube URL</label>
            <div className="input-row">
              <input
                id="youtube-url"
                type="url"
                value={url}
                onChange={(event) => onURLChange(event.target.value)}
                placeholder="https://youtube.com/watch?v=…"
                required
              />
              <button type="submit" disabled={isPreviewing}>
                {isPreviewing ? "確認しています…" : "プレビュー"}
              </button>
            </div>
          </form>
          {preview ? previewNode : null}
        </section>
      ) : null}

      {error ? (
        <p className="inline-error" role="alert">
          {error}
        </p>
      ) : isLoading ? (
        <p className="loading-state" role="status">
          配信を読み込んでいます…
        </p>
      ) : (
        <div className="stream-table-wrap">
          <table className="stream-table">
            <thead>
              <tr>
                <th scope="col">タイトル</th>
                <th scope="col">チャンネル</th>
                <th scope="col">配信日時</th>
                <th scope="col">配信時間</th>
                <th scope="col">配信状態</th>
                <th scope="col">収集状態</th>
                <th scope="col">チャット件数</th>
              </tr>
            </thead>
            <tbody>
              {streams.length === 0 ? (
                <tr>
                  <td colSpan={7} className="stream-table-empty">
                    登録済みの配信はありません。
                  </td>
                </tr>
              ) : (
                streams.map((stream) => (
                  <tr key={stream.id}>
                    <td>
                      <a
                        href={`/streams/${stream.id}`}
                        onClick={(event) => {
                          event.preventDefault();
                          onOpenStream(stream);
                        }}
                      >
                        {stream.title}
                      </a>
                    </td>
                    <td>{stream.channelTitle}</td>
                    <td>
                      {formatStreamDate(
                        stream.actualStartAt ?? stream.scheduledStartAt,
                      )}
                    </td>
                    <td>{formatDuration(stream.durationMs)}</td>
                    <td>{lifecycleLabel(stream.lifecycleStatus)}</td>
                    <td>{collectionLabel(stream.collectionStatus)}</td>
                    <td>{stream.chatMessageCount.toLocaleString("ja-JP")}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function lifecycleLabel(status: StreamListItem["lifecycleStatus"]) {
  return {
    ended: "終了",
    live: "ライブ配信中",
    scheduled: "配信予定",
    unavailable: "利用不可",
    unknown: "状態不明",
  }[status];
}

function collectionLabel(status?: CollectionJob["status"]) {
  if (!status) return "未収集";
  return {
    queued: "待機中",
    running: "収集中",
    succeeded: "完了",
    no_data: "データなし",
    failed: "失敗",
  }[status];
}

function formatStreamDate(value?: string) {
  if (!value) return "未確定";
  return new Intl.DateTimeFormat("ja-JP", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatDuration(durationMs?: number) {
  if (durationMs === undefined) return "不明";
  const totalMinutes = Math.round(durationMs / 60_000);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return hours > 0 ? `${hours}時間${minutes}分` : `${minutes}分`;
}
