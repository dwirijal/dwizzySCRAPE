ALTER TABLE public.samehadaku_anime_details
    ADD COLUMN IF NOT EXISTS primary_source_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS primary_source_domain TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS secondary_source_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS secondary_source_domain TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS effective_source_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS effective_source_domain TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS effective_source_kind TEXT NOT NULL DEFAULT 'primary';

ALTER TABLE public.samehadaku_episode_details
    ADD COLUMN IF NOT EXISTS primary_source_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS primary_source_domain TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS secondary_source_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS secondary_source_domain TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS effective_source_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS effective_source_domain TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS effective_source_kind TEXT NOT NULL DEFAULT 'primary',
    ADD COLUMN IF NOT EXISTS fetch_status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS fetch_error TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_samehadaku_anime_details_effective_source_kind
    ON public.samehadaku_anime_details (effective_source_kind);

CREATE INDEX IF NOT EXISTS idx_samehadaku_anime_details_source_fetch_status
    ON public.samehadaku_anime_details (source_fetch_status);

CREATE INDEX IF NOT EXISTS idx_samehadaku_episode_details_effective_source_kind
    ON public.samehadaku_episode_details (effective_source_kind);

CREATE INDEX IF NOT EXISTS idx_samehadaku_episode_details_fetch_status
    ON public.samehadaku_episode_details (fetch_status);
