WITH base AS (
  SELECT
    item_key,
    release_timezone,
    lower(release_day) AS release_day,
    regexp_match(COALESCE(release_window, ''), '(\d{1,2}):(\d{2})') AS time_match,
    now() AT TIME ZONE release_timezone AS local_now
  FROM public.media_items
  WHERE surface_type = 'series'
    AND cadence = 'weekly'
    AND COALESCE(release_day, '') <> ''
    AND COALESCE(release_timezone, '') <> ''
    AND COALESCE(release_window, '') ~ '(\d{1,2}):(\d{2})'
), expanded AS (
  SELECT
    item_key,
    release_timezone,
    local_now,
    CASE release_day
      WHEN 'monday' THEN 1
      WHEN 'tuesday' THEN 2
      WHEN 'wednesday' THEN 3
      WHEN 'thursday' THEN 4
      WHEN 'friday' THEN 5
      WHEN 'saturday' THEN 6
      WHEN 'sunday' THEN 7
      ELSE NULL
    END AS target_isodow,
    (time_match)[1]::int AS hour_part,
    (time_match)[2]::int AS minute_part
  FROM base
), computed AS (
  SELECT
    item_key,
    (
      (
        date_trunc('day', local_now)
        + make_interval(hours => hour_part, mins => minute_part)
        + make_interval(
            days => CASE
              WHEN target_isodow IS NULL THEN 0
              WHEN ((target_isodow - extract(isodow from local_now)::int + 7) % 7) = 0
                   AND (
                     date_trunc('day', local_now)
                     + make_interval(hours => hour_part, mins => minute_part)
                   ) <= local_now
                THEN 7
              ELSE ((target_isodow - extract(isodow from local_now)::int + 7) % 7)
            END
          )
      ) AT TIME ZONE release_timezone
    ) AS next_release_at
  FROM expanded
  WHERE target_isodow IS NOT NULL
)
UPDATE public.media_items AS i
SET
  next_release_at = c.next_release_at,
  updated_at = now()
FROM computed AS c
WHERE i.item_key = c.item_key;
