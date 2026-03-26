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

	store := NewAnimeDetailStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "upsert_samehadaku_anime_detail_v2") {
				t.Fatalf("unexpected query %q", query)
			}
			payload, ok := args[0].([]byte)
			if !ok {
				t.Fatalf("expected []byte payload, got %T", args[0])
			}
			var rows []map[string]any
			if err := json.Unmarshal(payload, &rows); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if len(rows) != 1 || rows[0]["slug"] != "ao-no-orchestra-season-2" {
				t.Fatalf("unexpected payload %#v", rows)
			}
			if _, ok := rows[0]["batch_links_json"]; !ok {
				t.Fatalf("expected batch_links_json in payload %#v", rows[0])
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int)) = 1
				return nil
			}}
		},
	})

	if err := store.UpsertAnimeDetail(context.Background(), detail); err != nil {
		t.Fatalf("UpsertAnimeDetail returned error: %v", err)
	}
}

func TestAnimeDetailStoreListAnimeSlugsWithDB(t *testing.T) {
	store := NewAnimeDetailStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.anime_detail_ready_v2_view") {
				t.Fatalf("unexpected query %q", query)
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
