import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ChatActivityChart } from "./ChatActivity";

const streamId = "f47ac10b-58cc-4372-a567-0e02b2c3d479";

afterEach(() => vi.unstubAllGlobals());

it("defaults to 10-second buckets, highlights playback, seeks, and changes interval", async () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    const bucketSeconds = Number(
      new URL(url, "http://localhost").searchParams.get("bucketSeconds"),
    );
    return new Response(
      JSON.stringify({
        bucketSeconds,
        items:
          bucketSeconds === 10
            ? [
                { startOffsetMilliseconds: 0, messageCount: 1 },
                { startOffsetMilliseconds: 10_000, messageCount: 4 },
              ]
            : [
                { startOffsetMilliseconds: 0, messageCount: 1 },
                { startOffsetMilliseconds: 5_000, messageCount: 2 },
              ],
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  });
  vi.stubGlobal("fetch", fetchMock);
  const onSeek = vi.fn();

  render(
    <ChatActivityChart
      streamId={streamId}
      playbackOffsetMilliseconds={12_000}
      onSeek={onSeek}
    />,
  );

  const tenSeconds = screen.getByRole("button", { name: "10秒" });
  expect(tenSeconds.getAttribute("aria-pressed")).toBe("true");
  const active = await screen.findByRole("button", {
    name: "0:10から10秒間: 4件",
  });
  expect(active.getAttribute("aria-current")).toBe("time");

  fireEvent.click(screen.getByRole("button", { name: "0:00から10秒間: 1件" }));
  expect(onSeek).toHaveBeenCalledWith(0);

  fireEvent.click(screen.getByRole("button", { name: "5秒" }));
  await waitFor(() =>
    expect(fetchMock).toHaveBeenCalledWith(
      `/v1/streams/${streamId}/chat-activity?bucketSeconds=5`,
      undefined,
    ),
  );
  expect(
    screen.getByRole("button", { name: "5秒" }).getAttribute("aria-pressed"),
  ).toBe("true");
});
