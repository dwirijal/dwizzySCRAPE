WITH weekday_counts AS (
    SELECT
        u.item_key,
        lower(trim(to_char(u.published_at, 'FMDay'))) AS release_day,
        count(*) AS hit_count,
        row_number() OVER (
            PARTITION BY u.item_key
            ORDER BY count(*) DESC, lower(trim(to_char(u.published_at, 'FMDay'))) ASC
        ) AS rank_in_item
    FROM public.media_units u
    JOIN public.media_items i
      ON i.item_key = u.item_key
    WHERE u.unit_type = 'episode'
      AND u.published_at IS NOT NULL
      AND i.surface_type = 'series'
    GROUP BY u.item_key, lower(trim(to_char(u.published_at, 'FMDay')))
),
chosen AS (
    SELECT item_key, release_day
    FROM weekday_counts
    WHERE rank_in_item = 1
)
UPDATE public.media_items AS i
SET release_day = chosen.release_day
FROM chosen
WHERE chosen.item_key = i.item_key
  AND i.surface_type = 'series'
  AND COALESCE(i.release_day, '') = ''
  AND chosen.release_day IN ('monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday');
