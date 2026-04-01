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
  echo "[monitor-cron-jobs] missing DB DSN (DATABASE_URL; POSTGRES_URL/NEON_DATABASE_URL are compatibility fallbacks)" >&2
  exit 1
fi

INTERVAL_SECONDS="${MONITOR_INTERVAL_SECONDS:-60}"
MONITOR_ONCE="${MONITOR_ONCE:-0}"
LOG_DIR="${DWIZZY_LOG_DIR:-${PROJECT_DIR}/logs}"

render_log_stamp() {
  local file_path="$1"
  if [[ ! -f "${file_path}" ]]; then
    echo "-"
    return 0
  fi
  stat -c '%y' "${file_path}" 2>/dev/null | cut -d'.' -f1 || echo "-"
}

render_snapshot() {
  clear
  echo "monitor-cron-jobs"
  echo "time_utc=$(date -u '+%Y-%m-%d %H:%M:%S UTC')"
  echo "interval_seconds=${INTERVAL_SECONDS}"
  echo
  echo "active_processes"
  pgrep -af 'cron-release-anime.sh|cron-release-comics.sh|samehadaku-backfill|anichin-backfill|manhwaindo-backfill|bacaman-backfill|komiku|mangasusuku-backfill|kanzenin-backfill' || echo "(none)"
  echo
  echo "log_files"
  echo "cron-release-anime.log $(render_log_stamp "${LOG_DIR}/cron-release-anime.log")"
  echo "cron-release-comics.log $(render_log_stamp "${LOG_DIR}/cron-release-comics.log")"
  echo
  echo "db_freshness"
  psql "${DB_DSN}" -P pager=off -F $'\t' -At <<'SQL'
WITH item_stats AS (
  SELECT
    source,
    count(*) AS item_count,
    max(updated_at) AS latest_item_update
  FROM public.media_items
  GROUP BY source
),
unit_stats AS (
  SELECT
    source,
    count(*) FILTER (WHERE unit_type = 'episode') AS episode_count,
    count(*) FILTER (WHERE unit_type = 'chapter') AS chapter_count,
    max(updated_at) AS latest_unit_update
  FROM public.media_units
  GROUP BY source
),
merged AS (
  SELECT
    coalesce(i.source, u.source) AS source,
    coalesce(i.item_count, 0) AS items,
    coalesce(u.episode_count, 0) AS episodes,
    coalesce(u.chapter_count, 0) AS chapters,
    i.latest_item_update,
    u.latest_unit_update,
    greatest(
      coalesce(i.latest_item_update, to_timestamp(0)),
      coalesce(u.latest_unit_update, to_timestamp(0))
    ) AS latest_touch
  FROM item_stats i
  FULL OUTER JOIN unit_stats u ON u.source = i.source
)
SELECT
  source,
  items,
  episodes,
  chapters,
  coalesce(to_char(latest_touch, 'YYYY-MM-DD HH24:MI:SS'), '-') AS latest_touch_utc,
  floor(extract(epoch FROM (now() - latest_touch)) / 60)::bigint AS age_minutes,
  CASE
    WHEN latest_touch >= now() - interval '4 hours' THEN 'fresh'
    WHEN latest_touch >= now() - interval '8 hours' THEN 'stale'
    ELSE 'late'
  END AS status
FROM merged
ORDER BY source;
SQL
  echo
  echo "last_log_tail"
  if [[ -f "${LOG_DIR}/cron-release-anime.log" ]]; then
    echo "[anime]"
    tail -n 5 "${LOG_DIR}/cron-release-anime.log"
  else
    echo "[anime] (missing)"
  fi
  if [[ -f "${LOG_DIR}/cron-release-comics.log" ]]; then
    echo "[comics]"
    tail -n 5 "${LOG_DIR}/cron-release-comics.log"
  else
    echo "[comics] (missing)"
  fi
}

while true; do
  render_snapshot
  if [[ "${MONITOR_ONCE}" == "1" ]]; then
    break
  fi
  sleep "${INTERVAL_SECONDS}"
done
