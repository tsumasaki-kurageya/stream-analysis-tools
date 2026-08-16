import { ApiProblem } from "./api/client";

const ERROR_MESSAGES: Record<string, string> = {
  INVALID_YOUTUBE_URL: "対応している YouTube 動画の URL を入力してください。",
  STREAM_NOT_FOUND:
    "指定されたストリームが見つかりません。ライブラリから選び直してください。",
  STREAM_ALREADY_REGISTERED:
    "このストリームは登録済みです。保存済みの動画を開きます。",
  COLLECTION_JOB_NOT_FOUND: "このストリームはまだ収集されていません。",
  CHAT_REPLAY_NOT_AVAILABLE:
    "このストリームではチャットリプレイを利用できません。",
  CHAT_REPLAY_UNAVAILABLE:
    "このストリームではチャットリプレイを利用できません。",
  SOURCE_NOT_READY:
    "YouTube のアーカイブはまだ準備中です。時間をおいて再試行してください。",
  YOUTUBE_ACCESS_DENIED:
    "YouTube から動画へアクセスできません。公開状態を確認してください。",
  YOUTUBE_RATE_LIMITED:
    "YouTube へのアクセスが集中しています。時間をおいて再試行してください。",
  YTDLP_TEMPORARY_FAILURE:
    "YouTube から一時的にデータを取得できませんでした。再試行してください。",
  YTDLP_TIMEOUT:
    "YouTube からの応答がタイムアウトしました。再試行してください。",
  YTDLP_OUTPUT_CHANGED:
    "YouTube の応答形式が変わったため収集できませんでした。",
  YTDLP_PROCESS_FAILED: "YouTube データの収集処理に失敗しました。",
  CHAT_IMPORT_FAILED: "チャットの保存中にエラーが発生しました。",
  WORKER_DISK_CAPACITY_LOW:
    "保存先の空き容量が不足しているため収集を開始できません。",
  COLLECTION_CANCELLED: "チャット収集はキャンセルされました。",
  VIDEO_UNAVAILABLE:
    "YouTube でこの配信を確認できません。公開状態を確認してください。",
};

export function userFacingError(
  error: unknown,
  fallback = "処理を完了できませんでした。接続を確認して、もう一度お試しください。",
): string {
  if (!(error instanceof ApiProblem)) return fallback;
  return messageForCode(error.problem.code, fallback);
}

export function messageForCode(code?: string, fallback?: string): string {
  if (code && ERROR_MESSAGES[code]) return ERROR_MESSAGES[code];
  return fallback ?? "予期しないエラーが発生しました。もう一度お試しください。";
}
