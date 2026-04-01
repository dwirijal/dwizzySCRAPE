package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMediaItemEnrichmentStoreHasItemEnrichment(t *testing.T) {
	t.Parallel()

	store := NewMediaItemEnrichmentStoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.media_item_enrichments") {
				t.Fatalf("unexpected query %q", query)
			}
			if args[0] != "samehadaku:anime:kusuriya-no-hitorigoto" {
				t.Fatalf("unexpected item_key arg %#v", args[0])
			}
			if args[1] != "jikan" {
				t.Fatalf("unexpected provider arg %#v", args[1])
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*bool)) = true
				return nil
			}}
		},
	})

	ok, err := store.HasItemEnrichment(context.Background(), "samehadaku:anime:kusuriya-no-hitorigoto", "jikan")
	if err != nil {
		t.Fatalf("HasItemEnrichment returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected enrichment to exist")
	}
}

func TestMediaItemEnrichmentStoreUpsertItemEnrichment(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 30, 5, 0, 0, 0, time.UTC)
	store := NewMediaItemEnrichmentStoreWithDB(&stubContentDB{
		execFn: func(ctx context.Context, query string, args ...any) error {
			if !strings.Contains(query, "INSERT INTO public.media_item_enrichments") {
				t.Fatalf("unexpected query %q", query)
			}
			if args[0] != "samehadaku:anime:kusuriya-no-hitorigoto" {
				t.Fatalf("unexpected item_key arg %#v", args[0])
			}
			if args[1] != "jikan" {
				t.Fatalf("unexpected provider arg %#v", args[1])
			}
			if args[2] != "56877" {
				t.Fatalf("unexpected external_id arg %#v", args[2])
			}
			if args[3] != "matched" {
				t.Fatalf("unexpected match_status arg %#v", args[3])
			}
			if args[4] != 97 {
				t.Fatalf("unexpected match_score arg %#v", args[4])
			}
			if args[5] != "Kusuriya no Hitorigoto" {
				t.Fatalf("unexpected matched_title arg %#v", args[5])
			}
			if args[6] != int16(2023) {
				t.Fatalf("unexpected matched_year arg %#v", args[6])
			}

			payload, ok := args[7].([]byte)
			if !ok {
				t.Fatalf("expected payload []byte, got %T", args[7])
			}
			var row map[string]any
			if err := json.Unmarshal(payload, &row); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if row["title"] != "Kusuriya no Hitorigoto" {
				t.Fatalf("unexpected payload title %#v", row["title"])
			}
			genres, ok := row["genres"].([]any)
			if !ok || len(genres) != 2 {
				t.Fatalf("unexpected payload genres %#v", row["genres"])
			}
			return nil
		},
	})

	err := store.UpsertItemEnrichment(context.Background(), MediaItemEnrichmentRecord{
		ItemKey:      "samehadaku:anime:kusuriya-no-hitorigoto",
		Provider:     "jikan",
		ExternalID:   "56877",
		MatchStatus:  "matched",
		MatchScore:   97,
		MatchedTitle: "Kusuriya no Hitorigoto",
		MatchedYear:  2023,
		Payload: map[string]any{
			"title":   "Kusuriya no Hitorigoto",
			"year":    2023,
			"genres":  []string{"Drama", "Mystery"},
			"score":   8.7,
			"country": "JP",
		},
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("UpsertItemEnrichment returned error: %v", err)
	}
}
