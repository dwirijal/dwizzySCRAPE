CREATE OR REPLACE VIEW public.anime_catalog_sync_v2_view AS
SELECT
    'samehadaku'::text AS source,
    'v2.samehadaku.how'::text AS source_domain,
    'anime'::text AS content_type,
    a.title,
    'https://v2.samehadaku.how/anime/' || a.slug || '/' AS canonical_url,
    a.slug,
    0::integer AS page_number,
    CASE
        WHEN a.poster_path = '' THEN ''
        WHEN a.poster_path LIKE '/images/%' THEN 'https://cdn.myanimelist.net' || a.poster_path
        WHEN a.poster_path LIKE '/%' THEN 'https://v2.samehadaku.how' || a.poster_path
        ELSE a.poster_path
    END AS poster_url,
    CASE a.type_code
        WHEN 't' THEN 'TV'
        WHEN 'm' THEN 'Movie'
        WHEN 'o' THEN 'OVA'
        WHEN 'n' THEN 'ONA'
        WHEN 'p' THEN 'Special'
        ELSE ''
    END AS anime_type,
    CASE a.status_code
        WHEN 'a' THEN 'Ongoing'
        WHEN 'f' THEN 'Completed'
        WHEN 'u' THEN 'Upcoming'
        ELSE ''
    END AS status,
    a.score,
    0::bigint AS views,
    a.synopsis_source AS synopsis_excerpt,
    ARRAY(
        SELECT gd.name
        FROM public.genre_dim gd
        WHERE gd.code = ANY(COALESCE(a.genre_codes, '{}'::smallint[]))
        ORDER BY gd.code
    )::text[] AS genres,
    a.updated_at AS scraped_at
FROM public.anime_list a;

CREATE OR REPLACE VIEW public.anime_detail_ready_v2_view AS
SELECT a.slug
FROM public.anime_list a
LEFT JOIN public.anime_meta m
    ON m.slug = a.slug
WHERE a.mal_id IS NOT NULL
   OR a.season_code <> 'x'
   OR COALESCE(array_length(a.studio_codes, 1), 0) > 0
   OR COALESCE(m.trailer_youtube_id, '') <> ''
   OR COALESCE(m.cast_json, '[]'::jsonb) <> '[]'::jsonb;

CREATE OR REPLACE VIEW public.anime_stream_ready_v2_view AS
SELECT DISTINCT e.anime_slug
FROM public.anime_episodes e
WHERE COALESCE(e.stream_links_json->>'primary', '') <> ''
   OR (
       jsonb_typeof(e.stream_links_json->'mirrors') = 'object'
       AND EXISTS (
           SELECT 1
           FROM jsonb_each(e.stream_links_json->'mirrors')
       )
   );

CREATE OR REPLACE FUNCTION public.upsert_samehadaku_catalog_v2(payload jsonb)
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    normalized_payload jsonb;
    affected integer := 0;
