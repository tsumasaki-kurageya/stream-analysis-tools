import { expect, test, type Page } from "@playwright/test";

const streamId = "f47ac10b-58cc-4372-a567-0e02b2c3d479";
const preview = {
  youtubeVideoId: "dQw4w9WgXcQ",
  canonicalUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  title: "An evening of live music",
  channelId: "UC-stream-analysis",
  channelTitle: "Harbor Sessions",
  thumbnailUrl: "https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg",
  actualStartAt: "2026-08-10T10:00:00Z",
  actualEndAt: "2026-08-10T11:32:00Z",
  durationMs: 5_520_000,
  lifecycleStatus: "ended",
  metadataFetchedAt: "2026-08-14T00:00:00Z",
};

test("registers an ended stream, restores it, and opens it from the library", async ({
  page,
}) => {
  await installStreamApi(page);
  await page.goto("/streams");

  await expect(
    page.getByRole("heading", { name: "Save the streams worth returning to." }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "YouTube URL" })
    .fill("https://youtu.be/dQw4w9WgXcQ");
  await page.getByRole("button", { name: "Preview stream" }).click();
  await expect(
    page.getByRole("heading", { name: "An evening of live music" }),
  ).toBeVisible();
  await expect(page.getByText("Harbor Sessions").first()).toBeVisible();

  await page.getByRole("button", { name: "Save to library" }).click();
  await expect(page).toHaveURL(`/streams/${streamId}`);
  await expect(
    page.getByRole("link", { name: "Back to library" }),
  ).toBeVisible();

  await page.reload();
  await expect(
    page.getByRole("heading", { name: "An evening of live music" }),
  ).toBeVisible();
  await expect(page.getByText("August 10, 2026")).toBeVisible();

  await page.getByRole("link", { name: "Back to library" }).click();
  await expect(page).toHaveURL("/streams");
  await page.getByRole("link", { name: "An evening of live music" }).click();
  await expect(page).toHaveURL(`/streams/${streamId}`);
});

async function installStreamApi(page: Page) {
  let savedStream:
    | (typeof preview & {
        id: string;
        createdAt: string;
        updatedAt: string;
      })
    | null = null;

  await page.route("**/v1/**", async (route) => {
    const request = route.request();
    const { pathname } = new URL(request.url());

    if (request.method() === "POST" && pathname === "/v1/streams/preview") {
      await route.fulfill({ json: preview });
      return;
    }

    if (request.method() === "POST" && pathname === "/v1/streams") {
      savedStream = {
        ...preview,
        id: streamId,
        createdAt: "2026-08-14T00:01:00Z",
        updatedAt: "2026-08-14T00:01:00Z",
      };
      await route.fulfill({ status: 201, json: savedStream });
      return;
    }

    if (request.method() === "GET" && pathname === "/v1/streams") {
      await route.fulfill({
        json: { items: savedStream ? [savedStream] : [], limit: 20, offset: 0 },
      });
      return;
    }

    if (request.method() === "GET" && pathname === `/v1/streams/${streamId}`) {
      await route.fulfill(
        savedStream
          ? { json: savedStream }
          : {
              status: 404,
              contentType: "application/problem+json",
              body: JSON.stringify({
                title: "Stream not found",
                status: 404,
                detail: "The requested stream does not exist.",
                code: "STREAM_NOT_FOUND",
              }),
            },
      );
      return;
    }

    await route.abort("failed");
  });
}
