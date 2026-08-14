import { FormEvent, MouseEvent, useCallback, useEffect, useState } from "react";
import {
  ApiProblem,
  createStream,
  findStreamByYouTubeVideoId,
  getStream,
  listStreams,
  previewStream,
  type Stream,
  type StreamPreview,
} from "./api/client";

export function App() {
  const [path, setPath] = useState(window.location.pathname);
  const [streams, setStreams] = useState<Stream[]>([]);
  const [url, setUrl] = useState("");
  const [preview, setPreview] = useState<StreamPreview | null>(null);
  const [isPreviewing, setIsPreviewing] = useState(false);
  const [isRegistering, setIsRegistering] = useState(false);
  const [isLoadingList, setIsLoadingList] = useState(
    !window.location.pathname.match(/^\/streams\/[^/]+$/),
  );
  const [selectedStream, setSelectedStream] = useState<Stream | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notFound, setNotFound] = useState(false);

  const handleRequestError = useCallback((requestError: unknown) => {
    if (
      requestError instanceof ApiProblem &&
      requestError.problem.code === "STREAM_NOT_FOUND"
    ) {
      setNotFound(true);
      setError(null);
      return;
    }
    setError(
      requestError instanceof ApiProblem
        ? requestError.problem.detail
        : "The request could not be completed. Check your connection and try again.",
    );
  }, []);

  useEffect(() => {
    const detailMatch = path.match(/^\/streams\/([^/]+)$/);
    if (detailMatch) {
      const streamId = detailMatch[1];
      if (selectedStream?.id !== streamId) {
        void getStream(streamId)
          .then(setSelectedStream)
          .catch(handleRequestError);
      }
      return;
    }
    void listStreams()
      .then((page) => setStreams(page.items))
      .catch(handleRequestError)
      .finally(() => setIsLoadingList(false));
  }, [handleRequestError, path, selectedStream?.id]);

  async function handlePreview(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsPreviewing(true);
    setError(null);
    setNotFound(false);
    try {
      setPreview(await previewStream(url));
    } catch (requestError) {
      handleRequestError(requestError);
    } finally {
      setIsPreviewing(false);
    }
  }

  async function handleRegister() {
    setIsRegistering(true);
    setError(null);
    setNotFound(false);
    try {
      const stream = await createStream(url);
      navigateToStream(stream);
    } catch (requestError) {
      let existing =
        requestError instanceof ApiProblem &&
        requestError.problem.code === "STREAM_ALREADY_REGISTERED" &&
        preview
          ? streams.find(
              (stream) => stream.youtubeVideoId === preview.youtubeVideoId,
            )
          : undefined;
      if (
        !existing &&
        requestError instanceof ApiProblem &&
        requestError.problem.code === "STREAM_ALREADY_REGISTERED" &&
        preview
      ) {
        existing = await findStreamByYouTubeVideoId(preview.youtubeVideoId);
      }
      if (existing) {
        navigateToStream(existing);
      } else {
        handleRequestError(requestError);
      }
    } finally {
      setIsRegistering(false);
    }
  }

  function openStream(event: MouseEvent<HTMLAnchorElement>, stream: Stream) {
    event.preventDefault();
    navigateToStream(stream);
  }

  function navigateToStream(stream: Stream) {
    setSelectedStream(stream);
    const nextPath = `/streams/${stream.id}`;
    window.history.pushState(null, "", nextPath);
    setPath(nextPath);
  }

  const detailMatch = path.match(/^\/streams\/[^/]+$/);

  return (
    <div className="app-shell">
      <header className="site-header">
        <a className="brand" href="/streams">
          <span className="brand-mark" aria-hidden="true">
            SA
          </span>
          <span>Stream Analysis</span>
        </a>
        <span className="milestone">M1 · Stream library</span>
      </header>

      <main>
        {error ? (
          <div className="error-banner" role="alert">
            <strong>We couldn’t complete that request.</strong>
            <span>{error}</span>
          </div>
        ) : null}
        {detailMatch ? (
          selectedStream ? (
            <StreamDetail stream={selectedStream} />
          ) : notFound ? (
            <NotFound />
          ) : (
            <p className="loading-state" role="status">
              Loading stream…
            </p>
          )
        ) : (
          <>
            <section className="hero">
              <p className="eyebrow">YouTube stream workspace</p>
              <h1>Save the streams worth returning to.</h1>
              <p className="lede">
                Preview a YouTube livestream, confirm its metadata, and keep it
                in your library for later analysis.
              </p>

              <form className="registration-form" onSubmit={handlePreview}>
                <label htmlFor="youtube-url">YouTube URL</label>
                <div className="input-row">
                  <input
                    id="youtube-url"
                    type="url"
                    value={url}
                    onChange={(event) => setUrl(event.target.value)}
                    placeholder="https://youtube.com/watch?v=…"
                    required
                  />
                  <button type="submit" disabled={isPreviewing}>
                    {isPreviewing ? "Checking…" : "Preview stream"}
                  </button>
                </div>
              </form>
            </section>

            {preview ? (
              <PreviewCard
                preview={preview}
                isRegistering={isRegistering}
                onRegister={handleRegister}
              />
            ) : null}

            <section className="library" aria-labelledby="library-heading">
              <div className="section-heading">
                <div>
                  <p className="eyebrow">Your collection</p>
                  <h2 id="library-heading">Stream library</h2>
                </div>
                <span className="count">{streams.length} saved</span>
              </div>
              {isLoadingList ? (
                <div className="loading-state" role="status">
                  Loading stream library…
                </div>
              ) : streams.length === 0 ? (
                <div className="empty-state">
                  <p>No streams saved yet.</p>
                  <span>Your first preview will appear above.</span>
                </div>
              ) : (
                <div className="stream-grid">
                  {streams.map((stream) => (
                    <article className="stream-card" key={stream.id}>
                      {stream.thumbnailUrl ? (
                        <img src={stream.thumbnailUrl} alt="" />
                      ) : (
                        <div className="thumbnail-placeholder" />
                      )}
                      <div className="stream-card-content">
                        <div className="status-row">
                          <span
                            className={`status status-${stream.lifecycleStatus}`}
                          >
                            {lifecycleLabel(stream.lifecycleStatus)}
                          </span>
                          <span>{formatDuration(stream.durationMs)}</span>
                        </div>
                        <h3>
                          <a
                            href={`/streams/${stream.id}`}
                            onClick={(event) => openStream(event, stream)}
                          >
                            {stream.title}
                          </a>
                        </h3>
                        <p>{stream.channelTitle}</p>
                      </div>
                    </article>
                  ))}
                </div>
              )}
            </section>
          </>
        )}
      </main>
    </div>
  );
}

