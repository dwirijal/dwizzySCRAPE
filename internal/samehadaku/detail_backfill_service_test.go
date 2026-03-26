package samehadaku

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

type fakeDetailAnimeSlugReader struct {
	slugs []string
	err   error
}

func (f *fakeDetailAnimeSlugReader) ListAnimeSlugs(ctx context.Context, offset, limit int) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	if offset > 0 {
		return nil, nil
	}
	return append([]string(nil), f.slugs...), nil
}

type fakeDetailSyncer struct {
	requested []string
	reports   map[string]DetailSyncReport
	errors    map[string]error
}

func (f *fakeDetailSyncer) SyncAnimeDetail(ctx context.Context, slug string) (DetailSyncReport, error) {
	f.requested = append(f.requested, slug)
	if err, ok := f.errors[slug]; ok {
		return DetailSyncReport{}, err
	}
	if report, ok := f.reports[slug]; ok {
		return report, nil
	}
	return DetailSyncReport{Slug: slug, MALID: 0, SourceFetchStatus: "primary_fetched"}, nil
}

func TestDetailBackfillServiceSkipsExistingSlugs(t *testing.T) {
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
	existing := &fakeDetailAnimeSlugReader{
		slugs: []string{"ao-no-orchestra-season-2"},
	}
	syncer := &fakeDetailSyncer{}
	service := NewDetailBackfillService(catalog, existing, syncer, time.Time{})

	report, err := service.Backfill(context.Background(), DetailBackfillOptions{
		BatchSize:    2,
		SkipExisting: true,
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

func TestDetailBackfillServiceContinuesOnPerSlugError(t *testing.T) {
	catalog := &fakeCatalogBatchReader{
		pages: map[int][]CatalogItem{
			0: {
				{Slug: "absolute-duo", PageNumber: 1},
				{Slug: "ao-no-orchestra-season-2", PageNumber: 2},
			},
		},
	}
	syncer := &fakeDetailSyncer{
		errors: map[string]error{
			"absolute-duo": errors.New("challenge blocked"),
		},
	}
	service := NewDetailBackfillService(catalog, &fakeDetailAnimeSlugReader{}, syncer, time.Time{})

	report, err := service.Backfill(context.Background(), DetailBackfillOptions{
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
