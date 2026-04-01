package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/kusonime"
)

func TestKusonimeEnrichmentStoreHasAnimeEnrichment(t *testing.T) {
	t.Parallel()

	store := NewKusonimeEnrichmentStoreWithDB(&stubContentDB{
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

	ok, err := store.HasAnimeEnrichment(context.Background(), "kusuriya-no-hitorigoto")
	if err != nil {
		t.Fatalf("HasAnimeEnrichment returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected enrichment to exist")
	}
}

func TestKusonimeEnrichmentStoreUpsertAnimeEnrichment(t *testing.T) {
	t.Parallel()

	store := NewKusonimeEnrichmentStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.media_items") {
				t.Fatalf("unexpected query %q", query)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "samehadaku:anime:kusuriya-no-hitorigoto"
				*(dest[1].(*[]byte)) = []byte(`{"legacy":{"post_url":"https://legacy.example/post"}}`)
				return nil
			}}
		},
		execFn: func(ctx context.Context, query string, args ...any) error {
			if !strings.Contains(query, "upsert_media_item") {
				t.Fatalf("unexpected query %q", query)
			}
			payload, ok := args[11].([]byte)
			if !ok {
				t.Fatalf("expected []byte payload, got %T", args[11])
			}
			var row map[string]any
			if err := json.Unmarshal(payload, &row); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			batchSources := row["batch_sources"].(map[string]any)
			if _, ok := batchSources["legacy"]; !ok {
				t.Fatalf("expected legacy batch source in %#v", batchSources)
			}
			kusonimePayload, ok := batchSources["kusonime"].(map[string]any)
			if !ok {
				t.Fatalf("expected kusonime payload in %#v", batchSources)
			}
			if kusonimePayload["post_url"] != "https://kusonime.com/kusuriyanohitorigoto-batch-subtitle-indonesia/" {
				t.Fatalf("unexpected post_url %#v", kusonimePayload["post_url"])
			}
			return nil
		},
	})

	err := store.UpsertAnimeEnrichment(context.Background(), kusonime.ReviewResult{
		DBAnimeSlug:  "kusuriya-no-hitorigoto",
		MatchedTitle: "Kusuriya no Hitorigoto",
		MatchedURL:   "https://kusonime.com/kusuriyanohitorigoto-batch-subtitle-indonesia/",
		MatchScore:   0.93,
		Page: kusonime.AnimePage{
			Title:         "Kusuriya no Hitorigoto",
			URL:           "https://kusonime.com/kusuriyanohitorigoto-batch-subtitle-indonesia/",
			JapaneseTitle: "薬屋のひとりごと",
			BatchType:     "BD",
			Status:        "Completed",
			TotalEpisodes: "24",
			Batches: []kusonime.BatchLinkGroup{
				{
					Label: "Download Kusuriya no Hitorigoto Episode 01-12 Batch BD Subtitle Indonesia",
					Downloads: map[string]map[string]string{
						"360P": {"Google Drive": "https://drive.example/360"},
					},
				},
			},
		},
	}, time.Date(2026, 3, 30, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("UpsertAnimeEnrichment returned error: %v", err)
	}
}
