package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

type fakeFetcher struct {
	catalog  map[int][]content.ManhwaSeries
	series   map[string]content.ManhwaSeries
	chapters map[string]content.ManhwaChapter
	errors   map[string]error
}

func (f fakeFetcher) FetchCatalog(_ context.Context, page int) ([]content.ManhwaSeries, error) {
	if err := f.errors[fmt.Sprintf("catalog:%d", page)]; err != nil {
		return nil, err
	}
	return f.catalog[page], nil
}

func (f fakeFetcher) FetchSeries(_ context.Context, slug string) (content.ManhwaSeries, error) {
	if err := f.errors["series:"+slug]; err != nil {
		return content.ManhwaSeries{}, err
	}
	series, ok := f.series[slug]
	if !ok {
		return content.ManhwaSeries{}, fmt.Errorf("missing series: %s", slug)
	}
	return series, nil
}

func (f fakeFetcher) FetchChapter(_ context.Context, slug string) (content.ManhwaChapter, error) {
	if err := f.errors["chapter:"+slug]; err != nil {
		return content.ManhwaChapter{}, err
	}
	chapter, ok := f.chapters[slug]
	if !ok {
		return content.ManhwaChapter{}, fmt.Errorf("missing chapter: %s", slug)
	}
	return chapter, nil
}

type fakeSeriesWriter struct {
	slugs []string
}

func (w *fakeSeriesWriter) UpsertManhwaSeries(_ context.Context, series content.ManhwaSeries) error {
	w.slugs = append(w.slugs, series.Slug)
	return nil
}

type fakeChapterWriter struct {
	slugs []string
}

func (w *fakeChapterWriter) UpsertManhwaChapter(_ context.Context, chapter content.ManhwaChapter) error {
	w.slugs = append(w.slugs, chapter.Slug)
	return nil
}

func TestRunFullBackfillStopsOnEmptyCatalogPage(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		catalog: map[int][]content.ManhwaSeries{
			1: {
				{Slug: "solo-leveling"},
				{Slug: "omniscient-reader"},
			},
			2: {
				{Slug: "solo-leveling"},
			},
			3: {},
		},
		series: map[string]content.ManhwaSeries{
			"solo-leveling": {
				Slug: "solo-leveling",
				Chapters: []content.ManhwaChapterRef{
					{Slug: "solo-leveling-chapter-2"},
					{Slug: "solo-leveling-chapter-1"},
				},
			},
			"omniscient-reader": {
				Slug: "omniscient-reader",
				Chapters: []content.ManhwaChapterRef{
					{Slug: "omniscient-reader-chapter-1"},
				},
			},
		},
		chapters: map[string]content.ManhwaChapter{
			"solo-leveling-chapter-2":     {Slug: "solo-leveling-chapter-2", SeriesSlug: "solo-leveling"},
			"solo-leveling-chapter-1":     {Slug: "solo-leveling-chapter-1", SeriesSlug: "solo-leveling"},
			"omniscient-reader-chapter-1": {Slug: "omniscient-reader-chapter-1", SeriesSlug: "omniscient-reader"},
		},
	}
	seriesWriter := &fakeSeriesWriter{}
	chapterWriter := &fakeChapterWriter{}

	report, err := runFullBackfill(context.Background(), fetcher, seriesWriter, chapterWriter, backfillOptions{
		StartPage:            1,
		MaxCatalogPages:      10,
		MaxChaptersPerSeries: 0,
	})
	if err != nil {
		t.Fatalf("runFullBackfill returned error: %v", err)
	}

	if report.CatalogPages != 2 {
		t.Fatalf("CatalogPages = %d, want 2", report.CatalogPages)
	}
	if report.DiscoveredSeries != 2 {
		t.Fatalf("DiscoveredSeries = %d, want 2", report.DiscoveredSeries)
	}
	if report.SucceededSeries != 2 {
		t.Fatalf("SucceededSeries = %d, want 2", report.SucceededSeries)
	}
	if report.AttemptedChapters != 3 {
		t.Fatalf("AttemptedChapters = %d, want 3", report.AttemptedChapters)
	}
	if report.SucceededChapters != 3 {
		t.Fatalf("SucceededChapters = %d, want 3", report.SucceededChapters)
	}
}

func TestRunFullBackfillContinuesOnSeriesAndChapterFailures(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		catalog: map[int][]content.ManhwaSeries{
			1: {
				{Slug: "solo-leveling"},
				{Slug: "bad-series"},
			},
			2: {},
		},
		series: map[string]content.ManhwaSeries{
			"solo-leveling": {
				Slug: "solo-leveling",
				Chapters: []content.ManhwaChapterRef{
					{Slug: "solo-leveling-chapter-2"},
					{Slug: "solo-leveling-chapter-1"},
				},
			},
		},
		chapters: map[string]content.ManhwaChapter{
			"solo-leveling-chapter-1": {Slug: "solo-leveling-chapter-1", SeriesSlug: "solo-leveling"},
		},
		errors: map[string]error{
			"series:bad-series":              fmt.Errorf("blocked"),
			"chapter:solo-leveling-chapter-2": fmt.Errorf("missing pages"),
		},
	}
	seriesWriter := &fakeSeriesWriter{}
	chapterWriter := &fakeChapterWriter{}

	report, err := runFullBackfill(context.Background(), fetcher, seriesWriter, chapterWriter, backfillOptions{
		StartPage:            1,
		MaxCatalogPages:      10,
		MaxChaptersPerSeries: 0,
	})
	if err != nil {
		t.Fatalf("runFullBackfill returned error: %v", err)
	}

	if report.DiscoveredSeries != 2 {
		t.Fatalf("DiscoveredSeries = %d, want 2", report.DiscoveredSeries)
	}
	if report.FailedSeries != 1 {
		t.Fatalf("FailedSeries = %d, want 1", report.FailedSeries)
	}
	if report.AttemptedChapters != 2 {
		t.Fatalf("AttemptedChapters = %d, want 2", report.AttemptedChapters)
	}
	if report.SucceededChapters != 1 {
		t.Fatalf("SucceededChapters = %d, want 1", report.SucceededChapters)
	}
	if report.FailedChapters != 1 {
		t.Fatalf("FailedChapters = %d, want 1", report.FailedChapters)
	}
	if report.Failures["bad-series"] == "" {
		t.Fatal("expected series failure to be recorded")
	}
	if report.Failures["solo-leveling-chapter-2"] == "" {
		t.Fatal("expected chapter failure to be recorded")
	}
}
