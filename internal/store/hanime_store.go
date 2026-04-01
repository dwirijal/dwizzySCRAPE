package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dwirijal/dwizzySCRAPE/internal/hanime"
	"github.com/jackc/pgx/v5"
)

type HanimeStore struct {
	db contentDB
}

func NewHanimeStoreWithDB(db contentDB) *HanimeStore {
	return &HanimeStore{db: db}
}

func (s *HanimeStore) UpsertCatalogItems(ctx context.Context, items []hanime.CatalogItem) (int, error) {
	if s.db == nil {
		return 0, fmt.Errorf("content db is required")
	}
	for _, item := range items {
		if err := s.upsertCatalogItem(ctx, item); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}

func (s *HanimeStore) ListCatalogItemsForDetailBackfill(ctx context.Context, limit int, missingOnly bool) ([]hanime.CatalogItem, error) {
	if s.db == nil {
		return nil, fmt.Errorf("content db is required")
	}
	if limit <= 0 {
		limit = 50
	}

	var payload []byte
	if err := s.db.QueryRow(ctx, `
SELECT COALESCE(json_agg(row_to_json(q)), '[]'::json)::text
FROM (
    SELECT
        COALESCE(detail->>'source_domain', '') AS source_domain,
        COALESCE(detail->>'catalog_source', '') AS catalog_source,
        title,
        COALESCE(detail->>'normalized_title', '') AS normalized_title,
        COALESCE(detail->>'entry_kind', '') AS entry_kind,
        COALESCE(detail->>'episode_number', '0')::integer AS episode_number,
        COALESCE(detail->>'series_candidate', 'false')::boolean AS series_candidate,
        COALESCE(detail->>'canonical_url', '') AS canonical_url,
        slug,
        COALESCE(cover_url, '') AS cover_url,
        COALESCE(detail->>'description', '') AS description,
        COALESCE(detail->>'description_excerpt', '') AS description_excerpt,
        CASE
            WHEN jsonb_typeof(COALESCE(detail->'tags', '[]'::jsonb)) = 'array' THEN COALESCE(detail->'tags', '[]'::jsonb)
            WHEN COALESCE(detail->>'tags', '') <> '' THEN to_jsonb(ARRAY[detail->>'tags'])
            ELSE '[]'::jsonb
        END AS tags,
        COALESCE(detail->>'brand', '') AS brand,
        COALESCE(detail->>'brand_slug', '') AS brand_slug,
        CASE
            WHEN jsonb_typeof(COALESCE(detail->'alternate_titles', '[]'::jsonb)) = 'array' THEN COALESCE(detail->'alternate_titles', '[]'::jsonb)
            WHEN COALESCE(detail->>'alternate_titles', '') <> '' THEN to_jsonb(ARRAY[detail->>'alternate_titles'])
            ELSE '[]'::jsonb
        END AS alternate_titles,
        COALESCE(detail->>'download_present', 'false')::boolean AS download_present,
        COALESCE(detail->>'manifest_present', 'false')::boolean AS manifest_present,
        NULLIF(detail->>'released_at', '') AS released_at,
        NULLIF(detail->>'scraped_at', '') AS scraped_at
    FROM public.media_items
    WHERE source = 'hanime'
      AND (
        NOT $2 OR
        COALESCE(cover_url, '') = '' OR
        COALESCE(detail->>'description', '') = '' OR
        COALESCE(detail->>'description_excerpt', '') = '' OR
        COALESCE(detail->>'brand', '') = '' OR
        CASE
            WHEN jsonb_typeof(COALESCE(detail->'tags', '[]'::jsonb)) = 'array' THEN jsonb_array_length(COALESCE(detail->'tags', '[]'::jsonb))
            WHEN COALESCE(detail->>'tags', '') <> '' THEN 1
            ELSE 0
        END = 0 OR
        COALESCE(detail->>'released_at', '') = ''
      )
    ORDER BY
        CASE WHEN COALESCE(cover_url, '') = '' THEN 0 ELSE 1 END,
        CASE WHEN COALESCE(detail->>'description', '') = '' THEN 0 ELSE 1 END,
        CASE WHEN COALESCE(detail->>'description_excerpt', '') = '' THEN 0 ELSE 1 END,
        CASE WHEN COALESCE(detail->>'brand', '') = '' THEN 0 ELSE 1 END,
        CASE
            WHEN CASE
                WHEN jsonb_typeof(COALESCE(detail->'tags', '[]'::jsonb)) = 'array' THEN jsonb_array_length(COALESCE(detail->'tags', '[]'::jsonb))
                WHEN COALESCE(detail->>'tags', '') <> '' THEN 1
                ELSE 0
            END = 0 THEN 0
            ELSE 1
        END,
        CASE WHEN COALESCE(detail->>'released_at', '') = '' THEN 0 ELSE 1 END,
        updated_at DESC
    LIMIT $1
) q
`, limit, missingOnly).Scan(&payload); err != nil {
		return nil, fmt.Errorf("query hanime catalog items: %w", err)
	}

	var items []hanime.CatalogItem
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil, fmt.Errorf("decode hanime catalog items: %w", err)
	}
	return items, nil
}

