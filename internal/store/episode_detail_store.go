package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
)

type episodeDetailPayload struct {
	AnimeSlug             string          `json:"anime_slug"`
	EpisodeSlug           string          `json:"episode_slug"`
	CanonicalURL          string          `json:"canonical_url"`
	PrimarySourceURL      string          `json:"primary_source_url"`
	PrimarySourceDomain   string          `json:"primary_source_domain"`
	SecondarySourceURL    string          `json:"secondary_source_url"`
	SecondarySourceDomain string          `json:"secondary_source_domain"`
	EffectiveSourceURL    string          `json:"effective_source_url"`
	EffectiveSourceDomain string          `json:"effective_source_domain"`
	EffectiveSourceKind   string          `json:"effective_source_kind"`
	Title                 string          `json:"title"`
	EpisodeNumber         float64         `json:"episode_number"`
	ReleaseLabel          string          `json:"release_label"`
	StreamLinksJSON       json.RawMessage `json:"stream_links_json"`
	DownloadLinksJSON     json.RawMessage `json:"download_links_json"`
	SourceMetaJSON        json.RawMessage `json:"source_meta_json"`
	FetchStatus           string          `json:"fetch_status"`
	FetchError            string          `json:"fetch_error"`
	ScrapedAt             string          `json:"scraped_at"`
}

type EpisodeDetailStore struct {
	client      httpDoer
	supabaseURL string
	secretKey   string
	db          contentDB
}

func NewEpisodeDetailStore(client httpDoer, supabaseURL, secretKey string) *EpisodeDetailStore {
	if client == nil {
		client = http.DefaultClient
	}
	return &EpisodeDetailStore{
		client:      client,
		supabaseURL: strings.TrimRight(strings.TrimSpace(supabaseURL), "/"),
		secretKey:   strings.TrimSpace(secretKey),
	}
}

func NewEpisodeDetailStoreWithDB(db contentDB) *EpisodeDetailStore {
	return &EpisodeDetailStore{db: db}
}

