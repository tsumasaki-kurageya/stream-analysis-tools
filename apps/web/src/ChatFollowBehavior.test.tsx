import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { App } from "./App";

const streamId = "f47ac10b-58cc-4372-a567-0e02b2c3d479";

beforeEach(() => {
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: vi.fn(),
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});

it("pauses Chat follow on manual interaction and resumes explicitly", async () => {
  window.history.replaceState(null, "", `/streams/${streamId}`);
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/v1/streams?limit=20&offset=0") {
        return jsonResponse({ items: [], limit: 20, offset: 0 });
      }
      if (url === `/v1/streams/${streamId}`) {
        return jsonResponse({
          id: streamId,
          youtubeVideoId: "dQw4w9WgXcQ",
          canonicalUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
          title: "Follow test",
          channelId: "UC-follow",
          channelTitle: "Follow Channel",
          lifecycleStatus: "ended",
          metadataFetchedAt: "2026-08-14T00:00:00Z",
          createdAt: "2026-08-14T00:00:00Z",
          updatedAt: "2026-08-14T00:00:00Z",
        });
      }
      if (url === `/v1/streams/${streamId}/collections/latest`) {
        return jsonResponse({
          id: "9d1a7f61-a56d-4a9c-9a0d-6c940b35c013",
          streamId,
          kind: "chat_replay",
          status: "succeeded",
          attempt: 1,
          processedCount: 2,
          skippedCount: 0,
          requestedAt: "2026-08-14T00:00:00Z",
          updatedAt: "2026-08-14T00:01:00Z",
        });
      }
      if (url === `/v1/streams/${streamId}/chat-messages?limit=50`) {
        return jsonResponse({
          items: [
            {
              id: "87c92d04-2d92-4c9b-9ea1-962984c2f901",
              authorDisplayName: "Viewer",
              messageText: "Opening",
              publishedAt: "2026-08-10T10:00:05Z",
              offsetMilliseconds: 5_000,
              messageType: "text",
            },
          ],
        });
      }
      throw new Error(`Unexpected request: ${url}`);
    }),
  );

  render(<App />);

  const chat = await screen.findByRole("list", { name: "収集済みチャット" });
  expect(chat.getAttribute("data-follow")).toBe("active");

  fireEvent.wheel(chat, { deltaY: 80 });
  expect(chat.getAttribute("data-follow")).toBe("paused");
  fireEvent.click(screen.getByRole("button", { name: "再生位置に戻る" }));
  expect(chat.getAttribute("data-follow")).toBe("active");
});

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
