import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  ApiProblem,
  cancelReservation,
  createReservation,
  getReservation,
  listReservations,
  type Reservation,
} from "./api/client";

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
    <ReservationDetail id={detailId} />
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
        <p className="eyebrow">Automatic collection</p>
        <h1>Reserve a livestream before it ends.</h1>
        <p className="lede">
          We’ll monitor the broadcast and collect its chat replay when the
          archive is ready.
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
              {submitting ? "Reserving…" : "Create reservation"}
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
            <p className="eyebrow">Monitoring queue</p>
            <h2 id="reservations-heading">Reservations</h2>
          </div>
          <span className="count">{items?.length ?? 0} total</span>
        </div>
        {items === undefined ? (
          <p className="loading-state" role="status">
            Loading reservations…
          </p>
        ) : items.length === 0 ? (
          <div className="empty-state">
            <p>No reservations yet.</p>
            <span>Add a scheduled or live YouTube URL above.</span>
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

function ReservationDetail({ id }: { id: string }) {
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
        Loading reservation…
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
      <a className="back-link" href="/reservations">
        ← Back to reservations
      </a>
      {error ? (
        <p className="inline-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="reservation-detail-heading">
        <div>
          <p className="eyebrow">Reservation {reservation.youtubeVideoId}</p>
          <h1>{stateLabel(reservation.state)}</h1>
          <p className="lede">{nextAction(reservation)}</p>
        </div>
        <span className={`status reservation-status-${reservation.state}`}>
          {stateLabel(reservation.state)}
        </span>
      </div>
      <dl className="metadata-grid reservation-metadata">
        <div>
          <dt>Next check</dt>
          <dd>{formatDateTime(reservation.nextCheckAt)}</dd>
        </div>
        <div>
          <dt>Monitor attempts</dt>
          <dd>{reservation.monitorAttempt}</dd>
        </div>
        <div>
          <dt>Scheduled start</dt>
          <dd>{formatDateTime(reservation.scheduledStartAt)}</dd>
        </div>
      </dl>
      {reservation.lastErrorMessage ? (
        <section
          className="reservation-issue"
          aria-labelledby="monitoring-issue"
        >
          <h2 id="monitoring-issue">Monitoring issue</h2>
          <p>{reservation.lastErrorMessage}</p>
          <span>
            {reservation.lastErrorRetryable
              ? "Monitoring will retry automatically."
              : "Monitoring cannot continue automatically."}
          </span>
        </section>
      ) : null}
      {reservation.collectionError ? (
        <section
          className="reservation-issue"
          aria-labelledby="collection-issue"
        >
          <h2 id="collection-issue">Collection issue</h2>
          <p>{reservation.collectionError.message}</p>
          <span>
            {reservation.collectionError.retryable
              ? "Collection can be retried from the stream page."
              : "Collection cannot be retried automatically."}
          </span>
        </section>
      ) : null}
      <div className="reservation-actions">
        <a href={reservation.sourceUrl} target="_blank" rel="noreferrer">
          Open on YouTube
        </a>
        {reservation.canCancel ? (
          <button
            type="button"
            disabled={canceling}
            onClick={() => void cancel()}
          >
            {canceling ? "Canceling…" : "Cancel reservation"}
          </button>
        ) : null}
        {reservation.state === "completed" && reservation.streamId ? (
          <a className="primary-link" href={`/streams/${reservation.streamId}`}>
            Open collected stream
          </a>
        ) : null}
      </div>
    </article>
  );
}

function stateLabel(state: Reservation["state"]): string {
  return {
    scheduled: "Scheduled",
    monitoring: "Monitoring",
    live: "Live",
    waiting_for_archive: "Waiting for archive",
    collecting: "Collecting",
    completed: "Completed",
    failed: "Failed",
    canceled: "Canceled",
  }[state];
}

function nextAction(reservation: Reservation): string {
  return {
    scheduled: "Waiting until the stream approaches its scheduled start.",
    monitoring: "Checking YouTube for the broadcast status.",
    live: "The stream is live. Monitoring continues until it ends.",
    waiting_for_archive:
      "Waiting for YouTube to prepare the archive and chat replay.",
    collecting: "Collecting and saving the archived chat replay.",
    completed: "Collection is complete. Open the stream to explore its chat.",
    failed: "Monitoring stopped. Review the monitoring issue below.",
    canceled: "This reservation was canceled. No further work will run.",
  }[reservation.state];
}

function formatDateTime(value?: string): string {
  return value
    ? new Intl.DateTimeFormat("en", {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(value))
    : "Not available";
}

function errorMessage(error: unknown): string {
  return error instanceof ApiProblem
    ? error.problem.detail
    : "The reservation request could not be completed. Try again.";
}
