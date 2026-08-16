import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  cancelReservation,
  createReservation,
  getReservation,
  listReservations,
  type Reservation,
} from "./api/client";
import { messageForCode, userFacingError } from "./userMessages";

const POLL_INTERVAL_MS = 2_000;
const TERMINAL_STATES = new Set<Reservation["state"]>([
  "completed",
  "failed",
  "canceled",
]);

export function ReservationsPage({
  path,
  onNavigate,
}: {
  path: string;
  onNavigate: (path: string) => void;
}) {
  const detailId = path.match(/^\/reservations\/([^/]+)$/)?.[1];
  return detailId ? (
    <ReservationDetail id={detailId} onNavigate={onNavigate} />
  ) : (
    <ReservationIndex onNavigate={onNavigate} />
  );
}

function ReservationIndex({
  onNavigate,
}: {
  onNavigate: (path: string) => void;
}) {
  const [items, setItems] = useState<Reservation[]>();
  const [url, setURL] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>();

  useEffect(() => {
    void listReservations()
      .then((page) => setItems(page.items))
      .catch((requestError: unknown) => setError(errorMessage(requestError)));
  }, []);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError(undefined);
    try {
      const reservation = await createReservation(url);
      onNavigate(`/reservations/${reservation.id}`);
    } catch (requestError) {
      setError(errorMessage(requestError));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <section className="hero reservation-hero">
        <p className="eyebrow">自動収集</p>
        <h1>配信終了前に、収集を予約。</h1>
        <p className="lede">
          配信状態を自動で確認し、アーカイブの準備ができたらチャットリプレイを収集します。
        </p>
        <form className="registration-form" onSubmit={submit}>
          <label htmlFor="reservation-url">YouTube URL</label>
          <div className="input-row">
            <input
              id="reservation-url"
              type="url"
              required
              value={url}
              placeholder="https://youtube.com/watch?v=…"
              onChange={(event) => setURL(event.target.value)}
            />
            <button disabled={submitting} type="submit">
              {submitting ? "予約しています…" : "収集を予約"}
            </button>
          </div>
        </form>
      </section>
      {error ? (
        <p className="inline-error" role="alert">
          {error}
        </p>
      ) : null}
      <section className="library" aria-labelledby="reservations-heading">
        <div className="section-heading">
          <div>
            <p className="eyebrow">監視キュー</p>
            <h2 id="reservations-heading">予約一覧</h2>
          </div>
          <span className="count">全{items?.length ?? 0}件</span>
        </div>
        {items === undefined ? (
          <p className="loading-state" role="status">
            予約を読み込んでいます…
          </p>
        ) : items.length === 0 ? (
          <div className="empty-state">
            <p>予約はまだありません。</p>
            <span>
              配信予定またはライブ配信中の YouTube URL を追加してください。
            </span>
          </div>
        ) : (
          <div className="reservation-list">
            {items.map((reservation) => (
              <a
                className="reservation-row"
                href={`/reservations/${reservation.id}`}
                key={reservation.id}
                onClick={(event) => {
                  event.preventDefault();
                  onNavigate(`/reservations/${reservation.id}`);
                }}
              >
                <span
                  className={`status reservation-status-${reservation.state}`}
                >
                  {stateLabel(reservation.state)}
                </span>
                <strong>{reservation.youtubeVideoId}</strong>
                <span>{nextAction(reservation)}</span>
              </a>
            ))}
          </div>
        )}
      </section>
    </>
  );
}

