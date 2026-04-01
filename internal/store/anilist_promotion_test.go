package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestApplyAniListPromotionKeepsAnichinDonghuaLock(t *testing.T) {
	store := NewAniListPromotionStoreWithDB(&stubContentDB{
		execFn: func(_ context.Context, query string, args ...any) error {
			if !strings.Contains(query, "UPDATE public.media_items") {
				t.Fatalf("unexpected query %q", query)
			}
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
			if got := args[8]; got != "provider_heuristic" {
				t.Fatalf("unexpected taxonomy_source %#v", got)
			}
			return nil
		},
	})

	err := store.ApplyPromotion(context.Background(), AniListPromotionInput{
		ItemKey:               "anichin:anime:stellar-transformation",
		Source:                "anichin",
		MediaType:             "anime",
		SurfaceType:           "series",
		PresentationType:      "animation",
		OriginType:            "donghua",
		ReleaseCountry:        "CN",
		GenreNames:            []string{"Action"},
		TaxonomyConfidence:    98,
		CurrentTaxonomySource: "provider_heuristic",
		CurrentDetail:         map[string]any{},
		CurrentCoverURL:       "",
		AniListMatchScore:     92,
		AniListPayload: map[string]any{
			"type":              "ANIME",
			"country_of_origin": "JP",
			"genres":            []string{"Fantasy"},
			"is_adult":          false,
			"title_english":     "Different Lock",
			"average_score":     82,
			"season_year":       2025,
			"cover_image":       "https://example.com/cover.jpg",
			"banner_image":      "https://example.com/banner.jpg",
			"description":       "desc",
			"site_url":          "https://anilist.co/anime/1",
			"format":            "TV",
			"status":            "FINISHED",
		},
	})
	if err != nil {
		t.Fatalf("ApplyPromotion returned error: %v", err)
	}
}

func TestApplyAniListPromotionPromotesComicOriginAndMetadata(t *testing.T) {
	called := false
	store := NewAniListPromotionStoreWithDB(&stubContentDB{
		execFn: func(_ context.Context, query string, args ...any) error {
			called = true
			if got := args[1]; got != "comic" {
				t.Fatalf("unexpected surface_type %#v", got)
			}
			if got := args[2]; got != "illustrated" {
				t.Fatalf("unexpected presentation_type %#v", got)
			}
			if got := args[3]; got != "manhwa" {
				t.Fatalf("unexpected origin_type %#v", got)
			}
			if got := args[4]; got != "KR" {
				t.Fatalf("unexpected release_country %#v", got)
			}
			if got := args[6].([]string); len(got) != 2 {
				t.Fatalf("unexpected genre_names %#v", got)
			}
			patch, ok := args[12].([]byte)
			if !ok {
				t.Fatalf("expected detail patch bytes, got %T", args[12])
			}
			var detail map[string]any
			if err := json.Unmarshal(patch, &detail); err != nil {
				t.Fatalf("decode detail patch: %v", err)
			}
			if detail["anilist_title_english"] != "Study Group" {
				t.Fatalf("unexpected promoted title %#v", detail["anilist_title_english"])
			}
			return nil
		},
	})

	err := store.ApplyPromotion(context.Background(), AniListPromotionInput{
		ItemKey:            "komiku:manga:study-group",
		Source:             "komiku",
		MediaType:          "manga",
		SurfaceType:        "comic",
		PresentationType:   "illustrated",
		OriginType:         "manga",
		ReleaseCountry:     "",
		GenreNames:         []string{"Action"},
		TaxonomyConfidence: 40,
		CurrentDetail:      map[string]any{},
		CurrentCoverURL:    "",
		AniListMatchScore:  78,
		AniListPayload: map[string]any{
			"type":              "MANGA",
			"country_of_origin": "KR",
			"genres":            []string{"Action", "School"},
			"is_adult":          true,
			"title_english":     "Study Group",
			"title_romaji":      "Study Group",
			"cover_image":       "https://example.com/cover.jpg",
			"banner_image":      "https://example.com/banner.jpg",
			"description":       "desc",
			"site_url":          "https://anilist.co/manga/2",
			"average_score":     84,
			"season_year":       2020,
			"format":            "MANGA",
			"status":            "RELEASING",
		},
	})
	if err != nil {
		t.Fatalf("ApplyPromotion returned error: %v", err)
	}
	if !called {
		t.Fatal("expected update call")
	}
}
