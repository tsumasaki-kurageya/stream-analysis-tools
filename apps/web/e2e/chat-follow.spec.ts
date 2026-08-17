import { expect, test, type Page } from "@playwright/test";

const streamId = "f47ac10b-58cc-4372-a567-0e02b2c3d479";
const stream = {
  id: streamId,
  youtubeVideoId: "dQw4w9WgXcQ",
  canonicalUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  title: "Follow test stream",
  channelId: "UC-follow",
  channelTitle: "Follow Channel",
  actualStartAt: "2026-08-10T10:00:00Z",
  actualEndAt: "2026-08-10T11:00:00Z",
  durationMs: 3_600_000,
  lifecycleStatus: "ended",
  metadataFetchedAt: "2026-08-14T00:00:00Z",
  createdAt: "2026-08-14T00:01:00Z",
  updatedAt: "2026-08-14T00:01:00Z",
};

test("SCR-002: pauses chat follow on manual scroll and resumes on demand", async ({
  page,
}) => {
  await installApi(page);
  await page.goto(`/streams/${streamId}`);

  const chat = page.getByRole("list", { name: "収集済みチャット" });
  await expect(chat).toHaveAttribute("data-follow", "active");
  await expect(chat.getByRole("listitem").first()).toHaveAttribute(
    "aria-current",
    "time",
  );

  await chat.dispatchEvent("wheel", { deltaY: 80 });
  await expect(chat).toHaveAttribute("data-follow", "paused");
  await expect(
    page.getByRole("button", { name: "再生位置に戻る" }),
  ).toBeVisible();

  await page.getByRole("button", { name: "再生位置に戻る" }).click();
  await expect(chat).toHaveAttribute("data-follow", "active");
  await expect(
    page.getByRole("button", { name: "再生位置に戻る" }),
  ).toHaveCount(0);
});

async function installApi(page: Page) {
  await page.route("**/v1/**", async (route) => {
    const request = route.request();
    const { pathname } = new URL(request.url());

    if (request.method() === "GET" && pathname === "/v1/streams") {
      await route.fulfill({ json: { items: [stream], limit: 20, offset: 0 } });
      return;
    }
    if (request.method() === "GET" && pathname === `/v1/streams/${streamId}`) {
      await route.fulfill({ json: stream });
      return;
    }
    if (
      request.method() === "GET" &&
      pathname === `/v1/streams/${streamId}/collections/latest`
    ) {
      await route.fulfill({
        json: {
          id: "9d1a7f61-a56d-4a9c-9a0d-6c940b35c013",
          streamId,
          kind: "chat_replay",
          status: "succeeded",
          attempt: 1,
          processedCount: 2,
          skippedCount: 0,
          requestedAt: "2026-08-14T00:02:00Z",
          updatedAt: "2026-08-14T00:03:00Z",
        },
      });
      return;
    }
    if (
      request.method() === "GET" &&
      pathname === `/v1/streams/${streamId}/chat-messages`
    ) {
      await route.fulfill({
        json: {
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
        },
      });
      return;
    }
    await route.abort("failed");
  });
}
