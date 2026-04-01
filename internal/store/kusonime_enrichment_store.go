package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/kusonime"
)

type KusonimeEnrichmentStore struct {
	db contentDB
}

func NewKusonimeEnrichmentStoreWithDB(db contentDB) *KusonimeEnrichmentStore {
	return &KusonimeEnrichmentStore{db: db}
}

func (s *KusonimeEnrichmentStore) HasAnimeEnrichment(ctx context.Context, slug string) (bool, error) {
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
    WHERE source = 'samehadaku'
      AND media_type = 'anime'
      AND slug = $1
      AND COALESCE(detail->'batch_sources'->'kusonime', '{}'::jsonb) <> '{}'::jsonb
)
`, slug).Scan(&exists); err != nil {
		return false, fmt.Errorf("query kusonime enrichment existence: %w", err)
	}
	return exists, nil
}

func (s *KusonimeEnrichmentStore) UpsertAnimeEnrichment(
	ctx context.Context,
	review kusonime.ReviewResult,
	scrapedAt time.Time,
) error {
	if s.db == nil {
		return fmt.Errorf("content db is required")
	}
	slug := strings.TrimSpace(review.DBAnimeSlug)
	if slug == "" {
		return fmt.Errorf("anime slug is required")
	}

	var (
		itemKey          string
		currentBatchJSON []byte
	)
	if err := s.db.QueryRow(ctx, `
SELECT
    item_key,
    COALESCE((detail->'batch_sources')::text, '{}'::text)
FROM public.media_items
WHERE source = 'samehadaku'
  AND media_type = 'anime'
  AND slug = $1
LIMIT 1
`, slug).Scan(&itemKey, &currentBatchJSON); err != nil {
		return fmt.Errorf("load existing anime payload: %w", err)
	}

	batchSources, err := decodeJSONMap(currentBatchJSON)
	if err != nil {
		return fmt.Errorf("decode existing batch sources: %w", err)
	}
	batchSources["kusonime"] = buildKusonimeBatchPayload(review, scrapedAt)

	detailPayload, err := json.Marshal(map[string]any{
		"batch_sources": batchSources,
	})
	if err != nil {
		return fmt.Errorf("encode kusonime detail payload: %w", err)
	}

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, "samehadaku", "anime", slug, "", "", "", nil, float32(0), nil, nil, detailPayload); err != nil {
		return fmt.Errorf("execute kusonime enrichment upsert: %w", err)
	}
	return nil
}

func buildKusonimeBatchPayload(review kusonime.ReviewResult, scrapedAt time.Time) map[string]any {
	page := review.Page
	payload := map[string]any{
		"post_url":       strings.TrimSpace(review.MatchedURL),
		"matched_title":  strings.TrimSpace(review.MatchedTitle),
		"match_score":    review.MatchScore,
		"scraped_at":     scrapedAt.UTC().Format(time.RFC3339Nano),
		"poster_url":     strings.TrimSpace(page.PosterURL),
		"japanese_title": strings.TrimSpace(page.JapaneseTitle),
		"synopsis":       strings.TrimSpace(page.Synopsis),
		"genres":         page.Genres,
		"producers":      page.Producers,
		"season":         strings.TrimSpace(page.Season),
		"type":           strings.TrimSpace(page.BatchType),
		"status":         strings.TrimSpace(page.Status),
		"total_episodes": strings.TrimSpace(page.TotalEpisodes),
		"duration":       strings.TrimSpace(page.Duration),
		"released_on":    strings.TrimSpace(page.ReleasedOn),
		"published_at":   strings.TrimSpace(page.PublishedAt),
		"modified_at":    strings.TrimSpace(page.ModifiedAt),
		"batches":        page.Batches,
	}
	if page.Score > 0 {
		payload["score"] = page.Score
	}
	return payload
}
