package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/anichin"
)

func TestAnichinStoreUpsertAnimeDetailWithDB(t *testing.T) {
	t.Parallel()

	upsertCalled := false
	taxonomyCalled := false
	store := NewAnichinStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			switch {
			case strings.Contains(query, "upsert_media_item"):
				upsertCalled = true
				if got := args[1]; got != "anichin" {
					t.Fatalf("unexpected source %#v", got)
				}
				payload, ok := args[11].([]byte)
				if !ok {
					t.Fatalf("expected []byte payload, got %T", args[11])
				}
				var row map[string]any
				if err := json.Unmarshal(payload, &row); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				if row["canonical_url"] != "https://anichin.cafe/seri/stellar-transformation-season-5/" {
					t.Fatalf("unexpected payload %#v", row)
				}
			case strings.Contains(query, "UPDATE public.media_items"):
				taxonomyCalled = true
				if got := args[1]; got != "series" {
					t.Fatalf("unexpected surface_type %#v", got)
				}
				if got := args[2]; got != "animation" {
					t.Fatalf("unexpected presentation_type %#v", got)
				}
				if got := args[3]; got != "donghua" {
					t.Fatalf("unexpected origin_type %#v", got)
				}
				if got := args[4]; got != "CN" {
					t.Fatalf("unexpected release_country %#v", got)
				}
			default:
				t.Fatalf("unexpected query %q", query)
			}
			return nil
		},
	})

	err := store.UpsertAnimeDetail(context.Background(), anichin.AnimeDetail{
		Slug:           "stellar-transformation-season-5",
		CanonicalURL:   "https://anichin.cafe/seri/stellar-transformation-season-5/",
		Title:          "Stellar Transformation Season 5",
		PosterURL:      "https://anichin.cafe/poster.webp",
		Status:         "Completed",
		AnimeType:      "Donghua",
		AltTitle:       "Xingchen Bian 5th Season",
		Synopsis:       "Musim kelima dari Stellar Transformation.",
		StudioNames:    []string{"Foch Films"},
		GenreNames:     []string{"Action", "Fantasy"},
		SourceMetaJSON: []byte(`{"network":"Tencent Penguin Pictures"}`),
		ScrapedAt:      time.Date(2026, 3, 29, 18, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("UpsertAnimeDetail returned error: %v", err)
	}
	if !upsertCalled {
		t.Fatal("expected upsert_media_item call")
	}
	if !taxonomyCalled {
		t.Fatal("expected taxonomy update call")
	}
}

func TestAnichinStoreUpsertEpisodeDetailWithDB(t *testing.T) {
	t.Parallel()

	store := NewAnichinStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			if !strings.Contains(query, "upsert_media_unit") {
				t.Fatalf("unexpected query %q", query)
			}
			if got := args[2]; got != "anichin" {
				t.Fatalf("unexpected source %#v", got)
			}
			if got := args[9]; got != "2026-03-29T00:00:00Z" {
				t.Fatalf("unexpected published_at %#v", got)
			}
			payload, ok := args[12].([]byte)
			if !ok {
				t.Fatalf("expected []byte payload, got %T", args[12])
			}
			var row map[string]any
			if err := json.Unmarshal(payload, &row); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if row["anime_slug"] != "tales-of-herding-gods" {
				t.Fatalf("unexpected anime slug %#v", row["anime_slug"])
			}
			if row["stream_url"] != "https://anichin.stream/?id=v75kz3s" {
				t.Fatalf("unexpected stream url %#v", row["stream_url"])
			}
			if _, ok := row["download_links_json"]; !ok {
				t.Fatalf("expected download_links_json in payload %#v", row)
			}
			return nil
		},
	})

	err := store.UpsertEpisodeDetail(context.Background(), anichin.EpisodeDetail{
		AnimeSlug:      "tales-of-herding-gods",
		EpisodeSlug:    "tales-of-herding-gods-episode-76-subtitle-indonesia",
		CanonicalURL:   "https://anichin.cafe/tales-of-herding-gods-episode-76-subtitle-indonesia/",
		Title:          "Tales of Herding Gods Episode 76 Subtitle Indonesia",
		EpisodeNumber:  76,
		ReleaseLabel:   "March 29, 2026",
		StreamURL:      "https://anichin.stream/?id=v75kz3s",
		StreamMirrors:  map[string]string{"OK.ru": "https://ok.ru/embed/abc"},
		DownloadLinks:  map[string]map[string]string{"360p": {"Mirrored": "https://www.mirrored.to/multilinks/e67a870f74"}},
		SourceMetaJSON: []byte(`{"source":"anichin"}`),
		ScrapedAt:      time.Date(2026, 3, 29, 18, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("UpsertEpisodeDetail returned error: %v", err)
	}
}

func TestAnichinStoreHasAnimeDetailWithDB(t *testing.T) {
	t.Parallel()

	store := NewAnichinStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.media_items") {
				t.Fatalf("unexpected query %q", query)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*bool)) = true
				return nil
			}}
		},
	})

	ok, err := store.HasAnimeDetail(context.Background(), "stellar-transformation-season-5")
	if err != nil {
		t.Fatalf("HasAnimeDetail returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected anime detail to exist")
	}
}

func TestAnichinStoreHasEpisodeDetailWithDB(t *testing.T) {
	t.Parallel()

	store := NewAnichinStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.media_units") {
				t.Fatalf("unexpected query %q", query)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*bool)) = true
				return nil
			}}
		},
	})

	ok, err := store.HasEpisodeDetail(context.Background(), "tales-of-herding-gods-episode-76-subtitle-indonesia")
	if err != nil {
		t.Fatalf("HasEpisodeDetail returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected episode detail to exist")
	}
}
