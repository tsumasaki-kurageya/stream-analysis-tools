import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { StreamListPage } from "./StreamListPage";

const stream = {
  id: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  youtubeVideoId: "dQw4w9WgXcQ",
  canonicalUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
  title: "Analysis stream",
  channelId: "UC-analysis",
  channelTitle: "Analysis Channel",
  actualStartAt: "2026-08-10T10:00:00Z",
  actualEndAt: "2026-08-10T11:00:00Z",
  durationMs: 3_600_000,
  lifecycleStatus: "ended" as const,
  metadataFetchedAt: "2026-08-14T00:00:00Z",
  createdAt: "2026-08-14T00:01:00Z",
  updatedAt: "2026-08-14T00:01:00Z",
  collectionStatus: "succeeded" as const,
  chatMessageCount: 42,
};

it("renders the seven comparison fields and keeps creation collapsed", () => {
  render(
    <StreamListPage
      streams={[stream]}
      isLoading={false}
      error={null}
      url=""
      onURLChange={vi.fn()}
      onPreview={vi.fn()}
      isPreviewing={false}
      preview={null}
      previewNode={null}
      onOpenStream={vi.fn()}
    />,
  );

  for (const heading of [
    "タイトル",
    "チャンネル",
    "配信日時",
    "配信時間",
    "配信状態",
    "収集状態",
    "チャット件数",
  ]) {
    expect(screen.getByRole("columnheader", { name: heading })).toBeDefined();
  }
  expect(screen.getByText("42")).toBeDefined();
  expect(screen.queryByRole("textbox", { name: "YouTube URL" })).toBeNull();

  fireEvent.click(screen.getByRole("button", { name: "配信を追加" }));
  expect(screen.getByRole("textbox", { name: "YouTube URL" })).toBeDefined();
});
