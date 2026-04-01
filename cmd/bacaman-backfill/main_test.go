package main

import (
	"context"
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

type fakeFetcher struct {
	catalog  []content.ManhwaSeries
	series   map[string]content.ManhwaSeries
	chapters map[string]content.ManhwaChapter
}

func (f fakeFetcher) FetchCatalog(context.Context) ([]content.ManhwaSeries, error) {
	return f.catalog, nil
}

func (f fakeFetcher) FetchSeries(_ context.Context, slug string) (content.ManhwaSeries, error) {
	return f.series[slug], nil
}

func (f fakeFetcher) FetchChapter(_ context.Context, slug string) (content.ManhwaChapter, error) {
	return f.chapters[slug], nil
}

type fakeWriter struct {
	series   []content.ManhwaSeries
	chapters []content.ManhwaChapter
}

func (w *fakeWriter) UpsertManhwaSeries(_ context.Context, series content.ManhwaSeries) error {
	w.series = append(w.series, series)
	return nil
}

func (w *fakeWriter) UpsertManhwaChapter(_ context.Context, chapter content.ManhwaChapter) error {
	w.chapters = append(w.chapters, chapter)
	return nil
}

func TestRunFullBackfill(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		catalog: []content.ManhwaSeries{
			{Slug: "series-a"},
		},
		series: map[string]content.ManhwaSeries{
			"series-a": {
				Slug: "series-a",
				Chapters: []content.ManhwaChapterRef{
					{Slug: "series-a-chapter-2"},
					{Slug: "series-a-chapter-1"},
				},
			},
		},
		chapters: map[string]content.ManhwaChapter{
			"series-a-chapter-2": {Slug: "series-a-chapter-2"},
			"series-a-chapter-1": {Slug: "series-a-chapter-1"},
		},
	}
	writer := &fakeWriter{}

	report, err := runFullBackfill(context.Background(), fetcher, writer, writer, backfillOptions{})
	if err != nil {
		t.Fatalf("runFullBackfill returned error: %v", err)
	}
	if report.DiscoveredSeries != 1 {
		t.Fatalf("DiscoveredSeries = %d", report.DiscoveredSeries)
	}
	if report.SucceededSeries != 1 {
		t.Fatalf("SucceededSeries = %d", report.SucceededSeries)
	}
	if report.SucceededChapters != 2 {
		t.Fatalf("SucceededChapters = %d", report.SucceededChapters)
	}
}
