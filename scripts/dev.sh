#!/usr/bin/env bash
set -Eeuo pipefail

repository_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repository_root"

if [[ ! -f .env ]]; then
  cp .env.example .env
  echo "Created .env. Set YSA_YOUTUBE_API_KEY in it, then run 'make dev' again." >&2
  exit 1
fi

set -a
# .env is intentionally shell-compatible so the same values configure every local process.
# shellcheck disable=SC1091
source .env
set +a

if [[ -z "${YSA_YOUTUBE_API_KEY:-}" ]]; then
  echo "YSA_YOUTUBE_API_KEY is empty. Set it in .env, then run 'make dev' again." >&2
  exit 1
fi

make db-up

if [[ "${REMOTE_CONTAINERS:-false}" == "true" ]]; then
  network_name="stream-analysis-tools_default"
  container_name=$(hostname)
  if ! network_error=$(docker network connect "$network_name" "$container_name" 2>&1); then
    if [[ "$network_error" != *"already exists"* ]]; then
      echo "$network_error" >&2
      exit 1
    fi
  fi

  YSA_DATABASE_URL=${YSA_DATABASE_URL/@localhost:5432/@postgres:5432}
  export YSA_DATABASE_URL
fi

make -C apps/api build

process_ids=()

stop_processes() {
  trap - EXIT INT TERM
  if ((${#process_ids[@]} > 0)); then
    kill "${process_ids[@]}" 2>/dev/null || true
    wait "${process_ids[@]}" 2>/dev/null || true
  fi
}
trap stop_processes EXIT INT TERM

echo "Starting Main API, Collection Worker, and Web UI. Press Ctrl+C to stop them."

./apps/api/bin/main-api &
process_ids+=("$!")

YSA_WORKER_QUEUE_ENABLED=true uv run --project apps/worker stream-analysis-worker &
process_ids+=("$!")

npm --prefix apps/web run dev &
process_ids+=("$!")

wait -n "${process_ids[@]}"
