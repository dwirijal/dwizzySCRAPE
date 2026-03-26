CREATE TABLE IF NOT EXISTS public.anime_episodes (
    episode_slug text PRIMARY KEY,
    anime_slug text NOT NULL REFERENCES public.anime_list(slug) ON DELETE CASCADE,
    episode_number real NOT NULL DEFAULT 0,
    title text NOT NULL DEFAULT '',
    release_at timestamptz,
    release_label text NOT NULL DEFAULT '',
    fetch_status_code char(1) NOT NULL DEFAULT 'x',
    stream_links_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    download_links_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_anime_episodes_anime_slug
    ON public.anime_episodes (anime_slug);

CREATE INDEX IF NOT EXISTS idx_anime_episodes_updated_at
    ON public.anime_episodes (updated_at DESC);