BEGIN
    normalized_payload := CASE
        WHEN payload IS NULL THEN '[]'::jsonb
        WHEN jsonb_typeof(payload) = 'array' THEN payload
        WHEN jsonb_typeof(payload) = 'object' THEN jsonb_build_array(payload)
        ELSE '[]'::jsonb
    END;

    IF jsonb_array_length(normalized_payload) = 0 THEN
        RETURN 0;
    END IF;

    WITH src AS (
        SELECT
            NULLIF(btrim(item.slug), '') AS slug,
            COALESCE(NULLIF(btrim(item.title), ''), NULLIF(btrim(item.slug), ''), '') AS title,
            COALESCE(NULLIF(public.media_strip_url_path(item.poster_url), ''), '') AS poster_path,
            public.media_anime_type_code(item.anime_type) AS type_code,
            public.media_anime_status_code(item.status) AS status_code,
            COALESCE(item.score::real, 0::real) AS score,
            public.media_ensure_genre_codes(item.genres) AS genre_codes,
            COALESCE(NULLIF(btrim(item.synopsis_excerpt), ''), '') AS synopsis_source,
            COALESCE(item.scraped_at, now()) AS updated_at
        FROM jsonb_to_recordset(normalized_payload) AS item(
            slug text,
            title text,
            poster_url text,
            anime_type text,
            status text,
            score double precision,
            genres text[],
            synopsis_excerpt text,
            scraped_at timestamptz
        )
        WHERE NULLIF(btrim(item.slug), '') IS NOT NULL
    )
    INSERT INTO public.anime_list (
        slug,
        source_code,
        meta_source_code,
        title,
        poster_path,
        type_code,
        status_code,
        score,
        genre_codes,
        synopsis_source,
        synopsis_enriched,
        updated_at
    )
    SELECT
        src.slug,
        's'::char(1),
        's'::char(1),
        src.title,
        src.poster_path,
        src.type_code,
        src.status_code,
        src.score,
        src.genre_codes,
        src.synopsis_source,
        src.synopsis_source,
        src.updated_at
    FROM src
    ON CONFLICT (slug) DO UPDATE
    SET
        source_code = 's'::char(1),
        title = CASE
            WHEN public.anime_list.meta_source_code = 'm'::char(1) AND public.anime_list.title <> '' THEN public.anime_list.title
            WHEN EXCLUDED.title <> '' THEN EXCLUDED.title
            ELSE public.anime_list.title
        END,
        poster_path = CASE
            WHEN public.anime_list.meta_source_code = 'm'::char(1) AND public.anime_list.poster_path <> '' THEN public.anime_list.poster_path
            WHEN EXCLUDED.poster_path <> '' THEN EXCLUDED.poster_path
            ELSE public.anime_list.poster_path
        END,
        type_code = CASE
            WHEN public.anime_list.type_code = 'u'::char(1) AND EXCLUDED.type_code <> 'u'::char(1) THEN EXCLUDED.type_code
            ELSE public.anime_list.type_code
        END,
        status_code = CASE
            WHEN public.anime_list.status_code = 'x'::char(1) AND EXCLUDED.status_code <> 'x'::char(1) THEN EXCLUDED.status_code
            ELSE public.anime_list.status_code
        END,
        score = CASE
            WHEN public.anime_list.score = 0 AND EXCLUDED.score <> 0 THEN EXCLUDED.score
            ELSE public.anime_list.score
        END,
        genre_codes = public.media_merge_smallint_arrays(public.anime_list.genre_codes, EXCLUDED.genre_codes),
        synopsis_source = CASE
            WHEN EXCLUDED.synopsis_source <> '' THEN EXCLUDED.synopsis_source
            ELSE public.anime_list.synopsis_source
        END,
        updated_at = GREATEST(public.anime_list.updated_at, EXCLUDED.updated_at);

    GET DIAGNOSTICS affected = ROW_COUNT;
    RETURN affected;
END;
$$;

CREATE OR REPLACE FUNCTION public.upsert_samehadaku_anime_detail_v2(payload jsonb)
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    normalized_payload jsonb;
    affected integer := 0;
