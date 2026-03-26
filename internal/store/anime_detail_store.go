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

type animeDetailPayload struct {
	Slug                  string          `json:"slug"`
	CanonicalURL          string          `json:"canonical_url"`
	PrimarySourceURL      string          `json:"primary_source_url"`
	PrimarySourceDomain   string          `json:"primary_source_domain"`
	SecondarySourceURL    string          `json:"secondary_source_url"`
	SecondarySourceDomain string          `json:"secondary_source_domain"`
	EffectiveSourceURL    string          `json:"effective_source_url"`
	EffectiveSourceDomain string          `json:"effective_source_domain"`
	EffectiveSourceKind   string          `json:"effective_source_kind"`
	SourceTitle           string          `json:"source_title"`
	MALID                 int             `json:"mal_id,omitempty"`
	MALURL                string          `json:"mal_url"`
	MALThumbnailURL       string          `json:"mal_thumbnail_url"`
	SynopsisSource        string          `json:"synopsis_source"`
	SynopsisEnriched      string          `json:"synopsis_enriched"`
	AnimeType             string          `json:"anime_type,omitempty"`
	Status                string          `json:"status,omitempty"`
	Season                string          `json:"season,omitempty"`
	StudioNames           []string        `json:"studio_names"`
	GenreNames            []string        `json:"genre_names"`
	BatchLinksJSON        json.RawMessage `json:"batch_links_json"`
	CastJSON              json.RawMessage `json:"cast_json"`
	SourceMetaJSON        json.RawMessage `json:"source_meta_json"`
	JikanMetaJSON         json.RawMessage `json:"jikan_meta_json"`
	SourceFetchStatus     string          `json:"source_fetch_status"`
	SourceFetchError      string          `json:"source_fetch_error"`
	ScrapedAt             string          `json:"scraped_at"`
}

type AnimeDetailStore struct {
	client      httpDoer
	supabaseURL string
	secretKey   string
	db          contentDB
}

func NewAnimeDetailStore(client httpDoer, supabaseURL, secretKey string) *AnimeDetailStore {
	if client == nil {
		client = http.DefaultClient
	}
	return &AnimeDetailStore{
		client:      client,
		supabaseURL: strings.TrimRight(strings.TrimSpace(supabaseURL), "/"),
		secretKey:   strings.TrimSpace(secretKey),
	}
}

func NewAnimeDetailStoreWithDB(db contentDB) *AnimeDetailStore {
	return &AnimeDetailStore{db: db}
}

