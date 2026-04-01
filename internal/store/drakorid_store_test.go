package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/drakorid"
)

func TestDrakoridStoreUpsertDetailUpdatesCanonicalTaxonomy(t *testing.T) {
	t.Parallel()

	upsertCalled := false
	taxonomyCalled := false
	store := NewDrakoridStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			switch {
			case strings.Contains(query, "upsert_media_item"):
				upsertCalled = true
				payload, ok := args[11].([]byte)
				if !ok {
					t.Fatalf("expected []byte payload, got %T", args[11])
				}
				var row map[string]any
				if err := json.Unmarshal(payload, &row); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				if row["country"] != "South Korea" {
					t.Fatalf("unexpected country %#v", row["country"])
				}
			case strings.Contains(query, "UPDATE public.media_items"):
				taxonomyCalled = true
				if got := args[1]; got != "series" {
					t.Fatalf("unexpected surface_type %#v", got)
				}
				if got := args[2]; got != "live_action" {
					t.Fatalf("unexpected presentation_type %#v", got)
				}
				if got := args[3]; got != "drama" {
					t.Fatalf("unexpected origin_type %#v", got)
				}
				if got := args[4]; got != "KR" {
					t.Fatalf("unexpected release_country %#v", got)
				}
			default:
				t.Fatalf("unexpected query %q", query)
			}
			return nil
		},
	})

	err := store.UpsertDetail(context.Background(), drakorid.Detail{
		MediaType:      "drama",
		Title:          "Study Group",
		Slug:           "study-group",
		CanonicalURL:   "https://drakorid.example/drama/study-group/",
		PosterURL:      "https://drakorid.example/poster.webp",
		Synopsis:       "A tough student forms a study group.",
		Status:         "ongoing",
		ReleaseYear:    "2025",
		Country:        "South Korea",
		Genres:         []string{"Action", "Drama"},
		SourceMetaJSON: []byte(`{"source":"drakorid"}`),
		ScrapedAt:      time.Date(2026, 3, 30, 6, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("UpsertDetail returned error: %v", err)
	}
	if !upsertCalled {
		t.Fatal("expected upsert_media_item call")
	}
	if !taxonomyCalled {
		t.Fatal("expected taxonomy update call")
	}
}

func TestDrakoridStoreUpsertCatalogVarietyPersistsTitleForTaxonomy(t *testing.T) {
	t.Parallel()

	upsertCalled := false
	taxonomyCalled := false
	store := NewDrakoridStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			switch {
			case strings.Contains(query, "upsert_media_item"):
				upsertCalled = true
				payload, ok := args[11].([]byte)
				if !ok {
					t.Fatalf("expected []byte payload, got %T", args[11])
				}
				var row map[string]any
				if err := json.Unmarshal(payload, &row); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				if row["source_title"] != "Singles Inferno S5 (2026)" {
					t.Fatalf("unexpected source_title %#v", row["source_title"])
				}
			case strings.Contains(query, "UPDATE public.media_items"):
				taxonomyCalled = true
				if got := args[3]; got != "variety" {
					t.Fatalf("unexpected origin_type %#v", got)
				}
			default:
				t.Fatalf("unexpected query %q", query)
			}
			return nil
		},
	})

	_, err := store.UpsertCatalogItems(context.Background(), []drakorid.CatalogItem{{
		MediaType:    "drama",
		Title:        "Singles Inferno S5 (2026)",
		Slug:         "singles-inferno-s5-2026",
		CanonicalURL: "https://drakorid.example/drama/singles-inferno-s5-2026/",
		PosterURL:    "https://drakorid.example/poster.webp",
		Status:       "ongoing",
		Category:     "Series",
		ScrapedAt:    time.Date(2026, 3, 30, 6, 30, 0, 0, time.UTC),
	}})
	if err != nil {
		t.Fatalf("UpsertCatalogItems returned error: %v", err)
	}
	if !upsertCalled {
		t.Fatal("expected upsert_media_item call")
	}
	if !taxonomyCalled {
		t.Fatal("expected taxonomy update call")
	}
}

func TestDrakoridStoreUpsertDetailMoviePreservesMovieSurface(t *testing.T) {
	t.Parallel()

	taxonomyCalled := false
	store := NewDrakoridStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			switch {
			case strings.Contains(query, "upsert_media_item"):
				return nil
			case strings.Contains(query, "UPDATE public.media_items"):
				taxonomyCalled = true
				if got := args[1]; got != "movie" {
					t.Fatalf("unexpected surface_type %#v", got)
				}
				if got := args[2]; got != "live_action" {
					t.Fatalf("unexpected presentation_type %#v", got)
				}
				if got := args[3]; got != "movie" {
					t.Fatalf("unexpected origin_type %#v", got)
				}
				return nil
			default:
				t.Fatalf("unexpected query %q", query)
			}
			return nil
		},
	})

	err := store.UpsertDetail(context.Background(), drakorid.Detail{
		MediaType:    "movie",
		Title:        "Mission Impossible Korea",
		Slug:         "mission-impossible-korea",
		CanonicalURL: "https://drakorid.example/movie/mission-impossible-korea/",
		PosterURL:    "https://drakorid.example/poster.webp",
		Status:       "completed",
		ReleaseYear:  "2026",
		Country:      "South Korea",
		ScrapedAt:    time.Date(2026, 3, 30, 7, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("UpsertDetail returned error: %v", err)
	}
	if !taxonomyCalled {
		t.Fatal("expected taxonomy update call")
	}
}

func TestDrakoridStoreUpsertEpisodeDetailExtractsPublishedAtFromSourceMeta(t *testing.T) {
	t.Parallel()

	upsertCalled := false
	store := NewDrakoridStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			if !strings.Contains(query, "upsert_media_unit") {
				t.Fatalf("unexpected query %q", query)
			}
			upsertCalled = true
			if got := args[9]; got != "2026-01-17T00:00:00Z" {
				t.Fatalf("unexpected published_at %#v", got)
			}
			return nil
		},
	})

	err := store.UpsertEpisodeDetail(context.Background(), drakorid.EpisodeDetail{
		MediaType:       "drama",
		ItemSlug:        "sharp-blade-in-the-snow-2026",
		EpisodeSlug:     "sharp-blade-in-the-snow-2026-episode-19",
		CanonicalURL:    "https://drakorid.example/watch/sharp-blade-in-the-snow-2026/19",
		Title:           "Sharp Blade in the Snow (2026) Episode 19",
		Label:           "Episode 19",
		EpisodeNumber:   19,
		StreamURL:       "https://stream.example/19",
		StreamLinksJSON: []byte(`{"lite":{"stream_url":"https://stream.example/19"}}`),
		SourceMetaJSON:  []byte(`{"episode_api":{"img":"http://sk13.drakor.cc/images/0-2026-01-17-272111178071ec41b1f550b2756ef697.jpg"}}`),
		ScrapedAt:       time.Date(2026, 3, 30, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("UpsertEpisodeDetail returned error: %v", err)
	}
	if !upsertCalled {
		t.Fatal("expected upsert_media_unit call")
	}
}
