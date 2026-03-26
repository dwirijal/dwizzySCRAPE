CREATE TABLE IF NOT EXISTS public.genre_dim (
    code smallserial PRIMARY KEY,
    name text NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS public.country_dim (
    code smallserial PRIMARY KEY,
    name text NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS public.studio_dim (
    code smallserial PRIMARY KEY,
    name text NOT NULL UNIQUE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_genre_dim_name_unique
    ON public.genre_dim (name);

CREATE UNIQUE INDEX IF NOT EXISTS idx_country_dim_name_unique
    ON public.country_dim (name);

CREATE UNIQUE INDEX IF NOT EXISTS idx_studio_dim_name_unique
    ON public.studio_dim (name);

DO $$
DECLARE
    next_code bigint;
BEGIN
    CREATE SEQUENCE IF NOT EXISTS public.genre_dim_code_seq AS smallint;
    SELECT COALESCE(MAX(code), 0) + 1 INTO next_code FROM public.genre_dim;
    PERFORM setval('public.genre_dim_code_seq', next_code, false);
    ALTER SEQUENCE public.genre_dim_code_seq OWNED BY public.genre_dim.code;
    ALTER TABLE public.genre_dim
        ALTER COLUMN code SET DEFAULT nextval('public.genre_dim_code_seq');
END;
$$;

DO $$
DECLARE
    next_code bigint;
BEGIN
    CREATE SEQUENCE IF NOT EXISTS public.country_dim_code_seq AS smallint;
    SELECT COALESCE(MAX(code), 0) + 1 INTO next_code FROM public.country_dim;
    PERFORM setval('public.country_dim_code_seq', next_code, false);
    ALTER SEQUENCE public.country_dim_code_seq OWNED BY public.country_dim.code;
    ALTER TABLE public.country_dim
        ALTER COLUMN code SET DEFAULT nextval('public.country_dim_code_seq');
END;
$$;

DO $$
DECLARE
    next_code bigint;
BEGIN
    CREATE SEQUENCE IF NOT EXISTS public.studio_dim_code_seq AS smallint;
    SELECT COALESCE(MAX(code), 0) + 1 INTO next_code FROM public.studio_dim;
    PERFORM setval('public.studio_dim_code_seq', next_code, false);
    ALTER SEQUENCE public.studio_dim_code_seq OWNED BY public.studio_dim.code;
    ALTER TABLE public.studio_dim
        ALTER COLUMN code SET DEFAULT nextval('public.studio_dim_code_seq');
END;
$$;

CREATE OR REPLACE FUNCTION public.media_strip_url_path(raw text)
RETURNS text
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN raw IS NULL OR btrim(raw) = '' THEN ''
        WHEN raw ~* '^https?://' THEN COALESCE(substring(raw FROM 'https?://[^/]+(/.*)$'), '')
        ELSE raw
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_extract_youtube_id(raw text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
    matches text[];
BEGIN
    IF raw IS NULL OR btrim(raw) = '' THEN
        RETURN '';
    END IF;

    IF raw ~* 'youtu\.be/' THEN
        matches := regexp_match(raw, 'youtu\.be/([A-Za-z0-9_-]{6,})');
    ELSIF raw ~* '[?&]v=' THEN
        matches := regexp_match(raw, '[?&]v=([A-Za-z0-9_-]{6,})');
    ELSIF raw ~* '/embed/' THEN
        matches := regexp_match(raw, '/embed/([A-Za-z0-9_-]{6,})');
    END IF;

    RETURN COALESCE(matches[1], '');
END;
$$;

CREATE OR REPLACE FUNCTION public.media_parse_smallint(raw text)
RETURNS smallint
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN raw IS NULL OR btrim(raw) = '' THEN NULL
        WHEN btrim(raw) ~ '^[0-9]{1,5}$' THEN btrim(raw)::smallint
        ELSE NULL
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_parse_integer(raw text)
RETURNS integer
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN raw IS NULL OR btrim(raw) = '' THEN NULL
        WHEN btrim(raw) ~ '^[0-9]{1,10}$' THEN btrim(raw)::integer
        ELSE NULL
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_parse_timestamptz(raw text)
RETURNS timestamptz
LANGUAGE plpgsql
IMMUTABLE
AS $$
BEGIN
    IF raw IS NULL OR btrim(raw) = '' THEN
        RETURN NULL;
    END IF;
    RETURN btrim(raw)::timestamptz;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION public.media_parse_real(raw text)
RETURNS real
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN raw IS NULL OR btrim(raw) = '' THEN NULL
        WHEN btrim(raw) ~ '^[0-9]+(\.[0-9]+)?$' THEN btrim(raw)::real
        ELSE NULL
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_parse_duration_minutes(raw text)
RETURNS integer
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
    normalized text;
    hour_match text[];
    minute_match text[];
    total integer := 0;
BEGIN
    IF raw IS NULL OR btrim(raw) = '' THEN
        RETURN NULL;
    END IF;

    normalized := lower(btrim(raw));

    IF normalized ~ '^[0-9]+\s*min$' THEN
        RETURN regexp_replace(normalized, '\s*min$', '')::integer;
    END IF;

    hour_match := regexp_match(normalized, '([0-9]+)\s*h');
    minute_match := regexp_match(normalized, '([0-9]+)\s*m');

    IF hour_match[1] IS NOT NULL THEN
        total := total + (hour_match[1]::integer * 60);
    END IF;
    IF minute_match[1] IS NOT NULL THEN
        total := total + minute_match[1]::integer;
    END IF;

    IF total = 0 THEN
        RETURN NULL;
    END IF;
    RETURN total;
END;
$$;

CREATE OR REPLACE FUNCTION public.media_anime_type_code(raw text)
RETURNS char(1)
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE lower(btrim(COALESCE(raw, '')))
        WHEN 'tv' THEN 't'::char(1)
        WHEN 'movie' THEN 'm'::char(1)
        WHEN 'ova' THEN 'o'::char(1)
        WHEN 'ona' THEN 'n'::char(1)
        WHEN 'special' THEN 'p'::char(1)
        ELSE 'u'::char(1)
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_anime_status_code(raw text)
RETURNS char(1)
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN lower(btrim(COALESCE(raw, ''))) IN ('ongoing', 'currently airing', 'airing') THEN 'a'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) IN ('completed', 'finished airing') THEN 'f'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) LIKE '%upcoming%' THEN 'u'::char(1)
        ELSE 'x'::char(1)
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_season_code(raw text)
RETURNS char(1)
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE lower(btrim(COALESCE(raw, '')))
        WHEN 'winter' THEN 'w'::char(1)
        WHEN 'spring' THEN 'p'::char(1)
        WHEN 'summer' THEN 's'::char(1)
        WHEN 'fall' THEN 'f'::char(1)
        WHEN 'autumn' THEN 'f'::char(1)
        ELSE 'x'::char(1)
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_movie_quality_code(raw text)
RETURNS char(1)
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN lower(btrim(COALESCE(raw, ''))) LIKE '%bluray%' THEN 'b'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) LIKE '%web%' THEN 'w'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) LIKE '%dvd%' THEN 'd'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) IN ('1080p', '720p', 'fullhd', 'mp4hd') THEN 'h'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) = '480p' THEN 'm'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) = '360p' THEN 'l'::char(1)
        ELSE 'u'::char(1)
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_movie_status_code(raw text)
RETURNS char(1)
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE lower(btrim(COALESCE(raw, '')))
        WHEN 'released' THEN 'r'::char(1)
        WHEN 'returning series' THEN 'r'::char(1)
        WHEN 'post production' THEN 'p'::char(1)
        WHEN 'in production' THEN 'p'::char(1)
        WHEN 'planned' THEN 'u'::char(1)
        WHEN 'rumored' THEN 'u'::char(1)
        ELSE 'x'::char(1)
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_movie_format_code(raw text)
RETURNS char(1)
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN lower(btrim(COALESCE(raw, ''))) LIKE '%x265%' THEN 'x'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) LIKE '%h265%' THEN 'x'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) LIKE '%hevc%' THEN 'x'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) LIKE '%mp4%' THEN 'm'::char(1)
        WHEN lower(btrim(COALESCE(raw, ''))) LIKE '%mkv%' THEN 'k'::char(1)
        ELSE 'u'::char(1)
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_movie_option_status_code(raw text)
RETURNS char(1)
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN lower(COALESCE(raw, '')) IN ('active', 'ready', 'verified') THEN 'a'::char(1)
        WHEN lower(COALESCE(raw, '')) LIKE '%block%' THEN 'b'::char(1)
        WHEN lower(COALESCE(raw, '')) LIKE '%dead%' OR lower(COALESCE(raw, '')) LIKE '%removed%' THEN 'd'::char(1)
        ELSE 'x'::char(1)
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_episode_fetch_status_code(fetch_status text, fetch_error text, effective_kind text)
RETURNS char(1)
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN lower(btrim(COALESCE(effective_kind, ''))) = 'secondary' THEN 's'::char(1)
        WHEN lower(COALESCE(fetch_status, '')) LIKE '%block%' OR lower(COALESCE(fetch_error, '')) LIKE '%challenge%' THEN 'b'::char(1)
        WHEN lower(COALESCE(fetch_status, '')) LIKE '%primary_fetched%' THEN 'p'::char(1)
        ELSE 'x'::char(1)
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_movie_download_status_code(fetch_status text, fetch_error text, payload jsonb)
RETURNS char(1)
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT CASE
        WHEN COALESCE((payload->>'resolved_count')::integer, 0) > 0 THEN 'r'::char(1)
        WHEN jsonb_typeof(payload->'links') = 'array' AND jsonb_array_length(payload->'links') > 0 THEN 'r'::char(1)
        WHEN lower(COALESCE(fetch_status, '')) LIKE '%block%' OR lower(COALESCE(fetch_error, '')) LIKE '%challenge%' OR COALESCE(payload->>'http_status', '') = '403' THEN 'b'::char(1)
        WHEN btrim(COALESCE(fetch_status, '')) <> '' THEN 'p'::char(1)
        ELSE 'x'::char(1)
    END;
