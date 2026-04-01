WITH episode_times AS (
  SELECT
    u.item_key,
    to_char(u.published_at AT TIME ZONE 'Asia/Tokyo', 'HH24:MI') AS local_release_time,
    count(*) AS hit_count,
    row_number() OVER (
      PARTITION BY u.item_key
      ORDER BY count(*) DESC, to_char(u.published_at AT TIME ZONE 'Asia/Tokyo', 'HH24:MI') ASC
    ) AS rank_in_item
  FROM public.media_units u
  JOIN public.media_items i
    ON i.item_key = u.item_key
  WHERE i.source = 'samehadaku'
    AND i.surface_type = 'series'
    AND u.unit_type = 'episode'
    AND u.published_at IS NOT NULL
  GROUP BY u.item_key, to_char(u.published_at AT TIME ZONE 'Asia/Tokyo', 'HH24:MI')
),
chosen AS (
  SELECT item_key, local_release_time
  FROM episode_times
  WHERE rank_in_item = 1
)
UPDATE public.media_items AS i
SET
  release_window = c.local_release_time,
  release_timezone = COALESCE(NULLIF(i.release_timezone, ''), 'Asia/Tokyo'),
  updated_at = now()
FROM chosen AS c
WHERE i.item_key = c.item_key
  AND i.source = 'samehadaku'
  AND i.surface_type = 'series';
