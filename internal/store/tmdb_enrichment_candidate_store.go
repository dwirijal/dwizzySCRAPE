package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type TMDBEnrichmentScope string

const (
	TMDBEnrichmentScopeAll    TMDBEnrichmentScope = "all"
	TMDBEnrichmentScopeMovie  TMDBEnrichmentScope = "movie"
	TMDBEnrichmentScopeSeries TMDBEnrichmentScope = "series"
)

type TMDBEnrichmentCandidateOptions struct {
	Scope        TMDBEnrichmentScope
	SkipExisting bool
}

type TMDBEnrichmentCandidate struct {
	ItemKey          string         `json:"item_key"`
	Source           string         `json:"source"`
	MediaType        string         `json:"media_type"`
	SurfaceType      string         `json:"surface_type"`
	PresentationType string         `json:"presentation_type"`
	OriginType       string         `json:"origin_type"`
	ReleaseCountry   string         `json:"release_country"`
	Slug             string         `json:"slug"`
	Title            string         `json:"title"`
	ReleaseYear      int            `json:"release_year"`
	Detail           map[string]any `json:"detail"`
}

type TMDBEnrichmentCandidateStore struct {
	db contentDB
}

func NewTMDBEnrichmentCandidateStoreWithDB(db contentDB) *TMDBEnrichmentCandidateStore {
	return &TMDBEnrichmentCandidateStore{db: db}
}

func (s *TMDBEnrichmentCandidateStore) ListTMDBEnrichmentCandidates(
	ctx context.Context,
	offset, limit int,
	options TMDBEnrichmentCandidateOptions,
) ([]TMDBEnrichmentCandidate, error) {
	if s.db == nil {
		return nil, fmt.Errorf("content db is required")
	}
	scope := strings.TrimSpace(string(options.Scope))
	if scope == "" {
		scope = string(TMDBEnrichmentScopeAll)
	}
	switch TMDBEnrichmentScope(scope) {
	case TMDBEnrichmentScopeAll, TMDBEnrichmentScopeMovie, TMDBEnrichmentScopeSeries:
	default:
		return nil, fmt.Errorf("unsupported tmdb enrichment scope %q", scope)
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 100
	}

	var payload string
	if err := s.db.QueryRow(ctx, `
SELECT COALESCE(json_agg(row_to_json(q)), '[]'::json)::text
FROM (
    SELECT
        i.item_key,
        i.source,
        i.media_type,
        i.surface_type,
        i.presentation_type,
        i.origin_type,
        i.release_country,
        i.slug,
        i.title,
        COALESCE(i.release_year::integer, 0) AS release_year,
        i.detail
    FROM public.media_items i
    WHERE (
        ($1 = 'movie' AND i.surface_type = 'movie')
        OR ($1 = 'series' AND i.surface_type = 'series' AND i.source = 'drakorid' AND coalesce(i.origin_type, '') <> 'variety')
        OR ($1 = 'all' AND (i.surface_type = 'movie' OR (i.surface_type = 'series' AND i.source = 'drakorid' AND coalesce(i.origin_type, '') <> 'variety')))
    )
      AND (
        NOT $2
        OR NOT EXISTS (
            SELECT 1
            FROM public.media_item_enrichments e
            WHERE e.item_key = i.item_key
              AND e.provider = 'tmdb'
              AND e.match_status = 'matched'
        )
      )
    ORDER BY i.updated_at DESC, i.item_key ASC
    OFFSET $3
    LIMIT $4
) q
`, scope, options.SkipExisting, offset, limit).Scan(&payload); err != nil {
		return nil, fmt.Errorf("list tmdb enrichment candidates: %w", err)
	}

	var items []TMDBEnrichmentCandidate
	if err := json.Unmarshal([]byte(payload), &items); err != nil {
		return nil, fmt.Errorf("decode tmdb enrichment candidates: %w", err)
	}
	return items, nil
}
