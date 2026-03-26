CREATE TABLE IF NOT EXISTS public.movies (
    tmdb_id bigint PRIMARY KEY,
    slug text NOT NULL,
    title text NOT NULL DEFAULT '',
    original_title text NOT NULL DEFAULT '',
    poster_path text NOT NULL DEFAULT '',
    backdrop_path text NOT NULL DEFAULT '',
    year smallint,
    runtime_minutes smallint,
    rating real NOT NULL DEFAULT 0,
    status_code char(1) NOT NULL DEFAULT 'r',
    language_code char(2) NOT NULL DEFAULT '',
    genre_codes smallint[] NOT NULL DEFAULT '{}'::smallint[],
    country_codes smallint[] NOT NULL DEFAULT '{}'::smallint[],
    overview text NOT NULL DEFAULT '',
    tagline text NOT NULL DEFAULT '',
    trailer_youtube_id text NOT NULL DEFAULT '',
    meta_source_code char(1) NOT NULL DEFAULT 't',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT movies_slug_not_blank CHECK (BTRIM(slug) <> '')
);

CREATE TABLE IF NOT EXISTS public.movie_meta (
    tmdb_id bigint PRIMARY KEY REFERENCES public.movies(tmdb_id) ON DELETE CASCADE,
    cast_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    director_names text[] NOT NULL DEFAULT '{}'::text[],
    alt_titles_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_movies_updated_at
    ON public.movies (updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_movies_year
    ON public.movies (year);

CREATE UNIQUE INDEX IF NOT EXISTS idx_movies_slug_unique
    ON public.movies (slug);

CREATE INDEX IF NOT EXISTS idx_movies_title_updated_at
    ON public.movies (title, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_movie_meta_updated_at
    ON public.movie_meta (updated_at DESC);
