#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ -f "${PROJECT_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${PROJECT_DIR}/.env"
  set +a
fi

DB_DSN="${POSTGRES_URL:-${DATABASE_URL:-${NEON_DATABASE_URL:-}}}"
if [[ -z "${DB_DSN}" ]]; then
  echo "[cron-media-maintenance] missing DB DSN (POSTGRES_URL/DATABASE_URL; NEON_DATABASE_URL is compatibility fallback)" >&2
  exit 1
fi

LOG_DIR="${DWIZZY_LOG_DIR:-${PROJECT_DIR}/logs}"
mkdir -p "${LOG_DIR}"
LOG_FILE="${LOG_DIR}/cron-media-maintenance.log"

exec >>"${LOG_FILE}" 2>&1

now_utc() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

echo "[$(now_utc)] start cron-media-maintenance"

LOCK_FILE="/tmp/dwizzyscrape-cron-media-maintenance.lock"
if command -v flock >/dev/null 2>&1; then
  exec 9>"${LOCK_FILE}"
  if ! flock -n 9; then
    echo "[$(now_utc)] skip: lock busy (${LOCK_FILE})"
    exit 0
  fi
fi

if [[ "${DWIZZYSCRAPE_USE_GO_RUN:-0}" == "1" ]]; then
  DWIZZY_CMD=(go run ./cmd/dwizzyscrape)
elif [[ -n "${DWIZZYSCRAPE_CMD:-}" ]]; then
  # shellcheck disable=SC2206
  DWIZZY_CMD=(${DWIZZYSCRAPE_CMD})
elif [[ -x "${PROJECT_DIR}/.bin/dwizzyscrape" ]]; then
  DWIZZY_CMD=("${PROJECT_DIR}/.bin/dwizzyscrape")
else
  DWIZZY_CMD=(go run ./cmd/dwizzyscrape)
fi

DRY_RUN="${DRY_RUN:-0}"
STEP_DELAY="${DWIZZY_STEP_DELAY_SEC:-0.25}"
CMD_TIMEOUT="${DWIZZY_CMD_TIMEOUT:-300s}"

run_dw() {
  local label="$1"
  shift
  echo "[$(now_utc)] run: ${label} -> ${DWIZZY_CMD[*]} $*"
  if [[ "${DRY_RUN}" == "1" ]]; then
    return 0
  fi
  if command -v timeout >/dev/null 2>&1; then
    timeout "${CMD_TIMEOUT}" "${DWIZZY_CMD[@]}" "$@"
  else
    "${DWIZZY_CMD[@]}" "$@"
  fi
  sleep "${STEP_DELAY}"
}

run_dw "refresh:anime-v2" refresh-anime-v2
run_dw "refresh:media-v2" refresh-media-v2
run_dw "refresh:movie-v3" refresh-movie-v3

echo "[$(now_utc)] done cron-media-maintenance"
