CREATE OR REPLACE VIEW public.anime_catalog_view AS
SELECT
    slug,
    title,
    poster_url,
    status,
    anime_type,
    genres,
    synopsis_excerpt,
    score,
    updated_at
FROM public.samehadaku_anime_catalog;

CREATE OR REPLACE VIEW public.anime_details_view AS
SELECT
    slug,
    source_title,
    mal_id,
    mal_thumbnail_url,
    synopsis_source,
    synopsis_enriched,
    anime_type,
    status,
    season,
    studio_names,
    genre_names,
    cast_json,
    source_meta_json,
    jikan_meta_json,
    source_fetch_status,
    source_fetch_error,
    updated_at
FROM public.samehadaku_anime_details;

CREATE OR REPLACE VIEW public.anime_episode_view AS
SELECT
    anime_slug,
    episode_slug,
    title,
    episode_number,
    release_label,
    stream_links_json,
    download_links_json,
    source_meta_json,
    updated_at
FROM public.samehadaku_episode_details;
