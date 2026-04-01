UPDATE public.media_units
SET
  published_at = CONCAT(
    SUBSTRING(detail::text FROM '((?:19|20)\d{2}-\d{2}-\d{2})'),
    'T00:00:00Z'
  )::timestamptz,
  updated_at = now()
WHERE source = 'drakorid'
  AND unit_type = 'episode'
  AND published_at IS NULL
  AND detail::text ~ '((?:19|20)\d{2}-\d{2}-\d{2})';
