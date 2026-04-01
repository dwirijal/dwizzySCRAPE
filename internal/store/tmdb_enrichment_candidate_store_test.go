package store

import (
	"context"
	"strings"
	"testing"
)

func TestTMDBEnrichmentCandidateStoreListCandidates(t *testing.T) {
	store := NewTMDBEnrichmentCandidateStoreWithDB(&stubContentDB{
		queryRowFn: func(_ context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM public.media_items i") {
				t.Fatalf("expected media_items query, got %q", query)
			}
			if !strings.Contains(query, "provider = 'tmdb'") {
				t.Fatalf("expected tmdb provider filter, got %q", query)
			}
			if !strings.Contains(query, "e.match_status = 'matched'") {
				t.Fatalf("expected matched-only skip filter, got %q", query)
			}
			if !strings.Contains(query, "coalesce(i.origin_type, '') <> 'variety'") {
				t.Fatalf("expected variety exclusion in series scope, got %q", query)
			}
			if len(args) != 4 {
				t.Fatalf("expected 4 query args, got %d", len(args))
			}
			if scope, _ := args[0].(string); scope != string(TMDBEnrichmentScopeSeries) {
				t.Fatalf("unexpected scope arg %#v", args[0])
			}
			if skipExisting, _ := args[1].(bool); !skipExisting {
				t.Fatalf("expected skip existing arg to be true")
			}
			if offset, _ := args[2].(int); offset != 0 {
				t.Fatalf("unexpected offset %#v", args[2])
			}
			if limit, _ := args[3].(int); limit != 10 {
				t.Fatalf("unexpected limit %#v", args[3])
			}

			payload := `[{"item_key":"drakorid:drama:honour-2026","source":"drakorid","media_type":"drama","surface_type":"series","presentation_type":"live_action","origin_type":"drama","release_country":"KR","slug":"honour-2026","title":"Drama Korea Honour (2026)","release_year":2026,"detail":{"source_title":"Drama Korea Honour (2026)","country":"South Korea"}}]`
			return stubRow{scanFn: func(dest ...any) error {
				text := dest[0].(*string)
				*text = payload
				return nil
			}}
		},
	})

	items, err := store.ListTMDBEnrichmentCandidates(context.Background(), 0, 10, TMDBEnrichmentCandidateOptions{
		Scope:        TMDBEnrichmentScopeSeries,
		SkipExisting: true,
	})
	if err != nil {
		t.Fatalf("ListTMDBEnrichmentCandidates returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Slug != "honour-2026" {
		t.Fatalf("unexpected slug %q", items[0].Slug)
	}
	if items[0].ReleaseCountry != "KR" {
		t.Fatalf("unexpected release country %q", items[0].ReleaseCountry)
	}
}
