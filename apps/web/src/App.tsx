import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
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
  type StreamListItem,
  type StreamPreview,
} from "./api/client";
import { YouTubePlayer, type PlayerSeekRequest } from "./YouTubePlayer";
import { ReservationsPage } from "./ReservationsPage";
import { StreamListPage } from "./StreamListPage";
import { messageForCode, userFacingError } from "./userMessages";

const COLLECTION_POLL_INTERVAL_MS = 2_000;
const PREVIEW_HISTORY_STORAGE_KEY = "stream-analysis.preview-history.v1";
const PREVIEW_HISTORY_LIMIT = 8;

export function App() {
  const [path, setPath] = useState(window.location.pathname);
  const [streams, setStreams] = useState<StreamListItem[]>([]);
  const [url, setUrl] = useState("");
  const [preview, setPreview] = useState<StreamPreview | null>(null);
  const [, setPreviewHistory] = useState<StreamPreview[]>(readPreviewHistory);
  const [isPreviewing, setIsPreviewing] = useState(false);
  const [isRegistering, setIsRegistering] = useState(false);
  const [isLoadingList, setIsLoadingList] = useState(
    !window.location.pathname.startsWith("/reservations"),
  );
  const [libraryError, setLibraryError] = useState<string | null>(null);
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
    setError(userFacingError(requestError));
  }, []);

  useEffect(() => {
    if (path.startsWith("/reservations")) return;
    void listStreams()
      .then((page) => {
        setStreams(page.items);
        setLibraryError(null);
      })
      .catch(() =>
        setLibraryError(
          "ライブラリを読み込めませんでした。接続を確認してください。",
        ),
      )
      .finally(() => setIsLoadingList(false));
  }, [path]);

  useEffect(() => {
    if (path.startsWith("/reservations")) return;
    const detailMatch = path.match(/^\/streams\/([^/]+)$/);
    if (detailMatch) {
      const streamId = detailMatch[1];
      if (selectedStream?.id !== streamId) {
        void getStream(streamId)
          .then(setSelectedStream)
          .catch(handleRequestError);
      }
    }
  }, [handleRequestError, path, selectedStream?.id]);

  async function handlePreview(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsPreviewing(true);
    setError(null);
    setNotFound(false);
    try {
      const nextPreview = await previewStream(url);
      setPreview(nextPreview);
      setPreviewHistory((current) => {
        const next = [
          nextPreview,
          ...current.filter(
            (item) => item.youtubeVideoId !== nextPreview.youtubeVideoId,
          ),
        ].slice(0, PREVIEW_HISTORY_LIMIT);
        writePreviewHistory(next);
        return next;
      });
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

  function navigateToStream(stream: Stream) {
    setSelectedStream(stream);
    const nextPath = `/streams/${stream.id}`;
    window.history.pushState(null, "", nextPath);
    setPath(nextPath);
  }

  const detailMatch = path.match(/^\/streams\/([^/]+)$/);
  const reservationPath = path.startsWith("/reservations");
  const selectedStreamForPath =
    detailMatch?.[1] === selectedStream?.id ? selectedStream : null;

  function navigate(pathname: string) {
    if (!pathname.startsWith("/reservations")) {
      setIsLoadingList(true);
      setLibraryError(null);
    }
    window.history.pushState(null, "", pathname);
    setPath(pathname);
  }

  useEffect(() => {
    const handlePopState = () => {
      const pathname = window.location.pathname;
      if (!pathname.startsWith("/reservations")) {
        setIsLoadingList(true);
        setLibraryError(null);
      }
      setPath(pathname);
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  return (
    <div className="app-shell">
      <header className="site-header">
        <a
          className="brand"
          href="/streams"
          onClick={(event) => {
            event.preventDefault();
            navigate("/streams");
          }}
        >
          <span className="brand-mark" aria-hidden="true">
            SA
          </span>
          <span>ストリーム分析</span>
        </a>
        <nav className="site-nav" aria-label="メインナビゲーション">
          <a
            href="/streams"
            aria-current={!reservationPath ? "page" : undefined}
            onClick={(event) => {
              event.preventDefault();
              navigate("/streams");
            }}
          >
            ストリーム
          </a>
          <a
            href="/reservations"
            aria-current={reservationPath ? "page" : undefined}
            onClick={(event) => {
              event.preventDefault();
              navigate("/reservations");
            }}
          >
            予約
          </a>
        </nav>
      </header>

      {error ? (
        <div className="error-banner" role="alert">
          <strong>リクエストを完了できませんでした。</strong>
          <span>{error}</span>
        </div>
      ) : null}

      {reservationPath ? (
        <main className="route-main" aria-label="メインコンテンツ">
          <ReservationsPage path={path} onNavigate={navigate} />
        </main>
      ) : detailMatch ? (
        <main
          className="route-main stream-workspace-host"
          aria-label="メインコンテンツ"
        >
          {selectedStreamForPath ? (
            <StreamDetail
              stream={selectedStreamForPath}
              onBack={() => navigate("/streams")}
            />
          ) : notFound ? (
            <NotFound />
          ) : (
            <p className="loading-state" role="status">
              ストリームを読み込んでいます…
            </p>
          )}
        </main>
      ) : (
        <main className="route-main" aria-label="メインコンテンツ">
          <StreamListPage
            streams={streams}
            isLoading={isLoadingList}
            error={libraryError}
            url={url}
            onURLChange={setUrl}
            onPreview={handlePreview}
            isPreviewing={isPreviewing}
            preview={preview}
            previewNode={
              preview ? (
                <PreviewCard
                  preview={preview}
                  isRegistering={isRegistering}
                  onRegister={handleRegister}
                />
              ) : null
            }
            onOpenStream={(stream) => navigate(`/streams/${stream.id}`)}
          />
        </main>
      )}
    </div>
  );
}

function readPreviewHistory(): StreamPreview[] {
  try {
    const stored = window.localStorage.getItem(PREVIEW_HISTORY_STORAGE_KEY);
    if (!stored) return [];
    const value: unknown = JSON.parse(stored);
    if (!Array.isArray(value)) return [];
    return value.filter(isStoredPreview).slice(0, PREVIEW_HISTORY_LIMIT);
  } catch {
    return [];
  }
}

function isStoredPreview(value: unknown): value is StreamPreview {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Partial<StreamPreview>;
  return (
    typeof candidate.youtubeVideoId === "string" &&
    typeof candidate.canonicalUrl === "string" &&
    typeof candidate.title === "string" &&
    typeof candidate.channelTitle === "string" &&
    typeof candidate.lifecycleStatus === "string"
  );
}

function writePreviewHistory(history: StreamPreview[]) {
  try {
    window.localStorage.setItem(
      PREVIEW_HISTORY_STORAGE_KEY,
      JSON.stringify(history),
    );
  } catch {
    // A blocked or full storage area must not prevent previewing a stream.
  }
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
    <section className="preview-card" aria-label="ストリームのプレビュー">
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
          {isRegistering ? "保存しています…" : "ライブラリに保存"}
        </button>
      </div>
    </section>
  );
}

function StreamDetail({
  stream,
  onBack,
}: {
  stream: Stream;
  onBack: () => void;
}) {
  const [playbackOffsetMilliseconds, setPlaybackOffsetMilliseconds] =
    useState(0);
  const [seekRequest, setSeekRequest] = useState<PlayerSeekRequest>();
  const [metadataOpen, setMetadataOpen] = useState(false);
  const [collectionStatus, setCollectionStatus] = useState<
    CollectionJob["status"] | null
  >(null);

  function seekTo(offsetMilliseconds: number) {
    setSeekRequest((current) => ({
      offsetMilliseconds,
      sequence: (current?.sequence ?? 0) + 1,
    }));
  }

  return (
    <article className="stream-detail timeline-workspace">
      <header className="timeline-header">
        <a
          className="back-link"
          href="/streams"
          aria-label="ライブラリに戻る"
          onClick={(event) => {
            event.preventDefault();
            onBack();
          }}
        >
          ← ライブラリに戻る
        </a>
        <div className="timeline-heading">
          <div>
            <h1>{stream.title}</h1>
            <p className="channel">{stream.channelTitle}</p>
          </div>
          <div className="timeline-statuses">
            <span className={`status status-${stream.lifecycleStatus}`}>
              {lifecycleLabel(stream.lifecycleStatus)}
            </span>
            <span className="collection-status">
              {collectionStatus
                ? collectionStatusLabel(collectionStatus)
                : "未収集"}
            </span>
            <button
              type="button"
              className="icon-button metadata-info-button"
              aria-label="配信情報を表示"
              onClick={() => setMetadataOpen(true)}
            >
              ⓘ
            </button>
          </div>
        </div>
      </header>

      {metadataOpen ? (
        <div className="metadata-dialog-backdrop" role="presentation">
          <section
            className="metadata-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="metadata-dialog-title"
          >
            <div className="metadata-dialog-heading">
              <h2 id="metadata-dialog-title">配信情報</h2>
              <button
                type="button"
                className="icon-button"
                aria-label="配信情報を閉じる"
                onClick={() => setMetadataOpen(false)}
              >
                ×
              </button>
            </div>
            <dl className="metadata-grid">
              <div>
                <dt>タイトル</dt>
                <dd>{stream.title}</dd>
              </div>
              <div>
                <dt>チャンネル</dt>
                <dd>{stream.channelTitle}</dd>
              </div>
              <div>
                <dt>配信日</dt>
                <dd>
                  {formatDate(stream.actualStartAt ?? stream.scheduledStartAt)}
                </dd>
              </div>
              <div>
                <dt>長さ</dt>
                <dd>{formatDuration(stream.durationMs)}</dd>
              </div>
              <div>
                <dt>配信状態</dt>
                <dd>{lifecycleLabel(stream.lifecycleStatus)}</dd>
              </div>
              <div>
                <dt>YouTube 動画 ID</dt>
                <dd>{stream.youtubeVideoId}</dd>
              </div>
            </dl>
            <a
              className="youtube-link"
              href={stream.canonicalUrl}
              target="_blank"
              rel="noreferrer"
            >
              YouTube で開く
            </a>
          </section>
        </div>
      ) : null}

      <div className="timeline-content">
        <div className="stream-video-pane">
          <YouTubePlayer
            videoId={stream.youtubeVideoId}
            canonicalUrl={stream.canonicalUrl}
            seekRequest={seekRequest}
            onTimeChange={setPlaybackOffsetMilliseconds}
          />
        </div>
        <aside className="stream-chat-pane" aria-label="チャットと収集">
          <CollectionWorkspace
            streamId={stream.id}
            playbackOffsetMilliseconds={playbackOffsetMilliseconds}
            onSeek={seekTo}
            onStatusChange={setCollectionStatus}
          />
        </aside>
      </div>
    </article>
  );
}

function CollectionWorkspace({
  streamId,
  playbackOffsetMilliseconds,
  onSeek,
  onStatusChange,
}: {
  streamId: string;
  playbackOffsetMilliseconds: number;
  onSeek: (offsetMilliseconds: number) => void;
  onStatusChange: (status: CollectionJob["status"] | null) => void;
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
        if (!cancelled) {
          setCollection(job);
          onStatusChange(job.status);
        }
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        if (
          error instanceof ApiProblem &&
          error.problem.code === "COLLECTION_JOB_NOT_FOUND"
        ) {
          setCollection(null);
          onStatusChange(null);
          return;
        }
        setRequestError(collectionErrorMessage(error));
      });
    return () => {
      cancelled = true;
    };
  }, [onStatusChange, streamId]);

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
            onStatusChange(job.status);
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
  }, [isActive, onStatusChange, streamId]);

  async function submit(action: () => Promise<CollectionJob>) {
    setIsSubmitting(true);
    setRequestError(null);
    try {
      const job = await action();
      setCollection(job);
      onStatusChange(job.status);
    } catch (submitError) {
      setRequestError(collectionErrorMessage(submitError));
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
          <h2 id="collection-title">収集とチャット</h2>
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
          収集状態を読み込んでいます…
        </p>
      ) : collection === null ? (
        <div className="collection-empty">
          <div>
            <h3>チャットリプレイを収集</h3>
            <p>
              バックグラウンドで収集を開始します。処理中もこの画面を離れられます。
            </p>
          </div>
          <button
            type="button"
            disabled={isSubmitting}
            onClick={() => void submit(() => startCollection(streamId))}
          >
            {isSubmitting ? "開始しています…" : "収集を開始"}
          </button>
        </div>
      ) : (
        <>
          {collection.status === "succeeded" ? (
            <p className="collection-compact-status" role="status">
              収集済み · {collection.processedCount.toLocaleString()}件
            </p>
          ) : (
            <div className="collection-summary">
              <div>
                <span>保存済み</span>
                <strong>{collection.processedCount.toLocaleString()}</strong>
              </div>
              <div>
                <span>スキップ</span>
                <strong>{collection.skippedCount.toLocaleString()}</strong>
              </div>
              <div>
                <span>試行回数</span>
                <strong>{collection.attempt}</strong>
              </div>
            </div>
          )}

          {collection.skippedCount > 0 ? (
            <p className="collection-notice" role="status">
              {collection.skippedCount.toLocaleString("ja-JP")}
              件のチャットを保存できませんでした。
              保存済みのメッセージは引き続き検索できます。
            </p>
          ) : null}

          {isActive ? (
            <p className="collection-activity" role="status">
              <span aria-hidden="true" />
              {collection.status === "queued"
                ? "処理を開始できるワーカーを待っています…"
                : "チャットメッセージを収集・保存しています…"}
            </p>
          ) : null}

          {collection.status === "no_data" ? (
            <p className="collection-notice">
              収集は完了しましたが、このストリームに利用可能なチャットリプレイはありません。
            </p>
          ) : null}

          {collection.status === "failed" ? (
            <div className="collection-failure" role="alert">
              <div>
                <strong>
                  {messageForCode(
                    collection.error?.code,
                    "チャット収集に失敗しました。",
                  )}
                </strong>
                <span>
                  {collection.error?.retryable
                    ? "再試行できます。新しい収集処理として開始します。"
                    : "このエラーは自動では再試行できません。"}
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
                  {isSubmitting ? "再試行しています…" : "収集を再試行"}
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
  const [searchError, setSearchError] = useState<string | null>(null);

  async function submitSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextQuery = query.trim();
    setIsSearching(true);
    setSearchError(null);
    try {
      const page = await searchChatMessages(streamId, nextQuery);
      setResults(page.items);
      setSubmittedQuery(nextQuery);
    } catch (requestError) {
      setSearchError(collectionErrorMessage(requestError));
    } finally {
      setIsSearching(false);
    }
  }

  return (
    <section className="chat-search" aria-labelledby="chat-search-title">
      <div className="chat-heading">
        <div>
          <h3 id="chat-search-title">チャット検索</h3>
        </div>
      </div>
      <form
        className="chat-search-form"
        role="search"
        aria-label="収集済みチャットを検索"
        onSubmit={submitSearch}
      >
        <label htmlFor="chat-search-query">収集済みチャットを検索</label>
        <div className="input-row">
          <input
            id="chat-search-query"
            type="search"
            minLength={3}
            maxLength={100}
            required
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="メッセージ本文を検索"
          />
          <button type="submit" disabled={isSearching}>
            {isSearching ? "検索しています…" : "検索"}
          </button>
        </div>
      </form>

      {searchError ? (
        <p className="inline-error" role="alert">
          {searchError}
        </p>
      ) : null}
      {submittedQuery ? (
        results.length === 0 ? (
          <p className="chat-empty" role="status">
            「{submittedQuery}」に一致するメッセージはありません。
          </p>
        ) : (
          <>
            <p className="search-summary" role="status">
              「{submittedQuery}」の検索結果：{results.length}件
            </p>
            <ol className="search-results" aria-label="チャット検索結果">
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
                      aria-label={`${formatOffset(message.offsetMilliseconds)}へ移動: ${message.messageText}`}
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
  const [chatError, setChatError] = useState<string | null>(null);
  const [followPaused, setFollowPaused] = useState(false);
  const listRef = useRef<HTMLOListElement | null>(null);
  const previousPlaybackRef = useRef(playbackOffsetMilliseconds);

  const loadPage = useCallback(
    async (cursor?: string) => {
      try {
        const page = await listChatMessages(streamId, 50, cursor);
        setMessages((current) =>
          cursor ? [...current, ...page.items] : page.items,
        );
        setNextCursor(page.nextCursor);
      } catch (requestError) {
        setChatError(collectionErrorMessage(requestError));
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
        if (!cancelled) setChatError(collectionErrorMessage(requestError));
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

  useEffect(() => {
    const previous = previousPlaybackRef.current;
    previousPlaybackRef.current = playbackOffsetMilliseconds;
    if (
      followPaused &&
      Math.abs(playbackOffsetMilliseconds - previous) >= 3_000
    ) {
      setFollowPaused(false);
    }
  }, [followPaused, playbackOffsetMilliseconds]);

  useEffect(() => {
    if (followPaused || !currentMessageId) return;
    const current = listRef.current?.querySelector<HTMLElement>(
      `[data-message-id="${currentMessageId}"]`,
    );
    current?.scrollIntoView({ block: "nearest" });
  }, [currentMessageId, followPaused]);

  function pauseFollow() {
    if (messages.length > 0) setFollowPaused(true);
  }

  function resumeFollow() {
    setFollowPaused(false);
    if (!currentMessageId) return;
    const current = listRef.current?.querySelector<HTMLElement>(
      `[data-message-id="${currentMessageId}"]`,
    );
    current?.scrollIntoView({ block: "nearest" });
  }

  return (
    <div className="chat-panel" aria-labelledby="chat-title">
      <div className="chat-heading">
        <h3 id="chat-title">収集済みチャット</h3>
        <div className="chat-heading-actions">
          {messages.length > 0 ? <span>{messages.length}件表示</span> : null}
          {followPaused ? (
            <button
              type="button"
              className="secondary-button chat-follow-resume"
              onClick={resumeFollow}
            >
              再生位置に戻る
            </button>
          ) : null}
        </div>
      </div>
      {chatError ? (
        <p className="inline-error" role="alert">
          {chatError}
        </p>
      ) : null}
      {messages.length === 0 && isLoading ? (
        <p className="chat-empty" role="status">
          チャットを読み込んでいます…
        </p>
      ) : messages.length === 0 ? (
        <p className="chat-empty">保存済みのチャットはありません。</p>
      ) : (
        <ol
          ref={listRef}
          className="chat-list"
          aria-label="収集済みチャット"
          data-follow={followPaused ? "paused" : "active"}
          onWheel={pauseFollow}
          onTouchMove={pauseFollow}
          onPointerDown={(event) => {
            if ((event.target as HTMLElement).closest("button")) return;
            pauseFollow();
          }}
        >
          {messages.map((message) => (
            <li
              key={message.id}
              data-message-id={message.id}
              aria-current={
                message.id === currentMessageId ? "time" : undefined
              }
            >
              <button
                className="chat-seek"
                type="button"
                aria-label={`${formatOffset(message.offsetMilliseconds)}へ移動: ${message.messageText}`}
                onClick={() => {
                  setFollowPaused(false);
                  onSeek(message.offsetMilliseconds);
                }}
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
            setChatError(null);
            void loadPage(nextCursor);
          }}
        >
          {isLoading ? "読み込んでいます…" : "さらに読み込む"}
        </button>
      ) : null}
    </div>
  );
}

function NotFound() {
  return (
    <section className="not-found">
      <p className="eyebrow">404 · ストリームが見つかりません</p>
      <h1>ストリームが見つかりません</h1>
      <p>ストリームが削除されたか、リンクが正しくない可能性があります。</p>
      <a href="/streams">ライブラリに戻る</a>
    </section>
  );
}

function lifecycleLabel(status: StreamPreview["lifecycleStatus"]) {
  const labels: Record<StreamPreview["lifecycleStatus"], string> = {
    ended: "終了",
    live: "ライブ配信中",
    scheduled: "配信予定",
    unavailable: "利用不可",
    unknown: "状態不明",
  };
  return labels[status];
}

function collectionStatusLabel(status: CollectionJob["status"]) {
  const labels: Record<CollectionJob["status"], string> = {
    queued: "待機中",
    running: "収集中",
    succeeded: "完了",
    no_data: "データなし",
    failed: "失敗",
  };
  return labels[status];
}

function collectionErrorMessage(error: unknown) {
  return userFacingError(
    error,
    "収集リクエストを完了できませんでした。接続を確認して、もう一度お試しください。",
  );
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
  if (durationMs === undefined) return "長さ不明";
  const totalMinutes = Math.round(durationMs / 60_000);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return hours > 0 ? `${hours}時間${minutes}分` : `${minutes}分`;
}

function formatDate(value?: string) {
  if (!value) return "日時不明";
  return new Intl.DateTimeFormat("ja-JP", {
    dateStyle: "long",
    timeZone: "UTC",
  }).format(new Date(value));
}
