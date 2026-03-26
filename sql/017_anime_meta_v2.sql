CREATE TABLE IF NOT EXISTS public.anime_meta (
    slug text PRIMARY KEY REFERENCES public.anime_list(slug) ON DELETE CASCADE,
    trailer_youtube_id text NOT NULL DEFAULT '',
    cast_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_anime_meta_updated_at
    ON public.anime_meta (updated_at DESC);