BEGIN
    normalized_payload := CASE
        WHEN payload IS NULL THEN '[]'::jsonb
        WHEN jsonb_typeof(payload) = 'array' THEN payload
        WHEN jsonb_typeof(payload) = 'object' THEN jsonb_build_array(payload)
        ELSE '[]'::jsonb
    END;

    IF jsonb_array_length(normalized_payload) = 0 THEN
        RETURN 0;
    END IF;

    WITH src AS (
        SELECT
            NULLIF(btrim(item.slug), '') AS slug,
            CASE
                WHEN item.mal_id IS NULL OR item.mal_id = 0 THEN NULL
                ELSE item.mal_id::bigint
            END AS mal_id,
            COALESCE(
                NULLIF(item.jikan_meta_json #>> '{anime_full,title}', ''),
                NULLIF(btrim(item.source_title), ''),
                NULLIF(btrim(item.slug), ''),
                ''
            ) AS title,
            COALESCE(
                NULLIF(public.media_strip_url_path(COALESCE(
                    NULLIF(item.mal_thumbnail_url, ''),
                    NULLIF(item.jikan_meta_json #>> '{anime_full,images,webp,large_image_url}', ''),
                    NULLIF(item.jikan_meta_json #>> '{anime_full,images,jpg,large_image_url}', ''),
                    NULLIF(item.jikan_meta_json #>> '{anime_full,images,webp,image_url}', ''),
                    NULLIF(item.jikan_meta_json #>> '{anime_full,images,jpg,image_url}', '')
                )), ''),
                ''
            ) AS poster_path,
            public.media_anime_type_code(COALESCE(NULLIF(item.anime_type, ''), item.jikan_meta_json #>> '{anime_full,type}')) AS type_code,
            public.media_anime_status_code(COALESCE(NULLIF(item.status, ''), item.jikan_meta_json #>> '{anime_full,status}')) AS status_code,
            public.media_season_code(COALESCE(NULLIF(item.season, ''), item.jikan_meta_json #>> '{anime_full,season}')) AS season_code,
            public.media_parse_smallint(item.jikan_meta_json #>> '{anime_full,year}') AS year,
            COALESCE(public.media_parse_real(item.jikan_meta_json #>> '{anime_full,score}'), 0::real) AS score,
            public.media_parse_integer(item.jikan_meta_json #>> '{anime_full,episodes}') AS episode_count,
            public.media_ensure_genre_codes(item.genre_names) AS genre_codes,
            public.media_ensure_studio_codes(item.studio_names) AS studio_codes,
            COALESCE(NULLIF(btrim(item.synopsis_source), ''), '') AS synopsis_source,
            COALESCE(NULLIF(btrim(item.synopsis_enriched), ''), NULLIF(btrim(item.synopsis_source), ''), '') AS synopsis_enriched,
            COALESCE(item.batch_links_json, '{}'::jsonb) AS batch_links_json,
            COALESCE(
                NULLIF(public.media_extract_youtube_id(item.jikan_meta_json #>> '{anime_full,trailer,embed_url}'), ''),
                NULLIF(public.media_extract_youtube_id(item.jikan_meta_json #>> '{anime_full,trailer,url}'), ''),
                ''
            ) AS trailer_youtube_id,
            COALESCE(item.cast_json, '[]'::jsonb) AS cast_json,
            COALESCE(item.scraped_at, now()) AS updated_at
        FROM jsonb_to_recordset(normalized_payload) AS item(
            slug text,
            source_title text,
            mal_id integer,
            mal_thumbnail_url text,
            synopsis_source text,
            synopsis_enriched text,
            anime_type text,
            status text,
            season text,
            studio_names text[],
            genre_names text[],
            batch_links_json jsonb,
            cast_json jsonb,
            jikan_meta_json jsonb,
            scraped_at timestamptz
        )
        WHERE NULLIF(btrim(item.slug), '') IS NOT NULL
    )
    INSERT INTO public.anime_list (
        slug,
        source_code,
        meta_source_code,
        mal_id,
        title,
        poster_path,
        type_code,
        status_code,
        season_code,
        year,
        score,
        episode_count,
        genre_codes,
        studio_codes,
        synopsis_source,
        synopsis_enriched,
        batch_links_json,
        updated_at
    )
    SELECT
        src.slug,
        's'::char(1),
        CASE WHEN src.mal_id IS NOT NULL THEN 'm'::char(1) ELSE 's'::char(1) END,
        src.mal_id,
        src.title,
        src.poster_path,
        src.type_code,
        src.status_code,
        src.season_code,
        src.year,
        src.score,
        src.episode_count,
        src.genre_codes,
        src.studio_codes,
        src.synopsis_source,
        src.synopsis_enriched,
        src.batch_links_json,
        src.updated_at
    FROM src
    ON CONFLICT (slug) DO UPDATE
    SET
        source_code = 's'::char(1),
        meta_source_code = CASE
            WHEN EXCLUDED.mal_id IS NOT NULL THEN 'm'::char(1)
            ELSE public.anime_list.meta_source_code
        END,
        mal_id = COALESCE(EXCLUDED.mal_id, public.anime_list.mal_id),
        title = CASE
            WHEN EXCLUDED.title <> '' THEN EXCLUDED.title
            ELSE public.anime_list.title
        END,
        poster_path = CASE
            WHEN EXCLUDED.poster_path <> '' THEN EXCLUDED.poster_path
            ELSE public.anime_list.poster_path
        END,
        type_code = CASE
            WHEN EXCLUDED.type_code <> 'u'::char(1) THEN EXCLUDED.type_code
            ELSE public.anime_list.type_code
        END,
        status_code = CASE
            WHEN EXCLUDED.status_code <> 'x'::char(1) THEN EXCLUDED.status_code
            ELSE public.anime_list.status_code
        END,
        season_code = CASE
            WHEN EXCLUDED.season_code <> 'x'::char(1) THEN EXCLUDED.season_code
            ELSE public.anime_list.season_code
        END,
        year = COALESCE(EXCLUDED.year, public.anime_list.year),
        score = CASE
            WHEN EXCLUDED.score <> 0 THEN EXCLUDED.score
            ELSE public.anime_list.score
        END,
        episode_count = COALESCE(EXCLUDED.episode_count, public.anime_list.episode_count),
        genre_codes = public.media_merge_smallint_arrays(public.anime_list.genre_codes, EXCLUDED.genre_codes),
        studio_codes = public.media_merge_smallint_arrays(public.anime_list.studio_codes, EXCLUDED.studio_codes),
        synopsis_source = CASE
            WHEN EXCLUDED.synopsis_source <> '' THEN EXCLUDED.synopsis_source
            ELSE public.anime_list.synopsis_source
        END,
        synopsis_enriched = CASE
            WHEN EXCLUDED.synopsis_enriched <> '' THEN EXCLUDED.synopsis_enriched
            ELSE public.anime_list.synopsis_enriched
        END,
        batch_links_json = CASE
            WHEN EXCLUDED.batch_links_json <> '{}'::jsonb THEN EXCLUDED.batch_links_json
            ELSE public.anime_list.batch_links_json
        END,
        updated_at = GREATEST(public.anime_list.updated_at, EXCLUDED.updated_at);

    WITH src AS (
        SELECT
            NULLIF(btrim(item.slug), '') AS slug,
            COALESCE(
                NULLIF(public.media_extract_youtube_id(item.jikan_meta_json #>> '{anime_full,trailer,embed_url}'), ''),
                NULLIF(public.media_extract_youtube_id(item.jikan_meta_json #>> '{anime_full,trailer,url}'), ''),
                ''
            ) AS trailer_youtube_id,
            COALESCE(item.cast_json, '[]'::jsonb) AS cast_json,
            COALESCE(item.scraped_at, now()) AS updated_at
        FROM jsonb_to_recordset(normalized_payload) AS item(
            slug text,
            cast_json jsonb,
            jikan_meta_json jsonb,
            scraped_at timestamptz
        )
        WHERE NULLIF(btrim(item.slug), '') IS NOT NULL
    )
    INSERT INTO public.anime_meta (
        slug,
        trailer_youtube_id,
        cast_json,
        updated_at
    )
    SELECT
        src.slug,
        src.trailer_youtube_id,
        src.cast_json,
        src.updated_at
    FROM src
    ON CONFLICT (slug) DO UPDATE
    SET
        trailer_youtube_id = CASE
            WHEN EXCLUDED.trailer_youtube_id <> '' THEN EXCLUDED.trailer_youtube_id
            ELSE public.anime_meta.trailer_youtube_id
        END,
        cast_json = CASE
            WHEN EXCLUDED.cast_json <> '[]'::jsonb THEN EXCLUDED.cast_json
            ELSE public.anime_meta.cast_json
        END,
        updated_at = GREATEST(public.anime_meta.updated_at, EXCLUDED.updated_at);

    DELETE FROM public.anime_meta
    WHERE slug IN (
        SELECT NULLIF(btrim(item.slug), '')
        FROM jsonb_to_recordset(normalized_payload) AS item(slug text)
        WHERE NULLIF(btrim(item.slug), '') IS NOT NULL
    )
      AND trailer_youtube_id = ''
      AND cast_json = '[]'::jsonb;

    GET DIAGNOSTICS affected = ROW_COUNT;
    RETURN affected;
END;
$$;

CREATE OR REPLACE FUNCTION public.upsert_samehadaku_episode_v2(payload jsonb)
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    normalized_payload jsonb;
    affected integer := 0;
BEGIN
    normalized_payload := CASE
        WHEN payload IS NULL THEN '[]'::jsonb
        WHEN jsonb_typeof(payload) = 'array' THEN payload
        WHEN jsonb_typeof(payload) = 'object' THEN jsonb_build_array(payload)
        ELSE '[]'::jsonb
    END;

    IF jsonb_array_length(normalized_payload) = 0 THEN
        RETURN 0;
    END IF;

    WITH src AS (
        SELECT
            NULLIF(btrim(item.anime_slug), '') AS anime_slug,
            COALESCE(item.scraped_at, now()) AS updated_at
        FROM jsonb_to_recordset(normalized_payload) AS item(
            anime_slug text,
            scraped_at timestamptz
        )
        WHERE NULLIF(btrim(item.anime_slug), '') IS NOT NULL
    )
    INSERT INTO public.anime_list (
        slug,
        source_code,
        meta_source_code,
        title,
        updated_at
    )
    SELECT DISTINCT
        src.anime_slug,
        's'::char(1),
        's'::char(1),
        src.anime_slug,
        src.updated_at
    FROM src
    ON CONFLICT (slug) DO NOTHING;

    WITH src AS (
        SELECT
            NULLIF(btrim(item.anime_slug), '') AS anime_slug,
            NULLIF(btrim(item.episode_slug), '') AS episode_slug,
            COALESCE(item.episode_number::real, 0::real) AS episode_number,
            COALESCE(NULLIF(btrim(item.title), ''), NULLIF(btrim(item.episode_slug), ''), '') AS title,
            public.media_parse_timestamptz(item.source_meta_json->>'published_at') AS release_at,
            COALESCE(NULLIF(btrim(item.release_label), ''), '') AS release_label,
            public.media_episode_fetch_status_code(item.fetch_status, item.fetch_error, item.effective_source_kind) AS fetch_status_code,
            COALESCE(item.stream_links_json, '{}'::jsonb) AS stream_links_json,
            COALESCE(item.download_links_json, '{}'::jsonb) AS download_links_json,
            COALESCE(item.scraped_at, now()) AS updated_at
        FROM jsonb_to_recordset(normalized_payload) AS item(
            anime_slug text,
            episode_slug text,
            title text,
            episode_number double precision,
            release_label text,
            effective_source_kind text,
            stream_links_json jsonb,
            download_links_json jsonb,
            source_meta_json jsonb,
            fetch_status text,
            fetch_error text,
            scraped_at timestamptz
        )
        WHERE NULLIF(btrim(item.anime_slug), '') IS NOT NULL
          AND NULLIF(btrim(item.episode_slug), '') IS NOT NULL
    )
    INSERT INTO public.anime_episodes (
        episode_slug,
        anime_slug,
        episode_number,
        title,
        release_at,
        release_label,
        fetch_status_code,
        stream_links_json,
        download_links_json,
        updated_at
    )
    SELECT
        src.episode_slug,
        src.anime_slug,
        src.episode_number,
        src.title,
        src.release_at,
        src.release_label,
        src.fetch_status_code,
        src.stream_links_json,
        src.download_links_json,
        src.updated_at
    FROM src
    ON CONFLICT (episode_slug) DO UPDATE
    SET
        anime_slug = EXCLUDED.anime_slug,
        episode_number = EXCLUDED.episode_number,
        title = EXCLUDED.title,
        release_at = COALESCE(EXCLUDED.release_at, public.anime_episodes.release_at),
        release_label = CASE
            WHEN EXCLUDED.release_label <> '' THEN EXCLUDED.release_label
            ELSE public.anime_episodes.release_label
        END,
        fetch_status_code = EXCLUDED.fetch_status_code,
        stream_links_json = EXCLUDED.stream_links_json,
        download_links_json = EXCLUDED.download_links_json,
        updated_at = GREATEST(public.anime_episodes.updated_at, EXCLUDED.updated_at);

    GET DIAGNOSTICS affected = ROW_COUNT;
    RETURN affected;
END;
$$;
