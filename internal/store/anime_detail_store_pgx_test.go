package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
)

func TestAnimeDetailStoreUpsertWithDB(t *testing.T) {
	scrapedAt := time.Date(2026, 3, 22, 16, 0, 0, 0, time.UTC)
	detail := samehadaku.AnimeDetail{
		Slug:                  "ao-no-orchestra-season-2",
		CanonicalURL:          "https://v2.samehadaku.how/anime/ao-no-orchestra-season-2/",
		PrimarySourceURL:      "https://v2.samehadaku.how/anime/ao-no-orchestra-season-2/",
		PrimarySourceDomain:   "v2.samehadaku.how",
		SecondarySourceURL:    "https://samehadaku.li/anime/ao-no-orchestra-season-2/",
		SecondarySourceDomain: "samehadaku.li",
		EffectiveSourceURL:    "https://samehadaku.li/anime/ao-no-orchestra-season-2/",
		EffectiveSourceDomain: "samehadaku.li",
		EffectiveSourceKind:   "secondary",
		SourceTitle:           "Ao no Orchestra Season 2",
		MALID:                 56877,
		MALThumbnailURL:       "https://cdn.myanimelist.net/images/anime/1078/151796.webp",
		SynopsisEnriched:      "Following the regular summer concert...",
		AnimeType:             "TV",
		Status:                "Finished Airing",
		Season:                "fall",
		StudioNames:           []string{"Nippon Animation"},
		GenreNames:            []string{"Drama", "Music", "School"},
		BatchLinksJSON:        []byte(`{"Batch":"https://gofile.io/d/demo-batch"}`),
		CastJSON:              []byte(`[{"character_mal_id":1,"name":"Hajime Aono"}]`),
		SourceMetaJSON:        []byte(`{"samehadaku_fetch_status":"primary_challenge_blocked_secondary_fetched"}`),
		JikanMetaJSON:         []byte(`{"score":7.5}`),
		SourceFetchStatus:     "primary_challenge_blocked_secondary_fetched",
		SourceFetchError:      "primary: cloudflare challenge detected",
		ScrapedAt:             scrapedAt,
	}

	upsertCalled := false
	taxonomyCalled := false
	store := NewAnimeDetailStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			switch {
			case strings.Contains(query, "upsert_media_item"):
				upsertCalled = true
				if args[0] != "samehadaku:anime:ao-no-orchestra-season-2" {
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
				if row["canonical_url"] != "https://v2.samehadaku.how/anime/ao-no-orchestra-season-2/" {
					t.Fatalf("unexpected payload %#v", row)
				}
			case strings.Contains(query, "UPDATE public.media_items"):
				taxonomyCalled = true
				if args[1] != "series" || args[2] != "animation" || args[3] != "anime" || args[4] != "JP" {
					t.Fatalf("unexpected taxonomy args %#v", args[1:5])
				}
			default:
				t.Fatalf("unexpected query %q", query)
			}
			return nil
		},
	})

	if err := store.UpsertAnimeDetail(context.Background(), detail); err != nil {
		t.Fatalf("UpsertAnimeDetail returned error: %v", err)
	}
	if !upsertCalled || !taxonomyCalled {
		t.Fatalf("expected upsert and taxonomy update calls, got upsert=%v taxonomy=%v", upsertCalled, taxonomyCalled)
	}
}

func TestAnimeDetailStoreUpsertMovieWithDB(t *testing.T) {
	detail := samehadaku.AnimeDetail{
		Slug:            "suzume-no-tojimari",
		SourceTitle:     "Suzume no Tojimari",
		MALThumbnailURL: "https://cdn.myanimelist.net/images/anime/suzume.webp",
		AnimeType:       "Movie",
		Status:          "Finished Airing",
		ScrapedAt:       time.Date(2026, 3, 22, 16, 0, 0, 0, time.UTC),
	}

	upsertCalled := false
	taxonomyCalled := false
	store := NewAnimeDetailStoreWithDB(&stubContentDB{
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

	if err := store.UpsertAnimeDetail(context.Background(), detail); err != nil {
		t.Fatalf("UpsertAnimeDetail returned error: %v", err)
	}
	if !upsertCalled || !taxonomyCalled {
		t.Fatalf("expected upsert and taxonomy update calls, got upsert=%v taxonomy=%v", upsertCalled, taxonomyCalled)
	}
}

func TestAnimeDetailStoreListAnimeSlugsWithDB(t *testing.T) {
	store := NewAnimeDetailStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.media_items") {
				t.Fatalf("unexpected query %q", query)
			}
			if !strings.Contains(query, "primary_source_url") {
				t.Fatalf("expected detail-level filter in query %q", query)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*[]byte)) = []byte(`[{"slug":"ao-no-orchestra-season-2"},{"slug":"ao-no-orchestra-season-2"},{"slug":"yamada-kun-to-lv999-no-koi-wo-suru"}]`)
				return nil
			}}
		},
	})

	slugs, err := store.ListAnimeSlugs(context.Background(), 6, 3)
	if err != nil {
		t.Fatalf("ListAnimeSlugs returned error: %v", err)
	}
	if len(slugs) != 2 {
		t.Fatalf("expected 2 unique slugs, got %d", len(slugs))
	}
}
