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
  const [createExpanded, setCreateExpanded] = useState(false);
  const [url, setURL] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [listError, setListError] = useState<string>();
  const [createError, setCreateError] = useState<string>();

  useEffect(() => {
    void listReservations()
      .then((page) => {
        setItems(page.items);
        setListError(undefined);
      })
      .catch((requestError: unknown) =>
        setListError(errorMessage(requestError)),
      );
  }, []);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setCreateError(undefined);
    try {
      const reservation = await createReservation(url);
      onNavigate(`/reservations/${reservation.id}`);
    } catch (requestError) {
      setCreateError(errorMessage(requestError));
    } finally {
      setSubmitting(false);
    }
  }

  const active = items?.filter((item) => !TERMINAL_STATES.has(item.state));
  const history = items?.filter((item) => TERMINAL_STATES.has(item.state));

  return (
    <section className="reservation-index" aria-labelledby="reservations-title">
      <div className="page-heading-row">
        <div>
          <h1 id="reservations-title">予約一覧</h1>
          {items ? <span className="count">全{items.length}件</span> : null}
        </div>
        <button
          type="button"
          className="secondary-button"
          aria-expanded={createExpanded}
          aria-controls="reservation-create-panel"
          onClick={() => {
            setCreateExpanded((expanded) => !expanded);
            setCreateError(undefined);
          }}
        >
          {createExpanded ? "予約作成を閉じる" : "収集を予約"}
        </button>
      </div>

      {createExpanded ? (
        <section
          id="reservation-create-panel"
          className="reservation-create-panel"
          aria-labelledby="reservation-create-title"
        >
          <h2 id="reservation-create-title">新しい収集予約</h2>
          <form
            className="registration-form reservation-form"
            onSubmit={submit}
          >
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
          {createError ? (
            <p className="inline-error" role="alert">
              {createError}
            </p>
          ) : null}
        </section>
      ) : null}

      {listError ? (
        <p className="inline-error" role="alert">
          {listError}
        </p>
      ) : items === undefined ? (
        <p className="loading-state" role="status">
          予約を読み込んでいます…
        </p>
      ) : (
        <div className="reservation-groups">
          <ReservationGroup
            title="進行中"
            items={active ?? []}
            emptyMessage="進行中の予約はありません。"
            onNavigate={onNavigate}
          />
          <ReservationGroup
            title="履歴"
            items={history ?? []}
            emptyMessage="履歴はありません。"
            onNavigate={onNavigate}
            subdued
          />
        </div>
      )}
    </section>
  );
}

function ReservationGroup({
  title,
  items,
  emptyMessage,
  onNavigate,
  subdued = false,
}: {
  title: string;
  items: Reservation[];
  emptyMessage: string;
  onNavigate: (path: string) => void;
  subdued?: boolean;
}) {
  return (
    <section
      className={`reservation-group${subdued ? " reservation-group-history" : ""}`}
      aria-labelledby={`reservation-group-${title}`}
    >
      <div className="section-heading compact-section-heading">
        <h2 id={`reservation-group-${title}`}>{title}</h2>
        <span className="count">{items.length}件</span>
      </div>
      {items.length === 0 ? (
        <p className="empty-state compact-empty-state">{emptyMessage}</p>
      ) : (
        <div className="reservation-table-wrap">
          <table className="reservation-table">
            <thead>
              <tr>
                <th scope="col">配信</th>
                <th scope="col">配信予定日時</th>
                <th scope="col">状態</th>
                <th scope="col">次回確認</th>
                <th scope="col">エラー</th>
              </tr>
            </thead>
            <tbody>
              {items.map((reservation) => (
                <tr key={reservation.id}>
                  <td>
                    <a
                      href={`/reservations/${reservation.id}`}
                      onClick={(event) => {
                        event.preventDefault();
                        onNavigate(`/reservations/${reservation.id}`);
                      }}
                    >
                      {reservation.youtubeVideoId}
                    </a>
                  </td>
                  <td>{formatDateTime(reservation.scheduledStartAt)}</td>
                  <td>
                    <span
                      className={`status reservation-status-${reservation.state}`}
                    >
                      {stateLabel(reservation.state)}
                    </span>
                  </td>
                  <td>
                    {TERMINAL_STATES.has(reservation.state)
                      ? "—"
                      : formatDateTime(reservation.nextCheckAt)}
                  </td>
                  <td>{hasReservationError(reservation) ? "あり" : "なし"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
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

  if (!reservation && !error) {
    return (
      <p className="loading-state" role="status">
        予約を読み込んでいます…
      </p>
    );
  }
  if (!reservation) {
    return (
      <p className="inline-error" role="alert">
        {error}
      </p>
    );
  }

  const terminal = TERMINAL_STATES.has(reservation.state);

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

      <header className="reservation-detail-heading">
        <div>
          <h1>{reservation.youtubeVideoId}</h1>
          <p>{nextAction(reservation)}</p>
        </div>
        <span className={`status reservation-status-${reservation.state}`}>
          {stateLabel(reservation.state)}
        </span>
      </header>

      <dl className="metadata-grid reservation-metadata">
        <div>
          <dt>配信予定時刻</dt>
          <dd>{formatDateTime(reservation.scheduledStartAt)}</dd>
        </div>
        {!terminal ? (
          <div>
            <dt>次回確認</dt>
            <dd>{formatDateTime(reservation.nextCheckAt)}</dd>
          </div>
        ) : null}
        <div>
          <dt>監視試行回数</dt>
          <dd>{reservation.monitorAttempt}</dd>
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

function hasReservationError(reservation: Reservation) {
  return Boolean(
    reservation.lastErrorCode ||
    reservation.lastErrorMessage ||
    reservation.collectionError,
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
    completed: "収集が完了しました。",
    failed: "監視を停止しました。エラー内容を確認してください。",
    canceled: "予約はキャンセルされました。",
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
