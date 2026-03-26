ALTER TABLE samehadaku_anime_catalog
    ADD COLUMN IF NOT EXISTS page_number INTEGER NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_samehadaku_anime_catalog_page_number
    ON samehadaku_anime_catalog (page_number);
