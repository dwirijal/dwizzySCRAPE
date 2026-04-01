INSERT INTO public.media_item_enrichments (
    item_key,
    provider,
    external_id,
    match_status,
    match_score,
    matched_title,
    matched_year,
    payload,
    created_at,
    updated_at
)
SELECT
    i.item_key,
    'jikan' AS provider,
    i.mal_id::text AS external_id,
    'matched' AS match_status,
    CASE
        WHEN i.mal_id IS NOT NULL THEN 95
        ELSE 0
    END AS match_score,
    COALESCE(NULLIF(btrim(COALESCE(i.detail->>'source_title', i.title)), ''), i.title) AS matched_title,
    i.release_year AS matched_year,
    jsonb_strip_nulls(jsonb_build_object(
        'title', COALESCE(NULLIF(btrim(COALESCE(i.detail->>'source_title', i.title)), ''), i.title),
        'synopsis', NULLIF(btrim(COALESCE(i.detail->>'synopsis_enriched', i.detail->>'synopsis')), ''),
        'year', i.release_year,
        'season', NULLIF(btrim(i.detail->>'season'), ''),
        'status', NULLIF(btrim(COALESCE(i.detail->>'status', i.status)), ''),
        'score', CASE WHEN i.score > 0 THEN i.score ELSE NULL END,
        'genres', to_jsonb(COALESCE(
            CASE
                WHEN jsonb_typeof(i.detail->'genre_names') = 'array' THEN ARRAY(
                    SELECT DISTINCT btrim(value)
                    FROM jsonb_array_elements_text(i.detail->'genre_names') AS value
                    WHERE btrim(value) <> ''
                    ORDER BY btrim(value)
                )
                WHEN jsonb_typeof(i.detail->'genres') = 'array' THEN ARRAY(
                    SELECT DISTINCT btrim(value)
                    FROM jsonb_array_elements_text(i.detail->'genres') AS value
                    WHERE btrim(value) <> ''
                    ORDER BY btrim(value)
                )
                ELSE ARRAY[]::text[]
            END,
            ARRAY[]::text[]
        )),
        'studios', to_jsonb(COALESCE(
            CASE
                WHEN jsonb_typeof(i.detail->'studio_names') = 'array' THEN ARRAY(
                    SELECT DISTINCT btrim(value)
                    FROM jsonb_array_elements_text(i.detail->'studio_names') AS value
                    WHERE btrim(value) <> ''
                    ORDER BY btrim(value)
                )
                ELSE ARRAY[]::text[]
            END,
            ARRAY[]::text[]
        )),
        'poster_url', NULLIF(btrim(COALESCE(i.detail->>'mal_thumbnail_url', i.cover_url)), ''),
        'mal_url', NULLIF(btrim(i.detail->>'mal_url'), ''),
        'source', 'jikan'
    )) AS payload,
    i.created_at,
    i.updated_at
FROM public.media_items i
WHERE i.mal_id IS NOT NULL
ON CONFLICT (item_key, provider) DO UPDATE
SET
    external_id = EXCLUDED.external_id,
    match_status = EXCLUDED.match_status,
    match_score = EXCLUDED.match_score,
    matched_title = EXCLUDED.matched_title,
    matched_year = EXCLUDED.matched_year,
    payload = EXCLUDED.payload,
    updated_at = EXCLUDED.updated_at;
