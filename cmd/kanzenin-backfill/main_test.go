package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

type fakeFetcher struct {
	catalog  map[string]map[int][]content.ManhwaSeries
	series   map[string]content.ManhwaSeries
	chapters map[string]content.ManhwaChapter
	errors   map[string]error
}

func (f fakeFetcher) FetchCatalog(_ context.Context, letter string, page int) ([]content.ManhwaSeries, error) {
	if err := f.errors[fmt.Sprintf("catalog:%s:%d", letter, page)]; err != nil {
		return nil, err
	}
	if pages, ok := f.catalog[letter]; ok {
		return pages[page], nil
	}
	return nil, nil
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

func TestRunFullBackfillStopsOnEmptyLetterPage(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		catalog: map[string]map[int][]content.ManhwaSeries{
			"A": {
				1: {{Slug: "alpha-one"}, {Slug: "alpha-two"}},
				2: {},
			},
		},
		series: map[string]content.ManhwaSeries{
			"alpha-one": {Slug: "alpha-one", Chapters: []content.ManhwaChapterRef{{Slug: "alpha-one-ch-1"}}},
			"alpha-two": {Slug: "alpha-two", Chapters: []content.ManhwaChapterRef{{Slug: "alpha-two-ch-1"}}},
		},
		chapters: map[string]content.ManhwaChapter{
			"alpha-one-ch-1": {Slug: "alpha-one-ch-1", SeriesSlug: "alpha-one"},
			"alpha-two-ch-1": {Slug: "alpha-two-ch-1", SeriesSlug: "alpha-two"},
		},
	}
	seriesWriter := &fakeSeriesWriter{}
	chapterWriter := &fakeChapterWriter{}

	report, err := runFullBackfill(context.Background(), fetcher, seriesWriter, chapterWriter, backfillOptions{
		StartLetter:          "A",
		MaxLetters:           1,
		MaxPagesPerLetter:    5,
		MaxChaptersPerSeries: 0,
	})
	if err != nil {
		t.Fatalf("runFullBackfill returned error: %v", err)
	}
	if report.LettersProcessed != 1 {
		t.Fatalf("LettersProcessed = %d, want 1", report.LettersProcessed)
	}
	if report.CatalogPages != 1 {
		t.Fatalf("CatalogPages = %d, want 1", report.CatalogPages)
	}
	if report.DiscoveredSeries != 2 {
		t.Fatalf("DiscoveredSeries = %d, want 2", report.DiscoveredSeries)
	}
	if report.SucceededChapters != 2 {
		t.Fatalf("SucceededChapters = %d, want 2", report.SucceededChapters)
	}
}

func TestRunFullBackfillRespectsMaxSeries(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		catalog: map[string]map[int][]content.ManhwaSeries{
			"A": {1: {{Slug: "alpha-one"}, {Slug: "alpha-two"}}},
		},
		series: map[string]content.ManhwaSeries{
			"alpha-one": {Slug: "alpha-one"},
			"alpha-two": {Slug: "alpha-two"},
		},
	}

	report, err := runFullBackfill(context.Background(), fetcher, &fakeSeriesWriter{}, &fakeChapterWriter{}, backfillOptions{
		StartLetter: "A",
		MaxLetters:  1,
		MaxSeries:   1,
	})
	if err != nil {
		t.Fatalf("runFullBackfill returned error: %v", err)
	}
	if report.DiscoveredSeries != 1 {
		t.Fatalf("DiscoveredSeries = %d, want 1", report.DiscoveredSeries)
	}
}
