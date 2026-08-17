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
  collectionStatus: "succeeded" as const,
  chatMessageCount: 42,
};

const succeededCollection = {
  id: "9d1a7f61-a56d-4a9c-9a0d-6c940b35c013",
  streamId: stream.id,
  kind: "chat_replay",
  status: "succeeded",
  attempt: 1,
  processedCount: 42,
  skippedCount: 0,
  requestedAt: "2026-08-14T00:02:00Z",
  updatedAt: "2026-08-14T00:03:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
  window.localStorage.clear();
  window.history.replaceState(null, "", "/");
});

describe("SCR-001 stream list", () => {
  it("uses the stream list as the primary content with collapsed creation", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [stream] });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "配信一覧" }),
    ).toBeDefined();
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(screen.queryByRole("textbox", { name: "YouTube URL" })).toBeNull();
    expect(
      screen.getByRole("columnheader", { name: "タイトル" }),
    ).toBeDefined();
    expect(
      screen.getByRole("columnheader", { name: "チャンネル" }),
    ).toBeDefined();
    expect(
      screen.getByRole("columnheader", { name: "配信日時" }),
    ).toBeDefined();
    expect(
      screen.getByRole("columnheader", { name: "配信時間" }),
    ).toBeDefined();
    expect(
      screen.getByRole("columnheader", { name: "配信状態" }),
    ).toBeDefined();
    expect(
      screen.getByRole("columnheader", { name: "収集状態" }),
    ).toBeDefined();
    expect(
      screen.getByRole("columnheader", { name: "チャット件数" }),
    ).toBeDefined();
    expect(screen.getByText("42")).toBeDefined();
  });

  it("requires preview before registration and opens the workspace directly", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [], preview: true, register: true });

    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "配信を追加" }));
    fireEvent.change(screen.getByRole("textbox", { name: "YouTube URL" }), {
      target: { value: preview.canonicalUrl },
    });
    expect(
      screen.queryByRole("button", { name: "ライブラリに保存" }),
    ).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "プレビュー" }));
    expect(
      await screen.findByRole("heading", { name: preview.title }),
    ).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: "ライブラリに保存" }));

    expect(
      await screen.findByRole("link", { name: "ライブラリに戻る" }),
    ).toBeDefined();
    expect(window.location.pathname).toBe(`/streams/${stream.id}`);
  });

  it("opens a listed stream directly", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [stream] });

    render(<App />);
    fireEvent.click(await screen.findByRole("link", { name: stream.title }));

    expect(window.location.pathname).toBe(`/streams/${stream.id}`);
    expect(
      await screen.findByRole("link", { name: "ライブラリに戻る" }),
    ).toBeDefined();
  });
});

describe("shared navigation", () => {
  it("navigates between Streams and Reservations", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [], reservations: true });

    render(<App />);
    await screen.findByRole("heading", { name: "配信一覧" });

    fireEvent.click(screen.getByRole("link", { name: "予約" }));
    expect(window.location.pathname).toBe("/reservations");
    expect(
      await screen.findByRole("heading", { name: "予約一覧" }),
    ).toBeDefined();

    fireEvent.click(screen.getByRole("link", { name: "ストリーム" }));
    expect(window.location.pathname).toBe("/streams");
    expect(
      await screen.findByRole("heading", { name: "配信一覧" }),
    ).toBeDefined();
  });

  it("keeps URL and route content aligned on popstate", async () => {
    window.history.replaceState(null, "", "/streams");
    installFetch({ streamList: [], reservations: true });

    render(<App />);
    await screen.findByRole("heading", { name: "配信一覧" });

    act(() => {
      window.history.pushState(null, "", "/reservations");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(
      await screen.findByRole("heading", { name: "予約一覧" }),
    ).toBeDefined();
  });
});

function installFetch(options: {
  streamList?: Array<typeof stream>;
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
      if (url === `/v1/streams/${stream.id}`) {
        return jsonResponse(stream);
      }
      if (url === `/v1/streams/${stream.id}/collections/latest`) {
        return jsonResponse(succeededCollection);
      }
      if (url === `/v1/streams/${stream.id}/chat-messages?limit=50`) {
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
