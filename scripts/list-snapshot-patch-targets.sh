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
  echo "[list-snapshot-patch-targets] missing DB DSN (DATABASE_URL; POSTGRES_URL/NEON_DATABASE_URL are compatibility fallbacks)" >&2
  exit 1
fi

LOOKBACK_MINUTES="${SNAPSHOT_PATCH_LOOKBACK_MINUTES:-45}"
MAX_PER_DOMAIN="${SNAPSHOT_PATCH_MAX_PER_DOMAIN:-8}"

psql "${DB_DSN}" -At -F $'\t' -v ON_ERROR_STOP=1 \
  -v lookback="${LOOKBACK_MINUTES}" \
  -v max_per_domain="${MAX_PER_DOMAIN}" <<'SQL'
WITH recent_anime AS (
  SELECT
    'anime' AS domain,
    i.slug,
    GREATEST(i.updated_at, COALESCE(MAX(u.updated_at), i.updated_at)) AS touched_at
  FROM media_items i
  LEFT JOIN media_units u
    ON u.item_key = i.item_key
   AND u.unit_type = 'episode'
  WHERE i.source = 'samehadaku'
    AND i.media_type = 'anime'
  GROUP BY i.slug, i.updated_at
),
ranked_anime AS (
  SELECT
    domain,
    slug,
    touched_at,
    row_number() OVER (PARTITION BY domain ORDER BY touched_at DESC, slug ASC) AS rn
  FROM recent_anime
  WHERE touched_at >= NOW() - ((:'lookback')::int * INTERVAL '1 minute')
),
recent_movies AS (
  SELECT
    'movie' AS domain,
    i.slug,
    GREATEST(i.updated_at, COALESCE(MAX(u.updated_at), i.updated_at)) AS touched_at
  FROM media_items i
  LEFT JOIN media_units u
    ON u.item_key = i.item_key
  WHERE i.media_type = 'movie'
  GROUP BY i.slug, i.updated_at
),
ranked_movies AS (
  SELECT
    domain,
    slug,
    touched_at,
    row_number() OVER (PARTITION BY domain ORDER BY touched_at DESC, slug ASC) AS rn
  FROM recent_movies
  WHERE touched_at >= NOW() - ((:'lookback')::int * INTERVAL '1 minute')
),
recent_reading AS (
  SELECT
    i.source AS domain,
    i.slug AS slug,
    GREATEST(i.updated_at, COALESCE(MAX(u.updated_at), i.updated_at)) AS touched_at
  FROM media_items i
  LEFT JOIN media_units u
    ON u.item_key = i.item_key
   AND u.unit_type = 'chapter'
  WHERE i.source IN ('manhwaindo', 'komiku')
  GROUP BY i.source, i.slug, i.updated_at
),
ranked_reading AS (
  SELECT
    domain,
    slug,
    touched_at,
    row_number() OVER (PARTITION BY domain ORDER BY touched_at DESC, slug ASC) AS rn
  FROM recent_reading
  WHERE touched_at >= NOW() - ((:'lookback')::int * INTERVAL '1 minute')
),
recent_donghua AS (
  SELECT
    'donghua' AS domain,
    i.slug,
    GREATEST(i.updated_at, COALESCE(MAX(u.updated_at), i.updated_at)) AS touched_at
  FROM media_items i
  LEFT JOIN media_units u
    ON u.item_key = i.item_key
   AND u.unit_type = 'episode'
  WHERE i.media_type = 'donghua'
  GROUP BY i.slug, i.updated_at
),
ranked_donghua AS (
  SELECT
    domain,
    slug,
    touched_at,
    row_number() OVER (PARTITION BY domain ORDER BY touched_at DESC, slug ASC) AS rn
  FROM recent_donghua
  WHERE touched_at >= NOW() - ((:'lookback')::int * INTERVAL '1 minute')
)
SELECT
  domain,
  slug,
  to_char(touched_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS touched_at
FROM (
  SELECT domain, slug, touched_at, rn FROM ranked_anime
  UNION ALL
  SELECT domain, slug, touched_at, rn FROM ranked_movies
  UNION ALL
  SELECT domain, slug, touched_at, rn FROM ranked_reading
  UNION ALL
  SELECT domain, slug, touched_at, rn FROM ranked_donghua
) targets
WHERE rn <= (:'max_per_domain')::int
ORDER BY touched_at DESC, domain ASC, slug ASC;
SQL
