WITH candidate_schedule AS (
  SELECT
    item_key,
    CASE
      WHEN COALESCE(detail->>'runtime', '') ~* 'monday' THEN 'monday'
      WHEN COALESCE(detail->>'runtime', '') ~* 'tuesday' THEN 'tuesday'
      WHEN COALESCE(detail->>'runtime', '') ~* 'wednesday' THEN 'wednesday'
      WHEN COALESCE(detail->>'runtime', '') ~* 'thursday' THEN 'thursday'
      WHEN COALESCE(detail->>'runtime', '') ~* 'friday' THEN 'friday'
      WHEN COALESCE(detail->>'runtime', '') ~* 'saturday' THEN 'saturday'
      WHEN COALESCE(detail->>'runtime', '') ~* 'sunday' THEN 'sunday'
      ELSE NULL
    END AS release_day,
    NULLIF(SUBSTRING(COALESCE(detail->>'runtime', '') FROM '(\d{1,2}:\d{2}(?:\s*-\s*\d{1,2}:\d{2})?)'), '') AS release_window
  FROM public.media_items
  WHERE source = 'drakorid'
    AND surface_type = 'series'
    AND origin_type = 'drama'
)
UPDATE public.media_items AS i
SET
  release_day = c.release_day,
  release_window = COALESCE(c.release_window, i.release_window),
  release_timezone = COALESCE(NULLIF(i.release_timezone, ''), 'Asia/Seoul'),
  updated_at = now()
FROM candidate_schedule AS c
WHERE i.item_key = c.item_key
  AND c.release_day IS NOT NULL;