function ReservationDetail({
  id,
  onNavigate,
}: {
  id: string;
  onNavigate: (path: string) => void;
}) {
  const [reservation, setReservation] = useState<Reservation>();
  const [error, setError] = useState<string>();
  const [canceling, setCanceling] = useState(false);

  const load = useCallback(async () => {
    try {
      setReservation(await getReservation(id));
      setError(undefined);
    } catch (requestError) {
      setError(errorMessage(requestError));
    }
  }, [id]);

  useEffect(() => {
    let canceled = false;
    void getReservation(id)
      .then((nextReservation) => {
        if (!canceled) setReservation(nextReservation);
      })
      .catch((requestError: unknown) => {
        if (!canceled) setError(errorMessage(requestError));
      });
    return () => {
      canceled = true;
    };
  }, [id]);
  useEffect(() => {
    if (!reservation || TERMINAL_STATES.has(reservation.state)) return;
    const timer = window.setTimeout(() => void load(), POLL_INTERVAL_MS);
    return () => window.clearTimeout(timer);
  }, [load, reservation]);

  async function cancel() {
    setCanceling(true);
    setError(undefined);
    try {
      setReservation(await cancelReservation(id));
    } catch (requestError) {
      setError(errorMessage(requestError));
    } finally {
      setCanceling(false);
    }
  }

  if (!reservation && !error)
    return (
      <p className="loading-state" role="status">
        予約を読み込んでいます…
      </p>
    );
  if (!reservation)
    return (
      <p className="inline-error" role="alert">
        {error}
      </p>
    );

  return (
    <article className="reservation-detail">
      <a
        className="back-link"
        href="/reservations"
        onClick={(event) => {
          event.preventDefault();
          onNavigate("/reservations");
        }}
      >
        ← 予約一覧に戻る
      </a>
      {error ? (
        <p className="inline-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="reservation-detail-heading">
        <div>
          <p className="eyebrow">予約 {reservation.youtubeVideoId}</p>
          <h1>{stateLabel(reservation.state)}</h1>
          <p className="lede">{nextAction(reservation)}</p>
        </div>
        <span className={`status reservation-status-${reservation.state}`}>
          {stateLabel(reservation.state)}
        </span>
      </div>
      <dl className="metadata-grid reservation-metadata">
        <div>
          <dt>次回確認</dt>
          <dd>{formatDateTime(reservation.nextCheckAt)}</dd>
        </div>
        <div>
          <dt>監視試行回数</dt>
          <dd>{reservation.monitorAttempt}</dd>
        </div>
        <div>
          <dt>配信予定時刻</dt>
          <dd>{formatDateTime(reservation.scheduledStartAt)}</dd>
        </div>
      </dl>
      {reservation.lastErrorMessage ? (
        <section
          className="reservation-issue"
          aria-labelledby="monitoring-issue"
        >
          <h2 id="monitoring-issue">監視エラー</h2>
          <p>{messageForCode(reservation.lastErrorCode)}</p>
          <span>
            {reservation.lastErrorRetryable
              ? "監視は自動的に再試行されます。"
              : "監視を自動で継続できません。"}
          </span>
        </section>
      ) : null}
      {reservation.collectionError ? (
        <section
          className="reservation-issue"
          aria-labelledby="collection-issue"
        >
          <h2 id="collection-issue">収集エラー</h2>
          <p>{messageForCode(reservation.collectionError.code)}</p>
          <span>
            {reservation.collectionError.retryable
              ? "ストリーム画面から収集を再試行できます。"
              : "収集を自動では再試行できません。"}
          </span>
        </section>
      ) : null}
      <div className="reservation-actions">
        <a href={reservation.sourceUrl} target="_blank" rel="noreferrer">
          YouTube で開く
        </a>
        {reservation.canCancel ? (
          <button
            type="button"
            disabled={canceling}
            onClick={() => void cancel()}
          >
            {canceling ? "キャンセルしています…" : "予約をキャンセル"}
          </button>
        ) : null}
        {reservation.state === "completed" && reservation.streamId ? (
          <a
            className="primary-link"
            href={`/streams/${reservation.streamId}`}
            onClick={(event) => {
              event.preventDefault();
              onNavigate(`/streams/${reservation.streamId}`);
            }}
          >
            収集済みストリームを開く
          </a>
        ) : null}
      </div>
    </article>
  );
}

function stateLabel(state: Reservation["state"]): string {
  return {
    scheduled: "配信待ち",
    monitoring: "監視中",
    live: "ライブ配信中",
    waiting_for_archive: "アーカイブ待ち",
    collecting: "収集中",
    completed: "完了",
    failed: "失敗",
    canceled: "キャンセル済み",
  }[state];
}

function nextAction(reservation: Reservation): string {
  return {
    scheduled: "配信予定時刻が近づくまで待機しています。",
    monitoring: "YouTube で配信状態を確認しています。",
    live: "ライブ配信中です。終了まで監視を続けます。",
    waiting_for_archive:
      "YouTube がアーカイブとチャットリプレイを準備するのを待っています。",
    collecting: "アーカイブされたチャットリプレイを収集・保存しています。",
    completed: "収集が完了しました。ストリームを開いてチャットを確認できます。",
    failed: "監視を停止しました。下の監視エラーを確認してください。",
    canceled: "予約はキャンセルされました。以降の処理は実行されません。",
  }[reservation.state];
}

function formatDateTime(value?: string): string {
  return value
    ? new Intl.DateTimeFormat("ja-JP", {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(value))
    : "未設定";
}

function errorMessage(error: unknown): string {
  return userFacingError(
    error,
    "予約リクエストを完了できませんでした。もう一度お試しください。",
  );
}
