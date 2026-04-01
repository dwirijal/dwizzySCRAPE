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

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type CatalogStore struct {
	client      httpDoer
	supabaseURL string
	secretKey   string
	db          contentDB
}

func NewCatalogStore(client httpDoer, supabaseURL, secretKey string) *CatalogStore {
	if client == nil {
		client = http.DefaultClient
	}
	return &CatalogStore{
		client:      client,
		supabaseURL: strings.TrimRight(strings.TrimSpace(supabaseURL), "/"),
		secretKey:   strings.TrimSpace(secretKey),
	}
}

func NewCatalogStoreWithDB(db contentDB) *CatalogStore {
	return &CatalogStore{db: db}
}

func (s *CatalogStore) UpsertCatalog(ctx context.Context, items []samehadaku.CatalogItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	if s.db != nil {
		for _, item := range items {
			if err := s.upsertCatalogItemWithDB(ctx, item); err != nil {
				return 0, err
			}
		}
		return len(items), nil
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

	for _, item := range items {
		if err := s.upsertCatalogItemHTTP(ctx, item); err != nil {
			return 0, err
		}
	}

	return len(items), nil
}

func (s *CatalogStore) upsertCatalogItemWithDB(ctx context.Context, item samehadaku.CatalogItem) error {
	mediaType := samehadakuMediaType(item.ContentType, item.AnimeType)
	itemKey := mediaItemKey("samehadaku", mediaType, item.Slug)
	detailMap := annotateSamehadakuDetail(map[string]any{
		"canonical_url":    item.CanonicalURL,
		"source_domain":    item.SourceDomain,
		"content_type":     item.ContentType,
		"page_number":      item.PageNumber,
		"poster_url":       item.PosterURL,
		"anime_type":       item.AnimeType,
		"type_code":        animeTypeCode(item.AnimeType),
		"status_label":     item.Status,
		"views":            item.Views,
		"synopsis":         item.SynopsisExcerpt,
		"synopsis_excerpt": item.SynopsisExcerpt,
		"genres":           item.Genres,
		"scraped_at":       item.ScrapedAt.UTC().Format(time.RFC3339Nano),
	}, item.Slug, item.Title)
	detail, err := json.Marshal(detailMap)
	if err != nil {
		return fmt.Errorf("encode media detail: %w", err)
	}

	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, "samehadaku", mediaType, item.Slug, item.Title, item.PosterURL, item.Status, nil, float32(item.Score), nil, nil, detail); err != nil {
		return fmt.Errorf("execute upsert_media_item: %w", err)
	}
	if err := upsertMediaTaxonomy(ctx, s.db, itemKey, ClassifyMediaItem("samehadaku", mediaType, detailMap)); err != nil {
		return fmt.Errorf("update media taxonomy: %w", err)
	}
	return nil
}

