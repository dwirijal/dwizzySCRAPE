DO $$
BEGIN
    IF to_regclass('public.samehadaku_anime_details') IS NULL
       AND to_regclass('public.samehadaku_anime_details_id_seq') IS NOT NULL THEN
        EXECUTE 'DROP SEQUENCE IF EXISTS public.samehadaku_anime_details_id_seq CASCADE';
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'samehadaku_anime_details'
          AND c.relkind = 'r'
    ) THEN
        EXECUTE $create$
            CREATE TABLE public.samehadaku_anime_details (
                id BIGSERIAL PRIMARY KEY,
                slug TEXT NOT NULL UNIQUE REFERENCES public.samehadaku_anime_catalog(slug) ON DELETE CASCADE,
                canonical_url TEXT NOT NULL,
                source_title TEXT NOT NULL,
                mal_id BIGINT,
                mal_url TEXT NOT NULL DEFAULT '',
                mal_thumbnail_url TEXT NOT NULL DEFAULT '',
                synopsis_source TEXT NOT NULL DEFAULT '',
                synopsis_enriched TEXT NOT NULL DEFAULT '',
                anime_type TEXT,
                status TEXT,
                season TEXT,
                studio_names TEXT[] NOT NULL DEFAULT '{}',
                genre_names TEXT[] NOT NULL DEFAULT '{}',
                cast_json JSONB NOT NULL DEFAULT '[]'::jsonb,
                source_meta_json JSONB NOT NULL DEFAULT '{}'::jsonb,
                jikan_meta_json JSONB NOT NULL DEFAULT '{}'::jsonb,
                source_fetch_status TEXT NOT NULL DEFAULT 'pending',
                source_fetch_error TEXT NOT NULL DEFAULT '',
                scraped_at TIMESTAMPTZ NOT NULL,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )
        $create$;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_samehadaku_anime_details_mal_id
    ON samehadaku_anime_details (mal_id);

CREATE INDEX IF NOT EXISTS idx_samehadaku_anime_details_updated_at
    ON samehadaku_anime_details (updated_at DESC);
