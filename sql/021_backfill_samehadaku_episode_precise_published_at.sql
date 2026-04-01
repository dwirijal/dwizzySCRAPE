UPDATE public.media_units
SET
  published_at = (detail->'source_meta_json'->>'published_at')::timestamptz,
  updated_at = now()
WHERE source = 'samehadaku'
  AND unit_type = 'episode'
  AND COALESCE(detail->'source_meta_json'->>'published_at', '') ~ '^(?:19|20)\d{2}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+\-]\d{2}:\d{2})$';
