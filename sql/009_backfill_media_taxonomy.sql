WITH normalized AS (
    SELECT
        i.item_key,
        CASE
            WHEN i.media_type = 'movie' THEN 'movie'
            WHEN i.media_type IN ('anime', 'drama', 'donghua') OR i.source IN ('samehadaku', 'anichin', 'drakorid') THEN 'series'
            WHEN i.media_type IN ('manga', 'manhwa', 'manhua', 'komiku') THEN 'comic'
            ELSE 'unknown'
        END AS surface_type,
        CASE
            WHEN i.source IN ('samehadaku', 'anichin') OR i.media_type IN ('anime', 'donghua') THEN 'animation'
            WHEN i.source = 'drakorid' OR i.media_type IN ('drama', 'movie') THEN 'live_action'
            WHEN i.media_type IN ('manga', 'manhwa', 'manhua', 'komiku') THEN 'illustrated'
            ELSE 'unknown'
        END AS presentation_type,
        CASE
            WHEN i.source = 'anichin' THEN 'donghua'
            WHEN i.source = 'samehadaku' AND i.media_type IN ('anime', 'movie') THEN 'anime'
            WHEN i.source = 'drakorid' THEN 'drama'
            WHEN i.media_type IN ('manga', 'manhwa', 'manhua') THEN i.media_type
            WHEN i.media_type = 'komiku' THEN 'manga'
            ELSE i.media_type
        END AS origin_type,
        CASE
            WHEN i.source = 'anichin' THEN 'CN'
            WHEN i.source = 'samehadaku' THEN 'JP'
            WHEN i.source = 'drakorid' THEN 'KR'
            WHEN upper(coalesce(i.detail->>'release_country', '')) IN ('JP', 'CN', 'KR', 'US') THEN upper(i.detail->>'release_country')
            WHEN upper(coalesce(i.detail->>'country_code', '')) IN ('JP', 'CN', 'KR', 'US') THEN upper(i.detail->>'country_code')
            WHEN upper(coalesce(i.detail->>'country', '')) IN ('JAPAN', 'JP') THEN 'JP'
            WHEN upper(coalesce(i.detail->>'country', '')) IN ('CHINA', 'CN') THEN 'CN'
            WHEN upper(coalesce(i.detail->>'country', '')) IN ('KOREA', 'SOUTH KOREA', 'KR') THEN 'KR'
            WHEN i.media_type IN ('manga', 'anime') THEN 'JP'
            WHEN i.media_type IN ('manhua', 'donghua') THEN 'CN'
            WHEN i.media_type IN ('manhwa', 'drama') THEN 'KR'
            ELSE NULL
        END AS release_country,
        CASE
            WHEN i.source IN ('kanzenin', 'mangasusuku') THEN true
            ELSE (
                EXISTS (
                    SELECT 1
                    FROM (
                        SELECT jsonb_array_elements_text(
                            CASE WHEN jsonb_typeof(i.detail->'genres') = 'array' THEN i.detail->'genres' ELSE '[]'::jsonb END
                        ) AS value
                        UNION ALL
                        SELECT jsonb_array_elements_text(
                            CASE WHEN jsonb_typeof(i.detail->'genre_names') = 'array' THEN i.detail->'genre_names' ELSE '[]'::jsonb END
                        ) AS value
                        UNION ALL
                        SELECT jsonb_array_elements_text(
                            CASE WHEN jsonb_typeof(i.detail->'tags') = 'array' THEN i.detail->'tags' ELSE '[]'::jsonb END
                        ) AS value
                        UNION ALL
                        SELECT jsonb_array_elements_text(
                            CASE WHEN jsonb_typeof(i.detail->'tag_names') = 'array' THEN i.detail->'tag_names' ELSE '[]'::jsonb END
                        ) AS value
                    ) labels
                    WHERE lower(btrim(labels.value)) IN ('nsfw', 'adult', '18+', 'hentai', 'smut', 'ecchi')
                )
            )
        END AS is_nsfw,
        COALESCE(
            ARRAY(
                SELECT DISTINCT btrim(labels.value)
                FROM (
                    SELECT jsonb_array_elements_text(
                        CASE WHEN jsonb_typeof(i.detail->'genres') = 'array' THEN i.detail->'genres' ELSE '[]'::jsonb END
                    ) AS value
                    UNION ALL
                    SELECT jsonb_array_elements_text(
                        CASE WHEN jsonb_typeof(i.detail->'genre_names') = 'array' THEN i.detail->'genre_names' ELSE '[]'::jsonb END
                    ) AS value
                    UNION ALL
                    SELECT jsonb_array_elements_text(
                        CASE WHEN jsonb_typeof(i.detail->'tags') = 'array' THEN i.detail->'tags' ELSE '[]'::jsonb END
                    ) AS value
                ) labels
                WHERE btrim(labels.value) <> ''
                ORDER BY btrim(labels.value)
            ),
            '{}'::text[]
        ) AS genre_names,
        CASE
            WHEN i.source = 'anichin' THEN 98
            WHEN i.source = 'samehadaku' THEN 95
            WHEN i.source = 'drakorid' THEN 94
            WHEN i.media_type IN ('manga', 'manhwa', 'manhua', 'komiku') THEN 92
            ELSE 60
        END AS taxonomy_confidence,
        CASE
            WHEN i.source IN ('samehadaku', 'anichin', 'drakorid', 'kanzenin', 'mangasusuku', 'manhwaindo') THEN 'provider_heuristic'
            ELSE 'legacy_backfill'
        END AS taxonomy_source
    FROM public.media_items i
)
UPDATE public.media_items AS i
SET
    surface_type = normalized.surface_type,
    presentation_type = normalized.presentation_type,
    origin_type = normalized.origin_type,
    release_country = normalized.release_country,
    is_nsfw = normalized.is_nsfw,
    genre_names = normalized.genre_names,
    taxonomy_confidence = normalized.taxonomy_confidence,
    taxonomy_source = normalized.taxonomy_source
FROM normalized
WHERE normalized.item_key = i.item_key;
