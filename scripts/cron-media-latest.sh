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

DB_DSN="${NEON_DATABASE_URL:-${POSTGRES_URL:-${DATABASE_URL:-}}}"
if [[ -z "${DB_DSN}" ]]; then
  echo "[cron-media-latest] missing DB DSN (NEON_DATABASE_URL/POSTGRES_URL/DATABASE_URL)" >&2
  exit 1
fi

LOG_DIR="${DWIZZY_LOG_DIR:-${PROJECT_DIR}/logs}"
mkdir -p "${LOG_DIR}"
LOG_FILE="${LOG_DIR}/cron-media-latest.log"

exec >>"${LOG_FILE}" 2>&1

now_utc() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

echo "[$(now_utc)] start cron-media-latest"

LOCK_FILE="/tmp/dwizzyscrape-cron-media-latest.lock"
if command -v flock >/dev/null 2>&1; then
  exec 9>"${LOCK_FILE}"
  if ! flock -n 9; then
    echo "[$(now_utc)] skip: lock busy (${LOCK_FILE})"
    exit 0
  fi
fi

if [[ "${RESPECT_BACKFILL:-1}" == "1" ]]; then
  if pgrep -f "dwizzyscrape backfill-" >/dev/null 2>&1; then
    echo "[$(now_utc)] skip: backfill process detected"
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

STEP_DELAY="${DWIZZY_STEP_DELAY_SEC:-0.25}"
CMD_TIMEOUT="${DWIZZY_CMD_TIMEOUT:-180s}"
DRY_RUN="${DRY_RUN:-0}"

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

query_lines() {
  local sql="$1"
  psql "${DB_DSN}" -At -v ON_ERROR_STOP=1 -c "${sql}"
}

safe_run() {
  local label="$1"
  shift
  if ! run_dw "${label}" "$@"; then
    echo "[$(now_utc)] warn: failed ${label}"
    return 1
  fi
  return 0
}

ANIME_RECENT_LIMIT="${ANIME_RECENT_LIMIT:-24}"
MANHWA_CATALOG_PAGES="${MANHWA_CATALOG_PAGES:-2}"
MANHWA_RECENT_SERIES_LIMIT="${MANHWA_RECENT_SERIES_LIMIT:-20}"
MANHWA_RECENT_CHAPTER_LIMIT="${MANHWA_RECENT_CHAPTER_LIMIT:-20}"
KOMIKU_CATALOG_PAGES="${KOMIKU_CATALOG_PAGES:-2}"
KOMIKU_RECENT_SERIES_LIMIT="${KOMIKU_RECENT_SERIES_LIMIT:-20}"
KOMIKU_RECENT_CHAPTER_LIMIT="${KOMIKU_RECENT_CHAPTER_LIMIT:-20}"
MOVIE_HOME_LIMIT="${MOVIE_HOME_LIMIT:-48}"

safe_run "samehadaku:catalog-sync" sync || true

mapfile -t anime_slugs < <(
  query_lines "select slug from anime_list where source_code='s' order by updated_at desc nulls last limit ${ANIME_RECENT_LIMIT};"
)
for slug in "${anime_slugs[@]}"; do
  [[ -z "${slug}" ]] && continue
  safe_run "samehadaku:detail:${slug}" detail-anime "${slug}" || true
  safe_run "samehadaku:episodes:${slug}" detail-episodes "${slug}" || true
done

for page in $(seq 1 "${MANHWA_CATALOG_PAGES}"); do
  safe_run "manhwaindo:catalog:p${page}" sync-manhwa-catalog "${page}" || true
done

mapfile -t manhwa_series_slugs < <(
  query_lines "select source_slug from content_source_links where source_key='manhwaindo' order by last_scraped_at desc nulls last limit ${MANHWA_RECENT_SERIES_LIMIT};"
)
for slug in "${manhwa_series_slugs[@]}"; do
  [[ -z "${slug}" ]] && continue
  safe_run "manhwaindo:series:${slug}" sync-manhwa-series "${slug}" || true
done

mapfile -t manhwa_latest_chapter_slugs < <(
  query_lines "select latest_unit_slug from content_titles t join content_source_links l on l.title_id=t.id and l.source_key='manhwaindo' where t.latest_unit_slug is not null and t.latest_unit_slug<>'' order by t.updated_at desc nulls last limit ${MANHWA_RECENT_CHAPTER_LIMIT};"
)
for slug in "${manhwa_latest_chapter_slugs[@]}"; do
  [[ -z "${slug}" ]] && continue
  safe_run "manhwaindo:chapter:${slug}" sync-manhwa-chapter "${slug}" || true
done

for page in $(seq 1 "${KOMIKU_CATALOG_PAGES}"); do
  safe_run "komiku:catalog:p${page}" sync-komiku-catalog "${page}" || true
done

mapfile -t komiku_series_slugs < <(
  query_lines "select source_slug from content_source_links where source_key='komiku' order by last_scraped_at desc nulls last limit ${KOMIKU_RECENT_SERIES_LIMIT};"
)
for slug in "${komiku_series_slugs[@]}"; do
  [[ -z "${slug}" ]] && continue
  safe_run "komiku:series:${slug}" sync-komiku-series "${slug}" || true
done

mapfile -t komiku_latest_chapter_slugs < <(
  query_lines "select latest_unit_slug from content_titles t join content_source_links l on l.title_id=t.id and l.source_key='komiku' where t.latest_unit_slug is not null and t.latest_unit_slug<>'' order by t.updated_at desc nulls last limit ${KOMIKU_RECENT_CHAPTER_LIMIT};"
)
for slug in "${komiku_latest_chapter_slugs[@]}"; do
  [[ -z "${slug}" ]] && continue
  safe_run "komiku:chapter:${slug}" sync-komiku-chapter "${slug}" || true
done

safe_run "movie:kanata-home" sync-movie-v3-kanata-home "${MOVIE_HOME_LIMIT}" || true

echo "[$(now_utc)] done cron-media-latest"
