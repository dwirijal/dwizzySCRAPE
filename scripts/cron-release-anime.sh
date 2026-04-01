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

DB_DSN="${DATABASE_URL:-${POSTGRES_URL:-${NEON_DATABASE_URL:-}}}"
if [[ -z "${DB_DSN}" ]]; then
  echo "[cron-release-anime] missing DB DSN (DATABASE_URL; POSTGRES_URL/NEON_DATABASE_URL are compatibility fallbacks)" >&2
  exit 1
fi

LOG_DIR="${DWIZZY_LOG_DIR:-${PROJECT_DIR}/logs}"
mkdir -p "${LOG_DIR}"
LOG_FILE="${LOG_DIR}/cron-release-anime.log"

exec >>"${LOG_FILE}" 2>&1

now_utc() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

echo "[$(now_utc)] start cron-release-anime"

LOCK_FILE="/tmp/dwizzyscrape-cron-release-anime.lock"
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

ANICHIN_CMD=("${PROJECT_DIR}/scripts/backfill-anichin.sh")

DRY_RUN="${DRY_RUN:-0}"
STEP_DELAY="${DWIZZY_STEP_DELAY_SEC:-0.25}"
CMD_TIMEOUT="${DWIZZY_CMD_TIMEOUT:-180s}"

SAMEHADAKU_RECENT_LIMIT="${SAMEHADAKU_RECENT_LIMIT:-12}"
MOVIE_HOME_LIMIT="${MOVIE_HOME_LIMIT:-18}"
ANICHIN_SECTIONS="${ANICHIN_SECTIONS:-ongoing,completed}"
ANICHIN_MAX_PAGES_PER_SECTION="${ANICHIN_MAX_PAGES_PER_SECTION:-1}"
ANICHIN_MAX_SERIES="${ANICHIN_MAX_SERIES:-12}"
ANICHIN_MAX_EPISODES_PER_SERIES="${ANICHIN_MAX_EPISODES_PER_SERIES:-2}"
ANICHIN_SKIP_EXISTING="${ANICHIN_SKIP_EXISTING:-1}"
ANICHIN_HTTP_TIMEOUT="${ANICHIN_HTTP_TIMEOUT:-45s}"

source_running() {
  local pattern="$1"
  pgrep -af "${pattern}" >/dev/null 2>&1
}

query_lines() {
  local sql="$1"
  psql "${DB_DSN}" -At -v ON_ERROR_STOP=1 -c "${sql}"
}

run_cmd() {
  local label="$1"
  shift
  echo "[$(now_utc)] run: ${label} -> $*"
  if [[ "${DRY_RUN}" == "1" ]]; then
    return 0
  fi
  if command -v timeout >/dev/null 2>&1; then
    timeout "${CMD_TIMEOUT}" "$@"
  else
    "$@"
  fi
  sleep "${STEP_DELAY}"
}

safe_run() {
  local label="$1"
  shift
  if ! run_cmd "${label}" "$@"; then
    echo "[$(now_utc)] warn: failed ${label}"
    return 1
  fi
  return 0
}

cd "${PROJECT_DIR}"

if [[ "${RESPECT_BACKFILL:-1}" == "1" ]] && source_running 'samehadaku-backfill'; then
  echo "[$(now_utc)] skip samehadaku: backfill process detected"
else
  safe_run "samehadaku:catalog-sync" "${DWIZZY_CMD[@]}" sync || true
  mapfile -t samehadaku_rows < <(
    query_lines "select slug || E'\t' || media_type from media_items where source='samehadaku' order by updated_at desc nulls last limit ${SAMEHADAKU_RECENT_LIMIT};"
  )
  for row in "${samehadaku_rows[@]}"; do
    IFS=$'\t' read -r slug media_type <<<"${row}"
    [[ -z "${slug}" ]] && continue
    safe_run "samehadaku:detail:${slug}" "${DWIZZY_CMD[@]}" detail-anime "${slug}" || true
    if [[ "${media_type}" == "anime" ]]; then
      safe_run "samehadaku:episodes:${slug}" "${DWIZZY_CMD[@]}" detail-episodes "${slug}" || true
    fi
  done
fi

safe_run "movie:kanata-home" "${DWIZZY_CMD[@]}" sync-movie-kanata-home "${MOVIE_HOME_LIMIT}" || true

if [[ "${RESPECT_BACKFILL:-1}" == "1" ]] && source_running 'anichin-backfill'; then
  echo "[$(now_utc)] skip anichin: backfill process detected"
else
  safe_run \
    "anichin:latest-window" \
    env \
    ANICHIN_SECTIONS="${ANICHIN_SECTIONS}" \
    ANICHIN_MAX_PAGES_PER_SECTION="${ANICHIN_MAX_PAGES_PER_SECTION}" \
    ANICHIN_MAX_SERIES="${ANICHIN_MAX_SERIES}" \
    ANICHIN_MAX_EPISODES_PER_SERIES="${ANICHIN_MAX_EPISODES_PER_SERIES}" \
    ANICHIN_SKIP_EXISTING="${ANICHIN_SKIP_EXISTING}" \
    ANICHIN_HTTP_TIMEOUT="${ANICHIN_HTTP_TIMEOUT}" \
    "${ANICHIN_CMD[@]}" || true
fi

echo "[$(now_utc)] done cron-release-anime"
