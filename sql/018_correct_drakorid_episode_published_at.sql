UPDATE public.media_units
SET
  published_at = CONCAT(
    SUBSTRING(
      COALESCE(detail->'source_meta_json'->'episode_api'->>'img', '')
      FROM '((?:19|20)\d{2}-\d{2}-\d{2})'
    ),
    'T00:00:00Z'
  )::timestamptz,
  updated_at = now()
WHERE source = 'drakorid'
  AND unit_type = 'episode'
  AND COALESCE(detail->'source_meta_json'->'episode_api'->>'img', '') ~ '((?:19|20)\d{2}-\d{2}-\d{2})';
