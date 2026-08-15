import { FormEvent, MouseEvent, useCallback, useEffect, useState } from "react";
import {
  ApiProblem,
  createStream,
  findStreamByYouTubeVideoId,
  getLatestCollection,
  getStream,
  listChatMessages,
  listStreams,
  previewStream,
  retryCollection,
  searchChatMessages,
  startCollection,
  type ChatMessage,
  type CollectionJob,
  type Stream,
  type StreamPreview,
} from "./api/client";
import { YouTubePlayer, type PlayerSeekRequest } from "./YouTubePlayer";

const COLLECTION_POLL_INTERVAL_MS = 2_000;

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
        <span className="milestone">M3 · Synchronized exploration</span>
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
  const [playbackOffsetMilliseconds, setPlaybackOffsetMilliseconds] =
    useState(0);
  const [seekRequest, setSeekRequest] = useState<PlayerSeekRequest>();

  function seekTo(offsetMilliseconds: number) {
    setSeekRequest((current) => ({
      offsetMilliseconds,
      sequence: (current?.sequence ?? 0) + 1,
    }));
  }

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
      <YouTubePlayer
        videoId={stream.youtubeVideoId}
        canonicalUrl={stream.canonicalUrl}
        seekRequest={seekRequest}
        onTimeChange={setPlaybackOffsetMilliseconds}
      />
      <CollectionWorkspace
        streamId={stream.id}
        playbackOffsetMilliseconds={playbackOffsetMilliseconds}
        onSeek={seekTo}
      />
    </article>
  );
}

