DO $$
BEGIN
    BEGIN
        EXECUTE $sql$
        CREATE OR REPLACE VIEW public.movie_route_card_view AS
        SELECT
            pr.id AS provider_record_id,
            pr.provider_code,
            pr.provider_movie_slug,
            pr.provider_title,
            pr.provider_year,
            pr.provider_rating,
            pr.quality_code,
            m.tmdb_id,
            m.slug,
            m.title,
            m.poster_path,
            m.year,
            m.rating,
            m.genre_codes,
            GREATEST(pr.updated_at, m.updated_at) AS updated_at
        FROM public.movie_provider_records pr
        JOIN public.movies m
            ON m.tmdb_id = pr.tmdb_id
        $sql$;
    EXCEPTION
        WHEN wrong_object_type THEN
            NULL;
    END;
END;
$$;

DO $$
BEGIN
    BEGIN
        EXECUTE $sql$
        CREATE OR REPLACE VIEW public.movie_route_detail_view AS
        SELECT
            pr.id AS provider_record_id,
            pr.provider_code,
            pr.provider_movie_slug,
            pr.provider_title,
            pr.provider_year,
            pr.provider_rating,
            pr.quality_code,
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
            GREATEST(pr.updated_at, m.updated_at, COALESCE(mm.updated_at, m.updated_at)) AS updated_at
        FROM public.movie_provider_records pr
        JOIN public.movies m
            ON m.tmdb_id = pr.tmdb_id
        LEFT JOIN public.movie_meta mm
            ON mm.tmdb_id = m.tmdb_id
        $sql$;
    EXCEPTION
        WHEN wrong_object_type THEN
            NULL;
    END;
END;
$$;
