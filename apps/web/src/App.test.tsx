import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const endedStreamPreview = {
  youtubeVideoId: "dQw4w9WgXcQ",
  canonicalUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  title: "An evening of live music",
  channelId: "UC-stream-analysis",
  channelTitle: "Harbor Sessions",
  thumbnailUrl: "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg",
  actualStartAt: "2026-08-10T10:00:00Z",
  actualEndAt: "2026-08-10T11:32:00Z",
  durationMs: 5_520_000,
  lifecycleStatus: "ended" as const,
  metadataFetchedAt: "2026-08-14T00:00:00Z",
};

const collectedStream = {
  ...endedStreamPreview,
  id: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  createdAt: "2026-08-14T00:01:00Z",
  updatedAt: "2026-08-14T00:01:00Z",
};

const failedCollection = {
  id: "9d1a7f61-a56d-4a9c-9a0d-6c940b35c013",
  streamId: collectedStream.id,
  kind: "chat_replay",
  status: "failed",
  attempt: 1,
  processedCount: 2,
  skippedCount: 1,
  requestedAt: "2026-08-14T00:02:00Z",
  updatedAt: "2026-08-14T00:03:00Z",
  finishedAt: "2026-08-14T00:03:00Z",
  error: {
    code: "YTDLP_TEMPORARY_FAILURE",
    message: "YouTube temporarily rejected the collection request.",
    retryable: true,
  },
};

afterEach(() => {
  vi.unstubAllGlobals();
  window.localStorage.clear();
  window.history.replaceState(null, "", "/");
});

describe("route-owned app shell", () => {
  it("keeps the legacy three-pane controls scoped to the stream list", async () => {
    window.history.replaceState(null, "", "/streams");
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse({ items: [], limit: 20, offset: 0 })),
    );

    render(<App />);

    expect(
      await screen.findByRole("navigation", { name: "メインナビゲーション" }),
    ).toBeDefined();
    expect(
      screen.getByRole("complementary", { name: "ストリームライブラリ" }),
    ).toBeDefined();
    expect(
      screen.getByRole("complementary", { name: "操作パネル" }),
    ).toBeDefined();
    expect(
      screen.getByRole("button", { name: "左パネルを閉じる" }),
    ).toBeDefined();
    expect(
      screen.getByRole("button", { name: "右パネルを閉じる" }),
    ).toBeDefined();
  });

  it("renders the stream workspace without global panel controls", async () => {
    window.history.replaceState(null, "", `/streams/${collectedStream.id}`);
    installWorkspaceFetch("succeeded");

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: collectedStream.title }),
    ).toBeDefined();
    expect(
      screen.getByRole("region", { name: "動画プレビュー" }),
    ).toBeDefined();
    expect(
      screen.getByRole("complementary", { name: "チャットと収集" }),
    ).toBeDefined();
    expect(
      await screen.findByRole("search", { name: "収集済みチャットを検索" }),
    ).toBeDefined();
    expect(
      screen.queryByRole("button", { name: "右パネルを閉じる" }),
    ).toBeNull();
    expect(
      screen.queryByRole("complementary", { name: "ストリームライブラリ" }),
    ).toBeNull();
  });

  it("renders reservations without the global side panels", async () => {
    window.history.replaceState(null, "", "/reservations");
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === "/v1/reservations?limit=20&offset=0") {
          return jsonResponse({ items: [], total: 0, limit: 20, offset: 0 });
        }
        throw new Error(`Unexpected request: ${String(input)}`);
      }),
    );

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "予約一覧" }),
    ).toBeDefined();
    expect(
      screen.queryByRole("complementary", { name: "ワークスペースナビゲーション" }),
    ).toBeNull();
    expect(
      screen.queryByRole("complementary", { name: "操作パネル" }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: "左パネルを閉じる" }),
    ).toBeNull();
  });

  it("navigates between Streams and Reservations from the shared header", async () => {
    window.history.replaceState(null, "", "/streams");
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === "/v1/streams?limit=20&offset=0") {
          return jsonResponse({ items: [], limit: 20, offset: 0 });
        }
        if (url === "/v1/reservations?limit=20&offset=0") {
          return jsonResponse({ items: [], total: 0, limit: 20, offset: 0 });
        }
        throw new Error(`Unexpected request: ${url}`);
      }),
    );

    render(<App />);
    await screen.findByText("保存済みのストリームはありません。");

    fireEvent.click(screen.getByRole("link", { name: "予約" }));
    expect(window.location.pathname).toBe("/reservations");
    expect(
      await screen.findByRole("heading", { name: "予約一覧" }),
    ).toBeDefined();

    fireEvent.click(screen.getByRole("link", { name: "ストリーム" }));
    expect(window.location.pathname).toBe("/streams");
    expect(
      await screen.findByText("保存済みのストリームはありません。"),
    ).toBeDefined();
  });

  it("keeps URL and rendered route aligned on browser history changes", async () => {
    window.history.replaceState(null, "", "/streams");
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === "/v1/streams?limit=20&offset=0") {
          return jsonResponse({ items: [], limit: 20, offset: 0 });
        }
        if (url === "/v1/reservations?limit=20&offset=0") {
          return jsonResponse({ items: [], total: 0, limit: 20, offset: 0 });
        }
        throw new Error(`Unexpected request: ${url}`);
      }),
    );

    render(<App />);
    await screen.findByText("保存済みのストリームはありません。");

    act(() => {
      window.history.pushState(null, "", "/reservations");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(
      await screen.findByRole("heading", { name: "予約一覧" }),
    ).toBeDefined();
  });
});

