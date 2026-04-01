CREATE OR REPLACE FUNCTION public.refresh_media()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM public.refresh_anime();
    PERFORM public.refresh_movies();
END;
$$;

DO $$
BEGIN
    BEGIN
        EXECUTE $sql$
        CREATE OR REPLACE VIEW public.anime_list_view AS
        SELECT
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
            batch_links_json,
            updated_at
        FROM public.anime_list
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
        CREATE OR REPLACE VIEW public.anime_detail_view AS
        SELECT
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
        FROM public.anime_list
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
        CREATE OR REPLACE VIEW public.anime_meta_view AS
        SELECT
            slug,
            trailer_youtube_id,
            cast_json,
            updated_at
        FROM public.anime_meta
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
        CREATE OR REPLACE VIEW public.anime_episode_view AS
        SELECT
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
        FROM public.anime_episodes
        $sql$;
    EXCEPTION
        WHEN wrong_object_type THEN
            NULL;
    END;
END;
$$;
