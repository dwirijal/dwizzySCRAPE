UPDATE public.media_items AS i
SET
    surface_type = 'movie',
    presentation_type = 'live_action',
    origin_type = 'movie',
    release_country = COALESCE(
        NULLIF(release_country, ''),
        CASE
            WHEN upper(coalesce(i.detail->>'country', '')) IN ('CHINA', 'CN') THEN 'CN'
            WHEN upper(coalesce(i.detail->>'country', '')) IN ('JAPAN', 'JP') THEN 'JP'
            ELSE 'KR'
        END
    ),
    taxonomy_confidence = GREATEST(taxonomy_confidence, 94),
    taxonomy_source = 'provider_heuristic'
WHERE i.source = 'drakorid'
  AND i.media_type = 'movie';

WITH drakorid_variety AS (
    SELECT i.item_key
    FROM public.media_items i
    CROSS JOIN LATERAL (
        SELECT lower(
            concat_ws(
                ' ',
                coalesce(i.detail->>'source_title', ''),
                coalesce(i.title, ''),
                coalesce(i.detail->>'title', ''),
                coalesce(i.detail->>'alt_title', ''),
                coalesce(i.detail->>'native_title', ''),
                coalesce(i.detail->>'format', ''),
                coalesce(i.detail->>'episodes_text', '')
            )
        ) AS searchable
    ) text_blob
    WHERE i.source = 'drakorid'
      AND i.media_type <> 'movie'
      AND (
        text_blob.searchable LIKE '%variety%'
        OR text_blob.searchable LIKE '%reality%'
        OR text_blob.searchable LIKE '%talk show%'
        OR text_blob.searchable LIKE '%dating show%'
        OR text_blob.searchable LIKE '%dating reality%'
        OR text_blob.searchable LIKE '%entertainment show%'
        OR text_blob.searchable LIKE '%competition show%'
        OR text_blob.searchable LIKE '%game show%'
        OR text_blob.searchable LIKE '%music show%'
        OR text_blob.searchable LIKE '%idol show%'
        OR text_blob.searchable LIKE '%travel show%'
        OR text_blob.searchable LIKE '%running man%'
        OR text_blob.searchable LIKE '%how do you play%'
        OR text_blob.searchable LIKE '%1 night 2 days%'
        OR text_blob.searchable LIKE '%knowing brother%'
        OR text_blob.searchable LIKE '%my ugly duckling%'
        OR text_blob.searchable LIKE '%moms diary%'
        OR text_blob.searchable LIKE '%mom''s diary%'
        OR text_blob.searchable LIKE '%the genius paik%'
        OR text_blob.searchable LIKE '%the return of superman%'
        OR text_blob.searchable LIKE '%superman returns%'
        OR text_blob.searchable LIKE '%whenever possible%'
        OR text_blob.searchable LIKE '%i live alone%'
        OR text_blob.searchable LIKE '%when our kids fall in love%'
        OR text_blob.searchable LIKE '%singles inferno%'
      )
)
UPDATE public.media_items AS i
SET
    surface_type = 'series',
    presentation_type = 'live_action',
    origin_type = 'variety',
    release_country = COALESCE(
        NULLIF(release_country, ''),
        CASE
            WHEN upper(coalesce(i.detail->>'country', '')) IN ('CHINA', 'CN') THEN 'CN'
            WHEN upper(coalesce(i.detail->>'country', '')) IN ('JAPAN', 'JP') THEN 'JP'
            ELSE 'KR'
        END
    ),
    taxonomy_confidence = GREATEST(taxonomy_confidence, 96),
    taxonomy_source = 'provider_heuristic'
FROM drakorid_variety
WHERE drakorid_variety.item_key = i.item_key;
