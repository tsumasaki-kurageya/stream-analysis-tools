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

describe("stream registration", () => {
  it("presents the stream workflow as a Japanese three-pane workspace", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse({ items: [], limit: 20, offset: 0 })),
    );

    render(<App />);

    expect(
      await screen.findByRole("heading", {
        name: "動画とチャットを、ひとつの場所で。",
      }),
    ).toBeDefined();
    expect(
      screen.getByRole("navigation", { name: "メインナビゲーション" }),
    ).toBeDefined();
    expect(
      screen.getByRole("complementary", { name: "ストリームライブラリ" }),
    ).toBeDefined();
    expect(
      screen.getByRole("complementary", { name: "操作パネル" }),
    ).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "左パネルを閉じる" }));
    expect(
      screen.queryByRole("complementary", { name: "ストリームライブラリ" }),
    ).toBeNull();
    expect(
      screen.getByRole("button", { name: "左パネルを開く" }),
    ).toBeDefined();
  });

  it("shows that the stream library is loading", async () => {
    let resolveList!: (response: Response) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn(
        () =>
          new Promise<Response>((resolve) => {
            resolveList = resolve;
          }),
      ),
    );

    render(<App />);

    expect(screen.getByRole("status").textContent).toContain(
      "ストリームを読み込んでいます",
    );
    resolveList(jsonResponse({ items: [], limit: 20, offset: 0 }));
    expect(
      await screen.findByText("保存済みのストリームはありません。"),
    ).toBeDefined();
  });

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

    render(<App />);

    fireEvent.change(
      await screen.findByRole("textbox", { name: "YouTube URL" }),
      { target: { value: "https://youtu.be/dQw4w9WgXcQ" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "ストリームをプレビュー" }),
    );

    expect(
      await screen.findByRole("heading", { name: "An evening of live music" }),
    ).toBeDefined();
    const previewRegion = screen.getByRole("region", {
      name: "ストリームのプレビュー",
    });
    expect(within(previewRegion).getByText("Harbor Sessions")).toBeDefined();
    expect(within(previewRegion).getByText("1時間32分")).toBeDefined();
    expect(fetchMock).toHaveBeenCalledWith(
      "/v1/streams/preview",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("reopens a previewed stream from persistent history without re-entering its URL", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const requestUrl = String(input);
      if (requestUrl === "/v1/streams?limit=20&offset=0") {
        return jsonResponse({ items: [], limit: 20, offset: 0 });
      }
      if (requestUrl === "/v1/streams/preview") {
        return jsonResponse(endedStreamPreview);
      }
      throw new Error(`Unexpected request: ${requestUrl}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    const firstRender = render(<App />);
    await screen.findByText("保存済みのストリームはありません。");
    fireEvent.change(
      await screen.findByRole("textbox", { name: "YouTube URL" }),
      { target: { value: endedStreamPreview.canonicalUrl } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "ストリームをプレビュー" }),
    );
    expect(
      await screen.findByRole("heading", { name: endedStreamPreview.title }),
    ).toBeDefined();
    firstRender.unmount();

    render(<App />);
    await screen.findByText("保存済みのストリームはありません。");
    const recentPreview = await screen.findByRole("button", {
      name: `${endedStreamPreview.title}を再び開く`,
    });
    fireEvent.click(recentPreview);

    expect(
      screen.getByRole("heading", { name: endedStreamPreview.title }),
    ).toBeDefined();
    expect(screen.getByRole("textbox", { name: "YouTube URL" })).toHaveProperty(
      "value",
      endedStreamPreview.canonicalUrl,
    );
    expect(
      fetchMock.mock.calls.filter(
        ([input]) => String(input) === "/v1/streams/preview",
      ),
    ).toHaveLength(1);
  });

  it("registers the previewed stream and opens its detail", async () => {
    const registeredStream = {
      ...endedStreamPreview,
      id: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      createdAt: "2026-08-14T00:01:00Z",
      updatedAt: "2026-08-14T00:01:00Z",
    };
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/v1/streams?limit=20&offset=0") {
        return jsonResponse({ items: [], limit: 20, offset: 0 });
      }
      if (url === "/v1/streams/preview") {
        return jsonResponse(endedStreamPreview);
      }
      if (url === "/v1/streams") {
        return jsonResponse(registeredStream, 201);
      }
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);
    fireEvent.change(
      await screen.findByRole("textbox", { name: "YouTube URL" }),
      { target: { value: "https://youtu.be/dQw4w9WgXcQ" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "ストリームをプレビュー" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "ライブラリに保存" }),
    );

    expect(
      await screen.findByRole("heading", { name: "An evening of live music" }),
    ).toBeDefined();
    expect(
      screen.getByRole("link", { name: "ライブラリに戻る" }),
    ).toBeDefined();
    expect(window.location.pathname).toBe(
      "/streams/f47ac10b-58cc-4372-a567-0e02b2c3d479",
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "/v1/streams",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("restores a registered stream when its detail URL is reloaded", async () => {
    const registeredStream = {
      ...endedStreamPreview,
      id: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      createdAt: "2026-08-14T00:01:00Z",
      updatedAt: "2026-08-14T00:01:00Z",
    };
    window.history.replaceState(null, "", `/streams/${registeredStream.id}`);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === `/v1/streams/${registeredStream.id}`) {
          return jsonResponse(registeredStream);
        }
        throw new Error(`Unexpected request: ${String(input)}`);
      }),
    );

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "An evening of live music" }),
    ).toBeDefined();
    expect(screen.getByText("2026年8月10日")).toBeDefined();
    expect(screen.getByRole("link", { name: "YouTube で開く" })).toHaveProperty(
      "href",
      endedStreamPreview.canonicalUrl,
    );
  });

  it("opens a saved stream from the library", async () => {
    const registeredStream = {
      ...endedStreamPreview,
      id: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      createdAt: "2026-08-14T00:01:00Z",
      updatedAt: "2026-08-14T00:01:00Z",
    };
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input) === "/v1/streams?limit=20&offset=0") {
          return jsonResponse({
            items: [registeredStream],
            limit: 20,
            offset: 0,
          });
        }
        throw new Error(`Unexpected request: ${String(input)}`);
      }),
    );

    render(<App />);
    fireEvent.click(
      await screen.findByRole("link", { name: "An evening of live music" }),
    );

    expect(
      screen.getByRole("link", { name: "ライブラリに戻る" }),
    ).toBeDefined();
    expect(window.location.pathname).toBe(`/streams/${registeredStream.id}`);
  });

  it("switches directly between registered streams from the detail workspace", async () => {
    const anotherStream = {
      ...collectedStream,
      id: "c56a4180-65aa-42ec-a945-5fd21dec0538",
      youtubeVideoId: "anotherVideo",
      canonicalUrl: "https://www.youtube.com/watch?v=anotherVideo",
      title: "Another archived stream",
    };
    window.history.replaceState(null, "", `/streams/${collectedStream.id}`);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const requestUrl = String(input);
        if (requestUrl === "/v1/streams?limit=20&offset=0") {
          return jsonResponse({
            items: [collectedStream, anotherStream],
            limit: 20,
            offset: 0,
          });
        }
        if (requestUrl === `/v1/streams/${collectedStream.id}`) {
          return jsonResponse(collectedStream);
        }
        if (requestUrl.endsWith("/collections/latest")) {
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
        throw new Error(`Unexpected request: ${requestUrl}`);
      }),
    );

    render(<App />);
    fireEvent.click(
      await screen.findByRole("link", { name: anotherStream.title }),
    );

    expect(window.location.pathname).toBe(`/streams/${anotherStream.id}`);
    expect(
      screen.getByRole("heading", { name: anotherStream.title }),
    ).toBeDefined();
  });

  it("shows an actionable message for an invalid YouTube URL", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === "/v1/streams?limit=20&offset=0") {
          return jsonResponse({ items: [], limit: 20, offset: 0 });
        }
        if (url === "/v1/streams/preview") {
          return jsonResponse(
            {
              title: "Invalid YouTube URL",
              status: 400,
              detail: "Provide a supported YouTube video URL.",
              code: "INVALID_YOUTUBE_URL",
            },
            400,
          );
        }
        throw new Error(`Unexpected request: ${url}`);
      }),
    );

    render(<App />);
    fireEvent.change(
      await screen.findByRole("textbox", { name: "YouTube URL" }),
      { target: { value: "https://example.com/not-youtube" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "ストリームをプレビュー" }),
    );

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(
      "対応している YouTube 動画の URL を入力してください。",
    );
    expect(alert.textContent).not.toContain(
      "Provide a supported YouTube video URL.",
    );
  });

  it("opens the existing stream after a duplicate registration", async () => {
    const registeredStream = {
      ...endedStreamPreview,
      id: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      createdAt: "2026-08-14T00:01:00Z",
      updatedAt: "2026-08-14T00:01:00Z",
    };
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === "/v1/streams?limit=20&offset=0") {
          return jsonResponse({ items: [], limit: 20, offset: 0 });
        }
        if (url === "/v1/streams?limit=100&offset=0") {
          return jsonResponse({
            items: [registeredStream],
            limit: 100,
            offset: 0,
          });
        }
        if (url === "/v1/streams/preview") {
          return jsonResponse(endedStreamPreview);
        }
        if (url === "/v1/streams") {
          return jsonResponse(
            {
              title: "Stream already registered",
              status: 409,
              detail: "This YouTube stream is already registered.",
              code: "STREAM_ALREADY_REGISTERED",
            },
            409,
          );
        }
        throw new Error(`Unexpected request: ${url}`);
      }),
    );

    render(<App />);
    fireEvent.change(
      await screen.findByRole("textbox", { name: "YouTube URL" }),
      { target: { value: "https://youtu.be/dQw4w9WgXcQ" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "ストリームをプレビュー" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "ライブラリに保存" }),
    );

    expect(
      await screen.findByRole("link", { name: "ライブラリに戻る" }),
    ).toBeDefined();
    expect(window.location.pathname).toBe(`/streams/${registeredStream.id}`);
  });

  it("shows a recoverable not-found state for a missing stream", async () => {
    const missingId = "f47ac10b-58cc-4372-a567-0e02b2c3d479";
    window.history.replaceState(null, "", `/streams/${missingId}`);
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(
          {
            title: "Stream not found",
            status: 404,
            detail: "The requested stream does not exist.",
            code: "STREAM_NOT_FOUND",
          },
          404,
        ),
      ),
    );

    render(<App />);

    expect(
      await screen.findByRole("heading", {
        name: "ストリームが見つかりません",
      }),
    ).toBeDefined();
    expect(
      screen.getByRole("link", { name: "ライブラリに戻る" }),
    ).toHaveProperty("href", `${window.location.origin}/streams`);
  });

  it("does not keep showing the previous stream when browser history points to a missing one", async () => {
    const missingId = "c56a4180-65aa-42ec-a945-5fd21dec0538";
    window.history.replaceState(null, "", `/streams/${collectedStream.id}`);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const requestUrl = String(input);
        if (requestUrl === "/v1/streams?limit=20&offset=0") {
          return jsonResponse({
            items: [collectedStream],
            limit: 20,
            offset: 0,
          });
        }
        if (requestUrl === `/v1/streams/${collectedStream.id}`) {
          return jsonResponse(collectedStream);
        }
        if (requestUrl === `/v1/streams/${missingId}`) {
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
        if (requestUrl.endsWith("/collections/latest")) {
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
        throw new Error(`Unexpected request: ${requestUrl}`);
      }),
    );

    render(<App />);
    expect(
      await screen.findByRole("heading", { name: collectedStream.title }),
    ).toBeDefined();

    act(() => {
      window.history.pushState(null, "", `/streams/${missingId}`);
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(
      await screen.findByRole("heading", {
        name: "ストリームが見つかりません",
      }),
    ).toBeDefined();
    expect(
      screen.queryByRole("heading", { name: collectedStream.title }),
    ).toBeNull();
  });
});

describe("chat replay collection", () => {
  it("keeps playback and chat controls in the same workspace", async () => {
    window.history.replaceState(null, "", `/streams/${collectedStream.id}`);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const requestUrl = String(input);
        if (requestUrl === `/v1/streams/${collectedStream.id}`) {
          return jsonResponse(collectedStream);
        }
        if (
          requestUrl === `/v1/streams/${collectedStream.id}/collections/latest`
        ) {
          return jsonResponse({
            ...failedCollection,
            status: "succeeded",
            error: undefined,
          });
        }
        if (
          requestUrl ===
          `/v1/streams/${collectedStream.id}/chat-messages?limit=50`
        ) {
          return jsonResponse({ items: [] });
        }
        throw new Error(`Unexpected request: ${requestUrl}`);
      }),
    );

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

    fireEvent.click(screen.getByRole("button", { name: "右パネルを閉じる" }));
    expect(
      screen.queryByRole("complementary", { name: "チャットと収集" }),
    ).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "右パネルを開く" }));
    expect(
      await screen.findByRole("complementary", { name: "チャットと収集" }),
    ).toBeDefined();
  });

  it.each([
    ["running", "収集中"],
    ["no_data", "データなし"],
  ] as const)("displays the %s collection state", async (status, label) => {
    window.history.replaceState(null, "", `/streams/${collectedStream.id}`);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === `/v1/streams/${collectedStream.id}`) {
          return jsonResponse(collectedStream);
        }
        if (url === `/v1/streams/${collectedStream.id}/collections/latest`) {
          return jsonResponse({
            ...failedCollection,
            status,
            error: undefined,
            finishedAt:
              status === "no_data" ? failedCollection.finishedAt : undefined,
          });
        }
        if (
          url === `/v1/streams/${collectedStream.id}/chat-messages?limit=50`
        ) {
          return jsonResponse({ items: [] });
        }
        throw new Error(`Unexpected request: ${url}`);
      }),
    );

    render(<App />);

    expect(await screen.findByText(label)).toBeDefined();
  });

  it("starts the first collection from a stream detail", async () => {
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
      if (url === `/v1/streams/${collectedStream.id}`) {
        return jsonResponse(collectedStream);
      }
      if (url === `/v1/streams/${collectedStream.id}/collections/latest`) {
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
      if (url === `/v1/streams/${collectedStream.id}/collections`) {
        return jsonResponse(queuedCollection, 202);
      }
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "収集を開始" }));

    expect(await screen.findByText("待機中")).toBeDefined();
    expect(
      screen.getByText("処理を開始できるワーカーを待っています…"),
    ).toBeDefined();
    expect(fetchMock).toHaveBeenCalledWith(
      `/v1/streams/${collectedStream.id}/collections`,
      { method: "POST" },
    );
  });

  it("retries a failed collection and browses chat in server order", async () => {
    window.history.replaceState(null, "", `/streams/${collectedStream.id}`);
    const succeededCollection = {
      ...failedCollection,
      id: "787f789a-c336-4db7-94aa-739730a2f0b8",
      status: "succeeded",
      attempt: 2,
      processedCount: 3,
      skippedCount: 1,
      error: undefined,
      updatedAt: "2026-08-14T00:05:00Z",
      finishedAt: "2026-08-14T00:05:00Z",
    };
    const firstPage = {
      items: [
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
      ],
      nextCursor: "next-page",
    };
    const secondPage = {
      items: [
        {
          id: "6649630b-e5d5-42bf-8909-c2056717d95a",
          authorDisplayName: "Third viewer",
          messageText: "Final message",
          publishedAt: "2026-08-10T11:02:08Z",
          offsetMilliseconds: 3_728_000,
          messageType: "text",
        },
      ],
    };
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
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
        return jsonResponse(firstPage);
      }
      if (
        url ===
        `/v1/streams/${collectedStream.id}/chat-messages?limit=50&cursor=next-page`
      ) {
        return jsonResponse(secondPage);
      }
      throw new Error(`Unexpected request: ${url}`);
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);
    expect(
      await screen.findByText(
        "YouTube から一時的にデータを取得できませんでした。再試行してください。",
      ),
    ).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: "収集を再試行" }));

    expect(await screen.findByText("完了")).toBeDefined();
    expect(screen.getByText("3")).toBeDefined();
    const messages = await screen.findAllByRole("listitem");
    expect(messages.map((message) => message.textContent)).toEqual([
      "0:05First viewerOpening message",
      "1:05Second viewerLater message",
    ]);

    fireEvent.click(screen.getByRole("button", { name: "さらに読み込む" }));
    expect(await screen.findByText("Final message")).toBeDefined();
    expect(screen.getByText("1:02:08")).toBeDefined();
  });
});

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
