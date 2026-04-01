package main

import (
	"context"
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

type fakeFetcher struct {
	catalogs map[string][]content.ManhwaSeries
	series   map[string]content.ManhwaSeries
	chapters map[string]content.ManhwaChapter
}

func (f fakeFetcher) FetchCatalog(_ context.Context, letter string, page int) ([]content.ManhwaSeries, error) {
	if page > 1 {
		return nil, nil
	}
	return f.catalogs[letter], nil
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
		catalogs: map[string][]content.ManhwaSeries{
			"A": {{Slug: "a-dangerous-deal-and-the-girl-next-door"}},
		},
		series: map[string]content.ManhwaSeries{
			"a-dangerous-deal-and-the-girl-next-door": {
				Slug: "a-dangerous-deal-and-the-girl-next-door",
				Chapters: []content.ManhwaChapterRef{
					{Slug: "a-dangerous-deal-and-the-girl-next-door-chapter-43"},
				},
			},
		},
		chapters: map[string]content.ManhwaChapter{
			"a-dangerous-deal-and-the-girl-next-door-chapter-43": {Slug: "a-dangerous-deal-and-the-girl-next-door-chapter-43"},
		},
	}
	writer := &fakeWriter{}

	report, err := runFullBackfill(context.Background(), fetcher, writer, writer, backfillOptions{
		StartLetter: "A",
		MaxLetters:  1,
	})
	if err != nil {
		t.Fatalf("runFullBackfill returned error: %v", err)
	}
	if report.DiscoveredSeries != 1 {
		t.Fatalf("DiscoveredSeries = %d", report.DiscoveredSeries)
	}
	if report.SucceededSeries != 1 {
		t.Fatalf("SucceededSeries = %d", report.SucceededSeries)
	}
	if report.SucceededChapters != 1 {
		t.Fatalf("SucceededChapters = %d", report.SucceededChapters)
	}
}
