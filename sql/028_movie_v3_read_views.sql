CREATE OR REPLACE VIEW public.movie_list_v3_view AS
SELECT
    tmdb_id,
    slug,
    title,
    poster_path,
    year,
    rating,
    genre_codes,
    updated_at
FROM public.movies;

CREATE OR REPLACE VIEW public.movie_detail_v3_view AS
SELECT
    m.tmdb_id,
    m.slug,
    m.title,
    m.original_title,
    m.poster_path,
    m.backdrop_path,
    m.year,
    m.runtime_minutes,
    m.rating,
    m.status_code,
    m.language_code,
    m.genre_codes,
    m.country_codes,
    m.overview,
    m.tagline,
    m.trailer_youtube_id,
    m.meta_source_code,
    COALESCE(mm.cast_json, '[]'::jsonb) AS cast_json,
    COALESCE(mm.director_names, '{}'::text[]) AS director_names,
    COALESCE(mm.alt_titles_json, '[]'::jsonb) AS alt_titles_json,
    GREATEST(m.updated_at, COALESCE(mm.updated_at, m.updated_at)) AS updated_at
FROM public.movies m
LEFT JOIN public.movie_meta mm
    ON mm.tmdb_id = m.tmdb_id;

CREATE OR REPLACE VIEW public.movie_watch_summary_v3_view AS
SELECT
    m.tmdb_id,
    m.slug,
    m.title,
    m.poster_path,
    m.backdrop_path,
    m.year,
    m.runtime_minutes,
    m.rating,
    m.overview,
    m.tagline,
    m.trailer_youtube_id,
    (
        SELECT COUNT(*)::integer
        FROM public.movie_watch_options w
        WHERE w.tmdb_id = m.tmdb_id
          AND w.status_code = 'a'
    ) AS watch_option_count,
    (
        SELECT COUNT(*)::integer
        FROM public.movie_download_options d
        WHERE d.tmdb_id = m.tmdb_id
          AND d.status_code = 'a'
    ) AS download_option_count,
    m.updated_at
FROM public.movies m;