describe("stream registration", () => {
  it("previews an ended stream before registration", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/v1/streams?limit=20&offset=0") {
        return jsonResponse({ items: [], limit: 20, offset: 0 });
      }
      if (url === "/v1/streams/preview") {
        return jsonResponse(endedStreamPreview);
      }
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    window.history.replaceState(null, "", "/streams");
    render(<App />);

    fireEvent.change(
      await screen.findByRole("textbox", { name: "YouTube URL" }),
      { target: { value: endedStreamPreview.canonicalUrl } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "ストリームをプレビュー" }),
    );

    const previewRegion = await screen.findByRole("region", {
      name: "ストリームのプレビュー",
    });
    expect(
      within(previewRegion).getByRole("heading", {
        name: endedStreamPreview.title,
      }),
    ).toBeDefined();
    expect(within(previewRegion).getByText("Harbor Sessions")).toBeDefined();
  });

  it("registers a preview and opens its workspace directly", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/v1/streams?limit=20&offset=0") {
        return jsonResponse({ items: [], limit: 20, offset: 0 });
      }
      if (url === "/v1/streams/preview") {
        return jsonResponse(endedStreamPreview);
      }
      if (url === "/v1/streams") {
        return jsonResponse(collectedStream, 201);
      }
      if (url === `/v1/streams/${collectedStream.id}/collections/latest`) {
        return collectionNotFound();
      }
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    window.history.replaceState(null, "", "/streams");
    render(<App />);
    fireEvent.change(
      await screen.findByRole("textbox", { name: "YouTube URL" }),
      { target: { value: endedStreamPreview.canonicalUrl } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "ストリームをプレビュー" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "ライブラリに保存" }),
    );

    expect(window.location.pathname).toBe(`/streams/${collectedStream.id}`);
    expect(
      await screen.findByRole("heading", { name: collectedStream.title }),
    ).toBeDefined();
    expect(
      screen.queryByRole("button", { name: "右パネルを閉じる" }),
    ).toBeNull();
  });

  it("opens a saved stream from the list and returns to the list", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === "/v1/streams?limit=20&offset=0") {
          return jsonResponse({ items: [collectedStream], limit: 20, offset: 0 });
        }
        if (url === `/v1/streams/${collectedStream.id}/collections/latest`) {
          return collectionNotFound();
        }
        throw new Error(`Unexpected request: ${url}`);
      }),
    );

    window.history.replaceState(null, "", "/streams");
    render(<App />);
    fireEvent.click(
      await screen.findByRole("link", { name: collectedStream.title }),
    );

    expect(window.location.pathname).toBe(`/streams/${collectedStream.id}`);
    fireEvent.click(screen.getByRole("link", { name: "ライブラリに戻る" }));
    expect(window.location.pathname).toBe("/streams");
    expect(
      await screen.findByRole("link", { name: collectedStream.title }),
    ).toBeDefined();
  });

  it("shows a recoverable not-found state for a missing stream", async () => {
    const missingId = "c56a4180-65aa-42ec-a945-5fd21dec0538";
    window.history.replaceState(null, "", `/streams/${missingId}`);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === "/v1/streams?limit=20&offset=0") {
          return jsonResponse({ items: [], limit: 20, offset: 0 });
        }
        if (url === `/v1/streams/${missingId}`) {
          return jsonResponse(
            {
              title: "Stream not found",
              status: 404,
              detail: "The requested stream does not exist.",
              code: "STREAM_NOT_FOUND",
            },
            404,
          );
        }
        throw new Error(`Unexpected request: ${url}`);
      }),
    );

    render(<App />);

    expect(
      await screen.findByRole("heading", {
        name: "ストリームが見つかりません",
      }),
    ).toBeDefined();
  });
});

