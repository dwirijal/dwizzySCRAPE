UPDATE public.movies
SET slug = 'movie-' || tmdb_id::text
WHERE slug IS NULL OR BTRIM(slug) = '';

ALTER TABLE public.movies
    ALTER COLUMN slug DROP DEFAULT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'movies_slug_not_blank'
          AND conrelid = 'public.movies'::regclass
    ) THEN
        ALTER TABLE public.movies
            ADD CONSTRAINT movies_slug_not_blank
            CHECK (BTRIM(slug) <> '');
    END IF;
END $$;
