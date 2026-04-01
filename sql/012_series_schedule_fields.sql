ALTER TABLE public.media_items
    ADD COLUMN IF NOT EXISTS release_day text,
    ADD COLUMN IF NOT EXISTS release_window text,
    ADD COLUMN IF NOT EXISTS release_timezone text,
    ADD COLUMN IF NOT EXISTS cadence text,
    ADD COLUMN IF NOT EXISTS next_release_at timestamptz;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'media_items_release_day_valid'
    ) THEN
        ALTER TABLE public.media_items
            ADD CONSTRAINT media_items_release_day_valid CHECK (
                release_day IS NULL
                OR release_day IN (
                    'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'
                )
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'media_items_release_timezone_not_blank'
    ) THEN
        ALTER TABLE public.media_items
            ADD CONSTRAINT media_items_release_timezone_not_blank CHECK (
                release_timezone IS NULL OR btrim(release_timezone) <> ''
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'media_items_release_window_not_blank'
    ) THEN
        ALTER TABLE public.media_items
            ADD CONSTRAINT media_items_release_window_not_blank CHECK (
                release_window IS NULL OR btrim(release_window) <> ''
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'media_items_cadence_valid'
    ) THEN
        ALTER TABLE public.media_items
            ADD CONSTRAINT media_items_cadence_valid CHECK (
                cadence IS NULL
                OR cadence IN ('daily', 'weekly', 'biweekly', 'monthly', 'irregular', 'completed', 'unknown')
            );
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_media_items_series_schedule
    ON public.media_items (surface_type, release_day, next_release_at)
    WHERE surface_type = 'series';

CREATE INDEX IF NOT EXISTS idx_media_items_series_cadence
    ON public.media_items (surface_type, cadence, updated_at DESC)
    WHERE surface_type = 'series';