func (s *HanimeStore) upsertCatalogItem(ctx context.Context, item hanime.CatalogItem) error {
	mediaType := hanimeMediaType(item)
	itemKey, err := s.resolveItemKey(ctx, item.Slug)
	if err != nil {
		return err
	}
	detailMap := map[string]any{
		"canonical_url":       item.CanonicalURL,
		"source_domain":       item.SourceDomain,
		"catalog_source":      item.CatalogSource,
		"source_title":        item.Title,
		"normalized_title":    item.NormalizedTitle,
		"entry_kind":          item.EntryKind,
		"episode_number":      item.EpisodeNumber,
		"series_candidate":    item.SeriesCandidate,
		"cover_url":           item.CoverURL,
		"description":         item.Description,
		"description_excerpt": item.DescriptionExcerpt,
		"tags":                item.Tags,
		"genres":              item.Tags,
		"content_format":      "animation_hentai",
		"brand":               item.Brand,
		"brand_slug":          item.BrandSlug,
		"alternate_titles":    item.AlternateTitles,
		"download_present":    item.DownloadPresent,
		"manifest_present":    item.ManifestPresent,
	}
	if ts := timestampOrNil(item.ReleasedAt); ts != nil {
		detailMap["released_at"] = ts
		detailMap["published_at"] = ts
	}
	if ts := timestampOrNil(item.ScrapedAt); ts != nil {
		detailMap["scraped_at"] = ts
	}
	payload, err := json.Marshal(detailMap)
	if err != nil {
		return fmt.Errorf("encode hanime payload: %w", err)
	}

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, "hanime", mediaType, item.Slug, item.Title, item.CoverURL, "published", publishedYear(item.ReleasedAt), float32(0), nil, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_item: %w", err)
	}
	if err := upsertMediaTaxonomy(ctx, s.db, itemKey, ClassifyMediaItem("hanime", mediaType, detailMap)); err != nil {
		return fmt.Errorf("update media taxonomy: %w", err)
	}
	return nil
}

func (s *HanimeStore) resolveItemKey(ctx context.Context, slug string) (string, error) {
	if strings.TrimSpace(slug) == "" {
		return "", fmt.Errorf("item slug is required")
	}

	var itemKey string
	err := s.db.QueryRow(ctx, `
SELECT item_key
FROM public.media_items
WHERE source = 'hanime' AND slug = $1
ORDER BY updated_at DESC
LIMIT 1
`, slug).Scan(&itemKey)
	if err == nil && strings.TrimSpace(itemKey) != "" {
		return strings.TrimSpace(itemKey), nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("resolve existing hanime item key: %w", err)
	}
	return mediaItemKey("hanime", "video", slug), nil
}

func hanimeMediaType(item hanime.CatalogItem) string {
	if item.SeriesCandidate {
		return "anime"
	}
	return "movie"
}
