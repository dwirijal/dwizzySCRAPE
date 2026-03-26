DO $$
BEGIN
    IF to_regclass('public.samehadaku_episode_details') IS NULL
       AND to_regclass('public.samehadaku_episode_details_id_seq') IS NOT NULL THEN
        EXECUTE 'DROP SEQUENCE IF EXISTS public.samehadaku_episode_details_id_seq CASCADE';
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'samehadaku_episode_details'
          AND c.relkind = 'r'
    ) THEN
        EXECUTE $create$
            CREATE TABLE public.samehadaku_episode_details (
                id BIGSERIAL PRIMARY KEY,
                anime_slug TEXT NOT NULL REFERENCES public.samehadaku_anime_catalog(slug) ON DELETE CASCADE,
                episode_slug TEXT NOT NULL UNIQUE,
                canonical_url TEXT NOT NULL,
                title TEXT NOT NULL,
                episode_number DOUBLE PRECISION NOT NULL DEFAULT 0,
                release_label TEXT NOT NULL DEFAULT '',
                stream_links_json JSONB NOT NULL DEFAULT '{}'::jsonb,
                download_links_json JSONB NOT NULL DEFAULT '{}'::jsonb,
                source_meta_json JSONB NOT NULL DEFAULT '{}'::jsonb,
                scraped_at TIMESTAMPTZ NOT NULL,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )
        $create$;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_samehadaku_episode_details_anime_slug
    ON samehadaku_episode_details (anime_slug);

CREATE INDEX IF NOT EXISTS idx_samehadaku_episode_details_updated_at
    ON samehadaku_episode_details (updated_at DESC);
