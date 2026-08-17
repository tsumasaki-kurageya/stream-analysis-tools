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

test("FLW-001 / FLW-002: registers and opens a stream from the list", async ({
  page,
}) => {
  await installStreamApi(page);
  await page.goto("/streams");

  await expect(page.getByRole("heading", { name: "配信一覧" })).toBeVisible();
  await expect(page.getByRole("textbox", { name: "YouTube URL" })).toHaveCount(
    0,
  );
  await expect(
    page.getByRole("columnheader", { name: "タイトル" }),
  ).toBeVisible();
  await expect(
    page.getByRole("columnheader", { name: "チャット件数" }),
  ).toBeVisible();

  await page.getByRole("button", { name: "配信を追加" }).click();
  await page
    .getByRole("textbox", { name: "YouTube URL" })
    .fill("https://youtu.be/dQw4w9WgXcQ");
  await expect(
    page.getByRole("button", { name: "ライブラリに保存" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "プレビュー" }).click();
  await expect(
    page.getByRole("heading", { name: "An evening of live music" }),
  ).toBeVisible();
  await expect(page.getByText("Harbor Sessions").first()).toBeVisible();

  await page.getByRole("button", { name: "ライブラリに保存" }).click();
  await expect(page).toHaveURL(`/streams/${streamId}`);
  await expect(
    page.getByRole("link", { name: "ライブラリに戻る" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "配信情報を表示" }).click();
  await expect(page.getByRole("dialog", { name: "配信情報" })).toContainText(
    preview.youtubeVideoId,
  );
  await page.getByRole("button", { name: "配信情報を閉じる" }).click();

  await page.getByRole("link", { name: "ライブラリに戻る" }).click();
  await expect(page).toHaveURL("/streams");
  await expect(page.getByRole("heading", { name: "配信一覧" })).toBeVisible();
  await expect(
    page.getByRole("link", { name: "An evening of live music" }),
  ).toBeVisible();
  await expect(page.getByText("0", { exact: true })).toBeVisible();

  await page.getByRole("link", { name: "An evening of live music" }).click();
  await expect(page).toHaveURL(`/streams/${streamId}`);
});

test("starts, restores, retries, and browses a chat collection", async ({
  page,
}) => {
  await installCollectionApi(page);
  await page.goto(`/streams/${streamId}`);
  const workspace = page.getByRole("complementary", { name: "チャットと収集" });

  await page.getByRole("button", { name: "収集を開始" }).click();
  await expect(workspace.getByText("待機中")).toBeVisible();

  await page.reload();
  const restoredWorkspace = page.getByRole("complementary", {
    name: "チャットと収集",
  });
  await expect(
    restoredWorkspace.getByText(
      "YouTube から一時的にデータを取得できませんでした。再試行してください。",
    ),
  ).toBeVisible({ timeout: 7_000 });
  await expect(
    restoredWorkspace.getByText(
      "1件のチャットを保存できませんでした。 保存済みのメッセージは引き続き検索できます。",
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("search", { name: "収集済みチャットを検索" }),
  ).toBeVisible();
  await expect(
    page.getByRole("list", { name: "収集済みチャット" }).getByRole("listitem"),
  ).toHaveCount(2);

  await page.getByRole("button", { name: "収集を再試行" }).click();
  await expect(restoredWorkspace.getByText("待機中")).toBeVisible();
  await expect(restoredWorkspace.getByText("完了")).toBeVisible();
  await expect(restoredWorkspace.getByText("収集済み · 3件")).toBeVisible();

  const chat = page.getByRole("list", { name: "収集済みチャット" });
  await expect(chat.getByRole("listitem")).toHaveCount(2);
  await expect(chat.getByRole("listitem").nth(0)).toContainText(
    "Opening message",
  );
  await expect(chat.getByRole("listitem").nth(1)).toContainText(
    "Later message",
  );

  await page.getByRole("button", { name: "さらに読み込む" }).click();
  await expect(chat.getByRole("listitem")).toHaveCount(3);
  await expect(chat.getByRole("listitem").nth(2)).toContainText(
    "Final message",
  );
});

test("keeps chat synchronized with playback and seeks from a message", async ({
  page,
}) => {
  await installSynchronizedPlaybackApi(page);
  await installFakeYouTubePlayer(page, 65);
  await page.goto(`/streams/${streamId}`);

  const chat = page.getByRole("list", { name: "収集済みチャット" });
  await expect(chat.getByRole("listitem").nth(1)).toHaveAttribute(
    "aria-current",
    "time",
  );
  await expect(page.getByText("再生位置 1:05")).toBeVisible();

  await chat
    .getByRole("button", { name: "0:05へ移動: Opening message" })
    .click();
  await expect(chat.getByRole("listitem").nth(0)).toHaveAttribute(
    "aria-current",
    "time",
  );
  await expect(page.getByText("再生位置 0:05")).toBeVisible();
  await expect(page.getByText("Fake player at 5 seconds")).toBeVisible();
});

test("searches chat and seeks playback from a keyboard-selected result", async ({
  page,
}) => {
  await installSynchronizedPlaybackApi(page);
  await installFakeYouTubePlayer(page);
  await page.goto(`/streams/${streamId}`);

  await expect(
    page.getByRole("button", { name: "Play at 1:05" }),
  ).toBeVisible();
  const search = page.getByRole("search", { name: "収集済みチャットを検索" });
  await search
    .getByRole("searchbox", { name: "収集済みチャットを検索" })
    .fill("later");
  await page.keyboard.press("Enter");

  const results = page.getByRole("list", { name: "チャット検索結果" });
  await expect(results).toContainText("Later message");
  const result = results.getByRole("button", {
    name: "1:05へ移動: Later message",
  });
  await result.focus();
  await page.keyboard.press("Enter");

  await expect(page.getByText("再生位置 1:05")).toBeVisible();
  await expect(page.getByText("Fake player at 65 seconds")).toBeVisible();
  await expect(result).toHaveAttribute("aria-current", "time");
  await expect(
    page
      .getByRole("list", { name: "収集済みチャット" })
      .getByRole("listitem")
      .nth(1),
  ).toHaveAttribute("aria-current", "time");
});

test("keeps collected chat available when YouTube embedding fails", async ({
  page,
}) => {
  await installSynchronizedPlaybackApi(page);
  await installUnavailableYouTubePlayer(page);
  await page.goto(`/streams/${streamId}`);

  await expect(
    page.getByRole("link", { name: "YouTube で動画を開く" }),
  ).toHaveAttribute("href", preview.canonicalUrl);

  const chat = page.getByRole("list", { name: "収集済みチャット" });
  await expect(chat.getByRole("listitem")).toHaveCount(2);
  await expect(chat).toContainText("Opening message");
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
        json: {
          items: savedStream
            ? [
                {
                  ...savedStream,
                  collectionStatus: undefined,
                  chatMessageCount: 0,
                },
              ]
            : [],
          limit: 20,
          offset: 0,
        },
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

async function installSynchronizedPlaybackApi(page: Page) {
  const savedStream = {
    ...preview,
    id: streamId,
    createdAt: "2026-08-14T00:01:00Z",
    updatedAt: "2026-08-14T00:01:00Z",
  };

  await page.route("**/v1/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    if (pathname === `/v1/streams/${streamId}`) {
      await route.fulfill({ json: savedStream });
      return;
    }
    if (pathname === `/v1/streams/${streamId}/collections/latest`) {
      await route.fulfill({
        json: {
          id: "787f789a-c336-4db7-94aa-739730a2f0b8",
          streamId,
          kind: "chat_replay",
          status: "succeeded",
          attempt: 1,
          processedCount: 2,
          skippedCount: 0,
          requestedAt: "2026-08-14T00:02:00Z",
          updatedAt: "2026-08-14T00:03:00Z",
          finishedAt: "2026-08-14T00:03:00Z",
        },
      });
      return;
    }
    if (pathname === `/v1/streams/${streamId}/chat-messages`) {
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
    if (pathname === `/v1/streams/${streamId}/chat-search`) {
      if (url.searchParams.get("q") !== "later") {
        await route.abort("failed");
        return;
      }
      await route.fulfill({
        json: {
          items: [
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

async function installFakeYouTubePlayer(page: Page, initialTime = 0) {
  await page.route("https://www.youtube.com/iframe_api", async (route) => {
    await route.fulfill({
      contentType: "application/javascript",
      body: `
window.YT = {
  PlayerState: { PLAYING: 1 },
  Player: function (element, options) {
    let currentTime = ${initialTime};
    const control = document.createElement("button");
    const updateControl = () => {
      control.textContent = "Fake player at " + currentTime + " seconds";
      control.setAttribute("aria-label", currentTime === 0 ? "Play at 1:05" : control.textContent);
    };
    control.addEventListener("click", () => {
      currentTime = 65;
      updateControl();
      options.events.onStateChange({ data: window.YT.PlayerState.PLAYING });
    });
    updateControl();
    element.replaceChildren(control);
    this.getCurrentTime = () => currentTime;
    this.seekTo = (seconds) => {
      currentTime = seconds;
      updateControl();
      options.events.onStateChange({ data: window.YT.PlayerState.PLAYING });
    };
    this.destroy = () => {};
    queueMicrotask(() => options.events.onReady());
  }
};
window.onYouTubeIframeAPIReady();
`,
    });
  });
}

async function installUnavailableYouTubePlayer(page: Page) {
  await page.route("https://www.youtube.com/iframe_api", async (route) => {
    await route.fulfill({
      contentType: "application/javascript",
      body: `
window.YT = {
  PlayerState: { PLAYING: 1 },
  Player: function (_element, options) {
    this.getCurrentTime = () => 0;
    this.seekTo = () => {};
    this.destroy = () => {};
    queueMicrotask(() => options.events.onError({ data: 101 }));
  }
};
window.onYouTubeIframeAPIReady();
`,
    });
  });
}
