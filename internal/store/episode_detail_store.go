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
		payload := make([]episodeDetailPayload, 0, len(details))
		for _, detail := range details {
			payload = append(payload, episodeDetailPayload{
				AnimeSlug:             detail.AnimeSlug,
				EpisodeSlug:           detail.EpisodeSlug,
				CanonicalURL:          detail.CanonicalURL,
				PrimarySourceURL:      detail.PrimarySourceURL,
				PrimarySourceDomain:   detail.PrimarySourceDomain,
				SecondarySourceURL:    detail.SecondarySourceURL,
				SecondarySourceDomain: detail.SecondarySourceDomain,
				EffectiveSourceURL:    detail.EffectiveSourceURL,
				EffectiveSourceDomain: detail.EffectiveSourceDomain,
				EffectiveSourceKind:   detail.EffectiveSourceKind,
				Title:                 detail.Title,
				EpisodeNumber:         detail.EpisodeNumber,
				ReleaseLabel:          detail.ReleaseLabel,
				StreamLinksJSON:       rawJSONOrFallback(detail.StreamLinksJSON, []byte("{}")),
				DownloadLinksJSON:     rawJSONOrFallback(detail.DownloadLinksJSON, []byte("{}")),
				SourceMetaJSON:        rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
				FetchStatus:           detail.FetchStatus,
				FetchError:            detail.FetchError,
				ScrapedAt:             detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
			})
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return 0, fmt.Errorf("encode rpc payload: %w", err)
		}
		var affected int
		if err := s.db.QueryRow(ctx, `SELECT public.upsert_samehadaku_episode_v2($1::jsonb)`, body).Scan(&affected); err != nil {
			return 0, fmt.Errorf("execute upsert_samehadaku_episode_v2: %w", err)
		}
		return affected, nil
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

	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/rpc/upsert_samehadaku_episode_v2")
	if err != nil {
		return 0, fmt.Errorf("build rpc endpoint: %w", err)
	}

	payload := make([]episodeDetailPayload, 0, len(details))
	for _, detail := range details {
		payload = append(payload, episodeDetailPayload{
			AnimeSlug:             detail.AnimeSlug,
			EpisodeSlug:           detail.EpisodeSlug,
			CanonicalURL:          detail.CanonicalURL,
			PrimarySourceURL:      detail.PrimarySourceURL,
			PrimarySourceDomain:   detail.PrimarySourceDomain,
			SecondarySourceURL:    detail.SecondarySourceURL,
			SecondarySourceDomain: detail.SecondarySourceDomain,
			EffectiveSourceURL:    detail.EffectiveSourceURL,
			EffectiveSourceDomain: detail.EffectiveSourceDomain,
			EffectiveSourceKind:   detail.EffectiveSourceKind,
			Title:                 detail.Title,
			EpisodeNumber:         detail.EpisodeNumber,
			ReleaseLabel:          detail.ReleaseLabel,
			StreamLinksJSON:       rawJSONOrFallback(detail.StreamLinksJSON, []byte("{}")),
			DownloadLinksJSON:     rawJSONOrFallback(detail.DownloadLinksJSON, []byte("{}")),
			SourceMetaJSON:        rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
			FetchStatus:           detail.FetchStatus,
			FetchError:            detail.FetchError,
			ScrapedAt:             detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
		})
	}

	body, err := json.Marshal(map[string]any{
		"payload": payload,
	})
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
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read rpc response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("supabase rpc failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
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
    FROM public.anime_stream_ready_v2_view
    ORDER BY anime_slug ASC
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

	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/anime_stream_ready_v2_view")
	if err != nil {
		return nil, fmt.Errorf("build list endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("select", "anime_slug")
	query.Set("order", "anime_slug.asc")
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
		AnimeSlug string `json:"anime_slug"`
	}
	if err := json.Unmarshal(respBody, &rows); err != nil {
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
