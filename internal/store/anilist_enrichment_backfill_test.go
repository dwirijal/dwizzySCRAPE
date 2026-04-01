package store

import (
	"context"
	"errors"
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/anilist"
)

type aniListClientStub struct {
	searchFn func(ctx context.Context, query string, mediaType anilist.MediaType, limit int) ([]anilist.SearchHit, error)
}

func (c aniListClientStub) SearchMedia(ctx context.Context, query string, mediaType anilist.MediaType, limit int) ([]anilist.SearchHit, error) {
	return c.searchFn(ctx, query, mediaType, limit)
}

type aniListPromotionWriterStub struct {
	calls []AniListPromotionInput
}

func (w *aniListPromotionWriterStub) ApplyPromotion(_ context.Context, input AniListPromotionInput) error {
	w.calls = append(w.calls, input)
	return nil
}

func TestAniListEnrichmentBackfillServiceBackfillsComicMatchAndPromotes(t *testing.T) {
	readerCalls := 0
	reader := aniListCandidateReaderStub{
		listFn: func(_ context.Context, offset, limit int, options AniListEnrichmentCandidateOptions) ([]AniListEnrichmentCandidate, error) {
			readerCalls++
			if readerCalls > 1 {
				return nil, nil
			}
			return []AniListEnrichmentCandidate{{
				ItemKey:            "komiku:manga:study-group",
				Source:             "komiku",
				MediaType:          "manga",
				SurfaceType:        "comic",
				PresentationType:   "illustrated",
				OriginType:         "manga",
				ReleaseCountry:     "",
				Slug:               "study-group",
				Title:              "Study Group",
				ReleaseYear:        2020,
				TaxonomyConfidence: 45,
				Detail: map[string]any{
					"source_title": "Study Group",
				},
			}}, nil
		},
	}
	writer := &tmdbEnrichmentWriterStub{}
	promotion := &aniListPromotionWriterStub{}
	var seenTypes []anilist.MediaType
	client := aniListClientStub{
		searchFn: func(_ context.Context, query string, mediaType anilist.MediaType, limit int) ([]anilist.SearchHit, error) {
			seenTypes = append(seenTypes, mediaType)
			return []anilist.SearchHit{{
				ID:              1,
				Title:           anilist.MediaTitle{English: "Study Group", Romaji: "Study Group"},
				Format:          "MANGA",
				CountryOfOrigin: "KR",
				SeasonYear:      2020,
				AverageScore:    84,
				IsAdult:         false,
				Genres:          []string{"Action", "School"},
				CoverImage:      anilist.CoverImage{Large: "https://example.com/cover.jpg"},
				BannerImage:     "https://example.com/banner.jpg",
				SiteURL:         "https://anilist.co/manga/1",
			}}, nil
		},
	}

	service := NewAniListEnrichmentBackfillService(reader, writer, promotion, client)
	report, err := service.Backfill(context.Background(), AniListEnrichmentBackfillOptions{
		Scope:     AniListEnrichmentScopeComic,
		BatchSize: 1,
	})
	if err != nil {
		t.Fatalf("Backfill returned error: %v", err)
	}
	if report.Succeeded != 1 || report.Failed != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(seenTypes) != 1 || seenTypes[0] != anilist.MediaTypeManga {
		t.Fatalf("unexpected media types %#v", seenTypes)
	}
	if len(writer.records) != 1 || writer.records[0].Provider != "anilist" {
		t.Fatalf("expected anilist enrichment record, got %+v", writer.records)
	}
	if len(promotion.calls) != 1 {
		t.Fatalf("expected one promotion call, got %d", len(promotion.calls))
	}
}

func TestAniListEnrichmentBackfillServiceStopsOnGlobalServiceUnavailability(t *testing.T) {
	readerCalls := 0
	reader := aniListCandidateReaderStub{
		listFn: func(_ context.Context, offset, limit int, options AniListEnrichmentCandidateOptions) ([]AniListEnrichmentCandidate, error) {
			readerCalls++
			if readerCalls > 1 {
				return nil, nil
			}
			return []AniListEnrichmentCandidate{{
				ItemKey:          "komiku:manga:study-group",
				Source:           "komiku",
				MediaType:        "manga",
				SurfaceType:      "comic",
				PresentationType: "illustrated",
				OriginType:       "manga",
				Slug:             "study-group",
				Title:            "Study Group",
				ReleaseYear:      2020,
				Detail: map[string]any{
					"source_title": "Study Group",
				},
			}}, nil
		},
	}
	writer := &tmdbEnrichmentWriterStub{}
	promotion := &aniListPromotionWriterStub{}
	client := aniListClientStub{
		searchFn: func(_ context.Context, query string, mediaType anilist.MediaType, limit int) ([]anilist.SearchHit, error) {
			return nil, anilist.UpstreamUnavailableError{StatusCode: 403, Message: "temporarily disabled"}
		},
	}

	service := NewAniListEnrichmentBackfillService(reader, writer, promotion, client)
	report, err := service.Backfill(context.Background(), AniListEnrichmentBackfillOptions{
		Scope:     AniListEnrichmentScopeComic,
		BatchSize: 1,
	})
	if err == nil {
		t.Fatal("Backfill returned nil error")
	}
	if !errors.Is(err, anilist.ErrServiceUnavailable) {
		t.Fatalf("expected service unavailable error, got %v", err)
	}
	if report.Attempted != 1 {
		t.Fatalf("Attempted = %d, want 1", report.Attempted)
	}
	if report.Failed != 0 {
		t.Fatalf("Failed = %d, want 0", report.Failed)
	}
	if len(writer.records) != 0 {
		t.Fatalf("expected no written records, got %+v", writer.records)
	}
	if len(promotion.calls) != 0 {
		t.Fatalf("expected no promotion calls, got %d", len(promotion.calls))
	}
}

type aniListCandidateReaderStub struct {
	listFn func(ctx context.Context, offset, limit int, options AniListEnrichmentCandidateOptions) ([]AniListEnrichmentCandidate, error)
}

func (r aniListCandidateReaderStub) ListAniListEnrichmentCandidates(ctx context.Context, offset, limit int, options AniListEnrichmentCandidateOptions) ([]AniListEnrichmentCandidate, error) {
	return r.listFn(ctx, offset, limit, options)
}
