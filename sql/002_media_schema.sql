CREATE TABLE IF NOT EXISTS public.media_items (
    item_key text PRIMARY KEY,
    source text NOT NULL,
    media_type text NOT NULL,
    slug text NOT NULL,
    title text NOT NULL DEFAULT '',
    cover_url text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT '',
    release_year smallint,
    score real NOT NULL DEFAULT 0,
    mal_id bigint,
    tmdb_id bigint,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT media_items_item_key_not_blank CHECK (btrim(item_key) <> ''),
    CONSTRAINT media_items_source_not_blank CHECK (btrim(source) <> ''),
    CONSTRAINT media_items_media_type_not_blank CHECK (btrim(media_type) <> ''),
    CONSTRAINT media_items_slug_not_blank CHECK (btrim(slug) <> ''),
    CONSTRAINT media_items_release_year_valid CHECK (
        release_year IS NULL OR release_year BETWEEN 1800 AND 9999
    ),
    CONSTRAINT media_items_detail_object CHECK (jsonb_typeof(detail) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_media_items_source_type_slug
    ON public.media_items (source, media_type, slug);
CREATE INDEX IF NOT EXISTS idx_media_items_type_updated
    ON public.media_items (media_type, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_items_type_score
    ON public.media_items (media_type, score DESC);
CREATE INDEX IF NOT EXISTS idx_media_items_type_year
    ON public.media_items (media_type, release_year DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_media_items_type_status
    ON public.media_items (media_type, status) WHERE status <> '';
CREATE INDEX IF NOT EXISTS idx_media_items_mal_id
    ON public.media_items (mal_id) WHERE mal_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_media_items_tmdb_id
    ON public.media_items (tmdb_id) WHERE tmdb_id IS NOT NULL;

ALTER TABLE public.media_items ADD COLUMN IF NOT EXISTS search_vec tsvector 
    GENERATED ALWAYS AS (
        to_tsvector('simple', title || ' ' || COALESCE(detail->>'synopsis', '') || ' ' || COALESCE(detail->>'overview', ''))
    ) STORED;
CREATE INDEX IF NOT EXISTS idx_media_items_search ON public.media_items USING GIN(search_vec);


CREATE TABLE IF NOT EXISTS public.media_units (
    unit_key text PRIMARY KEY,
    item_key text NOT NULL REFERENCES public.media_items(item_key) ON DELETE CASCADE,
    source text NOT NULL,
    unit_type text NOT NULL,
    slug text NOT NULL,
    title text NOT NULL DEFAULT '',
    label text NOT NULL DEFAULT '',
    number real,
    canonical_url text NOT NULL DEFAULT '',
    published_at timestamptz,
    prev_slug text,
    next_slug text,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT media_units_unit_key_not_blank CHECK (btrim(unit_key) <> ''),
    CONSTRAINT media_units_source_not_blank CHECK (btrim(source) <> ''),
    CONSTRAINT media_units_unit_type_not_blank CHECK (btrim(unit_type) <> ''),
    CONSTRAINT media_units_slug_not_blank CHECK (btrim(slug) <> ''),
    CONSTRAINT media_units_detail_object CHECK (jsonb_typeof(detail) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_media_units_item_type_slug
    ON public.media_units (item_key, unit_type, slug);
CREATE INDEX IF NOT EXISTS idx_media_units_item_number
    ON public.media_units (item_key, unit_type, number DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_media_units_item_published
    ON public.media_units (item_key, unit_type, published_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_media_units_updated
    ON public.media_units (updated_at DESC);
