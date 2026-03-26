package store

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
)

func TestCatalogStoreUpsert(t *testing.T) {
	t.Helper()

	scrapedAt := time.Date(2026, 3, 22, 15, 30, 0, 0, time.UTC)
	items := []samehadaku.CatalogItem{
		{
			Source:          "samehadaku",
			SourceDomain:    "v2.samehadaku.how",
			ContentType:     "anime",
			Title:           "#Compass2.0 Animation Project",
			CanonicalURL:    "https://v2.samehadaku.how/anime/compass2-0-animation-project/",
			Slug:            "compass2-0-animation-project",
			PageNumber:      1,
			PosterURL:       "https://cdn.samehadaku.example/posters/compass2.webp",
			AnimeType:       "TV",
			Status:          "Completed",
			Score:           6.01,
			Views:           884361,
			SynopsisExcerpt: "sample synopsis",
			Genres:          []string{"Action", "Strategy Game"},
			ScrapedAt:       scrapedAt,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/rest/v1/rpc/upsert_samehadaku_catalog_v2" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("apikey"); got != "sb_secret_test" {
			t.Fatalf("unexpected apikey header %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sb_secret_test" {
			t.Fatalf("unexpected authorization header %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload map[string][]map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		itemsPayload := payload["payload"]
		if len(itemsPayload) != 1 {
			t.Fatalf("expected single payload item, got %d", len(itemsPayload))
		}
		if got := itemsPayload[0]["poster_url"]; got != "https://cdn.samehadaku.example/posters/compass2.webp" {
			t.Fatalf("unexpected poster_url %#v", got)
		}
		if got := itemsPayload[0]["canonical_url"]; got != "https://v2.samehadaku.how/anime/compass2-0-animation-project/" {
			t.Fatalf("unexpected canonical_url %#v", got)
		}
		if got := itemsPayload[0]["page_number"]; got != float64(1) {
			t.Fatalf("unexpected page_number %#v", got)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	store := NewCatalogStore(server.Client(), server.URL, "sb_secret_test")
	upserted, err := store.UpsertCatalog(context.Background(), items)
	if err != nil {
		t.Fatalf("UpsertCatalog returned error: %v", err)
	}
	if upserted != 1 {
		t.Fatalf("expected 1 upserted row, got %d", upserted)
	}
}

func TestCatalogStoreGetBySlug(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/rest/v1/anime_catalog_sync_v2_view" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("slug"); got != "eq.ao-no-orchestra-season-2" {
			t.Fatalf("unexpected slug filter %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "1" {
			t.Fatalf("unexpected limit %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"title":"Ao no Orchestra Season 2","slug":"ao-no-orchestra-season-2","canonical_url":"https://v2.samehadaku.how/anime/ao-no-orchestra-season-2/","page_number":22}]`))
	}))
	defer server.Close()

	store := NewCatalogStore(server.Client(), server.URL, "sb_secret_test")
	item, err := store.GetCatalogBySlug(context.Background(), "ao-no-orchestra-season-2")
	if err != nil {
		t.Fatalf("GetCatalogBySlug returned error: %v", err)
	}
	if item.Slug != "ao-no-orchestra-season-2" {
		t.Fatalf("unexpected slug %q", item.Slug)
	}
	if item.PageNumber != 22 {
		t.Fatalf("unexpected page number %d", item.PageNumber)
	}
}

func TestCatalogStoreListCatalogSlugs(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/rest/v1/anime_catalog_sync_v2_view" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("select"); got != "slug,page_number" {
			t.Fatalf("unexpected select %q", got)
		}
		if got := r.URL.Query().Get("order"); got != "page_number.asc,slug.asc" {
			t.Fatalf("unexpected order %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Fatalf("unexpected limit %q", got)
		}
		if got := r.URL.Query().Get("offset"); got != "4" {
			t.Fatalf("unexpected offset %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"slug":"ao-no-orchestra-season-2","page_number":2},{"slug":"yamada-kun-to-lv999-no-koi-wo-suru","page_number":27}]`))
	}))
	defer server.Close()

	store := NewCatalogStore(server.Client(), server.URL, "sb_secret_test")
	items, err := store.ListCatalogSlugs(context.Background(), 4, 2)
	if err != nil {
		t.Fatalf("ListCatalogSlugs returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Slug != "ao-no-orchestra-season-2" {
		t.Fatalf("unexpected first slug %q", items[0].Slug)
	}
	if items[1].PageNumber != 27 {
		t.Fatalf("unexpected second page number %d", items[1].PageNumber)
	}
}
