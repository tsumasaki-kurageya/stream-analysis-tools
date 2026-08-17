import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const preview = {
  youtubeVideoId: "dQw4w9WgXcQ",
  canonicalUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  title: "An evening of live music",
  channelId: "UC-stream-analysis",
  channelTitle: "Harbor Sessions",
  actualStartAt: "2026-08-10T10:00:00Z",
  actualEndAt: "2026-08-10T11:32:00Z",
  durationMs: 5_520_000,
  lifecycleStatus: "ended" as const,
  metadataFetchedAt: "2026-08-14T00:00:00Z",
};

const stream = {
  ...preview,
  id: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  createdAt: "2026-08-14T00:01:00Z",
  updatedAt: "2026-08-14T00:01:00Z",
};

const succeededCollection = {
  id: "9d1a7f61-a56d-4a9c-9a0d-6c940b35c013",
  streamId: stream.id,
  kind: "chat_replay",
  status: "succeeded",
  attempt: 1,
  processedCount: 0,
  skippedCount: 0,
  requestedAt: "2026-08-14T00:02:00Z",
  updatedAt: "2026-08-14T00:03:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
  window.localStorage.clear();
  window.history.replaceState(null, "", "/");
});

describe("route-owned app shell", () => {
  it("scopes the legacy panels to the stream list route", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [] });

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
  });

  it("lets the stream workspace own player and chat layout", async () => {
    window.history.replaceState(null, "", `/streams/${stream.id}`);
    installFetch({ streamList: [stream], workspace: true });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: stream.title }),
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
    expect(screen.queryByRole("button", { name: /右パネルを/ })).toBeNull();
    expect(
      screen.queryByRole("complementary", { name: "ストリームライブラリ" }),
    ).toBeNull();
  });

  it("lets reservations own their main layout", async () => {
    window.history.replaceState(null, "", "/reservations");
    installFetch({ reservations: true });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "予約一覧" }),
    ).toBeDefined();
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(screen.queryByRole("button", { name: /パネルを/ })).toBeNull();
  });

  it("uses the shared header for Streams and Reservations navigation", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [], reservations: true });

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

  it("keeps URL and route content aligned on popstate", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [], reservations: true });

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

describe("primary stream flows", () => {
  it("previews, registers, and opens the workspace directly", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [], preview: true, register: true });

    render(<App />);
    fireEvent.change(
      await screen.findByRole("textbox", { name: "YouTube URL" }),
      { target: { value: preview.canonicalUrl } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "ストリームをプレビュー" }),
    );
    expect(
      await screen.findByRole("heading", { name: preview.title }),
    ).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "ライブラリに保存" }));

    expect(window.location.pathname).toBe(`/streams/${stream.id}`);
    expect(
      await screen.findByRole("link", { name: "ライブラリに戻る" }),
    ).toBeDefined();
    expect(screen.queryByRole("button", { name: /右パネルを/ })).toBeNull();
  });

  it("opens a saved stream from the list and returns to the list", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [stream] });

    render(<App />);
    fireEvent.click(await screen.findByRole("link", { name: stream.title }));

    expect(window.location.pathname).toBe(`/streams/${stream.id}`);
    fireEvent.click(screen.getByRole("link", { name: "ライブラリに戻る" }));
    expect(window.location.pathname).toBe("/streams");
    expect(
      await screen.findByRole("link", { name: stream.title }),
    ).toBeDefined();
  });
});

function installFetch(options: {
  streamList?: Array<typeof stream>;
  workspace?: boolean;
  reservations?: boolean;
  preview?: boolean;
  register?: boolean;
}) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);

      if (url === "/v1/streams?limit=20&offset=0") {
        return jsonResponse({
          items: options.streamList ?? [],
          limit: 20,
          offset: 0,
        });
      }
      if (
        url === "/v1/reservations?limit=20&offset=0" &&
        options.reservations
      ) {
        return jsonResponse({ items: [], total: 0, limit: 20, offset: 0 });
      }
      if (url === "/v1/streams/preview" && options.preview) {
        return jsonResponse(preview);
      }
      if (url === "/v1/streams" && options.register) {
        return jsonResponse(stream, 201);
      }
      if (url === `/v1/streams/${stream.id}` && options.workspace) {
        return jsonResponse(stream);
      }
      if (url === `/v1/streams/${stream.id}/collections/latest`) {
        if (options.workspace) {
          return jsonResponse(succeededCollection);
        }
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
      if (
        url === `/v1/streams/${stream.id}/chat-messages?limit=50` &&
        options.workspace
      ) {
        return jsonResponse({ items: [] });
      }

      throw new Error(`Unexpected request: ${url}`);
    }),
  );
}

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
