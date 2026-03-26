package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
)

func TestCatalogStoreUpsertWithDB(t *testing.T) {
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

	store := NewCatalogStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "upsert_samehadaku_catalog_v2") {
				t.Fatalf("expected function query, got %q", query)
			}
			if len(args) != 1 {
				t.Fatalf("expected single payload argument, got %d", len(args))
			}
			payload, ok := args[0].([]byte)
			if !ok {
				t.Fatalf("expected []byte payload, got %T", args[0])
			}
			var rows []map[string]any
			if err := json.Unmarshal(payload, &rows); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if len(rows) != 1 || rows[0]["slug"] != "compass2-0-animation-project" {
				t.Fatalf("unexpected payload %#v", rows)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int)) = 1
				return nil
			}}
		},
	})

	upserted, err := store.UpsertCatalog(context.Background(), items)
	if err != nil {
		t.Fatalf("UpsertCatalog returned error: %v", err)
	}
	if upserted != 1 {
		t.Fatalf("expected 1 upserted row, got %d", upserted)
	}
}

func TestCatalogStoreGetBySlugWithDB(t *testing.T) {
	store := NewCatalogStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.anime_catalog_sync_v2_view") {
				t.Fatalf("unexpected query %q", query)
			}
			if len(args) != 1 || args[0] != "ao-no-orchestra-season-2" {
				t.Fatalf("unexpected args %#v", args)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*[]byte)) = []byte(`[{"title":"Ao no Orchestra Season 2","slug":"ao-no-orchestra-season-2","canonical_url":"https://v2.samehadaku.how/anime/ao-no-orchestra-season-2/","page_number":22}]`)
				return nil
			}}
		},
	})

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

func TestCatalogStoreListCatalogSlugsWithDB(t *testing.T) {
	store := NewCatalogStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.anime_catalog_sync_v2_view") {
				t.Fatalf("unexpected query %q", query)
			}
			if !strings.Contains(query, "ORDER BY page_number ASC, slug ASC") {
				t.Fatalf("unexpected order in query %q", query)
			}
			if len(args) != 2 || args[0] != 2 || args[1] != 4 {
				t.Fatalf("unexpected args %#v", args)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*[]byte)) = []byte(`[{"slug":"ao-no-orchestra-season-2","page_number":2},{"slug":"yamada-kun-to-lv999-no-koi-wo-suru","page_number":27}]`)
				return nil
			}}
		},
	})

	items, err := store.ListCatalogSlugs(context.Background(), 4, 2)
	if err != nil {
		t.Fatalf("ListCatalogSlugs returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[1].PageNumber != 27 {
		t.Fatalf("unexpected second page number %d", items[1].PageNumber)
	}
}
