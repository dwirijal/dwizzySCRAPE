CREATE OR REPLACE VIEW public.v_anime_api AS
SELECT 
    m.item_key,
    m.source,
    m.slug,
    m.title,
    m.cover_url,
    m.status,
    m.release_year,
    m.score,
    m.mal_id,
    m.detail->>'type_code' AS type_code,
    m.detail->>'season_code' AS season_code,
    (m.detail->>'episode_count')::int AS episode_count,
    m.detail->>'synopsis' AS synopsis,
    m.detail->>'trailer_youtube_id' AS trailer_youtube_id,
    m.updated_at
FROM public.media_items m
WHERE m.media_type = 'anime';

CREATE OR REPLACE VIEW public.v_movie_api AS
SELECT 
    m.item_key,
    m.source,
    m.slug,
    m.title,
    m.cover_url,
    m.status,
    m.release_year,
    m.score,
    m.tmdb_id,
    m.detail->>'original_title' AS original_title,
    m.detail->>'overview' AS overview,
    (m.detail->>'runtime_minutes')::int AS runtime_minutes,
    m.detail->>'trailer_youtube_id' AS trailer_youtube_id,
    m.updated_at
FROM public.media_items m
WHERE m.media_type = 'movie';

CREATE OR REPLACE VIEW public.v_manhwa_api AS
SELECT 
    m.item_key,
    m.source,
    m.slug,
    m.title,
    m.cover_url,
    m.status,
    m.release_year,
    m.score,
    m.detail->>'author' AS author,
    m.detail->>'synopsis' AS synopsis,
    m.detail->>'latest_chapter_label' AS latest_chapter_label,
    m.updated_at
FROM public.media_items m
WHERE m.media_type IN ('manhwa', 'komiku');
