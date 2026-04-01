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

	upsertCalled := false
	taxonomyCalled := false
	store := NewCatalogStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			switch {
			case strings.Contains(query, "upsert_media_item"):
				upsertCalled = true
				if len(args) != 12 {
					t.Fatalf("expected 12 args, got %d", len(args))
				}
				if args[0] != "samehadaku:anime:compass2-0-animation-project" {
					t.Fatalf("unexpected item key %#v", args[0])
				}
				if args[2] != "anime" {
					t.Fatalf("unexpected media type %#v", args[2])
				}
				payload, ok := args[11].([]byte)
				if !ok {
					t.Fatalf("expected []byte payload, got %T", args[11])
				}
				var row map[string]any
				if err := json.Unmarshal(payload, &row); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				if row["canonical_url"] != "https://v2.samehadaku.how/anime/compass2-0-animation-project/" {
					t.Fatalf("unexpected payload %#v", row)
				}
			case strings.Contains(query, "UPDATE public.media_items"):
				taxonomyCalled = true
				if args[1] != "series" || args[2] != "animation" || args[3] != "anime" || args[4] != "JP" {
					t.Fatalf("unexpected taxonomy args %#v", args[1:5])
				}
			default:
				t.Fatalf("expected function query, got %q", query)
			}
			return nil
		},
	})

	upserted, err := store.UpsertCatalog(context.Background(), items)
	if err != nil {
		t.Fatalf("UpsertCatalog returned error: %v", err)
	}
	if upserted != 1 {
		t.Fatalf("expected 1 upserted row, got %d", upserted)
	}
	if !upsertCalled || !taxonomyCalled {
		t.Fatalf("expected upsert and taxonomy update calls, got upsert=%v taxonomy=%v", upsertCalled, taxonomyCalled)
	}
}

func TestCatalogStoreUpsertMovieWithDB(t *testing.T) {
	items := []samehadaku.CatalogItem{{
		Source:       "samehadaku",
		SourceDomain: "v2.samehadaku.how",
		ContentType:  "movie",
		Title:        "Suzume no Tojimari",
		CanonicalURL: "https://v2.samehadaku.how/anime/suzume-no-tojimari/",
		Slug:         "suzume-no-tojimari",
		PosterURL:    "https://cdn.samehadaku.example/posters/suzume.webp",
		AnimeType:    "Movie",
		Status:       "Completed",
	}}

	upsertCalled := false
	taxonomyCalled := false
	store := NewCatalogStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			switch {
			case strings.Contains(query, "upsert_media_item"):
				upsertCalled = true
				if args[0] != "samehadaku:movie:suzume-no-tojimari" {
					t.Fatalf("unexpected item key %#v", args[0])
				}
				if args[2] != "movie" {
					t.Fatalf("unexpected media type %#v", args[2])
				}
			case strings.Contains(query, "UPDATE public.media_items"):
				taxonomyCalled = true
				if args[1] != "movie" || args[2] != "animation" || args[3] != "anime" || args[4] != "JP" {
					t.Fatalf("unexpected taxonomy args %#v", args[1:5])
				}
			default:
				t.Fatalf("unexpected query %q", query)
			}
			return nil
		},
	})

	if _, err := store.UpsertCatalog(context.Background(), items); err != nil {
		t.Fatalf("UpsertCatalog returned error: %v", err)
	}
	if !upsertCalled || !taxonomyCalled {
		t.Fatalf("expected upsert and taxonomy update calls, got upsert=%v taxonomy=%v", upsertCalled, taxonomyCalled)
	}
}

func TestCatalogStoreGetBySlugWithDB(t *testing.T) {
	store := NewCatalogStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.media_items") {
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
			if !strings.Contains(query, "FROM public.media_items") {
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