func (s *EpisodeDetailStore) UpsertEpisodeDetails(ctx context.Context, details []samehadaku.EpisodeDetail) (int, error) {
	if len(details) == 0 {
		return 0, nil
	}
	if s.db != nil {
		for _, detail := range details {
			if err := s.upsertEpisodeDetailWithDB(ctx, detail); err != nil {
				return 0, err
			}
		}
		return len(details), nil
	}
	if s.client == nil {
		return 0, fmt.Errorf("http client is required")
	}
	if s.supabaseURL == "" {
		return 0, fmt.Errorf("supabase url is required")
	}
	if s.secretKey == "" {
		return 0, fmt.Errorf("supabase secret key is required")
	}

	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/rpc/upsert_media_unit")
	if err != nil {
		return 0, fmt.Errorf("build rpc endpoint: %w", err)
	}

	for _, detail := range details {
		body, err := json.Marshal(s.episodeMediaUnitPayload(detail))
		if err != nil {
			return 0, fmt.Errorf("encode rpc payload: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
		if err != nil {
			return 0, fmt.Errorf("build upsert request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("apikey", s.secretKey)
		req.Header.Set("Authorization", "Bearer "+s.secretKey)

		resp, err := s.client.Do(req)
		if err != nil {
			return 0, fmt.Errorf("perform rpc request: %w", err)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return 0, fmt.Errorf("read rpc response: %w", readErr)
		}
		if resp.StatusCode >= 300 {
			return 0, fmt.Errorf("supabase rpc failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
	}

	return len(details), nil
}

func (s *EpisodeDetailStore) ListAnimeSlugs(ctx context.Context, offset, limit int) ([]string, error) {
	if s.db != nil {
		if limit <= 0 {
			return nil, fmt.Errorf("limit must be positive")
		}
		if offset < 0 {
			return nil, fmt.Errorf("offset must be non-negative")
		}

		var body []byte
		if err := s.db.QueryRow(ctx, `
SELECT COALESCE(json_agg(row_to_json(q)), '[]'::json)::text
FROM (
    SELECT anime_slug
    FROM (
        SELECT
            detail->>'anime_slug' AS anime_slug,
            MAX(updated_at) AS last_updated
        FROM public.media_units
        WHERE source = 'samehadaku'
          AND unit_type = 'episode'
          AND COALESCE(detail->>'anime_slug', '') <> ''
        GROUP BY detail->>'anime_slug'
    ) units
    ORDER BY last_updated DESC, anime_slug ASC
    LIMIT $1 OFFSET $2
) q
`, limit, offset).Scan(&body); err != nil {
			return nil, fmt.Errorf("query stream-ready slugs: %w", err)
		}

		var rows []struct {
			AnimeSlug string `json:"anime_slug"`
		}
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, fmt.Errorf("decode list response: %w", err)
		}

		seen := make(map[string]struct{}, len(rows))
		slugs := make([]string, 0, len(rows))
		for _, row := range rows {
			slug := strings.TrimSpace(row.AnimeSlug)
			if slug == "" {
				continue
			}
			if _, ok := seen[slug]; ok {
				continue
			}
			seen[slug] = struct{}{}
			slugs = append(slugs, slug)
		}
		return slugs, nil
	}
	if s.client == nil {
		return nil, fmt.Errorf("http client is required")
	}
	if s.supabaseURL == "" {
		return nil, fmt.Errorf("supabase url is required")
	}
	if s.secretKey == "" {
		return nil, fmt.Errorf("supabase secret key is required")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}

	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/media_units")
	if err != nil {
		return nil, fmt.Errorf("build list endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("select", "detail")
	query.Set("source", "eq.samehadaku")
	query.Set("unit_type", "eq.episode")
	query.Set("order", "updated_at.desc,slug.asc")
	query.Set("limit", fmt.Sprintf("%d", limit))
	query.Set("offset", fmt.Sprintf("%d", offset))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build list request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apikey", s.secretKey)
	req.Header.Set("Authorization", "Bearer "+s.secretKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform list request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read list response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("supabase list failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var rows []struct {
		Detail map[string]any `json:"detail"`
	}
	if err := json.Unmarshal(respBody, &rows); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}

	seen := make(map[string]struct{}, len(rows))
	slugs := make([]string, 0, len(rows))
	for _, row := range rows {
		slug := strings.TrimSpace(stringValue(row.Detail["anime_slug"]))
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		slugs = append(slugs, slug)
	}
	return slugs, nil
}

func (s *EpisodeDetailStore) upsertEpisodeDetailWithDB(ctx context.Context, detail samehadaku.EpisodeDetail) error {
	payload, err := json.Marshal(s.episodeMediaUnitPayload(detail)["p_detail"])
	if err != nil {
		return fmt.Errorf("encode media detail: %w", err)
	}
	publishedAt := firstNonBlank(
		normalizePublishedAtFromEmbeddedTimestamp(string(detail.SourceMetaJSON)),
		normalizePublishedAt(detail.ReleaseLabel),
	)

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_unit(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb
)
`, mediaUnitKey("samehadaku", "episode", detail.EpisodeSlug), mediaItemKey("samehadaku", "anime", detail.AnimeSlug), "samehadaku", "episode", detail.EpisodeSlug, detail.Title, detail.ReleaseLabel, detail.EpisodeNumber, detail.CanonicalURL, emptyToNil(publishedAt), nil, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_unit: %w", err)
	}
	return nil
}

func (s *EpisodeDetailStore) episodeMediaUnitPayload(detail samehadaku.EpisodeDetail) map[string]any {
	publishedAt := firstNonBlank(
		normalizePublishedAtFromEmbeddedTimestamp(string(detail.SourceMetaJSON)),
		normalizePublishedAt(detail.ReleaseLabel),
	)
	return map[string]any{
		"p_unit_key":      mediaUnitKey("samehadaku", "episode", detail.EpisodeSlug),
		"p_item_key":      mediaItemKey("samehadaku", "anime", detail.AnimeSlug),
		"p_source":        "samehadaku",
		"p_unit_type":     "episode",
		"p_slug":          detail.EpisodeSlug,
		"p_title":         detail.Title,
		"p_label":         detail.ReleaseLabel,
		"p_number":        detail.EpisodeNumber,
		"p_canonical_url": detail.CanonicalURL,
		"p_published_at":  emptyToNil(publishedAt),
		"p_prev_slug":     nil,
		"p_next_slug":     nil,
		"p_detail": map[string]any{
			"anime_slug":              detail.AnimeSlug,
			"canonical_url":           detail.CanonicalURL,
			"primary_source_url":      detail.PrimarySourceURL,
			"primary_source_domain":   detail.PrimarySourceDomain,
			"secondary_source_url":    detail.SecondarySourceURL,
			"secondary_source_domain": detail.SecondarySourceDomain,
			"effective_source_url":    detail.EffectiveSourceURL,
			"effective_source_domain": detail.EffectiveSourceDomain,
			"effective_source_kind":   detail.EffectiveSourceKind,
			"stream_links_json":       rawJSONOrFallback(detail.StreamLinksJSON, []byte("{}")),
			"download_links_json":     rawJSONOrFallback(detail.DownloadLinksJSON, []byte("{}")),
			"source_meta_json":        rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
			"fetch_status":            detail.FetchStatus,
			"fetch_error":             detail.FetchError,
			"scraped_at":              detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
		},
	}
}

func mediaUnitKey(source, unitType, slug string) string {
	return strings.ToLower(strings.TrimSpace(source)) + ":" + strings.ToLower(strings.TrimSpace(unitType)) + ":" + strings.TrimSpace(slug)
}
