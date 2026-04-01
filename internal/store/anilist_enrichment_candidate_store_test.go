package store

import (
	"context"
	"strings"
	"testing"
)

func TestAniListEnrichmentCandidateStoreListCandidates(t *testing.T) {
	store := NewAniListEnrichmentCandidateStoreWithDB(&stubContentDB{
		queryRowFn: func(_ context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "provider = 'anilist'") {
				t.Fatalf("expected anilist provider filter, got %q", query)
			}
			if !strings.Contains(query, "provider = 'jikan'") {
				t.Fatalf("expected jikan fallback check, got %q", query)
			}
			if !strings.Contains(query, "provider = 'tmdb'") {
				t.Fatalf("expected tmdb fallback check, got %q", query)
			}
			if !strings.Contains(query, "i.taxonomy_confidence < 70") {
				t.Fatalf("expected low-confidence filter, got %q", query)
			}
			if strings.Contains(query, "AND (\n        i.surface_type = 'comic'\n        OR") || strings.Contains(query, "AND ( i.surface_type = 'comic' OR") {
				t.Fatalf("unexpected broad comic sweep in query %q", query)
			}
			payload := `[{"item_key":"samehadaku:movie:tenki-no-ko","source":"samehadaku","media_type":"movie","surface_type":"movie","presentation_type":"animation","origin_type":"anime","release_country":"JP","slug":"tenki-no-ko","title":"Tenki no Ko","release_year":2019,"taxonomy_confidence":65,"detail":{"source_title":"Tenki no Ko"}}]`
			return stubRow{scanFn: func(dest ...any) error {
				text := dest[0].(*string)
				*text = payload
				return nil
			}}
		},
	})

	items, err := store.ListAniListEnrichmentCandidates(context.Background(), 0, 10, AniListEnrichmentCandidateOptions{
		Scope:        AniListEnrichmentScopeAll,
		SkipExisting: true,
	})
	if err != nil {
		t.Fatalf("ListAniListEnrichmentCandidates returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Slug != "tenki-no-ko" {
		t.Fatalf("unexpected slug %q", items[0].Slug)
	}
	if items[0].TaxonomyConfidence != 65 {
		t.Fatalf("unexpected confidence %d", items[0].TaxonomyConfidence)
	}
}