function CollectionWorkspace({
  streamId,
  playbackOffsetMilliseconds,
  onSeek,
}: {
  streamId: string;
  playbackOffsetMilliseconds: number;
  onSeek: (offsetMilliseconds: number) => void;
}) {
  const [collection, setCollection] = useState<CollectionJob | null>();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [requestError, setRequestError] = useState<string | null>(null);
  const isActive =
    collection?.status === "queued" || collection?.status === "running";

  useEffect(() => {
    let cancelled = false;
    void getLatestCollection(streamId)
      .then((job) => {
        if (!cancelled) setCollection(job);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        if (
          error instanceof ApiProblem &&
          error.problem.code === "COLLECTION_JOB_NOT_FOUND"
        ) {
          setCollection(null);
          return;
        }
        setRequestError(collectionErrorMessage(error));
      });
    return () => {
      cancelled = true;
    };
  }, [streamId]);

  useEffect(() => {
    if (!isActive) return;
    let cancelled = false;
    let timeout: number;

    function schedulePoll() {
      timeout = window.setTimeout(() => {
        void getLatestCollection(streamId)
          .then((job) => {
            if (cancelled) return;
            setCollection(job);
            setRequestError(null);
            if (job.status === "queued" || job.status === "running") {
              schedulePoll();
            }
          })
          .catch((error: unknown) => {
            if (cancelled) return;
            setRequestError(collectionErrorMessage(error));
            schedulePoll();
          });
      }, COLLECTION_POLL_INTERVAL_MS);
    }

    schedulePoll();
    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
  }, [isActive, streamId]);

  async function submit(action: () => Promise<CollectionJob>) {
    setIsSubmitting(true);
    setRequestError(null);
    try {
      setCollection(await action());
    } catch (error) {
      setRequestError(collectionErrorMessage(error));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <section
      className="collection-workspace"
      aria-labelledby="collection-title"
    >
      <div className="workspace-heading">
        <div>
          <p className="eyebrow">Chat replay</p>
          <h2 id="collection-title">Collection</h2>
        </div>
        {collection ? (
          <span
            className={`collection-status collection-status-${collection.status}`}
          >
            {collectionStatusLabel(collection.status)}
          </span>
        ) : null}
      </div>

      {requestError ? (
        <p className="inline-error" role="alert">
          {requestError}
        </p>
      ) : null}

      {collection === undefined ? (
        <p className="collection-loading" role="status">
          Loading collection status…
        </p>
      ) : collection === null ? (
        <div className="collection-empty">
          <div>
            <h3>Collect this stream’s chat replay</h3>
            <p>
              Start a background collection. You can leave this page while the
              worker processes the archive.
            </p>
          </div>
          <button
            type="button"
            disabled={isSubmitting}
            onClick={() => void submit(() => startCollection(streamId))}
          >
            {isSubmitting ? "Starting…" : "Start collection"}
          </button>
        </div>
      ) : (
        <>
          <div className="collection-summary">
            <div>
              <span>Persisted</span>
              <strong>{collection.processedCount.toLocaleString()}</strong>
            </div>
            <div>
              <span>Skipped</span>
              <strong>{collection.skippedCount.toLocaleString()}</strong>
            </div>
            <div>
              <span>Attempt</span>
              <strong>{collection.attempt}</strong>
            </div>
          </div>

          {collection.skippedCount > 0 ? (
            <p className="collection-notice" role="status">
              {collection.skippedCount.toLocaleString()} chat{" "}
              {collection.skippedCount === 1 ? "message" : "messages"} could not
              be persisted. Available messages remain searchable.
            </p>
          ) : null}

          {isActive ? (
            <p className="collection-activity" role="status">
              <span aria-hidden="true" />
              {collection.status === "queued"
                ? "Waiting for an available worker…"
                : "Collecting and persisting chat messages…"}
            </p>
          ) : null}

          {collection.status === "no_data" ? (
            <p className="collection-notice">
              Collection finished, but this stream has no available chat replay.
            </p>
          ) : null}

          {collection.status === "failed" ? (
            <div className="collection-failure" role="alert">
              <div>
                <strong>
                  {collection.error?.message ?? "Collection failed."}
                </strong>
                <span>
                  {collection.error?.retryable
                    ? "The failure is retryable. Starting again creates a new attempt."
                    : "This failure cannot be retried automatically."}
                </span>
              </div>
              {collection.error?.retryable ? (
                <button
                  type="button"
                  disabled={isSubmitting}
                  onClick={() =>
                    void submit(() => retryCollection(collection.id))
                  }
                >
                  {isSubmitting ? "Retrying…" : "Retry collection"}
                </button>
              ) : null}
            </div>
          ) : null}

          {!isActive ? (
            <>
              <ChatSearch
                streamId={streamId}
                playbackOffsetMilliseconds={playbackOffsetMilliseconds}
                onSeek={onSeek}
              />
              <ChatMessageList
                key={streamId}
                streamId={streamId}
                playbackOffsetMilliseconds={playbackOffsetMilliseconds}
                onSeek={onSeek}
              />
            </>
          ) : null}
        </>
      )}
    </section>
  );
}

function ChatSearch({
  streamId,
  playbackOffsetMilliseconds,
  onSeek,
}: {
  streamId: string;
  playbackOffsetMilliseconds: number;
  onSeek: (offsetMilliseconds: number) => void;
}) {
  const [query, setQuery] = useState("");
  const [submittedQuery, setSubmittedQuery] = useState("");
  const [results, setResults] = useState<ChatMessage[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submitSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextQuery = query.trim();
    setIsSearching(true);
    setError(null);
    try {
      const page = await searchChatMessages(streamId, nextQuery);
      setResults(page.items);
      setSubmittedQuery(nextQuery);
    } catch (requestError) {
      setError(collectionErrorMessage(requestError));
    } finally {
      setIsSearching(false);
    }
  }

  return (
    <section className="chat-search" aria-labelledby="chat-search-title">
      <div className="chat-heading">
        <div>
          <p className="eyebrow">Find a moment</p>
          <h3 id="chat-search-title">Search chat</h3>
        </div>
      </div>
      <form
        className="chat-search-form"
        role="search"
        aria-label="Search collected chat"
        onSubmit={submitSearch}
      >
        <label htmlFor="chat-search-query">Search collected chat</label>
        <div className="input-row">
          <input
            id="chat-search-query"
            type="search"
            minLength={3}
            maxLength={100}
            required
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search message text"
          />
          <button type="submit" disabled={isSearching}>
            {isSearching ? "Searching…" : "Search chat"}
          </button>
        </div>
      </form>

      {error ? (
        <p className="inline-error" role="alert">
          {error}
        </p>
      ) : null}
      {submittedQuery ? (
        results.length === 0 ? (
          <p className="chat-empty" role="status">
            No messages found for “{submittedQuery}”.
          </p>
        ) : (
          <>
            <p className="search-summary" role="status">
              {results.length} {results.length === 1 ? "result" : "results"} for
              “{submittedQuery}”
            </p>
            <ol className="search-results" aria-label="Chat search results">
              {results.map((message) => {
                const isCurrent =
                  Math.abs(
                    message.offsetMilliseconds - playbackOffsetMilliseconds,
                  ) < 1_000;
                return (
                  <li key={message.id}>
                    <button
                      type="button"
                      aria-current={isCurrent ? "time" : undefined}
                      aria-label={`Seek to ${formatOffset(message.offsetMilliseconds)}: ${message.messageText}`}
                      onClick={() => onSeek(message.offsetMilliseconds)}
                    >
                      <time dateTime={message.publishedAt}>
                        {formatOffset(message.offsetMilliseconds)}
                      </time>
                      <span>
                        <strong>{message.authorDisplayName}</strong>
                        {message.messageText}
                      </span>
                    </button>
                  </li>
                );
              })}
            </ol>
          </>
        )
      ) : null}
    </section>
  );
}

function ChatMessageList({
  streamId,
  playbackOffsetMilliseconds,
  onSeek,
}: {
  streamId: string;
  playbackOffsetMilliseconds: number;
  onSeek: (offsetMilliseconds: number) => void;
}) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [nextCursor, setNextCursor] = useState<string>();
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadPage = useCallback(
    async (cursor?: string) => {
      try {
        const page = await listChatMessages(streamId, 50, cursor);
        setMessages((current) =>
          cursor ? [...current, ...page.items] : page.items,
        );
        setNextCursor(page.nextCursor);
      } catch (requestError) {
        setError(collectionErrorMessage(requestError));
      } finally {
        setIsLoading(false);
      }
    },
    [streamId],
  );

  useEffect(() => {
    let cancelled = false;
    void listChatMessages(streamId, 50)
      .then((page) => {
        if (cancelled) return;
        setMessages(page.items);
        setNextCursor(page.nextCursor);
      })
      .catch((requestError: unknown) => {
        if (!cancelled) setError(collectionErrorMessage(requestError));
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [streamId]);

  const currentMessageId = messages.reduce<ChatMessage | undefined>(
    (closest, message) => {
      if (!closest) return message;
      return Math.abs(message.offsetMilliseconds - playbackOffsetMilliseconds) <
        Math.abs(closest.offsetMilliseconds - playbackOffsetMilliseconds)
        ? message
        : closest;
    },
    undefined,
  )?.id;

  return (
    <div className="chat-panel" aria-labelledby="chat-title">
      <div className="chat-heading">
        <h3 id="chat-title">Collected chat</h3>
        {messages.length > 0 ? <span>{messages.length} loaded</span> : null}
      </div>
      {error ? (
        <p className="inline-error" role="alert">
          {error}
        </p>
      ) : null}
      {messages.length === 0 && isLoading ? (
        <p className="chat-empty" role="status">
          Loading chat…
        </p>
      ) : messages.length === 0 ? (
        <p className="chat-empty">No persisted chat messages.</p>
      ) : (
        <ol className="chat-list" aria-label="Collected chat">
          {messages.map((message) => (
            <li
              key={message.id}
              aria-current={
                message.id === currentMessageId ? "time" : undefined
              }
            >
              <button
                className="chat-seek"
                type="button"
                aria-label={`Seek to ${formatOffset(message.offsetMilliseconds)}: ${message.messageText}`}
                onClick={() => onSeek(message.offsetMilliseconds)}
              >
                <time dateTime={message.publishedAt}>
                  {formatOffset(message.offsetMilliseconds)}
                </time>
              </button>
              <div>
                <strong>{message.authorDisplayName}</strong>
                <p>{message.messageText}</p>
              </div>
            </li>
          ))}
        </ol>
      )}
      {nextCursor ? (
        <button
          className="secondary-button"
          type="button"
          disabled={isLoading}
          onClick={() => {
            setIsLoading(true);
            setError(null);
            void loadPage(nextCursor);
          }}
        >
          {isLoading ? "Loading…" : "Load more chat"}
        </button>
      ) : null}
    </div>
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

function collectionStatusLabel(status: CollectionJob["status"]) {
  const labels: Record<CollectionJob["status"], string> = {
    queued: "Queued",
    running: "Running",
    succeeded: "Succeeded",
    no_data: "No data",
    failed: "Failed",
  };
  return labels[status];
}

function collectionErrorMessage(error: unknown) {
  return error instanceof ApiProblem
    ? error.problem.detail
    : "The collection request could not be completed. Check your connection and try again.";
}

function formatOffset(offsetMilliseconds: number) {
  const totalSeconds = Math.max(0, Math.floor(offsetMilliseconds / 1_000));
  const hours = Math.floor(totalSeconds / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  const seconds = totalSeconds % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`
    : `${minutes}:${String(seconds).padStart(2, "0")}`;
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