func (s *AnimeDetailStore) UpsertAnimeDetail(ctx context.Context, detail samehadaku.AnimeDetail) error {
	if strings.TrimSpace(detail.Slug) == "" {
		return fmt.Errorf("detail slug is required")
	}
	if s.db != nil {
		payloadItem := animeDetailPayload{
			Slug:                  detail.Slug,
			CanonicalURL:          detail.CanonicalURL,
			PrimarySourceURL:      detail.PrimarySourceURL,
			PrimarySourceDomain:   detail.PrimarySourceDomain,
			SecondarySourceURL:    detail.SecondarySourceURL,
			SecondarySourceDomain: detail.SecondarySourceDomain,
			EffectiveSourceURL:    detail.EffectiveSourceURL,
			EffectiveSourceDomain: detail.EffectiveSourceDomain,
			EffectiveSourceKind:   detail.EffectiveSourceKind,
			SourceTitle:           detail.SourceTitle,
			MALID:                 detail.MALID,
			MALURL:                detail.MALURL,
			MALThumbnailURL:       detail.MALThumbnailURL,
			SynopsisSource:        detail.SynopsisSource,
			SynopsisEnriched:      detail.SynopsisEnriched,
			AnimeType:             detail.AnimeType,
			Status:                detail.Status,
			Season:                detail.Season,
			StudioNames:           detail.StudioNames,
			GenreNames:            detail.GenreNames,
			BatchLinksJSON:        rawJSONOrFallback(detail.BatchLinksJSON, []byte("{}")),
			CastJSON:              rawJSONOrFallback(detail.CastJSON, []byte("[]")),
			SourceMetaJSON:        rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
			JikanMetaJSON:         rawJSONOrFallback(detail.JikanMetaJSON, []byte("{}")),
			SourceFetchStatus:     detail.SourceFetchStatus,
			SourceFetchError:      detail.SourceFetchError,
			ScrapedAt:             detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
		}
		payload, err := json.Marshal([]animeDetailPayload{payloadItem})
		if err != nil {
			return fmt.Errorf("encode rpc payload: %w", err)
		}
		var affected int
		if err := s.db.QueryRow(ctx, `SELECT public.upsert_samehadaku_anime_detail_v2($1::jsonb)`, payload).Scan(&affected); err != nil {
			return fmt.Errorf("execute upsert_samehadaku_anime_detail_v2: %w", err)
		}
		return nil
	}
	if s.client == nil {
		return fmt.Errorf("http client is required")
	}
	if s.supabaseURL == "" {
		return fmt.Errorf("supabase url is required")
	}
	if s.secretKey == "" {
		return fmt.Errorf("supabase secret key is required")
	}
	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/rpc/upsert_samehadaku_anime_detail_v2")
	if err != nil {
		return fmt.Errorf("build rpc endpoint: %w", err)
	}

	payloadItem := animeDetailPayload{
		Slug:                  detail.Slug,
		CanonicalURL:          detail.CanonicalURL,
		PrimarySourceURL:      detail.PrimarySourceURL,
		PrimarySourceDomain:   detail.PrimarySourceDomain,
		SecondarySourceURL:    detail.SecondarySourceURL,
		SecondarySourceDomain: detail.SecondarySourceDomain,
		EffectiveSourceURL:    detail.EffectiveSourceURL,
		EffectiveSourceDomain: detail.EffectiveSourceDomain,
		EffectiveSourceKind:   detail.EffectiveSourceKind,
		SourceTitle:           detail.SourceTitle,
		MALID:                 detail.MALID,
		MALURL:                detail.MALURL,
		MALThumbnailURL:       detail.MALThumbnailURL,
		SynopsisSource:        detail.SynopsisSource,
		SynopsisEnriched:      detail.SynopsisEnriched,
		AnimeType:             detail.AnimeType,
		Status:                detail.Status,
		Season:                detail.Season,
		StudioNames:           detail.StudioNames,
		GenreNames:            detail.GenreNames,
		BatchLinksJSON:        rawJSONOrFallback(detail.BatchLinksJSON, []byte("{}")),
		CastJSON:              rawJSONOrFallback(detail.CastJSON, []byte("[]")),
		SourceMetaJSON:        rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
		JikanMetaJSON:         rawJSONOrFallback(detail.JikanMetaJSON, []byte("{}")),
		SourceFetchStatus:     detail.SourceFetchStatus,
		SourceFetchError:      detail.SourceFetchError,
		ScrapedAt:             detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
	}

	payload, err := json.Marshal(map[string]any{
		"payload": []animeDetailPayload{payloadItem},
	})
	if err != nil {
		return fmt.Errorf("encode rpc payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build upsert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", s.secretKey)
	req.Header.Set("Authorization", "Bearer "+s.secretKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("perform rpc request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read rpc response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("supabase rpc failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

func (s *AnimeDetailStore) ListAnimeSlugs(ctx context.Context, offset, limit int) ([]string, error) {
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
    SELECT slug
    FROM public.anime_detail_ready_v2_view
    ORDER BY slug ASC
    LIMIT $1 OFFSET $2
) q
`, limit, offset).Scan(&body); err != nil {
			return nil, fmt.Errorf("query detail-ready slugs: %w", err)
		}

		var rows []struct {
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, fmt.Errorf("decode list response: %w", err)
		}

		seen := make(map[string]struct{}, len(rows))
		slugs := make([]string, 0, len(rows))
		for _, row := range rows {
			slug := strings.TrimSpace(row.Slug)
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

	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/anime_detail_ready_v2_view")
	if err != nil {
		return nil, fmt.Errorf("build list endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("select", "slug")
	query.Set("order", "slug.asc")
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read list response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("supabase list failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rows []struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}

	seen := make(map[string]struct{}, len(rows))
	slugs := make([]string, 0, len(rows))
	for _, row := range rows {
		slug := strings.TrimSpace(row.Slug)
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

func rawJSONOrFallback(value []byte, fallback []byte) json.RawMessage {
	if len(bytes.TrimSpace(value)) == 0 {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(value)
}
