WITH normalized AS (
    SELECT
        unit_key,
        CASE
            WHEN source = 'samehadaku' AND label ~* '^[0-9]{1,2}\s+[A-Za-z]+\s+[0-9]{4}$' THEN
                to_date(
                    replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(
                        lower(label),
                        'januari', 'january'
                    ), 'februari', 'february'
                    ), 'maret', 'march'
                    ), 'mei', 'may'
                    ), 'juni', 'june'
                    ), 'juli', 'july'
                    ), 'agustus', 'august'
                    ), 'oktober', 'october'
                    ), 'desember', 'december'
                    ), 'april', 'april'
                    ), 'september', 'september'
                    ), 'november', 'november'),
                    'DD Month YYYY'
                )::timestamptz
            WHEN source = 'anichin' AND label ~* '^[A-Za-z]+\s+[0-9]{1,2},\s+[0-9]{4}$' THEN
                to_date(label, 'Month DD, YYYY')::timestamptz
            ELSE NULL
        END AS published_at
    FROM public.media_units
    WHERE unit_type = 'episode'
      AND published_at IS NULL
      AND source IN ('samehadaku', 'anichin')
      AND btrim(label) <> ''
)
UPDATE public.media_units AS u
SET published_at = normalized.published_at
FROM normalized
WHERE normalized.unit_key = u.unit_key
  AND normalized.published_at IS NOT NULL;
