import { useEffect, useMemo, useState } from "react";
import { getChatActivity, type ChatActivity } from "./api/client";
import { userFacingError } from "./userMessages";

const BUCKET_OPTIONS = [5, 10, 30] as const;
type BucketSeconds = (typeof BUCKET_OPTIONS)[number];
type ActivityLoadState = {
  key: string;
  activity?: ChatActivity;
  error?: string;
};

export function ChatActivityChart({
  streamId,
  playbackOffsetMilliseconds,
  onSeek,
}: {
  streamId: string;
  playbackOffsetMilliseconds: number;
  onSeek: (offsetMilliseconds: number) => void;
}) {
  const [bucketSeconds, setBucketSeconds] = useState<BucketSeconds>(10);
  const [loadState, setLoadState] = useState<ActivityLoadState>();
  const requestKey = `${streamId}:${bucketSeconds}`;

  useEffect(() => {
    let cancelled = false;
    void getChatActivity(streamId, bucketSeconds)
      .then((next) => {
        if (!cancelled) {
          setLoadState({ key: requestKey, activity: next });
        }
      })
      .catch((requestError: unknown) => {
        if (!cancelled) {
          setLoadState({
            key: requestKey,
            error: userFacingError(
              requestError,
              "チャット量を読み込めませんでした。もう一度お試しください。",
            ),
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [bucketSeconds, requestKey, streamId]);

  const currentState = loadState?.key === requestKey ? loadState : undefined;
  const activity = currentState?.activity;
  const error = currentState?.error;
  const loading = currentState === undefined;
  const maxCount = useMemo(
    () =>
      Math.max(1, ...(activity?.items.map((item) => item.messageCount) ?? [])),
    [activity],
  );
  const bucketMilliseconds = bucketSeconds * 1000;

  return (
    <section className="chat-activity" aria-labelledby="chat-activity-title">
      <div className="chat-activity-heading">
        <h3 id="chat-activity-title">チャット量</h3>
        <div className="bucket-selector" role="group" aria-label="集計単位">
          {BUCKET_OPTIONS.map((seconds) => (
            <button
              type="button"
              key={seconds}
              aria-pressed={bucketSeconds === seconds}
              onClick={() => setBucketSeconds(seconds)}
            >
              {seconds}秒
            </button>
          ))}
        </div>
      </div>

      {error ? (
        <p className="inline-error" role="alert">
          {error}
        </p>
      ) : loading ? (
        <p className="chat-empty" role="status">
          チャット量を読み込んでいます…
        </p>
      ) : !activity || activity.items.length === 0 ? (
        <p className="chat-empty">収集済みチャットはありません。</p>
      ) : (
        <div className="chat-activity-plot" aria-label="時間ごとのチャット件数">
          <div className="chat-activity-y-axis" aria-hidden="true">
            <span>{maxCount.toLocaleString("ja-JP")}</span>
            <span>0</span>
          </div>
          <div className="chat-activity-scroll">
            <div
              className="chat-activity-bars"
              style={{
                width: `${Math.max(100, activity.items.length * 6)}px`,
              }}
            >
              {activity.items.map((item) => {
                const active =
                  playbackOffsetMilliseconds >= item.startOffsetMilliseconds &&
                  playbackOffsetMilliseconds <
                    item.startOffsetMilliseconds + bucketMilliseconds;
                return (
                  <button
                    type="button"
                    key={item.startOffsetMilliseconds}
                    className="chat-activity-bar"
                    aria-current={active ? "time" : undefined}
                    aria-label={`${formatOffset(item.startOffsetMilliseconds)}から${bucketSeconds}秒間: ${item.messageCount}件`}
                    title={`${formatOffset(item.startOffsetMilliseconds)} · ${item.messageCount}件`}
                    onClick={() => onSeek(item.startOffsetMilliseconds)}
                  >
                    <span
                      aria-hidden="true"
                      style={{
                        height: `${Math.max(3, (item.messageCount / maxCount) * 100)}%`,
                      }}
                    />
                  </button>
                );
              })}
            </div>
            <div className="chat-activity-x-axis" aria-hidden="true">
              <span>0:00</span>
              <span>
                {formatOffset(
                  activity.items[activity.items.length - 1]
                    .startOffsetMilliseconds,
                )}
              </span>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

function formatOffset(offsetMilliseconds: number) {
  const totalSeconds = Math.max(0, Math.floor(offsetMilliseconds / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`
    : `${minutes}:${String(seconds).padStart(2, "0")}`;
}
