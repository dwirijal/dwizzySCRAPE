CREATE TABLE IF NOT EXISTS public.anime_list (
    slug text PRIMARY KEY,
    source_code char(1) NOT NULL DEFAULT 's',
    meta_source_code char(1) NOT NULL DEFAULT 's',
    mal_id bigint,
    title text NOT NULL DEFAULT '',
    poster_path text NOT NULL DEFAULT '',
    type_code char(1) NOT NULL DEFAULT 'u',
    status_code char(1) NOT NULL DEFAULT 'x',
    season_code char(1) NOT NULL DEFAULT 'x',
    year smallint,
    score real NOT NULL DEFAULT 0,
    episode_count integer,
    genre_codes smallint[] NOT NULL DEFAULT '{}',
    studio_codes smallint[] NOT NULL DEFAULT '{}',
    synopsis_source text NOT NULL DEFAULT '',
    synopsis_enriched text NOT NULL DEFAULT '',
    batch_links_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE public.anime_list
    ADD COLUMN IF NOT EXISTS batch_links_json jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_anime_list_mal_id
    ON public.anime_list (mal_id);

CREATE INDEX IF NOT EXISTS idx_anime_list_updated_at
    ON public.anime_list (updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_anime_list_status_code
    ON public.anime_list (status_code);
