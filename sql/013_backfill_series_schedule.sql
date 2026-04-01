WITH normalized AS (
    SELECT
        i.item_key,
        CASE lower(coalesce(i.detail->>'release_day', i.detail->>'schedule_day', i.detail->>'broadcast_day', ''))
            WHEN 'monday' THEN 'monday'
            WHEN 'mondays' THEN 'monday'
            WHEN 'senin' THEN 'monday'
            WHEN 'tuesday' THEN 'tuesday'
            WHEN 'tuesdays' THEN 'tuesday'
            WHEN 'selasa' THEN 'tuesday'
            WHEN 'wednesday' THEN 'wednesday'
            WHEN 'wednesdays' THEN 'wednesday'
            WHEN 'rabu' THEN 'wednesday'
            WHEN 'thursday' THEN 'thursday'
            WHEN 'thursdays' THEN 'thursday'
            WHEN 'kamis' THEN 'thursday'
            WHEN 'friday' THEN 'friday'
            WHEN 'fridays' THEN 'friday'
            WHEN 'jumat' THEN 'friday'
            WHEN 'jum''at' THEN 'friday'
            WHEN 'saturday' THEN 'saturday'
            WHEN 'saturdays' THEN 'saturday'
            WHEN 'sabtu' THEN 'saturday'
            WHEN 'sunday' THEN 'sunday'
            WHEN 'sundays' THEN 'sunday'
            WHEN 'minggu' THEN 'sunday'
            ELSE NULL
        END AS release_day,
        NULLIF(coalesce(i.detail->>'release_window', i.detail->>'broadcast_time', i.detail->>'release_time', ''), '') AS release_window,
        CASE
            WHEN coalesce(i.release_country, '') = 'JP' THEN 'Asia/Tokyo'
            WHEN coalesce(i.release_country, '') = 'CN' THEN 'Asia/Shanghai'
            WHEN coalesce(i.release_country, '') = 'KR' THEN 'Asia/Seoul'
            WHEN coalesce(i.release_country, '') = 'US' THEN 'America/New_York'
            ELSE NULL
        END AS release_timezone,
        CASE
            WHEN lower(coalesce(i.status, '')) IN ('completed', 'finished', 'finished airing') THEN 'completed'
            WHEN i.surface_type = 'series'
             AND i.presentation_type = 'animation'
             AND i.source IN ('samehadaku', 'anichin')
             AND lower(coalesce(i.status, '')) IN ('ongoing', 'airing', 'currently airing')
                THEN 'weekly'
            WHEN i.surface_type = 'series'
             AND i.presentation_type = 'live_action'
             AND i.source = 'drakorid'
             AND coalesce(i.origin_type, '') <> 'variety'
             AND lower(coalesce(i.status, '')) IN ('ongoing', 'airing')
                THEN 'weekly'
            ELSE NULL
        END AS cadence
    FROM public.media_items i
    WHERE i.surface_type = 'series'
)
UPDATE public.media_items AS i
SET
    release_day = COALESCE(normalized.release_day, i.release_day),
    release_window = COALESCE(normalized.release_window, i.release_window),
    release_timezone = COALESCE(normalized.release_timezone, i.release_timezone),
    cadence = COALESCE(normalized.cadence, i.cadence)
FROM normalized
WHERE normalized.item_key = i.item_key;
