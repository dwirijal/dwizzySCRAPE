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
		payload, err := json.Marshal(items)
		if err != nil {
			return 0, fmt.Errorf("encode rpc payload: %w", err)
		}
		var affected int
		if err := s.db.QueryRow(ctx, `SELECT public.upsert_samehadaku_catalog_v2($1::jsonb)`, payload).Scan(&affected); err != nil {
			return 0, fmt.Errorf("execute upsert_samehadaku_catalog_v2: %w", err)
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

	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/rpc/upsert_samehadaku_catalog_v2")
	if err != nil {
		return 0, fmt.Errorf("build rpc endpoint: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"payload": items,
	})
	if err != nil {
		return 0, fmt.Errorf("encode rpc payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read rpc response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("supabase rpc failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return len(items), nil
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
    SELECT title, slug, canonical_url, page_number
    FROM public.anime_catalog_sync_v2_view
    WHERE slug = $1
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
	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/anime_catalog_sync_v2_view")
	if err != nil {
		return samehadaku.CatalogItem{}, fmt.Errorf("build select endpoint: %w", err)
	}
	query := endpoint.Query()
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

	var items []samehadaku.CatalogItem
	if err := json.Unmarshal(body, &items); err != nil {
		return samehadaku.CatalogItem{}, fmt.Errorf("decode select response: %w", err)
	}
	if len(items) == 0 {
		return samehadaku.CatalogItem{}, fmt.Errorf("catalog slug %q not found", slug)
	}
	return items[0], nil
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
    SELECT slug, page_number
    FROM public.anime_catalog_sync_v2_view
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

	endpoint, err := url.Parse(s.supabaseURL + "/rest/v1/anime_catalog_sync_v2_view")
	if err != nil {
		return nil, fmt.Errorf("build list endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("select", "slug,page_number")
	query.Set("order", "page_number.asc,slug.asc")
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

	var items []samehadaku.CatalogItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}
	return items, nil
}