function PreviewCard({
  preview,
  isRegistering,
  onRegister,
}: {
  preview: StreamPreview;
  isRegistering: boolean;
  onRegister: () => void;
}) {
  return (
    <section className="preview-card" aria-label="Stream preview">
      {preview.thumbnailUrl ? (
        <img src={preview.thumbnailUrl} alt="" className="thumbnail" />
      ) : (
        <div className="thumbnail-placeholder" />
      )}
      <div className="preview-content">
        <div className="status-row">
          <span className={`status status-${preview.lifecycleStatus}`}>
            {lifecycleLabel(preview.lifecycleStatus)}
          </span>
          <span>{formatDuration(preview.durationMs)}</span>
        </div>
        <h2>{preview.title}</h2>
        <p className="channel">{preview.channelTitle}</p>
        <button type="button" onClick={onRegister} disabled={isRegistering}>
          {isRegistering ? "Saving…" : "Save to library"}
        </button>
      </div>
    </section>
  );
}

function StreamDetail({ stream }: { stream: Stream }) {
  return (
    <article className="stream-detail">
      <a className="back-link" href="/streams" aria-label="Back to library">
        ← Back to library
      </a>
      <div className="detail-hero">
        {stream.thumbnailUrl ? (
          <img src={stream.thumbnailUrl} alt="" className="detail-thumbnail" />
        ) : null}
        <div>
          <div className="status-row">
            <span className={`status status-${stream.lifecycleStatus}`}>
              {lifecycleLabel(stream.lifecycleStatus)}
            </span>
            <span>{formatDuration(stream.durationMs)}</span>
          </div>
          <h1>{stream.title}</h1>
          <p className="channel">{stream.channelTitle}</p>
          <dl className="metadata-grid">
            <div>
              <dt>Stream date</dt>
              <dd>
                {formatDate(stream.actualStartAt ?? stream.scheduledStartAt)}
              </dd>
            </div>
            <div>
              <dt>Duration</dt>
              <dd>{formatDuration(stream.durationMs)}</dd>
            </div>
            <div>
              <dt>YouTube video ID</dt>
              <dd>{stream.youtubeVideoId}</dd>
            </div>
          </dl>
          <a
            className="youtube-link"
            href={stream.canonicalUrl}
            target="_blank"
            rel="noreferrer"
          >
            Watch on YouTube
          </a>
        </div>
      </div>
    </article>
  );
}

function NotFound() {
  return (
    <section className="not-found">
      <p className="eyebrow">404 · Missing stream</p>
      <h1>Stream not found</h1>
      <p>The stream may have been removed, or the link may be incorrect.</p>
      <a href="/streams">Return to library</a>
    </section>
  );
}

function lifecycleLabel(status: StreamPreview["lifecycleStatus"]) {
  const labels: Record<StreamPreview["lifecycleStatus"], string> = {
    ended: "Ended",
    live: "Live now",
    scheduled: "Scheduled",
    unavailable: "Unavailable",
    unknown: "Status unknown",
  };
  return labels[status];
}

function formatDuration(durationMs?: number) {
  if (durationMs === undefined) return "Duration unavailable";
  const totalMinutes = Math.round(durationMs / 60_000);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return hours > 0 ? `${hours} hr ${minutes} min` : `${minutes} min`;
}

function formatDate(value?: string) {
  if (!value) return "Date unavailable";
  return new Intl.DateTimeFormat("en-US", {
    dateStyle: "long",
    timeZone: "UTC",
  }).format(new Date(value));
}
