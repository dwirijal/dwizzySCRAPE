package samehadaku

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

type fakeCatalogBatchReader struct {
	pages map[int][]CatalogItem
	err   error
}

func (f *fakeCatalogBatchReader) ListCatalogSlugs(ctx context.Context, offset, limit int) ([]CatalogItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	if items, ok := f.pages[offset]; ok {
		return items, nil
	}
	return nil, nil
}

type fakeEpisodeAnimeSlugReader struct {
	slugs []string
	err   error
}

func (f *fakeEpisodeAnimeSlugReader) ListAnimeSlugs(ctx context.Context, offset, limit int) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	if offset > 0 {
		return nil, nil
	}
	return append([]string(nil), f.slugs...), nil
}

type fakeEpisodeSyncer struct {
	requested []string
	reports   map[string]EpisodeSyncReport
	errors    map[string]error
}

func (f *fakeEpisodeSyncer) SyncAnimeEpisodes(ctx context.Context, animeSlug string) (EpisodeSyncReport, error) {
	f.requested = append(f.requested, animeSlug)
	if err, ok := f.errors[animeSlug]; ok {
		return EpisodeSyncReport{}, err
	}
	if report, ok := f.reports[animeSlug]; ok {
		return report, nil
	}
	return EpisodeSyncReport{AnimeSlug: animeSlug, Parsed: 12, Upserted: 12}, nil
}

func TestEpisodeBackfillServiceSkipsExistingSlugs(t *testing.T) {
	catalog := &fakeCatalogBatchReader{
		pages: map[int][]CatalogItem{
			0: {
				{Slug: "absolute-duo", PageNumber: 1},
				{Slug: "ao-no-orchestra-season-2", PageNumber: 2},
			},
			2: {
				{Slug: "yamada-kun-to-lv999-no-koi-wo-suru", PageNumber: 27},
			},
		},
	}
	existing := &fakeEpisodeAnimeSlugReader{
		slugs: []string{"ao-no-orchestra-season-2"},
	}
	syncer := &fakeEpisodeSyncer{}
	service := NewEpisodeBackfillService(catalog, existing, syncer, time.Time{})

	report, err := service.Backfill(context.Background(), EpisodeBackfillOptions{
		BatchSize:    2,
		SkipExisting: true,
		DelayBetween: 0,
	})
	if err != nil {
		t.Fatalf("Backfill returned error: %v", err)
	}
	if report.Discovered != 3 {
		t.Fatalf("expected 3 discovered, got %d", report.Discovered)
	}
	if report.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", report.Skipped)
	}
	if report.Succeeded != 2 {
		t.Fatalf("expected 2 succeeded, got %d", report.Succeeded)
	}
	if report.Failed != 0 {
		t.Fatalf("expected 0 failed, got %d", report.Failed)
	}
	if !slices.Equal(syncer.requested, []string{"absolute-duo", "yamada-kun-to-lv999-no-koi-wo-suru"}) {
		t.Fatalf("unexpected requested slugs %#v", syncer.requested)
	}
}

func TestEpisodeBackfillServiceContinuesOnPerSlugError(t *testing.T) {
	catalog := &fakeCatalogBatchReader{
		pages: map[int][]CatalogItem{
			0: {
				{Slug: "absolute-duo", PageNumber: 1},
				{Slug: "ao-no-orchestra-season-2", PageNumber: 2},
			},
		},
	}
	syncer := &fakeEpisodeSyncer{
		errors: map[string]error{
			"absolute-duo": errors.New("challenge blocked"),
		},
	}
	service := NewEpisodeBackfillService(catalog, &fakeEpisodeAnimeSlugReader{}, syncer, time.Time{})

	report, err := service.Backfill(context.Background(), EpisodeBackfillOptions{
		BatchSize:    2,
		SkipExisting: false,
	})
	if err != nil {
		t.Fatalf("Backfill returned error: %v", err)
	}
	if report.Attempted != 2 {
		t.Fatalf("expected 2 attempted, got %d", report.Attempted)
	}
	if report.Succeeded != 1 {
		t.Fatalf("expected 1 succeeded, got %d", report.Succeeded)
	}
	if report.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", report.Failed)
	}
	if got := report.Failures["absolute-duo"]; got != "challenge blocked" {
		t.Fatalf("unexpected failure for absolute-duo %q", got)
	}
	if !slices.Equal(syncer.requested, []string{"absolute-duo", "ao-no-orchestra-season-2"}) {
		t.Fatalf("unexpected requested slugs %#v", syncer.requested)
	}
}
