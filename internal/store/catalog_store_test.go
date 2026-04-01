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
		if r.URL.Path != "/rest/v1/rpc/upsert_media_item" {
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
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := payload["p_cover_url"]; got != "https://cdn.samehadaku.example/posters/compass2.webp" {
			t.Fatalf("unexpected p_cover_url %#v", got)
		}
		if got := payload["p_media_type"]; got != "anime" {
			t.Fatalf("unexpected p_media_type %#v", got)
		}
		detailPayload, ok := payload["p_detail"].(map[string]any)
		if !ok {
			t.Fatalf("expected object p_detail, got %#v", payload["p_detail"])
		}
		if got := detailPayload["canonical_url"]; got != "https://v2.samehadaku.how/anime/compass2-0-animation-project/" {
			t.Fatalf("unexpected canonical_url %#v", got)
		}
		if got := detailPayload["page_number"]; got != float64(1) {
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
		if r.URL.Path != "/rest/v1/media_items" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("select"); got != "title,slug,detail" {
			t.Fatalf("unexpected select %q", got)
		}
		if got := r.URL.Query().Get("source"); got != "eq.samehadaku" {
			t.Fatalf("unexpected source filter %q", got)
		}
		if got := r.URL.Query().Get("slug"); got != "eq.ao-no-orchestra-season-2" {
			t.Fatalf("unexpected slug filter %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "1" {
			t.Fatalf("unexpected limit %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"title":"Ao no Orchestra Season 2","slug":"ao-no-orchestra-season-2","detail":{"canonical_url":"https://v2.samehadaku.how/anime/ao-no-orchestra-season-2/","page_number":22}}]`))
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
		if r.URL.Path != "/rest/v1/media_items" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("select"); got != "slug,detail" {
			t.Fatalf("unexpected select %q", got)
		}
		if got := r.URL.Query().Get("source"); got != "eq.samehadaku" {
			t.Fatalf("unexpected source filter %q", got)
		}
		if got := r.URL.Query().Get("order"); got != "slug.asc" {
			t.Fatalf("unexpected order %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Fatalf("unexpected limit %q", got)
		}
		if got := r.URL.Query().Get("offset"); got != "4" {
			t.Fatalf("unexpected offset %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"slug":"ao-no-orchestra-season-2","detail":{"page_number":2}},{"slug":"yamada-kun-to-lv999-no-koi-wo-suru","detail":{"page_number":27}}]`))
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

func TestCatalogStoreUpsertMovie(t *testing.T) {
	t.Helper()

	items := []samehadaku.CatalogItem{{
		Source:       "samehadaku",
		SourceDomain: "v2.samehadaku.how",
		ContentType:  "movie",
		Title:        "One Piece Live Action",
		CanonicalURL: "https://v2.samehadaku.how/anime/one-piece-live-action/",
		Slug:         "one-piece-live-action",
		PosterURL:    "https://cdn.samehadaku.example/posters/one-piece-live-action.webp",
		AnimeType:    "Movie",
		Status:       "Completed",
	}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := payload["p_item_key"]; got != "samehadaku:movie:one-piece-live-action" {
			t.Fatalf("unexpected p_item_key %#v", got)
		}
		if got := payload["p_media_type"]; got != "movie" {
			t.Fatalf("unexpected p_media_type %#v", got)
		}
		detailPayload, ok := payload["p_detail"].(map[string]any)
		if !ok {
			t.Fatalf("expected object p_detail, got %#v", payload["p_detail"])
		}
		if got := detailPayload["adaptation_type"]; got != "live_action" {
			t.Fatalf("unexpected adaptation_type %#v", got)
		}
		if got := detailPayload["entry_format"]; got != "series" {
			t.Fatalf("unexpected entry_format %#v", got)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	store := NewCatalogStore(server.Client(), server.URL, "sb_secret_test")
	if _, err := store.UpsertCatalog(context.Background(), items); err != nil {
		t.Fatalf("UpsertCatalog returned error: %v", err)
	}
}
