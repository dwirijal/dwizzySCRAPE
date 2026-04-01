package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/otakudesu"
)

type OtakudesuEnrichmentStore struct {
	db contentDB
}

func NewOtakudesuEnrichmentStoreWithDB(db contentDB) *OtakudesuEnrichmentStore {
	return &OtakudesuEnrichmentStore{db: db}
}

func (s *OtakudesuEnrichmentStore) HasEpisodeEnrichment(ctx context.Context, slug string) (bool, error) {
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
    WHERE source = 'samehadaku'
      AND unit_type = 'episode'
      AND slug = $1
      AND COALESCE(detail->'source_meta_json'->'otakudesu', '{}'::jsonb) <> '{}'::jsonb
)
`, slug).Scan(&exists); err != nil {
		return false, fmt.Errorf("query otakudesu enrichment existence: %w", err)
	}
	return exists, nil
}

func (s *OtakudesuEnrichmentStore) UpsertEpisodeEnrichment(
	ctx context.Context,
	anime otakudesu.ReviewResult,
	episode otakudesu.EpisodeReview,
	scrapedAt time.Time,
) error {
	if s.db == nil {
		return fmt.Errorf("content db is required")
	}
	episodeSlug := strings.TrimSpace(episode.DBEpisodeSlug)
	if episodeSlug == "" {
		return fmt.Errorf("episode slug is required")
	}

	var (
		itemKey             string
		currentStreamJSON   []byte
		currentDownloadJSON []byte
		currentMetaJSON     []byte
	)
	if err := s.db.QueryRow(ctx, `
SELECT
    item_key,
    COALESCE((detail->'stream_links_json')::text, '{}'::text),
    COALESCE((detail->'download_links_json')::text, '{}'::text),
    COALESCE((detail->'source_meta_json')::text, '{}'::text)
FROM public.media_units
WHERE source = 'samehadaku'
  AND unit_type = 'episode'
  AND slug = $1
LIMIT 1
`, episodeSlug).Scan(&itemKey, &currentStreamJSON, &currentDownloadJSON, &currentMetaJSON); err != nil {
		return fmt.Errorf("load existing episode payload: %w", err)
	}

	streamLinks, err := decodeJSONMap(currentStreamJSON)
	if err != nil {
		return fmt.Errorf("decode existing stream links: %w", err)
	}
	downloadLinks, err := decodeJSONMap(currentDownloadJSON)
	if err != nil {
		return fmt.Errorf("decode existing download links: %w", err)
	}
	sourceMeta, err := decodeJSONMap(currentMetaJSON)
	if err != nil {
		return fmt.Errorf("decode existing source meta: %w", err)
	}

	streamLinks["otakudesu"] = buildOtakudesuStreamPayload(episode)
	downloadLinks["otakudesu"] = buildOtakudesuDownloadPayload(episode)
	sourceMeta["otakudesu"] = map[string]any{
		"match_score":   anime.MatchScore,
		"matched_title": strings.TrimSpace(anime.MatchedTitle),
		"anime_url":     strings.TrimSpace(anime.MatchedURL),
		"episode_url":   strings.TrimSpace(episode.OtakudesuEpisodeURL),
		"scraped_at":    scrapedAt.UTC().Format(time.RFC3339Nano),
	}

	streamPayload, err := json.Marshal(streamLinks)
	if err != nil {
		return fmt.Errorf("encode merged stream links: %w", err)
	}
	downloadPayload, err := json.Marshal(downloadLinks)
	if err != nil {
		return fmt.Errorf("encode merged download links: %w", err)
	}
	metaPayload, err := json.Marshal(sourceMeta)
	if err != nil {
		return fmt.Errorf("encode merged source meta: %w", err)
	}

	detailPayload, err := json.Marshal(map[string]any{
		"stream_links_json":   rawJSONOrFallback(streamPayload, []byte("{}")),
		"download_links_json": rawJSONOrFallback(downloadPayload, []byte("{}")),
		"source_meta_json":    rawJSONOrFallback(metaPayload, []byte("{}")),
	})
	if err != nil {
		return fmt.Errorf("encode otakudesu detail payload: %w", err)
	}

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_unit(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb
)
`, mediaUnitKey("samehadaku", "episode", episodeSlug), itemKey, "samehadaku", "episode", episodeSlug, "", "", nil, "", nil, nil, nil, detailPayload); err != nil {
		return fmt.Errorf("execute otakudesu enrichment upsert: %w", err)
	}
	return nil
}

func buildOtakudesuStreamPayload(episode otakudesu.EpisodeReview) map[string]any {
	payload := make(map[string]any)
	if primary := strings.TrimSpace(episode.StreamURL); primary != "" {
		payload["primary"] = primary
	}
	if len(episode.StreamMirrors) > 0 {
		payload["mirrors"] = episode.StreamMirrors
	}
	return payload
}

func buildOtakudesuDownloadPayload(episode otakudesu.EpisodeReview) map[string]any {
	if len(episode.DownloadLinks) == 0 {
		return map[string]any{}
	}
	payload := make(map[string]any, len(episode.DownloadLinks))
	for quality, hosts := range episode.DownloadLinks {
		trimmedQuality := strings.TrimSpace(quality)
		if trimmedQuality == "" {
			trimmedQuality = "unknown"
		}
		payload[trimmedQuality] = hosts
	}
	return payload
}

func decodeJSONMap(raw []byte) (map[string]any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return map[string]any{}, nil
	}
	out := make(map[string]any)
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, err
	}
	return out, nil
}
