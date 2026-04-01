package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/anichin"
)

type AnichinStore struct {
	db contentDB
}

func NewAnichinStoreWithDB(db contentDB) *AnichinStore {
	return &AnichinStore{db: db}
}

func (s *AnichinStore) HasAnimeDetail(ctx context.Context, slug string) (bool, error) {
	if s.db == nil {
		return false, fmt.Errorf("content db is required")
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return false, fmt.Errorf("slug is required")
	}
	var exists bool
	if err := s.db.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM public.media_items
    WHERE source = 'anichin'
      AND media_type = 'anime'
      AND slug = $1
      AND COALESCE(detail->>'synopsis', '') <> ''
)
`, slug).Scan(&exists); err != nil {
		return false, fmt.Errorf("query anime detail existence: %w", err)
	}
	return exists, nil
}

func (s *AnichinStore) HasEpisodeDetail(ctx context.Context, slug string) (bool, error) {
	if s.db == nil {
		return false, fmt.Errorf("content db is required")
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return false, fmt.Errorf("slug is required")
	}
	var exists bool
	if err := s.db.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM public.media_units
    WHERE source = 'anichin'
      AND unit_type = 'episode'
      AND slug = $1
      AND COALESCE(detail->>'stream_url', '') <> ''
)
`, slug).Scan(&exists); err != nil {
		return false, fmt.Errorf("query episode detail existence: %w", err)
	}
	return exists, nil
}

func (s *AnichinStore) UpsertCatalogItems(ctx context.Context, items []anichin.CatalogItem) (int, error) {
	for _, item := range items {
		if err := s.upsertCatalogItem(ctx, item); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}

func (s *AnichinStore) UpsertAnimeDetail(ctx context.Context, detail anichin.AnimeDetail) error {
	itemKey := mediaItemKey("anichin", "anime", detail.Slug)
	detailMap := map[string]any{
		"canonical_url":    detail.CanonicalURL,
		"source_title":     detail.Title,
		"alt_title":        detail.AltTitle,
		"synopsis":         detail.Synopsis,
		"anime_type":       detail.AnimeType,
		"season":           detail.Season,
		"studio_names":     detail.StudioNames,
		"genre_names":      detail.GenreNames,
		"source_meta_json": rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
		"scraped_at":       detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(detailMap)
	if err != nil {
		return fmt.Errorf("encode anime detail payload: %w", err)
	}

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, "anichin", "anime", detail.Slug, detail.Title, detail.PosterURL, detail.Status, parseReleaseYear(detail.ReleasedYear), float32(0), nil, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_item: %w", err)
	}
	if err := upsertMediaTaxonomy(ctx, s.db, itemKey, ClassifyMediaItem("anichin", "anime", detailMap)); err != nil {
		return fmt.Errorf("update media taxonomy: %w", err)
	}
	return nil
}

func (s *AnichinStore) UpsertEpisodeDetail(ctx context.Context, detail anichin.EpisodeDetail) error {
	streamLinksJSON, err := json.Marshal(map[string]any{
		"primary": detail.StreamURL,
		"mirrors": detail.StreamMirrors,
	})
	if err != nil {
		return fmt.Errorf("encode stream links payload: %w", err)
	}
	downloadLinksJSON, err := json.Marshal(detail.DownloadLinks)
	if err != nil {
		return fmt.Errorf("encode download links payload: %w", err)
	}
	payload, err := json.Marshal(map[string]any{
		"anime_slug":          detail.AnimeSlug,
		"canonical_url":       detail.CanonicalURL,
		"stream_url":          detail.StreamURL,
		"stream_links_json":   rawJSONOrFallback(streamLinksJSON, []byte("{}")),
		"download_links_json": rawJSONOrFallback(downloadLinksJSON, []byte("{}")),
		"source_meta_json":    rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
		"scraped_at":          detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("encode episode detail payload: %w", err)
	}
	publishedAt := normalizePublishedAt(detail.ReleaseLabel)

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_unit(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb
)
`, mediaUnitKey("anichin", "episode", detail.EpisodeSlug), mediaItemKey("anichin", "anime", detail.AnimeSlug), "anichin", "episode", detail.EpisodeSlug, detail.Title, detail.ReleaseLabel, detail.EpisodeNumber, detail.CanonicalURL, emptyToNil(publishedAt), nil, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_unit: %w", err)
	}
	return nil
}

func (s *AnichinStore) upsertCatalogItem(ctx context.Context, item anichin.CatalogItem) error {
	itemKey := mediaItemKey("anichin", "anime", item.Slug)
	detailMap := map[string]any{
		"canonical_url": item.CanonicalURL,
		"source_domain": item.SourceDomain,
		"section":       item.Section,
		"page_number":   item.PageNumber,
		"poster_url":    item.PosterURL,
		"anime_type":    item.AnimeType,
		"type_code":     animeTypeCode(item.AnimeType),
		"scraped_at":    item.ScrapedAt.UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(detailMap)
	if err != nil {
		return fmt.Errorf("encode catalog payload: %w", err)
	}

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, "anichin", "anime", item.Slug, item.Title, item.PosterURL, item.Status, nil, float32(0), nil, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_item: %w", err)
	}
	if err := upsertMediaTaxonomy(ctx, s.db, itemKey, ClassifyMediaItem("anichin", "anime", detailMap)); err != nil {
		return fmt.Errorf("update media taxonomy: %w", err)
	}
	return nil
}