func (s *CatalogStore) upsertCatalogItemHTTP(ctx context.Context, item samehadaku.CatalogItem) error {
	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/rpc/upsert_media_item")
	if err != nil {
		return fmt.Errorf("build rpc endpoint: %w", err)
	}

	mediaType := samehadakuMediaType(item.ContentType, item.AnimeType)
	payload := map[string]any{
		"p_item_key":   mediaItemKey("samehadaku", mediaType, item.Slug),
		"p_source":     "samehadaku",
		"p_media_type": mediaType,
		"p_slug":       item.Slug,
		"p_title":      item.Title,
		"p_cover_url":  item.PosterURL,
		"p_status":     item.Status,
		"p_year":       nil,
		"p_score":      item.Score,
		"p_mal_id":     nil,
		"p_tmdb_id":    nil,
		"p_detail": annotateSamehadakuDetail(map[string]any{
			"canonical_url":    item.CanonicalURL,
			"source_domain":    item.SourceDomain,
			"content_type":     item.ContentType,
			"page_number":      item.PageNumber,
			"poster_url":       item.PosterURL,
			"anime_type":       item.AnimeType,
			"type_code":        animeTypeCode(item.AnimeType),
			"status_label":     item.Status,
			"views":            item.Views,
			"synopsis":         item.SynopsisExcerpt,
			"synopsis_excerpt": item.SynopsisExcerpt,
			"genres":           item.Genres,
			"scraped_at":       item.ScrapedAt.UTC().Format(time.RFC3339Nano),
		}, item.Slug, item.Title),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode rpc payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read rpc response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("supabase rpc failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (s *CatalogStore) GetCatalogBySlug(ctx context.Context, slug string) (samehadaku.CatalogItem, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return samehadaku.CatalogItem{}, fmt.Errorf("slug is required")
	}
	if s.db != nil {
		var payload []byte
		if err := s.db.QueryRow(ctx, `
SELECT COALESCE(json_agg(row_to_json(q)), '[]'::json)::text
FROM (
    SELECT
        title,
        slug,
        COALESCE(detail->>'canonical_url', '') AS canonical_url,
        COALESCE((detail->>'page_number')::integer, 0) AS page_number,
        COALESCE(detail->>'content_type', media_type) AS content_type,
        COALESCE(detail->>'anime_type', '') AS anime_type
    FROM public.media_items
    WHERE source = 'samehadaku'
      AND slug = $1
    LIMIT 1
) q
`, slug).Scan(&payload); err != nil {
			return samehadaku.CatalogItem{}, fmt.Errorf("query catalog by slug: %w", err)
		}

		var items []samehadaku.CatalogItem
		if err := json.Unmarshal(payload, &items); err != nil {
			return samehadaku.CatalogItem{}, fmt.Errorf("decode select response: %w", err)
		}
		if len(items) == 0 {
			return samehadaku.CatalogItem{}, fmt.Errorf("catalog slug %q not found", slug)
		}
		return items[0], nil
	}
	if s.client == nil {
		return samehadaku.CatalogItem{}, fmt.Errorf("http client is required")
	}
	if s.supabaseURL == "" {
		return samehadaku.CatalogItem{}, fmt.Errorf("supabase url is required")
	}
	if s.secretKey == "" {
		return samehadaku.CatalogItem{}, fmt.Errorf("supabase secret key is required")
	}
	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/media_items")
	if err != nil {
		return samehadaku.CatalogItem{}, fmt.Errorf("build select endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("select", "title,slug,detail")
	query.Set("source", "eq.samehadaku")
	query.Set("slug", "eq."+slug)
	query.Set("limit", "1")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return samehadaku.CatalogItem{}, fmt.Errorf("build select request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apikey", s.secretKey)
	req.Header.Set("Authorization", "Bearer "+s.secretKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return samehadaku.CatalogItem{}, fmt.Errorf("perform select request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return samehadaku.CatalogItem{}, fmt.Errorf("read select response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return samehadaku.CatalogItem{}, fmt.Errorf("supabase select failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rows []struct {
		Title  string          `json:"title"`
		Slug   string          `json:"slug"`
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return samehadaku.CatalogItem{}, fmt.Errorf("decode select response: %w", err)
	}
	if len(rows) == 0 {
		return samehadaku.CatalogItem{}, fmt.Errorf("catalog slug %q not found", slug)
	}
	return decodeCatalogItemRow(rows[0].Title, rows[0].Slug, rows[0].Detail), nil
}

func (s *CatalogStore) ListCatalogSlugs(ctx context.Context, offset, limit int) ([]samehadaku.CatalogItem, error) {
	if s.db != nil {
		if limit <= 0 {
			return nil, fmt.Errorf("limit must be positive")
		}
		if offset < 0 {
			return nil, fmt.Errorf("offset must be non-negative")
		}

		var payload []byte
		if err := s.db.QueryRow(ctx, `
SELECT COALESCE(json_agg(row_to_json(q)), '[]'::json)::text
FROM (
    SELECT
        slug,
        COALESCE((detail->>'page_number')::integer, 0) AS page_number,
        COALESCE(detail->>'content_type', media_type) AS content_type,
        COALESCE(detail->>'anime_type', '') AS anime_type
    FROM public.media_items
    WHERE source = 'samehadaku'
    ORDER BY page_number ASC, slug ASC
    LIMIT $1 OFFSET $2
) q
`, limit, offset).Scan(&payload); err != nil {
			return nil, fmt.Errorf("query catalog slugs: %w", err)
		}

		var items []samehadaku.CatalogItem
		if err := json.Unmarshal(payload, &items); err != nil {
			return nil, fmt.Errorf("decode list response: %w", err)
		}
		return items, nil
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
		Slug   string          `json:"slug"`
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}

	items := make([]samehadaku.CatalogItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, decodeCatalogItemRow("", row.Slug, row.Detail))
	}
	return items, nil
}

func decodeCatalogItemRow(title, slug string, detailRaw json.RawMessage) samehadaku.CatalogItem {
	var detail struct {
		CanonicalURL string `json:"canonical_url"`
		PageNumber   int    `json:"page_number"`
		ContentType  string `json:"content_type"`
		AnimeType    string `json:"anime_type"`
	}
	_ = json.Unmarshal(detailRaw, &detail)
	return samehadaku.CatalogItem{
		Title:        title,
		Slug:         slug,
		CanonicalURL: detail.CanonicalURL,
		PageNumber:   detail.PageNumber,
		ContentType:  detail.ContentType,
		AnimeType:    detail.AnimeType,
	}
}

func mediaItemKey(source, mediaType, slug string) string {
	return strings.ToLower(strings.TrimSpace(source)) + ":" + strings.ToLower(strings.TrimSpace(mediaType)) + ":" + strings.TrimSpace(slug)
}

func animeTypeCode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "tv":
		return "t"
	case "movie":
		return "m"
	case "ova":
		return "o"
	case "ona":
		return "n"
	case "special":
		return "p"
	default:
		return ""
	}
}

func samehadakuMediaType(contentType, animeType string) string {
	if strings.EqualFold(strings.TrimSpace(contentType), "movie") {
		return "movie"
	}
	if strings.EqualFold(strings.TrimSpace(animeType), "movie") {
		return "movie"
	}
	return "anime"
}
