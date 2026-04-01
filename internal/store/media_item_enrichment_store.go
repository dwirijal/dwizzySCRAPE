package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type MediaItemEnrichmentRecord struct {
	ItemKey      string
	Provider     string
	ExternalID   string
	MatchStatus  string
	MatchScore   int
	MatchedTitle string
	MatchedYear  int
	Payload      map[string]any
	UpdatedAt    time.Time
}

type MediaItemEnrichmentStore struct {
	db contentDB
}

func NewMediaItemEnrichmentStoreWithDB(db contentDB) *MediaItemEnrichmentStore {
	return &MediaItemEnrichmentStore{db: db}
}

func (s *MediaItemEnrichmentStore) HasItemEnrichment(ctx context.Context, itemKey, provider string) (bool, error) {
	if s.db == nil {
		return false, fmt.Errorf("content db is required")
	}

	itemKey = strings.TrimSpace(itemKey)
	provider = strings.TrimSpace(provider)
	if itemKey == "" {
		return false, fmt.Errorf("item key is required")
	}
	if provider == "" {
		return false, fmt.Errorf("provider is required")
	}

	var exists bool
	if err := s.db.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM public.media_item_enrichments
    WHERE item_key = $1
      AND provider = $2
)
`, itemKey, provider).Scan(&exists); err != nil {
		return false, fmt.Errorf("query media item enrichment existence: %w", err)
	}
	return exists, nil
}

func (s *MediaItemEnrichmentStore) UpsertItemEnrichment(ctx context.Context, record MediaItemEnrichmentRecord) error {
	if s.db == nil {
		return fmt.Errorf("content db is required")
	}

	record.ItemKey = strings.TrimSpace(record.ItemKey)
	record.Provider = strings.TrimSpace(record.Provider)
	record.ExternalID = strings.TrimSpace(record.ExternalID)
	record.MatchStatus = strings.TrimSpace(record.MatchStatus)
	record.MatchedTitle = strings.TrimSpace(record.MatchedTitle)
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = time.Now().UTC()
	}
	if record.ItemKey == "" {
		return fmt.Errorf("item key is required")
	}
	if record.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if record.MatchStatus == "" {
		record.MatchStatus = "matched"
	}
	if record.Payload == nil {
		record.Payload = map[string]any{}
	}

	payload, err := json.Marshal(record.Payload)
	if err != nil {
		return fmt.Errorf("marshal enrichment payload: %w", err)
	}

	var matchedYear any
	if record.MatchedYear > 0 {
		matchedYear = int16(record.MatchedYear)
	}

	if err := s.db.Exec(ctx, `
INSERT INTO public.media_item_enrichments (
    item_key,
    provider,
    external_id,
    match_status,
    match_score,
    matched_title,
    matched_year,
    payload,
    created_at,
    updated_at
)
VALUES (
    $1, $2, NULLIF($3, ''), $4, $5, NULLIF($6, ''), $7, $8::jsonb, $9, $9
)
ON CONFLICT (item_key, provider) DO UPDATE
SET
    external_id = EXCLUDED.external_id,
    match_status = EXCLUDED.match_status,
    match_score = EXCLUDED.match_score,
    matched_title = EXCLUDED.matched_title,
    matched_year = EXCLUDED.matched_year,
    payload = EXCLUDED.payload,
    updated_at = EXCLUDED.updated_at
`, record.ItemKey, record.Provider, record.ExternalID, record.MatchStatus, record.MatchScore, record.MatchedTitle, matchedYear, payload, record.UpdatedAt.UTC()); err != nil {
		return fmt.Errorf("upsert media item enrichment: %w", err)
	}
	return nil
}