$$;

CREATE OR REPLACE FUNCTION public.media_genre_codes(names text[])
RETURNS smallint[]
LANGUAGE sql
STABLE
AS $$
    SELECT COALESCE(
        ARRAY(
            SELECT gd.code
            FROM public.genre_dim gd
            WHERE gd.name = ANY(COALESCE(names, '{}'::text[]))
            ORDER BY gd.code
        ),
        '{}'::smallint[]
    );
$$;

CREATE OR REPLACE FUNCTION public.media_studio_codes(names text[])
RETURNS smallint[]
LANGUAGE sql
STABLE
AS $$
    SELECT COALESCE(
        ARRAY(
            SELECT sd.code
            FROM public.studio_dim sd
            WHERE sd.name = ANY(COALESCE(names, '{}'::text[]))
            ORDER BY sd.code
        ),
        '{}'::smallint[]
    );
$$;

CREATE OR REPLACE FUNCTION public.media_country_codes(names text[])
RETURNS smallint[]
LANGUAGE sql
STABLE
AS $$
    SELECT COALESCE(
        ARRAY(
            SELECT cd.code
            FROM public.country_dim cd
            WHERE cd.name = ANY(COALESCE(names, '{}'::text[]))
            ORDER BY cd.code
        ),
        '{}'::smallint[]
    );
