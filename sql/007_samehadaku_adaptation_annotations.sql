UPDATE public.media_items
SET
    detail = jsonb_set(
        jsonb_set(
            coalesce(detail, '{}'::jsonb),
            '{adaptation_type}',
            '"live_action"'::jsonb,
            true
        ),
        '{entry_format}',
        '"series"'::jsonb,
        true
    ),
    updated_at = timezone('utc', now())
WHERE source = 'samehadaku'
  AND slug IN ('one-piece-live-action', 'one-piece-season-2-live-action');

UPDATE public.media_items
SET
    detail = jsonb_set(
        coalesce(detail, '{}'::jsonb),
        '{entry_format}',
        '"special_series"'::jsonb,
        true
    ),
    updated_at = timezone('utc', now())
WHERE source = 'samehadaku'
  AND slug = 'kaguya-sama-wa-kokurasetai-first-kiss-wa-owaranai';
