package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/drakorid"
)

type DrakoridStore struct {
	db contentDB
}

func NewDrakoridStoreWithDB(db contentDB) *DrakoridStore {
	return &DrakoridStore{db: db}
}

func (s *DrakoridStore) HasDetail(ctx context.Context, mediaType, slug string) (bool, error) {
	if s.db == nil {
		return false, fmt.Errorf("content db is required")
	}
	var exists bool
	if err := s.db.QueryRow(ctx, `
SELECT EXISTS (
	SELECT 1
	FROM public.media_items
	WHERE source = 'drakorid'
	  AND media_type = $1
	  AND slug = $2
	  AND COALESCE(detail->>'synopsis', '') <> ''
)
`, strings.TrimSpace(mediaType), strings.TrimSpace(slug)).Scan(&exists); err != nil {
		return false, fmt.Errorf("query drakorid detail existence: %w", err)
	}
	return exists, nil
}

func (s *DrakoridStore) HasEpisodeDetail(ctx context.Context, slug string) (bool, error) {
	if s.db == nil {
		return false, fmt.Errorf("content db is required")
	}
	var exists bool
	if err := s.db.QueryRow(ctx, `
SELECT EXISTS (
	SELECT 1
	FROM public.media_units
	WHERE source = 'drakorid'
	  AND unit_type = 'episode'
	  AND slug = $1
	  AND (
		COALESCE(detail->>'stream_url', '') <> ''
		OR COALESCE(detail->>'download_links_json', '') <> ''
	  )
)
`, strings.TrimSpace(slug)).Scan(&exists); err != nil {
		return false, fmt.Errorf("query drakorid episode existence: %w", err)
	}
	return exists, nil
}

func (s *DrakoridStore) UpsertCatalogItems(ctx context.Context, items []drakorid.CatalogItem) (int, error) {
	for _, item := range items {
		if err := s.upsertCatalogItem(ctx, item); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}

func (s *DrakoridStore) UpsertDetail(ctx context.Context, detail drakorid.Detail) error {
	itemKey := mediaItemKey("drakorid", detail.MediaType, detail.Slug)
	detailMap := map[string]any{
		"canonical_url":    detail.CanonicalURL,
		"source_title":     detail.Title,
		"alt_title":        detail.AltTitle,
		"native_title":     detail.NativeTitle,
		"synopsis":         detail.Synopsis,
		"genre_names":      detail.Genres,
		"category_names":   detail.Categories,
		"country":          detail.Country,
		"language":         detail.Language,
		"aired":            detail.Aired,
		"runtime":          detail.Runtime,
		"format":           detail.Format,
		"director":         detail.Director,
		"network":          detail.Network,
		"episodes_text":    detail.EpisodesText,
		"source_item_id":   detail.SourceItemID,
		"source_type_id":   detail.SourceTypeID,
		"source_meta_json": rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
		"scraped_at":       detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(detailMap)
	if err != nil {
		return fmt.Errorf("encode drakorid detail payload: %w", err)
	}
	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, "drakorid", detail.MediaType, detail.Slug, detail.Title, detail.PosterURL, detail.Status, parseReleaseYear(detail.ReleaseYear), float32(0), nil, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_item: %w", err)
	}
	if err := upsertMediaTaxonomy(ctx, s.db, itemKey, ClassifyMediaItem("drakorid", detail.MediaType, detailMap)); err != nil {
		return fmt.Errorf("update media taxonomy: %w", err)
	}
	return nil
}

func (s *DrakoridStore) UpsertEpisodeDetail(ctx context.Context, detail drakorid.EpisodeDetail) error {
	publishedAt := normalizePublishedAtFromEmbeddedDate(string(detail.SourceMetaJSON))
	payload, err := json.Marshal(map[string]any{
		"item_slug":           detail.ItemSlug,
		"canonical_url":       detail.CanonicalURL,
		"stream_url":          detail.StreamURL,
		"stream_links_json":   rawJSONOrFallback(detail.StreamLinksJSON, []byte("{}")),
		"download_links_json": rawJSONOrFallback(detail.DownloadLinksJSON, []byte("{}")),
		"source_meta_json":    rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
		"scraped_at":          detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("encode drakorid episode payload: %w", err)
	}
	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_unit(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb
)
`, mediaUnitKey("drakorid", "episode", detail.EpisodeSlug), mediaItemKey("drakorid", detail.MediaType, detail.ItemSlug), "drakorid", "episode", detail.EpisodeSlug, detail.Title, detail.Label, detail.EpisodeNumber, detail.CanonicalURL, emptyToNil(publishedAt), nil, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_unit: %w", err)
	}
	return nil
}

func (s *DrakoridStore) upsertCatalogItem(ctx context.Context, item drakorid.CatalogItem) error {
	itemKey := mediaItemKey("drakorid", item.MediaType, item.Slug)
	detailMap := map[string]any{
		"canonical_url": item.CanonicalURL,
		"source_title":  item.Title,
		"source_domain": item.SourceDomain,
		"category":      item.Category,
		"page_number":   item.PageNumber,
		"poster_url":    item.PosterURL,
		"latest_label":  item.LatestLabel,
		"latest_number": item.LatestNumber,
		"scraped_at":    item.ScrapedAt.UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(detailMap)
	if err != nil {
		return fmt.Errorf("encode drakorid catalog payload: %w", err)
	}
	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, "drakorid", item.MediaType, item.Slug, item.Title, item.PosterURL, item.Status, nil, float32(0), nil, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_item: %w", err)
	}
	if err := upsertMediaTaxonomy(ctx, s.db, itemKey, ClassifyMediaItem("drakorid", item.MediaType, detailMap)); err != nil {
		return fmt.Errorf("update media taxonomy: %w", err)
	}
	return nil
}
