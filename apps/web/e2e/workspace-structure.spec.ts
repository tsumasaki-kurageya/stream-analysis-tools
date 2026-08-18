import { expect, test, type Page } from "@playwright/test";

const streamId = "f47ac10b-58cc-4372-a567-0e02b2c3d479";
const stream = {
  id: streamId,
  youtubeVideoId: "dQw4w9WgXcQ",
  canonicalUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  title: "Workspace stream",
  channelId: "UC-workspace",
  channelTitle: "Workspace Channel",
  actualStartAt: "2026-08-10T10:00:00Z",
  actualEndAt: "2026-08-10T11:32:00Z",
  durationMs: 5_520_000,
  lifecycleStatus: "ended",
  metadataFetchedAt: "2026-08-14T00:00:00Z",
  createdAt: "2026-08-14T00:01:00Z",
  updatedAt: "2026-08-14T00:01:00Z",
};

test("SCR-002: shows metadata on demand and keeps successful collection compact", async ({
  page,
}) => {
  await installWorkspaceApi(page);
  await page.goto(`/streams/${streamId}`);

  await expect(page.getByRole("heading", { name: stream.title })).toBeVisible();
  await expect(page.getByText("YouTube 動画 ID")).toHaveCount(0);
  await expect(page.getByText("収集済み · 3件")).toBeVisible();
  await expect(page.getByText("動画プレビュー")).toHaveCount(0);
  await expect(page.getByText("YouTube 再生")).toHaveCount(0);
  await expect(page.getByText("終了", { exact: true })).toHaveCount(0);
  await expect(page.getByText("完了", { exact: true })).toHaveCount(0);

  const layout = await page.evaluate(() => {
    const header = document
      .querySelector(".timeline-header")!
      .getBoundingClientRect();
    const content = document
      .querySelector(".timeline-content")!
      .getBoundingClientRect();
    const video = document
      .querySelector(".stream-video-pane")!
      .getBoundingClientRect();
    const chat = document
      .querySelector(".stream-chat-pane")!
      .getBoundingClientRect();
    const searchButton = document
      .querySelector<HTMLButtonElement>(".chat-search button[type='submit']")!
      .getBoundingClientRect();
    return {
      headerBottom: header.bottom,
      contentTop: content.top,
      videoWidth: video.width,
      videoRight: video.right,
      chatLeft: chat.left,
      searchButtonHeight: searchButton.height,
    };
  });
  expect(layout.contentTop).toBeGreaterThanOrEqual(layout.headerBottom);
  expect(layout.contentTop - layout.headerBottom).toBeLessThanOrEqual(12);
  expect(layout.videoWidth).toBeGreaterThan(300);
  expect(layout.videoRight).toBeLessThanOrEqual(layout.chatLeft);
  expect(layout.searchButtonHeight).toBeLessThanOrEqual(40);

  await page.getByRole("button", { name: "配信情報を表示" }).click();
  const dialog = page.getByRole("dialog", { name: "配信情報" });
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText(stream.youtubeVideoId);
  await expect(dialog).toContainText("1時間32分");

  await page.getByRole("button", { name: "配信情報を閉じる" }).click();
  await expect(dialog).toHaveCount(0);
});

async function installWorkspaceApi(page: Page) {
  await page.route("**/v1/**", async (route) => {
    const request = route.request();
    const { pathname } = new URL(request.url());

    if (request.method() === "GET" && pathname === "/v1/streams") {
      await route.fulfill({
        json: {
          items: [
            { ...stream, chatMessageCount: 3, collectionStatus: "succeeded" },
          ],
          limit: 20,
          offset: 0,
        },
      });
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
          processedCount: 3,
          skippedCount: 0,
          requestedAt: "2026-08-14T00:02:00Z",
          updatedAt: "2026-08-14T00:03:00Z",
          finishedAt: "2026-08-14T00:03:00Z",
        },
      });
      return;
    }
    if (
      request.method() === "GET" &&
      pathname === `/v1/streams/${streamId}/chat-messages`
    ) {
      await route.fulfill({ json: { items: [] } });
      return;
    }
    await route.abort("failed");
  });
}
