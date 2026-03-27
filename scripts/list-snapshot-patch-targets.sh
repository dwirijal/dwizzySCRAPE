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
  echo "[list-snapshot-patch-targets] missing DB DSN (POSTGRES_URL/DATABASE_URL; NEON_DATABASE_URL is compatibility fallback)" >&2
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
    l.slug,
    GREATEST(l.updated_at, COALESCE(MAX(e.updated_at), l.updated_at)) AS touched_at
  FROM anime_list l
  LEFT JOIN anime_episodes e ON e.anime_slug = l.slug
  GROUP BY l.slug, l.updated_at
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
    m.slug,
    GREATEST(
      m.updated_at,
      COALESCE(mm.updated_at, m.updated_at),
      COALESCE(MAX(pr.updated_at), m.updated_at),
      COALESCE(MAX(wo.updated_at), m.updated_at),
      COALESCE(MAX(d.updated_at), m.updated_at)
    ) AS touched_at
  FROM movies m
  LEFT JOIN movie_meta mm ON mm.tmdb_id = m.tmdb_id
  LEFT JOIN movie_provider_records pr ON pr.tmdb_id = m.tmdb_id
  LEFT JOIN movie_watch_options wo ON wo.provider_record_id = pr.id
  LEFT JOIN movie_download_options d ON d.provider_record_id = pr.id
  GROUP BY m.slug, m.updated_at, mm.updated_at
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
    l.source_key AS domain,
    l.source_slug AS slug,
    GREATEST(
      t.updated_at,
      COALESCE(MAX(u.updated_at), t.updated_at),
      COALESCE(MAX(l.last_scraped_at), t.updated_at)
    ) AS touched_at
  FROM content_titles t
  JOIN content_source_links l
    ON l.title_id = t.id
   AND l.source_key IN ('manhwaindo', 'komiku')
  LEFT JOIN content_units u ON u.title_id = t.id
  WHERE t.media_type IN ('manga', 'manhwa', 'manhua')
  GROUP BY l.source_key, l.source_slug, t.updated_at
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
    t.slug,
    GREATEST(t.updated_at, COALESCE(MAX(u.updated_at), t.updated_at)) AS touched_at
  FROM content_titles t
  LEFT JOIN content_units u
    ON u.title_id = t.id
   AND u.unit_type = 'episode'
  WHERE t.media_type = 'donghua'
  GROUP BY t.slug, t.updated_at
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