$$;

CREATE OR REPLACE FUNCTION public.media_distinct_trimmed_texts(names text[])
RETURNS text[]
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT COALESCE(
        ARRAY(
            SELECT DISTINCT cleaned.name
            FROM (
                SELECT NULLIF(btrim(value), '') AS name
                FROM unnest(COALESCE(names, '{}'::text[])) AS value
            ) cleaned
            WHERE cleaned.name IS NOT NULL
            ORDER BY cleaned.name
        ),
        '{}'::text[]
    );
$$;

CREATE OR REPLACE FUNCTION public.media_merge_smallint_arrays(current_values smallint[], incoming_values smallint[])
RETURNS smallint[]
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT COALESCE(
        ARRAY(
            SELECT DISTINCT code
            FROM unnest(COALESCE(current_values, '{}'::smallint[]) || COALESCE(incoming_values, '{}'::smallint[])) AS code
            ORDER BY code
        ),
        '{}'::smallint[]
    );
$$;

CREATE OR REPLACE FUNCTION public.media_ensure_genre_codes(names text[])
RETURNS smallint[]
LANGUAGE plpgsql
AS $$
DECLARE
    normalized text[];
BEGIN
    normalized := public.media_distinct_trimmed_texts(names);
    IF COALESCE(array_length(normalized, 1), 0) = 0 THEN
        RETURN '{}'::smallint[];
    END IF;

    INSERT INTO public.genre_dim (name)
    SELECT unnest(normalized)
    ON CONFLICT (name) DO NOTHING;

    RETURN public.media_genre_codes(normalized);
END;
$$;

CREATE OR REPLACE FUNCTION public.media_ensure_studio_codes(names text[])
RETURNS smallint[]
LANGUAGE plpgsql
AS $$
DECLARE
    normalized text[];
BEGIN
    normalized := public.media_distinct_trimmed_texts(names);
    IF COALESCE(array_length(normalized, 1), 0) = 0 THEN
        RETURN '{}'::smallint[];
    END IF;

    INSERT INTO public.studio_dim (name)
    SELECT unnest(normalized)
    ON CONFLICT (name) DO NOTHING;

    RETURN public.media_studio_codes(normalized);
END;
$$;

CREATE OR REPLACE FUNCTION public.refresh_media_lookup_dims_v2()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN;
END;
$$;
