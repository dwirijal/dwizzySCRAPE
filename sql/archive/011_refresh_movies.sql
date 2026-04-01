CREATE OR REPLACE FUNCTION public.refresh_movies()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'movie_route_card_view'
          AND c.relkind IN ('r', 'p')
    ) THEN
        INSERT INTO public.movie_route_card_view (
            provider_record_id,
            provider_code,
            provider_movie_slug,
            provider_title,
            provider_year,
            provider_rating,
            quality_code,
            tmdb_id,
            slug,
            title,
            poster_path,
            year,
            rating,
            genre_codes,
            updated_at
        )
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
        ON CONFLICT (provider_record_id) DO UPDATE
        SET
            provider_code = EXCLUDED.provider_code,
            provider_movie_slug = EXCLUDED.provider_movie_slug,
            provider_title = EXCLUDED.provider_title,
            provider_year = EXCLUDED.provider_year,
            provider_rating = EXCLUDED.provider_rating,
            quality_code = EXCLUDED.quality_code,
            tmdb_id = EXCLUDED.tmdb_id,
            slug = EXCLUDED.slug,
            title = EXCLUDED.title,
            poster_path = EXCLUDED.poster_path,
            year = EXCLUDED.year,
            rating = EXCLUDED.rating,
            genre_codes = EXCLUDED.genre_codes,
            updated_at = EXCLUDED.updated_at;

        DELETE FROM public.movie_route_card_view v
        WHERE NOT EXISTS (
            SELECT 1 FROM public.movie_provider_records pr WHERE pr.id = v.provider_record_id
        );
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'movie_route_detail_view'
          AND c.relkind IN ('r', 'p')
    ) THEN
        INSERT INTO public.movie_route_detail_view (
            provider_record_id,
            provider_code,
            provider_movie_slug,
            provider_title,
            provider_year,
            provider_rating,
            quality_code,
            tmdb_id,
            slug,
            title,
            original_title,
            poster_path,
            backdrop_path,
            year,
            runtime_minutes,
            rating,
            status_code,
            language_code,
            genre_codes,
            country_codes,
            overview,
            tagline,
            trailer_youtube_id,
            meta_source_code,
            cast_json,
            director_names
        )
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
            COALESCE(mm.director_names, '{}'::text[]) AS director_names
        FROM public.movie_provider_records pr
        JOIN public.movies m
            ON m.tmdb_id = pr.tmdb_id
        LEFT JOIN public.movie_meta mm
            ON mm.tmdb_id = m.tmdb_id
        ON CONFLICT (provider_record_id) DO UPDATE
        SET
            provider_code = EXCLUDED.provider_code,
            provider_movie_slug = EXCLUDED.provider_movie_slug,
            provider_title = EXCLUDED.provider_title,
            provider_year = EXCLUDED.provider_year,
            provider_rating = EXCLUDED.provider_rating,
            quality_code = EXCLUDED.quality_code,
            tmdb_id = EXCLUDED.tmdb_id,
            slug = EXCLUDED.slug,
            title = EXCLUDED.title,
            original_title = EXCLUDED.original_title,
            poster_path = EXCLUDED.poster_path,
            backdrop_path = EXCLUDED.backdrop_path,
            year = EXCLUDED.year,
            runtime_minutes = EXCLUDED.runtime_minutes,
            rating = EXCLUDED.rating,
            status_code = EXCLUDED.status_code,
            language_code = EXCLUDED.language_code,
            genre_codes = EXCLUDED.genre_codes,
            country_codes = EXCLUDED.country_codes,
            overview = EXCLUDED.overview,
            tagline = EXCLUDED.tagline,
            trailer_youtube_id = EXCLUDED.trailer_youtube_id,
            meta_source_code = EXCLUDED.meta_source_code,
            cast_json = EXCLUDED.cast_json,
            director_names = EXCLUDED.director_names;

        DELETE FROM public.movie_route_detail_view v
        WHERE NOT EXISTS (
            SELECT 1 FROM public.movie_provider_records pr WHERE pr.id = v.provider_record_id
        );
    END IF;

    RETURN;
END;
$$;
