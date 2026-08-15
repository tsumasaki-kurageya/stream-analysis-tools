import type { components } from "./generated/v1";

export type Stream = components["schemas"]["Stream"];
export type StreamList = components["schemas"]["StreamList"];
export type StreamPreview = components["schemas"]["StreamPreview"];
export type CollectionJob = components["schemas"]["CollectionJob"];
export type ChatMessage = components["schemas"]["ChatMessage"];
export type ChatMessagePage = components["schemas"]["ChatMessagePage"];
export type ProblemDetails = components["schemas"]["ProblemDetails"];

export class ApiProblem extends Error {
  constructor(public readonly problem: ProblemDetails) {
    super(problem.detail);
    this.name = "ApiProblem";
  }
}

export function listStreams(limit = 20, offset = 0): Promise<StreamList> {
  return request(`/v1/streams?limit=${limit}&offset=${offset}`);
}

export function previewStream(url: string): Promise<StreamPreview> {
  return request("/v1/streams/preview", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url }),
  });
}

export function createStream(url: string): Promise<Stream> {
  return request("/v1/streams", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url }),
  });
}

export function getStream(streamId: string): Promise<Stream> {
  return request(`/v1/streams/${encodeURIComponent(streamId)}`);
}

export function startCollection(streamId: string): Promise<CollectionJob> {
  return request(`/v1/streams/${encodeURIComponent(streamId)}/collections`, {
    method: "POST",
  });
}

export function getLatestCollection(streamId: string): Promise<CollectionJob> {
  return request(
    `/v1/streams/${encodeURIComponent(streamId)}/collections/latest`,
  );
}

export function retryCollection(jobId: string): Promise<CollectionJob> {
  return request(`/v1/collection-jobs/${encodeURIComponent(jobId)}/retry`, {
    method: "POST",
  });
}

export function listChatMessages(
  streamId: string,
  limit = 50,
  cursor?: string,
): Promise<ChatMessagePage> {
  const search = new URLSearchParams({ limit: String(limit) });
  if (cursor) search.set("cursor", cursor);
  return request(
    `/v1/streams/${encodeURIComponent(streamId)}/chat-messages?${search}`,
  );
}

export function searchChatMessages(
  streamId: string,
  query: string,
  limit = 50,
  cursor?: string,
): Promise<ChatMessagePage> {
  const search = new URLSearchParams({ q: query, limit: String(limit) });
  if (cursor) search.set("cursor", cursor);
  return request(
    `/v1/streams/${encodeURIComponent(streamId)}/chat-search?${search}`,
  );
}

export async function findStreamByYouTubeVideoId(
  youtubeVideoId: string,
): Promise<Stream | undefined> {
  const pageSize = 100;
  for (let offset = 0; ; offset += pageSize) {
    const page = await listStreams(pageSize, offset);
    const match = page.items.find(
      (stream) => stream.youtubeVideoId === youtubeVideoId,
    );
    if (match || page.items.length < pageSize) return match;
  }
}

async function request<T>(input: string, init?: RequestInit): Promise<T> {
  const response = await fetch(input, init);
  const body: unknown = await response.json();
  if (!response.ok) {
    throw new ApiProblem(body as ProblemDetails);
  }
  return body as T;
}
