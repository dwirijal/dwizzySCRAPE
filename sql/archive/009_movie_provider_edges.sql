CREATE TABLE IF NOT EXISTS public.movie_provider_records (
    id bigserial PRIMARY KEY,
    tmdb_id bigint NOT NULL REFERENCES public.movies(tmdb_id) ON DELETE CASCADE,
    provider_code char(1) NOT NULL,
    provider_movie_slug text NOT NULL,
    provider_title text NOT NULL DEFAULT '',
    provider_poster_path text NOT NULL DEFAULT '',
    provider_year smallint,
    provider_rating real NOT NULL DEFAULT 0,
    quality_code char(1) NOT NULL DEFAULT 'u',
    scrape_status_code char(1) NOT NULL DEFAULT 'x',
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT movie_provider_records_provider_slug_unique UNIQUE (provider_code, provider_movie_slug)
);

CREATE TABLE IF NOT EXISTS public.movie_watch_options (
    id bigserial PRIMARY KEY,
    tmdb_id bigint NOT NULL REFERENCES public.movies(tmdb_id) ON DELETE CASCADE,
    provider_record_id bigint NOT NULL REFERENCES public.movie_provider_records(id) ON DELETE CASCADE,
    provider_code char(1) NOT NULL,
    host_code char(1) NOT NULL DEFAULT 'u',
    label text NOT NULL DEFAULT '',
    embed_url text NOT NULL DEFAULT '',
    lang_code char(2) NOT NULL DEFAULT '',
    quality_code char(1) NOT NULL DEFAULT 'u',
    priority smallint NOT NULL DEFAULT 0,
    status_code char(1) NOT NULL DEFAULT 'a',
    last_verified_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT movie_watch_options_provider_record_label_embed_unique UNIQUE (provider_record_id, label, embed_url)
);

CREATE TABLE IF NOT EXISTS public.movie_download_options (
    id bigserial PRIMARY KEY,
    tmdb_id bigint NOT NULL REFERENCES public.movies(tmdb_id) ON DELETE CASCADE,
    provider_record_id bigint NOT NULL REFERENCES public.movie_provider_records(id) ON DELETE CASCADE,
    provider_code char(1) NOT NULL,
    host_code char(1) NOT NULL DEFAULT 'u',
    label text NOT NULL DEFAULT '',
    download_url text NOT NULL DEFAULT '',
    quality_code char(1) NOT NULL DEFAULT 'u',
    format_code char(1) NOT NULL DEFAULT 'u',
    size_label text NOT NULL DEFAULT '',
    status_code char(1) NOT NULL DEFAULT 'a',
    last_verified_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT movie_download_options_provider_record_label_url_unique UNIQUE (provider_record_id, label, download_url)
);

ALTER TABLE public.movie_watch_options
    ADD COLUMN IF NOT EXISTS tmdb_id bigint,
    ADD COLUMN IF NOT EXISTS provider_code char(1),
    ADD COLUMN IF NOT EXISTS host_code char(1) NOT NULL DEFAULT 'u',
    ADD COLUMN IF NOT EXISTS label text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embed_url text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS lang_code char(2) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS quality_code char(1) NOT NULL DEFAULT 'u',
    ADD COLUMN IF NOT EXISTS priority smallint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS status_code char(1) NOT NULL DEFAULT 'a',
    ADD COLUMN IF NOT EXISTS last_verified_at timestamptz,
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE public.movie_download_options
    ADD COLUMN IF NOT EXISTS tmdb_id bigint,
    ADD COLUMN IF NOT EXISTS provider_code char(1),
    ADD COLUMN IF NOT EXISTS host_code char(1) NOT NULL DEFAULT 'u',
    ADD COLUMN IF NOT EXISTS label text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS download_url text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS quality_code char(1) NOT NULL DEFAULT 'u',
    ADD COLUMN IF NOT EXISTS format_code char(1) NOT NULL DEFAULT 'u',
    ADD COLUMN IF NOT EXISTS size_label text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status_code char(1) NOT NULL DEFAULT 'a',
    ADD COLUMN IF NOT EXISTS last_verified_at timestamptz,
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

DELETE FROM public.movie_watch_options a
USING public.movie_watch_options b
WHERE a.id < b.id
  AND a.provider_record_id = b.provider_record_id
  AND COALESCE(a.label, '') = COALESCE(b.label, '')
  AND COALESCE(a.embed_url, '') = COALESCE(b.embed_url, '');

DELETE FROM public.movie_download_options a
USING public.movie_download_options b
WHERE a.id < b.id
  AND a.provider_record_id = b.provider_record_id
  AND COALESCE(a.label, '') = COALESCE(b.label, '')
  AND COALESCE(a.download_url, '') = COALESCE(b.download_url, '');

CREATE UNIQUE INDEX IF NOT EXISTS idx_movie_watch_options_provider_record_label_embed_unique
    ON public.movie_watch_options (provider_record_id, label, embed_url);

CREATE UNIQUE INDEX IF NOT EXISTS idx_movie_download_options_provider_record_label_url_unique
    ON public.movie_download_options (provider_record_id, label, download_url);

CREATE INDEX IF NOT EXISTS idx_movie_provider_records_tmdb_id
    ON public.movie_provider_records (tmdb_id);

CREATE INDEX IF NOT EXISTS idx_movie_provider_records_updated_at
    ON public.movie_provider_records (updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_movie_watch_options_tmdb_id_status_priority
    ON public.movie_watch_options (tmdb_id, status_code, priority);

CREATE INDEX IF NOT EXISTS idx_movie_download_options_tmdb_id_status_quality
    ON public.movie_download_options (tmdb_id, status_code, quality_code);
