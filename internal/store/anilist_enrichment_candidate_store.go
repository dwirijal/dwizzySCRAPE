package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type AniListEnrichmentScope string

const (
	AniListEnrichmentScopeAll   AniListEnrichmentScope = "all"
	AniListEnrichmentScopeVideo AniListEnrichmentScope = "video"
	AniListEnrichmentScopeComic AniListEnrichmentScope = "comic"
)

type AniListEnrichmentCandidateOptions struct {
	Scope        AniListEnrichmentScope
	SkipExisting bool
}

type AniListEnrichmentCandidate struct {
	ItemKey            string         `json:"item_key"`
	Source             string         `json:"source"`
	MediaType          string         `json:"media_type"`
	SurfaceType        string         `json:"surface_type"`
	PresentationType   string         `json:"presentation_type"`
	OriginType         string         `json:"origin_type"`
	ReleaseCountry     string         `json:"release_country"`
	Slug               string         `json:"slug"`
	Title              string         `json:"title"`
	ReleaseYear        int            `json:"release_year"`
	TaxonomyConfidence int16          `json:"taxonomy_confidence"`
	TaxonomySource     string         `json:"taxonomy_source"`
	Detail             map[string]any `json:"detail"`
}

type AniListEnrichmentCandidateStore struct {
	db contentDB
}

func NewAniListEnrichmentCandidateStoreWithDB(db contentDB) *AniListEnrichmentCandidateStore {
	return &AniListEnrichmentCandidateStore{db: db}
}

func (s *AniListEnrichmentCandidateStore) ListAniListEnrichmentCandidates(
	ctx context.Context,
	offset, limit int,
	options AniListEnrichmentCandidateOptions,
) ([]AniListEnrichmentCandidate, error) {
	if s.db == nil {
		return nil, fmt.Errorf("content db is required")
	}
	scope := strings.TrimSpace(string(options.Scope))
	if scope == "" {
		scope = string(AniListEnrichmentScopeAll)
	}
	switch AniListEnrichmentScope(scope) {
	case AniListEnrichmentScopeAll, AniListEnrichmentScopeVideo, AniListEnrichmentScopeComic:
	default:
		return nil, fmt.Errorf("unsupported anilist enrichment scope %q", scope)
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
        i.taxonomy_confidence,
        i.taxonomy_source,
        i.detail
    FROM public.media_items i
    WHERE (
        ($1 = 'comic' AND i.surface_type = 'comic')
        OR ($1 = 'video' AND i.presentation_type = 'animation' AND i.surface_type IN ('movie', 'series'))
        OR ($1 = 'all' AND (i.surface_type = 'comic' OR (i.presentation_type = 'animation' AND i.surface_type IN ('movie', 'series'))))
    )
      AND (
        i.taxonomy_confidence < 70
        OR (i.surface_type = 'movie' AND NOT EXISTS (
            SELECT 1
            FROM public.media_item_enrichments e
            WHERE e.item_key = i.item_key
              AND e.provider = 'tmdb'
              AND e.match_status = 'matched'
        ))
        OR (i.surface_type = 'series' AND NOT EXISTS (
            SELECT 1
            FROM public.media_item_enrichments e
            WHERE e.item_key = i.item_key
              AND e.provider = 'jikan'
              AND e.match_status = 'matched'
        ))
      )
      AND (
        NOT $2
        OR NOT EXISTS (
            SELECT 1
            FROM public.media_item_enrichments e
            WHERE e.item_key = i.item_key
              AND e.provider = 'anilist'
              AND e.match_status = 'matched'
        )
      )
    ORDER BY i.updated_at DESC, i.item_key ASC
    OFFSET $3
    LIMIT $4
) q
`, scope, options.SkipExisting, offset, limit).Scan(&payload); err != nil {
		return nil, fmt.Errorf("list anilist enrichment candidates: %w", err)
	}

	var items []AniListEnrichmentCandidate
	if err := json.Unmarshal([]byte(payload), &items); err != nil {
		return nil, fmt.Errorf("decode anilist enrichment candidates: %w", err)
	}
	return items, nil
}
