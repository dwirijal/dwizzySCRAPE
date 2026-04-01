package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
)

func TestEpisodeDetailStoreUpsertWithDB(t *testing.T) {
	scrapedAt := time.Date(2026, 3, 22, 16, 30, 0, 0, time.UTC)
	details := []samehadaku.EpisodeDetail{
		{
			AnimeSlug:             "ao-no-orchestra-season-2",
			EpisodeSlug:           "ao-no-orchestra-season-2-episode-1-subtitle-indonesia",
			CanonicalURL:          "https://v2.samehadaku.how/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/",
			PrimarySourceURL:      "https://v2.samehadaku.how/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/",
			PrimarySourceDomain:   "v2.samehadaku.how",
			SecondarySourceURL:    "https://samehadaku.li/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/",
			SecondarySourceDomain: "samehadaku.li",
			EffectiveSourceURL:    "https://samehadaku.li/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/",
			EffectiveSourceDomain: "samehadaku.li",
			EffectiveSourceKind:   "secondary",
			Title:                 "Ao no Orchestra Season 2 Episode 1 Subtitle Indonesia",
			EpisodeNumber:         1,
			ReleaseLabel:          "October 5, 2025",
			StreamLinksJSON:       []byte(`{"primary":"https://video.example/embed/1","mirrors":{"Video":"https://video.example/embed/1"}}`),
			DownloadLinksJSON:     []byte(`{"Download":"https://gofile.io/d/demo"}`),
			SourceMetaJSON:        []byte(`{"source":"samehadaku","fetch_status":"primary_challenge_blocked_secondary_fetched"}`),
			FetchStatus:           "primary_challenge_blocked_secondary_fetched",
			FetchError:            "primary: cloudflare challenge detected",
			ScrapedAt:             scrapedAt,
		},
	}

	store := NewEpisodeDetailStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			if !strings.Contains(query, "upsert_media_unit") {
				t.Fatalf("unexpected query %q", query)
			}
			if got := args[9]; got != "2025-10-05T00:00:00Z" {
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
			if row["anime_slug"] != "ao-no-orchestra-season-2" {
				t.Fatalf("unexpected payload %#v", row)
			}
			return nil
		},
	})

	upserted, err := store.UpsertEpisodeDetails(context.Background(), details)
	if err != nil {
		t.Fatalf("UpsertEpisodeDetails returned error: %v", err)
	}
	if upserted != 1 {
		t.Fatalf("expected 1 upserted row, got %d", upserted)
	}
}

func TestEpisodeDetailStoreListAnimeSlugsWithDB(t *testing.T) {
	store := NewEpisodeDetailStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.media_units") {
				t.Fatalf("unexpected query %q", query)
			}
			if !strings.Contains(query, "detail->>'anime_slug'") {
				t.Fatalf("expected anime_slug projection in query %q", query)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*[]byte)) = []byte(`[{"anime_slug":"ao-no-orchestra-season-2"},{"anime_slug":"ao-no-orchestra-season-2"},{"anime_slug":"yamada-kun-to-lv999-no-koi-wo-suru"}]`)
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
