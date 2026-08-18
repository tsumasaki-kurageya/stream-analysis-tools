import { useEffect, useRef, useState } from "react";

const PLAYBACK_POLL_INTERVAL_MS = 250;

export type PlayerSeekRequest = {
  offsetMilliseconds: number;
  sequence: number;
};

type YouTubePlayerProps = {
  videoId: string;
  canonicalUrl: string;
  seekRequest?: PlayerSeekRequest;
  onTimeChange: (offsetMilliseconds: number) => void;
};

type PlayerStatus = "loading" | "ready" | "unavailable";

type YouTubePlayerInstance = {
  destroy(): void;
  getCurrentTime(): number;
  seekTo(seconds: number, allowSeekAhead: boolean): void;
};

type YouTubePlayerEvent = {
  data: number;
};

type YouTubePlayerNamespace = {
  Player: new (
    element: HTMLElement,
    options: {
      videoId: string;
      host: string;
      playerVars: Record<string, number | string>;
      events: {
        onReady: () => void;
        onStateChange: (event: YouTubePlayerEvent) => void;
        onError: () => void;
      };
    },
  ) => YouTubePlayerInstance;
  PlayerState: {
    PLAYING: number;
  };
};

declare global {
  interface Window {
    YT?: YouTubePlayerNamespace;
    onYouTubeIframeAPIReady?: () => void;
  }
}

let iframeApiPromise: Promise<YouTubePlayerNamespace> | undefined;

export function YouTubePlayer({
  videoId,
  canonicalUrl,
  seekRequest,
  onTimeChange,
}: YouTubePlayerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<YouTubePlayerInstance | undefined>(undefined);
  const pollRef = useRef<number | undefined>(undefined);
  const [status, setStatus] = useState<PlayerStatus>("loading");

  useEffect(() => {
    let cancelled = false;

    function stopPolling() {
      if (pollRef.current !== undefined) {
        window.clearInterval(pollRef.current);
        pollRef.current = undefined;
      }
    }

    function updateCurrentTime() {
      const player = playerRef.current;
      if (!player) return;
      const offsetMilliseconds = Math.max(
        0,
        Math.round(player.getCurrentTime() * 1_000),
      );
      onTimeChange(offsetMilliseconds);
    }

    void loadYouTubeIframeApi()
      .then((youTube) => {
        if (cancelled || !containerRef.current) return;
        playerRef.current = new youTube.Player(containerRef.current, {
          videoId,
          host: "https://www.youtube-nocookie.com",
          playerVars: {
            origin: window.location.origin,
            playsinline: 1,
            rel: 0,
          },
          events: {
            onReady: () => {
              if (cancelled) return;
              setStatus("ready");
              updateCurrentTime();
            },
            onStateChange: (event) => {
              if (cancelled) return;
              updateCurrentTime();
              stopPolling();
              if (event.data === youTube.PlayerState.PLAYING) {
                pollRef.current = window.setInterval(
                  updateCurrentTime,
                  PLAYBACK_POLL_INTERVAL_MS,
                );
              }
            },
            onError: () => {
              if (cancelled) return;
              stopPolling();
              setStatus("unavailable");
            },
          },
        });
      })
      .catch(() => {
        if (!cancelled) setStatus("unavailable");
      });

    return () => {
      cancelled = true;
      stopPolling();
      playerRef.current?.destroy();
      playerRef.current = undefined;
    };
  }, [onTimeChange, videoId]);

  useEffect(() => {
    const player = playerRef.current;
    if (!player || status !== "ready" || !seekRequest) return;
    const seconds = Math.max(0, seekRequest.offsetMilliseconds / 1_000);
    player.seekTo(seconds, true);
    onTimeChange(seekRequest.offsetMilliseconds);
  }, [onTimeChange, seekRequest, status]);

  return (
    <section className="player-panel" aria-label="動画プレイヤー">
      <div className="player-frame" ref={containerRef} />
      {status !== "ready" ? (
        <p className={`player-state player-state-${status}`} role="status">
          {status === "loading"
            ? "プレイヤーを読み込んでいます…"
            : "この動画は埋め込みプレイヤーで再生できません。"}
        </p>
      ) : null}
      {status === "unavailable" ? (
        <a
          className="player-fallback"
          href={canonicalUrl}
          target="_blank"
          rel="noreferrer"
        >
          YouTube で動画を開く
        </a>
      ) : null}
    </section>
  );
}

function loadYouTubeIframeApi(): Promise<YouTubePlayerNamespace> {
  if (window.YT?.Player) return Promise.resolve(window.YT);
  if (iframeApiPromise) return iframeApiPromise;

  iframeApiPromise = new Promise((resolve, reject) => {
    const previousReady = window.onYouTubeIframeAPIReady;
    window.onYouTubeIframeAPIReady = () => {
      previousReady?.();
      if (window.YT?.Player) resolve(window.YT);
      else reject(new Error("YouTube IFrame API initialized without Player"));
    };

    const existingScript = document.querySelector<HTMLScriptElement>(
      'script[src="https://www.youtube.com/iframe_api"]',
    );
    const script = existingScript ?? document.createElement("script");
    script.addEventListener(
      "error",
      () => reject(new Error("YouTube IFrame API failed")),
      {
        once: true,
      },
    );
    if (!existingScript) {
      script.src = "https://www.youtube.com/iframe_api";
      script.async = true;
      document.head.append(script);
    }
  });
  return iframeApiPromise;
}
