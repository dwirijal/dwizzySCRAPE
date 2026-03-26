ALTER TABLE public.movies
    ADD COLUMN IF NOT EXISTS slug text;

UPDATE public.movies
SET slug = 'movie-' || tmdb_id::text
WHERE slug IS NULL OR btrim(slug) = '';

ALTER TABLE public.movies
    ALTER COLUMN slug SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_movies_slug_unique
    ON public.movies (slug);

CREATE INDEX IF NOT EXISTS idx_movies_title_updated_at
    ON public.movies (title, updated_at DESC);
