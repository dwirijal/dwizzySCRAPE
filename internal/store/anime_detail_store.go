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
		return s.upsertAnimeDetailWithDB(ctx, detail)
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
	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/rpc/upsert_media_item")
	if err != nil {
		return fmt.Errorf("build rpc endpoint: %w", err)
	}

	payload, err := json.Marshal(s.animeMediaItemPayload(detail))
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
    FROM public.media_items
    WHERE source = 'samehadaku'
      AND COALESCE(detail->>'primary_source_url', '') <> ''
    ORDER BY updated_at DESC, slug ASC
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

	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/media_items")
	if err != nil {
		return nil, fmt.Errorf("build list endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("select", "slug,detail")
	query.Set("source", "eq.samehadaku")
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read list response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("supabase list failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rows []struct {
		Slug   string         `json:"slug"`
		Detail map[string]any `json:"detail"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}

	seen := make(map[string]struct{}, len(rows))
	slugs := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(stringValue(row.Detail["primary_source_url"])) == "" {
			continue
		}
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

func (s *AnimeDetailStore) upsertAnimeDetailWithDB(ctx context.Context, detail samehadaku.AnimeDetail) error {
	mediaType := samehadakuMediaType("", detail.AnimeType)
	itemKey := mediaItemKey("samehadaku", mediaType, detail.Slug)
	payloadMap := s.animeMediaItemPayload(detail)
	detailMap, _ := payloadMap["p_detail"].(map[string]any)
	payload, err := json.Marshal(detailMap)
	if err != nil {
		return fmt.Errorf("encode media detail: %w", err)
	}

	var malID any
	if detail.MALID > 0 {
		malID = int64(detail.MALID)
	}

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, "samehadaku", mediaType, detail.Slug, detail.SourceTitle, detail.MALThumbnailURL, detail.Status, nil, float32(0), malID, nil, payload); err != nil {
		return fmt.Errorf("execute upsert_media_item: %w", err)
	}
	if err := upsertMediaTaxonomy(ctx, s.db, itemKey, ClassifyMediaItem("samehadaku", mediaType, detailMap)); err != nil {
		return fmt.Errorf("update media taxonomy: %w", err)
	}
	return nil
}

func (s *AnimeDetailStore) animeMediaItemPayload(detail samehadaku.AnimeDetail) map[string]any {
	mediaType := samehadakuMediaType("", detail.AnimeType)
	return map[string]any{
		"p_item_key":   mediaItemKey("samehadaku", mediaType, detail.Slug),
		"p_source":     "samehadaku",
		"p_media_type": mediaType,
		"p_slug":       detail.Slug,
		"p_title":      detail.SourceTitle,
		"p_cover_url":  detail.MALThumbnailURL,
		"p_status":     detail.Status,
		"p_year":       nil,
		"p_score":      0,
		"p_mal_id": func() any {
			if detail.MALID > 0 {
				return detail.MALID
			}
			return nil
		}(),
		"p_tmdb_id": nil,
		"p_detail": annotateSamehadakuDetail(map[string]any{
			"canonical_url":           detail.CanonicalURL,
			"primary_source_url":      detail.PrimarySourceURL,
			"primary_source_domain":   detail.PrimarySourceDomain,
			"secondary_source_url":    detail.SecondarySourceURL,
			"secondary_source_domain": detail.SecondarySourceDomain,
			"effective_source_url":    detail.EffectiveSourceURL,
			"effective_source_domain": detail.EffectiveSourceDomain,
			"effective_source_kind":   detail.EffectiveSourceKind,
			"source_title":            detail.SourceTitle,
			"mal_url":                 detail.MALURL,
			"mal_thumbnail_url":       detail.MALThumbnailURL,
			"synopsis":                firstNonBlank(detail.SynopsisEnriched, detail.SynopsisSource),
			"synopsis_source":         detail.SynopsisSource,
			"synopsis_enriched":       detail.SynopsisEnriched,
			"anime_type":              detail.AnimeType,
			"type_code":               animeTypeCode(detail.AnimeType),
			"season":                  detail.Season,
			"season_code":             animeSeasonCode(detail.Season),
			"studio_names":            detail.StudioNames,
			"genre_names":             detail.GenreNames,
			"batch_links_json":        rawJSONOrFallback(detail.BatchLinksJSON, []byte("{}")),
			"cast_json":               rawJSONOrFallback(detail.CastJSON, []byte("[]")),
			"source_meta_json":        rawJSONOrFallback(detail.SourceMetaJSON, []byte("{}")),
			"jikan_meta_json":         rawJSONOrFallback(detail.JikanMetaJSON, []byte("{}")),
			"source_fetch_status":     detail.SourceFetchStatus,
			"source_fetch_error":      detail.SourceFetchError,
			"scraped_at":              detail.ScrapedAt.UTC().Format(time.RFC3339Nano),
		}, detail.Slug, detail.SourceTitle),
	}
}

func animeSeasonCode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "winter":
		return "w"
	case "spring":
		return "p"
	case "summer":
		return "s"
	case "fall", "autumn":
		return "f"
	default:
		return ""
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
