CREATE OR REPLACE FUNCTION public.refresh_anime()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM public.refresh_media_lookup_dims();

    IF EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'anime_list_view'
          AND c.relkind IN ('r', 'p')
    ) THEN
        INSERT INTO public.anime_list_view (
            slug, source_code, meta_source_code, title, poster_path, type_code, status_code, genre_codes, score, updated_at
        )
        SELECT
            slug, source_code, meta_source_code, title, poster_path, type_code, status_code, genre_codes, score, updated_at
        FROM public.anime_list
        ON CONFLICT (slug) DO UPDATE
        SET
            source_code = EXCLUDED.source_code,
            meta_source_code = EXCLUDED.meta_source_code,
            title = EXCLUDED.title,
            poster_path = EXCLUDED.poster_path,
            type_code = EXCLUDED.type_code,
            status_code = EXCLUDED.status_code,
            genre_codes = EXCLUDED.genre_codes,
            score = EXCLUDED.score,
            updated_at = EXCLUDED.updated_at;

        DELETE FROM public.anime_list_view v
        WHERE NOT EXISTS (
            SELECT 1 FROM public.anime_list a WHERE a.slug = v.slug
        );
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'anime_detail_view'
          AND c.relkind IN ('r', 'p')
    ) THEN
        INSERT INTO public.anime_detail_view (
            slug, source_code, meta_source_code, mal_id, title, poster_path, type_code, status_code, season_code,
            year, score, episode_count, genre_codes, studio_codes, synopsis_source, synopsis_enriched
        )
        SELECT
            slug, source_code, meta_source_code, mal_id, title, poster_path, type_code, status_code, season_code,
            year, score, episode_count, genre_codes, studio_codes, synopsis_source, synopsis_enriched
        FROM public.anime_list
        ON CONFLICT (slug) DO UPDATE
        SET
            source_code = EXCLUDED.source_code,
            meta_source_code = EXCLUDED.meta_source_code,
            mal_id = EXCLUDED.mal_id,
            title = EXCLUDED.title,
            poster_path = EXCLUDED.poster_path,
            type_code = EXCLUDED.type_code,
            status_code = EXCLUDED.status_code,
            season_code = EXCLUDED.season_code,
            year = EXCLUDED.year,
            score = EXCLUDED.score,
            episode_count = EXCLUDED.episode_count,
            genre_codes = EXCLUDED.genre_codes,
            studio_codes = EXCLUDED.studio_codes,
            synopsis_source = EXCLUDED.synopsis_source,
            synopsis_enriched = EXCLUDED.synopsis_enriched;

        DELETE FROM public.anime_detail_view v
        WHERE NOT EXISTS (
            SELECT 1 FROM public.anime_list a WHERE a.slug = v.slug
        );
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'anime_meta_view'
          AND c.relkind IN ('r', 'p')
    ) THEN
        INSERT INTO public.anime_meta_view (slug, trailer_youtube_id, cast_json)
        SELECT
            a.slug,
            COALESCE(m.trailer_youtube_id, '') AS trailer_youtube_id,
            COALESCE(m.cast_json, '[]'::jsonb) AS cast_json
        FROM public.anime_list a
        LEFT JOIN public.anime_meta m
            ON m.slug = a.slug
        ON CONFLICT (slug) DO UPDATE
        SET
            trailer_youtube_id = EXCLUDED.trailer_youtube_id,
            cast_json = EXCLUDED.cast_json;

        DELETE FROM public.anime_meta_view v
        WHERE NOT EXISTS (
            SELECT 1 FROM public.anime_list a WHERE a.slug = v.slug
        );
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'anime_episode_view'
          AND c.relkind IN ('r', 'p')
    ) THEN
        INSERT INTO public.anime_episode_view (
            episode_slug, anime_slug, episode_number, title, release_label, fetch_status_code, stream_links_json, download_links_json
        )
        SELECT
            episode_slug,
            anime_slug,
            episode_number,
            title,
            release_label,
            fetch_status_code,
            COALESCE(stream_links_json, '{}'::jsonb),
            COALESCE(download_links_json, '{}'::jsonb)
        FROM public.anime_episodes
        ON CONFLICT (episode_slug) DO UPDATE
        SET
            anime_slug = EXCLUDED.anime_slug,
            episode_number = EXCLUDED.episode_number,
            title = EXCLUDED.title,
            release_label = EXCLUDED.release_label,
            fetch_status_code = EXCLUDED.fetch_status_code,
            stream_links_json = EXCLUDED.stream_links_json,
            download_links_json = EXCLUDED.download_links_json;

        DELETE FROM public.anime_episode_view v
        WHERE NOT EXISTS (
            SELECT 1 FROM public.anime_episodes e WHERE e.episode_slug = v.episode_slug
        );
    END IF;

    DELETE FROM public.anime_meta
    WHERE trailer_youtube_id = ''
      AND cast_json = '[]'::jsonb;
END;
$$;
