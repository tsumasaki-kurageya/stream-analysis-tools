import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { App } from "./App";
import type { Reservation } from "./api/client";

const reservation = {
  id: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  youtubeVideoId: "dQw4w9WgXcQ",
  sourceUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  state: "scheduled",
  nextCheckAt: "2026-08-15T12:00:00Z",
  monitorAttempt: 0,
  canCancel: true,
  createdAt: "2026-08-15T12:00:00Z",
  updatedAt: "2026-08-15T12:00:00Z",
} as const;

afterEach(() => {
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});

it("creates and cancels a reservation through its detail", async () => {
  let current: Reservation = reservation;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "/v1/reservations?limit=20&offset=0") {
        return jsonResponse({ items: [], total: 0, limit: 20, offset: 0 });
      }
      if (url === "/v1/reservations" && init?.method === "POST") {
        return jsonResponse(current, 201);
      }
      if (url === `/v1/reservations/${reservation.id}`) {
        return jsonResponse(current);
      }
      if (
        url === `/v1/reservations/${reservation.id}/cancel` &&
        init?.method === "POST"
      ) {
        current = { ...reservation, state: "canceled", canCancel: false };
        return jsonResponse(current);
      }
      throw new Error(`Unexpected request: ${url}`);
    }),
  );
  window.history.replaceState(null, "", "/reservations");
  render(<App />);

  fireEvent.change(
    await screen.findByRole("textbox", { name: "YouTube URL" }),
    { target: { value: reservation.sourceUrl } },
  );
  fireEvent.click(screen.getByRole("button", { name: "収集を予約" }));
  expect(
    await screen.findByRole("heading", { name: "配信待ち" }),
  ).toBeDefined();
  expect(screen.getByText(/配信予定時刻が近づくまで待機/)).toBeDefined();

  fireEvent.click(screen.getByRole("button", { name: "予約をキャンセル" }));
  expect(
    await screen.findByRole("heading", { name: "キャンセル済み" }),
  ).toBeDefined();
  expect(screen.queryByRole("button", { name: "予約をキャンセル" })).toBeNull();
});

it("separates monitoring and collection failures", async () => {
  window.history.replaceState(null, "", `/reservations/${reservation.id}`);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () =>
      jsonResponse({
        ...reservation,
        state: "failed",
        canCancel: false,
        lastErrorCode: "VIDEO_UNAVAILABLE",
        lastErrorMessage: "YouTube no longer exposes this broadcast.",
        lastErrorRetryable: false,
        collectionStatus: "failed",
        collectionError: {
          code: "CHAT_REPLAY_NOT_AVAILABLE",
          message: "Chat replay is not available for this stream.",
          retryable: false,
        },
      }),
    ),
  );
  render(<App />);

  expect(
    await screen.findByRole("heading", { name: "監視エラー" }),
  ).toBeDefined();
  expect(screen.getByRole("heading", { name: "収集エラー" })).toBeDefined();
});

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
