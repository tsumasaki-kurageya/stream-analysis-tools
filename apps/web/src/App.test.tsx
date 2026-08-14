import { fireEvent, render, screen } from "@testing-library/react";
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

afterEach(() => {
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});

describe("stream registration", () => {
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
      "Loading stream library",
    );
    resolveList(jsonResponse({ items: [], limit: 20, offset: 0 }));
    expect(await screen.findByText("No streams saved yet.")).toBeDefined();
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
    fireEvent.click(screen.getByRole("button", { name: "Preview stream" }));

    expect(
      await screen.findByRole("heading", { name: "An evening of live music" }),
    ).toBeDefined();
    expect(screen.getByText("Harbor Sessions")).toBeDefined();
    expect(screen.getByText("1 hr 32 min")).toBeDefined();
    expect(fetchMock).toHaveBeenCalledWith(
      "/v1/streams/preview",
      expect.objectContaining({ method: "POST" }),
    );
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
    fireEvent.click(screen.getByRole("button", { name: "Preview stream" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Save to library" }),
    );

    expect(
      await screen.findByRole("heading", { name: "An evening of live music" }),
    ).toBeDefined();
    expect(screen.getByRole("link", { name: "Back to library" })).toBeDefined();
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
    expect(screen.getByText("August 10, 2026")).toBeDefined();
    expect(
      screen.getByRole("link", { name: "Watch on YouTube" }),
    ).toHaveProperty("href", endedStreamPreview.canonicalUrl);
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

    expect(screen.getByRole("link", { name: "Back to library" })).toBeDefined();
    expect(window.location.pathname).toBe(`/streams/${registeredStream.id}`);
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
    fireEvent.click(screen.getByRole("button", { name: "Preview stream" }));

    expect((await screen.findByRole("alert")).textContent).toContain(
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
    fireEvent.click(screen.getByRole("button", { name: "Preview stream" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Save to library" }),
    );

    expect(
      await screen.findByRole("link", { name: "Back to library" }),
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
      await screen.findByRole("heading", { name: "Stream not found" }),
    ).toBeDefined();
    expect(
      screen.getByRole("link", { name: "Return to library" }),
    ).toHaveProperty("href", `${window.location.origin}/streams`);
  });
});

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
