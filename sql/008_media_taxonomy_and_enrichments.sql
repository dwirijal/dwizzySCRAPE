ALTER TABLE public.media_items
    ADD COLUMN IF NOT EXISTS surface_type text NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS presentation_type text NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS origin_type text NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS release_country text,
    ADD COLUMN IF NOT EXISTS is_nsfw boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS genre_names text[] NOT NULL DEFAULT '{}'::text[],
    ADD COLUMN IF NOT EXISTS taxonomy_confidence smallint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS taxonomy_source text NOT NULL DEFAULT 'legacy_fallback';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'media_items_surface_type_valid'
    ) THEN
        ALTER TABLE public.media_items
            ADD CONSTRAINT media_items_surface_type_valid CHECK (
                surface_type IN ('unknown', 'movie', 'series', 'comic', 'novel')
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'media_items_presentation_type_valid'
    ) THEN
        ALTER TABLE public.media_items
            ADD CONSTRAINT media_items_presentation_type_valid CHECK (
                presentation_type IN ('unknown', 'animation', 'live_action', 'illustrated')
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'media_items_origin_type_not_blank'
    ) THEN
        ALTER TABLE public.media_items
            ADD CONSTRAINT media_items_origin_type_not_blank CHECK (btrim(origin_type) <> '');
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'media_items_taxonomy_source_not_blank'
    ) THEN
        ALTER TABLE public.media_items
            ADD CONSTRAINT media_items_taxonomy_source_not_blank CHECK (btrim(taxonomy_source) <> '');
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'media_items_taxonomy_confidence_valid'
    ) THEN
        ALTER TABLE public.media_items
            ADD CONSTRAINT media_items_taxonomy_confidence_valid CHECK (
                taxonomy_confidence BETWEEN 0 AND 100
            );
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_media_items_surface_updated
    ON public.media_items (surface_type, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_items_surface_year
    ON public.media_items (surface_type, release_year DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_media_items_presentation_country
    ON public.media_items (presentation_type, release_country);
CREATE INDEX IF NOT EXISTS idx_media_items_nsfw
    ON public.media_items (is_nsfw);
CREATE INDEX IF NOT EXISTS idx_media_items_origin_type
    ON public.media_items (origin_type);
CREATE INDEX IF NOT EXISTS idx_media_items_genre_names
    ON public.media_items USING GIN (genre_names);

CREATE TABLE IF NOT EXISTS public.media_item_enrichments (
    item_key text NOT NULL REFERENCES public.media_items(item_key) ON DELETE CASCADE,
    provider text NOT NULL,
    external_id text,
    match_status text NOT NULL DEFAULT 'matched',
    match_score smallint NOT NULL DEFAULT 0,
    matched_title text NOT NULL DEFAULT '',
    matched_year smallint,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT media_item_enrichments_pk PRIMARY KEY (item_key, provider),
    CONSTRAINT media_item_enrichments_provider_not_blank CHECK (btrim(provider) <> ''),
    CONSTRAINT media_item_enrichments_match_status_not_blank CHECK (btrim(match_status) <> ''),
    CONSTRAINT media_item_enrichments_match_score_valid CHECK (match_score BETWEEN 0 AND 100),
    CONSTRAINT media_item_enrichments_matched_year_valid CHECK (
        matched_year IS NULL OR matched_year BETWEEN 1800 AND 9999
    ),
    CONSTRAINT media_item_enrichments_payload_object CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_media_item_enrichments_provider_updated
    ON public.media_item_enrichments (provider, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_item_enrichments_status
    ON public.media_item_enrichments (match_status, updated_at DESC);
