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
  echo "[cron-release-comics] missing DB DSN (DATABASE_URL; POSTGRES_URL/NEON_DATABASE_URL are compatibility fallbacks)" >&2
  exit 1
fi

LOG_DIR="${DWIZZY_LOG_DIR:-${PROJECT_DIR}/logs}"
mkdir -p "${LOG_DIR}"
LOG_FILE="${LOG_DIR}/cron-release-comics.log"

exec >>"${LOG_FILE}" 2>&1

now_utc() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

echo "[$(now_utc)] start cron-release-comics"

LOCK_FILE="/tmp/dwizzyscrape-cron-release-comics.lock"
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

BACAMAN_CMD=("${PROJECT_DIR}/scripts/backfill-bacaman.sh")

DRY_RUN="${DRY_RUN:-0}"
STEP_DELAY="${DWIZZY_STEP_DELAY_SEC:-0.25}"
CMD_TIMEOUT="${DWIZZY_CMD_TIMEOUT:-180s}"

MANHWA_CATALOG_PAGES="${MANHWA_CATALOG_PAGES:-1}"
MANHWA_RECENT_SERIES_LIMIT="${MANHWA_RECENT_SERIES_LIMIT:-10}"
MANHWA_RECENT_CHAPTER_LIMIT="${MANHWA_RECENT_CHAPTER_LIMIT:-10}"
KOMIKU_CATALOG_PAGES="${KOMIKU_CATALOG_PAGES:-1}"
KOMIKU_RECENT_SERIES_LIMIT="${KOMIKU_RECENT_SERIES_LIMIT:-10}"
KOMIKU_RECENT_CHAPTER_LIMIT="${KOMIKU_RECENT_CHAPTER_LIMIT:-10}"
BACAMAN_ENABLED="${BACAMAN_ENABLED:-1}"
BACAMAN_MAX_SERIES="${BACAMAN_MAX_SERIES:-8}"
BACAMAN_MAX_CHAPTERS_PER_SERIES="${BACAMAN_MAX_CHAPTERS_PER_SERIES:-1}"
BACAMAN_HTTP_TIMEOUT="${BACAMAN_HTTP_TIMEOUT:-45s}"

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

if [[ "${RESPECT_BACKFILL:-1}" == "1" ]] && source_running 'manhwaindo-backfill'; then
  echo "[$(now_utc)] skip manhwaindo: backfill process detected"
else
  for page in $(seq 1 "${MANHWA_CATALOG_PAGES}"); do
    safe_run "manhwaindo:catalog:p${page}" "${DWIZZY_CMD[@]}" sync-manhwa-catalog "${page}" || true
  done
  mapfile -t manhwa_series_slugs < <(
    query_lines "select slug from media_items where source='manhwaindo' order by updated_at desc nulls last limit ${MANHWA_RECENT_SERIES_LIMIT};"
  )
  for slug in "${manhwa_series_slugs[@]}"; do
    [[ -z "${slug}" ]] && continue
    safe_run "manhwaindo:series:${slug}" "${DWIZZY_CMD[@]}" sync-manhwa-series "${slug}" || true
  done
  mapfile -t manhwa_chapter_slugs < <(
    query_lines "select slug from media_units where source='manhwaindo' and unit_type='chapter' order by updated_at desc nulls last limit ${MANHWA_RECENT_CHAPTER_LIMIT};"
  )
  for slug in "${manhwa_chapter_slugs[@]}"; do
    [[ -z "${slug}" ]] && continue
    safe_run "manhwaindo:chapter:${slug}" "${DWIZZY_CMD[@]}" sync-manhwa-chapter "${slug}" || true
  done
fi

if [[ "${RESPECT_BACKFILL:-1}" == "1" ]] && source_running 'komiku-backfill'; then
  echo "[$(now_utc)] skip komiku: backfill process detected"
else
  for page in $(seq 1 "${KOMIKU_CATALOG_PAGES}"); do
    safe_run "komiku:catalog:p${page}" "${DWIZZY_CMD[@]}" sync-komiku-catalog "${page}" || true
  done
  mapfile -t komiku_series_slugs < <(
    query_lines "select slug from media_items where source='komiku' order by updated_at desc nulls last limit ${KOMIKU_RECENT_SERIES_LIMIT};"
  )
  for slug in "${komiku_series_slugs[@]}"; do
    [[ -z "${slug}" ]] && continue
    safe_run "komiku:series:${slug}" "${DWIZZY_CMD[@]}" sync-komiku-series "${slug}" || true
  done
  mapfile -t komiku_chapter_slugs < <(
    query_lines "select slug from media_units where source='komiku' and unit_type='chapter' order by updated_at desc nulls last limit ${KOMIKU_RECENT_CHAPTER_LIMIT};"
  )
  for slug in "${komiku_chapter_slugs[@]}"; do
    [[ -z "${slug}" ]] && continue
    safe_run "komiku:chapter:${slug}" "${DWIZZY_CMD[@]}" sync-komiku-chapter "${slug}" || true
  done
fi

if [[ "${BACAMAN_ENABLED}" != "1" ]]; then
  echo "[$(now_utc)] skip bacaman: BACAMAN_ENABLED=${BACAMAN_ENABLED}"
elif [[ "${RESPECT_BACKFILL:-1}" == "1" ]] && source_running 'bacaman-backfill'; then
  echo "[$(now_utc)] skip bacaman: backfill process detected"
else
  safe_run \
    "bacaman:latest-window" \
    env \
    BACAMAN_MAX_SERIES="${BACAMAN_MAX_SERIES}" \
    BACAMAN_MAX_CHAPTERS_PER_SERIES="${BACAMAN_MAX_CHAPTERS_PER_SERIES}" \
    BACAMAN_HTTP_TIMEOUT="${BACAMAN_HTTP_TIMEOUT}" \
    "${BACAMAN_CMD[@]}" || true
fi

echo "[$(now_utc)] note: kanzenin and mangasusuku are intentionally excluded from 4h cron because their current entrypoints are A-Z crawls, not lightweight latest feeds"
echo "[$(now_utc)] done cron-release-comics"
