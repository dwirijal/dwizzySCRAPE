package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/otakudesu"
)

func TestOtakudesuEnrichmentStoreHasEpisodeEnrichment(t *testing.T) {
	t.Parallel()

	store := NewOtakudesuEnrichmentStoreWithDB(&stubContentDB{
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

	ok, err := store.HasEpisodeEnrichment(context.Background(), "demo-episode-1")
	if err != nil {
		t.Fatalf("HasEpisodeEnrichment returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected enrichment to exist")
	}
}

func TestOtakudesuEnrichmentStoreUpsertEpisodeEnrichment(t *testing.T) {
	t.Parallel()

	store := NewOtakudesuEnrichmentStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.media_units") {
				t.Fatalf("unexpected query %q", query)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "samehadaku:anime:kusuriya-no-hitorigoto"
				*(dest[1].(*[]byte)) = []byte(`{"primary":"https://samehadaku.example/stream","mirrors":{"Server 1":"https://samehadaku.example/mirror"}}`)
				*(dest[2].(*[]byte)) = []byte(`{"MKV":{"360p":{"Mega":"https://samehadaku.example/download"}}}`)
				*(dest[3].(*[]byte)) = []byte(`{"source":"samehadaku"}`)
				return nil
			}}
		},
		execFn: func(ctx context.Context, query string, args ...any) error {
			if !strings.Contains(query, "upsert_media_unit") {
				t.Fatalf("unexpected query %q", query)
			}
			payload, ok := args[12].([]byte)
			if !ok {
				t.Fatalf("expected []byte payload, got %T", args[12])
			}
			var row map[string]any
			if err := json.Unmarshal(payload, &row); err != nil {
				t.Fatalf("decode payload: %v", err)
			}

			streamLinks := row["stream_links_json"].(map[string]any)
			if _, ok := streamLinks["otakudesu"]; !ok {
				t.Fatalf("expected otakudesu stream payload in %#v", streamLinks)
			}
			downloadLinks := row["download_links_json"].(map[string]any)
			if _, ok := downloadLinks["otakudesu"]; !ok {
				t.Fatalf("expected otakudesu download payload in %#v", downloadLinks)
			}
			sourceMeta := row["source_meta_json"].(map[string]any)
			if _, ok := sourceMeta["otakudesu"]; !ok {
				t.Fatalf("expected otakudesu source meta in %#v", sourceMeta)
			}
			return nil
		},
	})

	err := store.UpsertEpisodeEnrichment(context.Background(), otakudesu.ReviewResult{
		DBAnimeSlug:  "kusuriya-no-hitorigoto",
		MatchedTitle: "Kusuriya no Hitorigoto",
		MatchedURL:   "https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/",
		MatchScore:   0.92,
	}, otakudesu.EpisodeReview{
		DBEpisodeSlug:       "kusuriya-episode-1",
		OtakudesuEpisodeURL: "https://otakudesu.blog/episode/knh-episode-1-sub-indo/",
		StreamURL:           "https://otakudesu.example/stream",
		StreamMirrors:       map[string]string{"Mirror 2": "https://otakudesu.example/stream-2"},
		DownloadLinks: map[string]map[string]string{
			"360p": {"OtakuFiles": "https://otakudesu.example/download-360"},
		},
	}, time.Date(2026, 3, 30, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("UpsertEpisodeEnrichment returned error: %v", err)
	}
}
