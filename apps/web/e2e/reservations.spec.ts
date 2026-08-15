import { expect, test, type Page } from "@playwright/test";

const reservationId = "f47ac10b-58cc-4372-a567-0e02b2c3d479";
const streamId = "c56a4180-65aa-42ec-a945-5fd21dec0538";
const sourceUrl = "https://www.youtube.com/watch?v=dQw4w9WgXcQ";

test("creates and cancels a supported reservation", async ({ page }) => {
  await installReservationApi(page, "cancel");
  await page.goto("/reservations");

  await page.getByRole("textbox", { name: "YouTube URL" }).fill(sourceUrl);
  await page.getByRole("button", { name: "収集を予約" }).click();
  await expect(page).toHaveURL(`/reservations/${reservationId}`);
  await expect(page.getByRole("heading", { name: "配信待ち" })).toBeVisible();

  await page.getByRole("button", { name: "予約をキャンセル" }).click();
  await expect(
    page.getByRole("heading", { name: "キャンセル済み" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "予約をキャンセル" }),
  ).toHaveCount(0);
});

test("follows automatic collection through completion to stream detail", async ({
  page,
}) => {
  await installReservationApi(page, "complete");
  await page.goto("/reservations");

  await page.getByRole("textbox", { name: "YouTube URL" }).fill(sourceUrl);
  await page.getByRole("button", { name: "収集を予約" }).click();
  await expect(page.getByRole("heading", { name: "収集中" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "完了" })).toBeVisible({
    timeout: 7_000,
  });

  await page.getByRole("link", { name: "収集済みストリームを開く" }).click();
  await expect(page).toHaveURL(`/streams/${streamId}`);
  await expect(
    page.getByRole("heading", { name: "Reserved broadcast" }),
  ).toBeVisible();
});

async function installReservationApi(
  page: Page,
  scenario: "cancel" | "complete",
) {
  let canceled = false;
  let createdAt = 0;
  await page.route("**/v1/**", async (route) => {
    const request = route.request();
    const { pathname } = new URL(request.url());

    if (request.method() === "GET" && pathname === "/v1/reservations") {
      await route.fulfill({
        json: { items: [], total: 0, limit: 20, offset: 0 },
      });
      return;
    }
    if (request.method() === "POST" && pathname === "/v1/reservations") {
      createdAt = Date.now();
      await route.fulfill({
        status: 201,
        json: reservation("scheduled", true),
      });
      return;
    }
    if (
      request.method() === "POST" &&
      pathname === `/v1/reservations/${reservationId}/cancel`
    ) {
      canceled = true;
      await route.fulfill({ json: reservation("canceled", false) });
      return;
    }
    if (
      request.method() === "GET" &&
      pathname === `/v1/reservations/${reservationId}`
    ) {
      if (scenario === "cancel") {
        await route.fulfill({
          json: reservation(canceled ? "canceled" : "scheduled", !canceled),
        });
      } else {
        await route.fulfill({
          json: reservation(
            Date.now() - createdAt >= 1_000 ? "completed" : "collecting",
            false,
          ),
        });
      }
      return;
    }
    if (request.method() === "GET" && pathname === `/v1/streams/${streamId}`) {
      await route.fulfill({ json: stream() });
      return;
    }
    if (
      request.method() === "GET" &&
      pathname === `/v1/streams/${streamId}/collections/latest`
    ) {
      await route.fulfill({
        json: {
          id: reservationId,
          streamId,
          kind: "chat_replay",
          status: "succeeded",
          attempt: 1,
          processedCount: 0,
          skippedCount: 0,
          requestedAt: "2026-08-15T12:00:00Z",
          updatedAt: "2026-08-15T12:10:00Z",
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

function reservation(
  state: "scheduled" | "collecting" | "completed" | "canceled",
  canCancel: boolean,
) {
  return {
    id: reservationId,
    youtubeVideoId: "dQw4w9WgXcQ",
    sourceUrl,
    state,
    nextCheckAt: "2026-08-15T12:05:00Z",
    monitorAttempt: 2,
    canCancel,
    streamId:
      state === "collecting" || state === "completed" ? streamId : undefined,
    collectionJobId:
      state === "collecting" || state === "completed"
        ? reservationId
        : undefined,
    collectionStatus:
      state === "collecting"
        ? "running"
        : state === "completed"
          ? "succeeded"
          : undefined,
    createdAt: "2026-08-15T12:00:00Z",
    updatedAt: "2026-08-15T12:05:00Z",
  };
}

function stream() {
  return {
    id: streamId,
    youtubeVideoId: "dQw4w9WgXcQ",
    canonicalUrl: sourceUrl,
    title: "Reserved broadcast",
    channelId: "UC-reservation",
    channelTitle: "Reservation Channel",
    lifecycleStatus: "ended",
    metadataFetchedAt: "2026-08-15T12:05:00Z",
    createdAt: "2026-08-15T12:05:00Z",
    updatedAt: "2026-08-15T12:05:00Z",
  };
}