describe("chat replay collection", () => {
  it("keeps playback and chat controls together without a global panel toggle", async () => {
    window.history.replaceState(null, "", `/streams/${collectedStream.id}`);
    installWorkspaceFetch("succeeded");

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: collectedStream.title }),
    ).toBeDefined();
    expect(
      screen.getByRole("region", { name: "動画プレビュー" }),
    ).toBeDefined();
    expect(
      await screen.findByRole("search", { name: "収集済みチャットを検索" }),
    ).toBeDefined();
    expect(
      screen.getByRole("complementary", { name: "チャットと収集" }),
    ).toBeDefined();
    expect(
      screen.queryByRole("button", { name: /右パネルを/ }),
    ).toBeNull();
  });

  it("starts the first collection from a stream workspace", async () => {
    window.history.replaceState(null, "", `/streams/${collectedStream.id}`);
    const queuedCollection = {
      ...failedCollection,
      status: "queued",
      attempt: 0,
      processedCount: 0,
      skippedCount: 0,
      error: undefined,
      finishedAt: undefined,
    };
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/v1/streams?limit=20&offset=0") {
        return jsonResponse({ items: [collectedStream], limit: 20, offset: 0 });
      }
      if (url === `/v1/streams/${collectedStream.id}`) {
        return jsonResponse(collectedStream);
      }
      if (url === `/v1/streams/${collectedStream.id}/collections/latest`) {
        return collectionNotFound();
      }
      if (url === `/v1/streams/${collectedStream.id}/collections`) {
        return jsonResponse(queuedCollection, 202);
      }
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "収集を開始" }));

    expect(await screen.findByText("待機中")).toBeDefined();
    expect(fetchMock).toHaveBeenCalledWith(
      `/v1/streams/${collectedStream.id}/collections`,
      { method: "POST" },
    );
  });

  it("retries a failed collection and preserves server chat order", async () => {
    window.history.replaceState(null, "", `/streams/${collectedStream.id}`);
    const succeededCollection = {
      ...failedCollection,
      id: "787f789a-c336-4db7-94aa-739730a2f0b8",
      status: "succeeded",
      attempt: 2,
      processedCount: 2,
      skippedCount: 0,
      error: undefined,
      updatedAt: "2026-08-14T00:05:00Z",
      finishedAt: "2026-08-14T00:05:00Z",
    };
    const messages = [
      {
        id: "87c92d04-2d92-4c9b-9ea1-962984c2f901",
        authorDisplayName: "First viewer",
        messageText: "Opening message",
        publishedAt: "2026-08-10T10:00:05Z",
        offsetMilliseconds: 5_000,
        messageType: "text",
      },
      {
        id: "547ec39e-d9e7-497a-86f1-6d154ff08b78",
        authorDisplayName: "Second viewer",
        messageText: "Later message",
        publishedAt: "2026-08-10T10:01:05Z",
        offsetMilliseconds: 65_000,
        messageType: "text",
      },
    ];
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/v1/streams?limit=20&offset=0") {
        return jsonResponse({ items: [collectedStream], limit: 20, offset: 0 });
      }
      if (url === `/v1/streams/${collectedStream.id}`) {
        return jsonResponse(collectedStream);
      }
      if (url === `/v1/streams/${collectedStream.id}/collections/latest`) {
        return jsonResponse(failedCollection);
      }
      if (url === `/v1/collection-jobs/${failedCollection.id}/retry`) {
        return jsonResponse(succeededCollection, 202);
      }
      if (url === `/v1/streams/${collectedStream.id}/chat-messages?limit=50`) {
        return jsonResponse({ items: messages });
      }
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);
    fireEvent.click(
      await screen.findByRole("button", { name: "収集を再試行" }),
    );

    expect(await screen.findByText("完了")).toBeDefined();
    const chat = screen.getByRole("list", { name: "収集済みチャット" });
    expect(within(chat).getAllByRole("listitem").map((item) => item.textContent)).toEqual([
      "0:05First viewerOpening message",
      "1:05Second viewerLater message",
    ]);
  });
});

function installWorkspaceFetch(
  collectionStatus: "succeeded" | "running" | "no_data",
) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/v1/streams?limit=20&offset=0") {
        return jsonResponse({ items: [collectedStream], limit: 20, offset: 0 });
      }
      if (url === `/v1/streams/${collectedStream.id}`) {
        return jsonResponse(collectedStream);
      }
      if (url === `/v1/streams/${collectedStream.id}/collections/latest`) {
        return jsonResponse({
          ...failedCollection,
          status: collectionStatus,
          error: undefined,
          skippedCount: 0,
        });
      }
      if (url === `/v1/streams/${collectedStream.id}/chat-messages?limit=50`) {
        return jsonResponse({ items: [] });
      }
      throw new Error(`Unexpected request: ${url}`);
    }),
  );
}

function collectionNotFound() {
  return jsonResponse(
    {
      title: "Collection job not found",
      status: 404,
      detail: "No collection has been requested for this stream.",
      code: "COLLECTION_JOB_NOT_FOUND",
    },
    404,
  );
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
