CREATE OR REPLACE FUNCTION public.upsert_media_item(
    p_item_key   text,
    p_source     text,
    p_media_type text,
    p_slug       text,
    p_title      text,
    p_cover_url  text,
    p_status     text,
    p_year       smallint,
    p_score      real,
    p_mal_id     bigint,
    p_tmdb_id    bigint,
    p_detail     jsonb
) RETURNS void
LANGUAGE sql AS $$
    INSERT INTO public.media_items (
        item_key, source, media_type, slug, title, cover_url, 
        status, release_year, score, mal_id, tmdb_id, detail, updated_at
    ) VALUES (
        p_item_key, p_source, p_media_type, p_slug, p_title, p_cover_url,
        p_status, p_year, p_score, p_mal_id, p_tmdb_id, p_detail, now()
    )
    ON CONFLICT (item_key) DO UPDATE SET
        title      = CASE WHEN EXCLUDED.title <> '' THEN EXCLUDED.title ELSE media_items.title END,
        cover_url  = CASE WHEN EXCLUDED.cover_url <> '' THEN EXCLUDED.cover_url ELSE media_items.cover_url END,
        status     = CASE WHEN EXCLUDED.status <> '' THEN EXCLUDED.status ELSE media_items.status END,
        release_year = COALESCE(EXCLUDED.release_year, media_items.release_year),
        score      = CASE WHEN EXCLUDED.score > 0 THEN EXCLUDED.score ELSE media_items.score END,
        mal_id     = COALESCE(EXCLUDED.mal_id, media_items.mal_id),
        tmdb_id    = COALESCE(EXCLUDED.tmdb_id, media_items.tmdb_id),
        detail     = media_items.detail || EXCLUDED.detail,
        updated_at = now();
$$;

CREATE OR REPLACE FUNCTION public.upsert_media_unit(
    p_unit_key  text,
    p_item_key  text,
    p_source    text,
    p_unit_type text,
    p_slug      text,
    p_title     text,
    p_label     text,
    p_number    real,
    p_canonical_url text,
    p_published_at  timestamptz,
    p_prev_slug text,
    p_next_slug text,
    p_detail    jsonb
) RETURNS void
LANGUAGE sql AS $$
    INSERT INTO public.media_units (
        unit_key, item_key, source, unit_type, slug, title, label, number,
        canonical_url, published_at, prev_slug, next_slug, detail, updated_at
    ) VALUES (
        p_unit_key, p_item_key, p_source, p_unit_type, p_slug, p_title, p_label, p_number,
        p_canonical_url, p_published_at, p_prev_slug, p_next_slug, p_detail, now()
    )
    ON CONFLICT (unit_key) DO UPDATE SET
        title         = CASE WHEN EXCLUDED.title <> '' THEN EXCLUDED.title ELSE media_units.title END,
        label         = CASE WHEN EXCLUDED.label <> '' THEN EXCLUDED.label ELSE media_units.label END,
        number        = COALESCE(EXCLUDED.number, media_units.number),
        published_at  = COALESCE(EXCLUDED.published_at, media_units.published_at),
        prev_slug     = COALESCE(EXCLUDED.prev_slug, media_units.prev_slug),
        next_slug     = COALESCE(EXCLUDED.next_slug, media_units.next_slug),
        detail        = media_units.detail || EXCLUDED.detail,
        updated_at    = now();
$$;
