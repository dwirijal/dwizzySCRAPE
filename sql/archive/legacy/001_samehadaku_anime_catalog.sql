DO $$
BEGIN
    IF to_regclass('public.samehadaku_anime_catalog') IS NULL
       AND to_regclass('public.samehadaku_anime_catalog_id_seq') IS NOT NULL THEN
        EXECUTE 'DROP SEQUENCE IF EXISTS public.samehadaku_anime_catalog_id_seq CASCADE';
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'samehadaku_anime_catalog'
          AND c.relkind = 'r'
    ) THEN
        EXECUTE $create$
            CREATE TABLE public.samehadaku_anime_catalog (
                id BIGSERIAL PRIMARY KEY,
                source TEXT NOT NULL,
                source_domain TEXT NOT NULL,
                content_type TEXT NOT NULL DEFAULT 'anime',
                title TEXT NOT NULL,
                canonical_url TEXT NOT NULL UNIQUE,
                slug TEXT NOT NULL UNIQUE,
                poster_url TEXT NOT NULL DEFAULT '',
                anime_type TEXT,
                status TEXT,
                score DOUBLE PRECISION NOT NULL DEFAULT 0,
                views BIGINT NOT NULL DEFAULT 0,
                synopsis_excerpt TEXT NOT NULL DEFAULT '',
                genres TEXT[] NOT NULL DEFAULT '{}',
                scraped_at TIMESTAMPTZ NOT NULL,
                created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )
        $create$;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_samehadaku_anime_catalog_status
    ON samehadaku_anime_catalog (status);

CREATE INDEX IF NOT EXISTS idx_samehadaku_anime_catalog_updated_at
    ON samehadaku_anime_catalog (updated_at DESC);
