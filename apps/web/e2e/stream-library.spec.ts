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

test("starts, restores, retries, and browses a chat collection", async ({
  page,
}) => {
  await installCollectionApi(page);
  await page.goto(`/streams/${streamId}`);

  await page.getByRole("button", { name: "Start collection" }).click();
  await expect(page.getByText("Queued")).toBeVisible();

  await page.reload();
  await expect(
    page.getByText("YouTube temporarily rejected the collection request."),
  ).toBeVisible({ timeout: 7_000 });

  await page.getByRole("button", { name: "Retry collection" }).click();
  await expect(page.getByText("Queued")).toBeVisible();
  await expect(page.getByText("Succeeded")).toBeVisible();
  await expect(page.getByText("3", { exact: true })).toBeVisible();

  const chat = page.getByRole("list", { name: "Collected chat" });
  await expect(chat.getByRole("listitem")).toHaveCount(2);
  await expect(chat.getByRole("listitem").nth(0)).toContainText(
    "Opening message",
  );
  await expect(chat.getByRole("listitem").nth(1)).toContainText(
    "Later message",
  );

  await page.getByRole("button", { name: "Load more chat" }).click();
  await expect(chat.getByRole("listitem")).toHaveCount(3);
  await expect(chat.getByRole("listitem").nth(2)).toContainText(
    "Final message",
  );
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

    if (
      request.method() === "GET" &&
      pathname === `/v1/streams/${streamId}/collections/latest`
    ) {
      await route.fulfill({
        status: 404,
        contentType: "application/problem+json",
        body: JSON.stringify({
          title: "Collection job not found",
          status: 404,
          detail: "No collection has been requested for this stream.",
          code: "COLLECTION_JOB_NOT_FOUND",
        }),
      });
      return;
    }

    await route.abort("failed");
  });
}

async function installCollectionApi(page: Page) {
  const savedStream = {
    ...preview,
    id: streamId,
    createdAt: "2026-08-14T00:01:00Z",
    updatedAt: "2026-08-14T00:01:00Z",
  };
  const firstJobId = "9d1a7f61-a56d-4a9c-9a0d-6c940b35c013";
  const retryJobId = "787f789a-c336-4db7-94aa-739730a2f0b8";
  let phase: "none" | "first" | "failed" | "retry" | "succeeded" = "none";
  let firstJobPolls = 0;

  function job(
    id: string,
    status: "queued" | "running" | "succeeded" | "failed",
    attempt: number,
  ) {
    return {
      id,
      streamId,
      kind: "chat_replay",
      status,
      attempt,
      processedCount: status === "succeeded" ? 3 : status === "failed" ? 2 : 0,
      skippedCount: status === "succeeded" || status === "failed" ? 1 : 0,
      requestedAt: "2026-08-14T00:02:00Z",
      updatedAt: `2026-08-14T00:0${attempt + 2}:00Z`,
      ...(status === "failed"
        ? {
            finishedAt: "2026-08-14T00:04:00Z",
            error: {
              code: "YTDLP_TEMPORARY_FAILURE",
              message: "YouTube temporarily rejected the collection request.",
              retryable: true,
            },
          }
        : {}),
    };
  }

  await page.route("**/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;

    if (request.method() === "GET" && pathname === `/v1/streams/${streamId}`) {
      await route.fulfill({ json: savedStream });
      return;
    }

    if (
      request.method() === "POST" &&
      pathname === `/v1/streams/${streamId}/collections`
    ) {
      phase = "first";
      await route.fulfill({ status: 202, json: job(firstJobId, "queued", 0) });
      return;
    }

    if (
      request.method() === "GET" &&
      pathname === `/v1/streams/${streamId}/collections/latest`
    ) {
      if (phase === "none") {
        await route.fulfill({
          status: 404,
          contentType: "application/problem+json",
          body: JSON.stringify({
            title: "Collection job not found",
            status: 404,
            detail: "No collection has been requested for this stream.",
            code: "COLLECTION_JOB_NOT_FOUND",
          }),
        });
        return;
      }
      if (phase === "first") {
        firstJobPolls += 1;
        if (firstJobPolls < 3) {
          await route.fulfill({ json: job(firstJobId, "running", 1) });
          return;
        }
        phase = "failed";
        await route.fulfill({ json: job(firstJobId, "failed", 1) });
        return;
      }
      if (phase === "failed") {
        await route.fulfill({ json: job(firstJobId, "failed", 1) });
        return;
      }
      phase = "succeeded";
      await route.fulfill({ json: job(retryJobId, "succeeded", 2) });
      return;
    }

    if (
      request.method() === "POST" &&
      pathname === `/v1/collection-jobs/${firstJobId}/retry`
    ) {
      phase = "retry";
      await route.fulfill({ status: 202, json: job(retryJobId, "queued", 2) });
      return;
    }

    if (
      request.method() === "GET" &&
      pathname === `/v1/streams/${streamId}/chat-messages`
    ) {
      if (url.searchParams.get("cursor") === "next-page") {
        await route.fulfill({
          json: {
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
          },
        });
        return;
      }
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
          nextCursor: "next-page",
        },
      });
      return;
    }

    await route.abort("failed");
  });
}
