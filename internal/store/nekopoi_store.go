package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/nekopoi"
)

type NekopoiStore struct {
	db contentDB
}

func NewNekopoiStoreWithDB(db contentDB) *NekopoiStore {
	return &NekopoiStore{db: db}
}

func (s *NekopoiStore) UpsertFeedItems(ctx context.Context, items []nekopoi.FeedItem) (int, error) {
	if s.db == nil {
		return 0, fmt.Errorf("content db is required")
	}
	for _, item := range items {
		if err := s.upsertFeedItem(ctx, item); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}

func (s *NekopoiStore) ListFeedItemsForDetailBackfill(ctx context.Context, limit int, missingOnly bool) ([]nekopoi.FeedItem, error) {
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
        title,
        COALESCE(detail->>'normalized_title', '') AS normalized_title,
        COALESCE(detail->'title_labels', '[]'::jsonb) AS title_labels,
        COALESCE(detail->>'entry_kind', '') AS entry_kind,
        COALESCE(detail->>'episode_number', '0')::integer AS episode_number,
        COALESCE(detail->>'part_number', '0')::integer AS part_number,
        COALESCE(detail->>'series_candidate', 'false')::boolean AS series_candidate,
        COALESCE(detail->>'canonical_url', '') AS canonical_url,
        slug,
        COALESCE(cover_url, '') AS cover_url,
        COALESCE(detail->'categories', '[]'::jsonb) AS categories,
        COALESCE(detail->'genres', '[]'::jsonb) AS genres,
        COALESCE(detail->>'content_format', '') AS content_format,
        COALESCE(detail->>'description_html', '') AS description_html,
        COALESCE(detail->>'content_html', '') AS content_html,
        COALESCE(detail->>'description_excerpt', '') AS description_excerpt,
        COALESCE(detail->>'original_title', '') AS original_title,
        COALESCE(detail->>'nuclear_code', '') AS nuclear_code,
        COALESCE(detail->>'actress', '') AS actress,
        COALESCE(detail->>'parody', '') AS parody,
        COALESCE(detail->'producers', '[]'::jsonb) AS producers,
        COALESCE(detail->>'duration', '') AS duration,
        COALESCE(detail->>'size', '') AS size,
        NULLIF(detail->>'published_at', '') AS published_at,
        NULLIF(detail->>'scraped_at', '') AS scraped_at,
        COALESCE(detail->>'post_id', '') AS post_id,
        COALESCE(detail->>'player_count', '0')::integer AS player_count,
        COALESCE(detail->'player_hosts', '[]'::jsonb) AS player_hosts,
        COALESCE(detail->>'download_count', '0')::integer AS download_count,
        COALESCE(detail->'download_labels', '[]'::jsonb) AS download_labels,
        COALESCE(detail->'download_hosts', '[]'::jsonb) AS download_hosts
    FROM public.media_items
    WHERE source = 'nekopoi'
      AND (
        NOT $2 OR
        COALESCE(detail->>'post_id', '') = '' OR
        jsonb_array_length(COALESCE(detail->'player_hosts', '[]'::jsonb)) = 0 OR
        jsonb_array_length(COALESCE(detail->'download_hosts', '[]'::jsonb)) = 0
      )
    ORDER BY
        CASE WHEN COALESCE(detail->>'post_id', '') = '' THEN 0 ELSE 1 END,
        CASE WHEN jsonb_array_length(COALESCE(detail->'player_hosts', '[]'::jsonb)) = 0 THEN 0 ELSE 1 END,
        CASE WHEN jsonb_array_length(COALESCE(detail->'download_hosts', '[]'::jsonb)) = 0 THEN 0 ELSE 1 END,
        updated_at DESC
    LIMIT $1
) q
`, limit, missingOnly).Scan(&payload); err != nil {
		return nil, fmt.Errorf("query nekopoi feed items: %w", err)
	}

	var items []nekopoi.FeedItem
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil, fmt.Errorf("decode nekopoi feed items: %w", err)
	}
	return items, nil
}

func (s *NekopoiStore) upsertFeedItem(ctx context.Context, item nekopoi.FeedItem) error {
	mediaType := nekopoiMediaType(item)
	itemKey := mediaItemKey("nekopoi", mediaType, item.Slug)
	detailMap := map[string]any{
		"canonical_url":       item.CanonicalURL,
		"source_domain":       item.SourceDomain,
		"source_title":        item.Title,
		"normalized_title":    item.NormalizedTitle,
		"title_labels":        item.TitleLabels,
		"entry_kind":          item.EntryKind,
		"episode_number":      item.EpisodeNumber,
		"part_number":         item.PartNumber,
		"series_candidate":    item.SeriesCandidate,
		"cover_url":           item.CoverURL,
		"categories":          item.Categories,
		"tags":                item.Categories,
		"genres":              item.Genres,
		"genre_names":         item.Genres,
		"content_format":      item.ContentFormat,
		"description_html":    item.DescriptionHTML,
		"content_html":        item.ContentHTML,
		"description_excerpt": item.DescriptionExcerpt,
		"original_title":      item.OriginalTitle,
		"nuclear_code":        item.NuclearCode,
		"actress":             item.Actress,
		"parody":              item.Parody,
		"producers":           item.Producers,
		"duration":            item.Duration,
		"size":                item.Size,
		"post_id":             item.PostID,
		"player_count":        item.PlayerCount,
		"player_hosts":        item.PlayerHosts,
		"download_count":      item.DownloadCount,
		"download_labels":     item.DownloadLabels,
		"download_hosts":      item.DownloadHosts,
	}
	if ts := timestampOrNil(item.PublishedAt); ts != nil {
		detailMap["published_at"] = ts
	}
	if ts := timestampOrNil(item.ScrapedAt); ts != nil {
		detailMap["scraped_at"] = ts
	}
	payload, err := json.Marshal(detailMap)
	if err != nil {
		return fmt.Errorf("encode nekopoi payload: %w", err)
	}

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, "nekopoi", mediaType, item.Slug, item.Title, item.CoverURL, "published", publishedYear(item.PublishedAt), float32(0), nil, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_item: %w", err)
	}
	if err := upsertMediaTaxonomy(ctx, s.db, itemKey, ClassifyMediaItem("nekopoi", mediaType, detailMap)); err != nil {
		return fmt.Errorf("update media taxonomy: %w", err)
	}
	return nil
}

func nekopoiMediaType(item nekopoi.FeedItem) string {
	if item.SeriesCandidate {
		return "anime"
	}
	return "movie"
}

func publishedYear(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	year := value.UTC().Year()
	if year < 1800 || year > 9999 {
		return nil
	}
	normalized := int16(year)
	return normalized
}

func timestampOrNil(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}
