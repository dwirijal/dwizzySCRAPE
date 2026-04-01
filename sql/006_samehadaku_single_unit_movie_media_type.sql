WITH candidates AS (
    SELECT
        i.item_key AS old_item_key,
        'samehadaku:movie:' || i.slug AS new_item_key,
        i.slug
    FROM public.media_items i
    LEFT JOIN public.media_units u
        ON u.item_key = i.item_key
    WHERE i.source = 'samehadaku'
      AND i.media_type = 'anime'
      AND lower(coalesce(i.detail->>'anime_type', '')) = 'movie'
    GROUP BY i.item_key, i.slug
    HAVING count(u.*) = 1
       AND NOT EXISTS (
            SELECT 1
            FROM public.media_items x
            WHERE x.item_key = 'samehadaku:movie:' || i.slug
        )
),
inserted AS (
    INSERT INTO public.media_items (
        item_key, source, media_type, slug, title, cover_url,
        status, release_year, score, mal_id, tmdb_id, detail,
        created_at, updated_at
    )
    SELECT
        c.new_item_key,
        i.source,
        'movie',
        i.slug,
        i.title,
        i.cover_url,
        i.status,
        i.release_year,
        i.score,
        i.mal_id,
        i.tmdb_id,
        i.detail,
        i.created_at,
        timezone('utc', now())
    FROM public.media_items i
    JOIN candidates c
      ON c.old_item_key = i.item_key
),
updated_units AS (
    UPDATE public.media_units u
    SET
        item_key = c.new_item_key,
        detail = jsonb_set(
            coalesce(u.detail, '{}'::jsonb),
            '{anime_slug}',
            to_jsonb(c.slug),
            true
        ),
        updated_at = timezone('utc', now())
    FROM candidates c
    WHERE u.item_key = c.old_item_key
    RETURNING u.unit_key
)
DELETE FROM public.media_items i
USING candidates c
WHERE i.item_key = c.old_item_key;
