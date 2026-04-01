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
  echo "[verify-backfill-data] missing DB DSN (DATABASE_URL; POSTGRES_URL/NEON_DATABASE_URL are compatibility fallbacks)" >&2
  exit 1
fi

SAMEHADAKU_EXCEPTION_FILE="${PROJECT_DIR}/config/samehadaku-episode-gap-exceptions.txt"
INTERVAL_SECONDS="${VERIFY_INTERVAL_SECONDS:-30}"

render_samehadaku_exception_values() {
  if [[ ! -f "${SAMEHADAKU_EXCEPTION_FILE}" ]]; then
    return 0
  fi

  awk -F '\t' '
    /^[[:space:]]*$/ { next }
    /^[[:space:]]*#/ { next }
    {
      value = $1
      gsub(/\047/, "\047\047", value)
      printf "%s(\047%s\047)", sep, value
      sep = ","
    }
  ' "${SAMEHADAKU_EXCEPTION_FILE}"
}

render_samehadaku_exception_summary() {
  if [[ ! -f "${SAMEHADAKU_EXCEPTION_FILE}" ]]; then
    echo "(none)"
    return 0
  fi

  awk -F '\t' '
    /^[[:space:]]*$/ { next }
    /^[[:space:]]*#/ { next }
    { printf "%s\t%s\n", $1, $2 }
  ' "${SAMEHADAKU_EXCEPTION_FILE}"
}

run_snapshot() {
  local samehadaku_exception_values
  samehadaku_exception_values="$(render_samehadaku_exception_values)"
  if [[ -z "${samehadaku_exception_values}" ]]; then
    samehadaku_exception_values="('__none__')"
  fi

  clear
  echo "verify-backfill-data"
  echo "time_utc=$(date -u '+%Y-%m-%d %H:%M:%S UTC')"
  echo "interval_seconds=${INTERVAL_SECONDS}"
  echo
  echo "tmux_sessions"
  if ! tmux ls 2>/dev/null; then
    echo "(none)"
  fi
  echo
  echo "db_snapshot"
  psql "${DB_DSN}" -P pager=off -F $'\t' -At <<'SQL'
WITH item_stats AS (
  SELECT
    source,
    count(*) AS item_count,
    count(*) FILTER (
      WHERE coalesce(detail->>'primary_source_url', '') <> ''
    ) AS detail_ready_count,
    max(updated_at) AS latest_item_update
  FROM public.media_items
  GROUP BY source
),
unit_stats AS (
  SELECT
    source,
    count(*) FILTER (WHERE unit_type = 'episode') AS episode_count,
    count(*) FILTER (WHERE unit_type = 'chapter') AS chapter_count,
    count(DISTINCT detail->>'anime_slug') FILTER (
      WHERE unit_type = 'episode'
        AND coalesce(detail->>'anime_slug', '') <> ''
    ) AS episode_anime_count,
    count(DISTINCT detail->>'series_slug') FILTER (
      WHERE unit_type = 'chapter'
        AND coalesce(detail->>'series_slug', '') <> ''
    ) AS chapter_series_count,
    max(updated_at) AS latest_unit_update
  FROM public.media_units
  GROUP BY source
)
SELECT
  coalesce(i.source, u.source) AS source,
  coalesce(i.item_count, 0) AS items,
  coalesce(i.detail_ready_count, 0) AS detail_ready,
  coalesce(u.episode_anime_count, 0) AS episode_anime,
  coalesce(u.episode_count, 0) AS episodes,
  coalesce(u.chapter_series_count, 0) AS chapter_series,
  coalesce(u.chapter_count, 0) AS chapters,
  coalesce(to_char(i.latest_item_update, 'YYYY-MM-DD HH24:MI:SS'), '-') AS latest_item_utc,
  coalesce(to_char(u.latest_unit_update, 'YYYY-MM-DD HH24:MI:SS'), '-') AS latest_unit_utc
FROM item_stats i
FULL OUTER JOIN unit_stats u ON u.source = i.source
ORDER BY 1;
SQL
  echo
  echo "samehadaku_gap"
  psql "${DB_DSN}" -P pager=off -F $'\t' -At <<SQL
WITH exceptions(slug) AS (
  VALUES ${samehadaku_exception_values}
),
detail_ready AS (
  SELECT slug
  FROM public.media_items
  WHERE source = 'samehadaku'
    AND coalesce(detail->>'primary_source_url', '') <> ''
),
episode_ready AS (
  SELECT DISTINCT detail->>'anime_slug' AS slug
  FROM public.media_units
  WHERE source = 'samehadaku'
    AND unit_type = 'episode'
    AND coalesce(detail->>'anime_slug', '') <> ''
),
pending AS (
  SELECT d.slug
  FROM detail_ready d
  LEFT JOIN episode_ready e ON e.slug = d.slug
  LEFT JOIN exceptions x ON x.slug = d.slug
  WHERE e.slug IS NULL
    AND x.slug IS NULL
)
SELECT
  (SELECT count(*) FROM detail_ready) AS detail_ready,
  (SELECT count(*) FROM episode_ready) AS episode_anime,
  (SELECT count(*) FROM exceptions WHERE slug <> '__none__') AS exception_count,
  (SELECT count(*) FROM pending) AS pending_anime_without_episode_rows;
SQL
  echo
  echo "samehadaku_episode_exceptions"
  render_samehadaku_exception_summary
}

cd "${PROJECT_DIR}"
while true; do
  run_snapshot
  sleep "${INTERVAL_SECONDS}"
done
