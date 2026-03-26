package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

type fakeManhwaFetcher struct {
	catalog  map[int][]content.ManhwaSeries
	series   map[string]content.ManhwaSeries
	chapters map[string]content.ManhwaChapter
	errors   map[string]error
}

func (f fakeManhwaFetcher) FetchCatalog(_ context.Context, page int) ([]content.ManhwaSeries, error) {
	if err := f.errors[fmt.Sprintf("catalog:%d", page)]; err != nil {
		return nil, err
	}
	return f.catalog[page], nil
}

func (f fakeManhwaFetcher) FetchSeries(_ context.Context, slug string) (content.ManhwaSeries, error) {
	if err := f.errors["series:"+slug]; err != nil {
		return content.ManhwaSeries{}, err
	}
	series, ok := f.series[slug]
	if !ok {
		return content.ManhwaSeries{}, fmt.Errorf("missing series: %s", slug)
	}
	return series, nil
}

func (f fakeManhwaFetcher) FetchChapter(_ context.Context, slug string) (content.ManhwaChapter, error) {
	if err := f.errors["chapter:"+slug]; err != nil {
		return content.ManhwaChapter{}, err
	}
	chapter, ok := f.chapters[slug]
	if !ok {
		return content.ManhwaChapter{}, fmt.Errorf("missing chapter: %s", slug)
	}
	return chapter, nil
}

type fakeManhwaWriter struct {
	slugs  []string
	errors map[string]error
}

func (w *fakeManhwaWriter) UpsertManhwaSeries(_ context.Context, series content.ManhwaSeries) error {
	if err := w.errors[series.Slug]; err != nil {
		return err
	}
	w.slugs = append(w.slugs, series.Slug)
	return nil
}

type fakeManhwaChapterWriter struct {
	slugs  []string
	errors map[string]error
}

func (w *fakeManhwaChapterWriter) UpsertManhwaChapter(_ context.Context, chapter content.ManhwaChapter) error {
	if err := w.errors[chapter.Slug]; err != nil {
		return err
	}
	w.slugs = append(w.slugs, chapter.Slug)
	return nil
}

func TestBackfillManhwaSeriesPages(t *testing.T) {
	t.Parallel()

	fetcher := fakeManhwaFetcher{
		catalog: map[int][]content.ManhwaSeries{
			1: {
				{Slug: "solo-leveling"},
				{Slug: "omniscient-reader"},
			},
			2: {
				{Slug: "solo-leveling"},
				{Slug: "legend-of-the-northern-blade"},
			},
		},
		series: map[string]content.ManhwaSeries{
			"solo-leveling":                {Slug: "solo-leveling", Chapters: []content.ManhwaChapterRef{{Slug: "a"}}},
			"omniscient-reader":            {Slug: "omniscient-reader", Chapters: []content.ManhwaChapterRef{{Slug: "b"}}},
			"legend-of-the-northern-blade": {Slug: "legend-of-the-northern-blade", Chapters: []content.ManhwaChapterRef{{Slug: "c"}}},
		},
	}
	writer := &fakeManhwaWriter{}

	report, err := backfillManhwaSeriesPages(context.Background(), fetcher, writer, 1, 2)
	if err != nil {
		t.Fatalf("backfillManhwaSeriesPages returned error: %v", err)
	}

	if report.Discovered != 3 {
		t.Fatalf("Discovered = %d, want 3", report.Discovered)
	}
	if report.Succeeded != 3 {
		t.Fatalf("Succeeded = %d, want 3", report.Succeeded)
	}
	if report.Failed != 0 {
		t.Fatalf("Failed = %d, want 0", report.Failed)
	}
	if len(writer.slugs) != 3 {
		t.Fatalf("synced slugs = %d, want 3", len(writer.slugs))
	}
}

func TestBackfillManhwaSeriesPagesContinuesOnSeriesFailure(t *testing.T) {
	t.Parallel()

	fetcher := fakeManhwaFetcher{
		catalog: map[int][]content.ManhwaSeries{
			1: {
				{Slug: "solo-leveling"},
				{Slug: "bad-series"},
			},
		},
		series: map[string]content.ManhwaSeries{
			"solo-leveling": {Slug: "solo-leveling"},
		},
		errors: map[string]error{
			"series:bad-series": fmt.Errorf("blocked"),
		},
	}
	writer := &fakeManhwaWriter{}

	report, err := backfillManhwaSeriesPages(context.Background(), fetcher, writer, 1, 1)
	if err != nil {
		t.Fatalf("backfillManhwaSeriesPages returned error: %v", err)
	}

	if report.Discovered != 2 {
		t.Fatalf("Discovered = %d, want 2", report.Discovered)
	}
	if report.Succeeded != 1 {
		t.Fatalf("Succeeded = %d, want 1", report.Succeeded)
	}
	if report.Failed != 1 {
		t.Fatalf("Failed = %d, want 1", report.Failed)
	}
	if report.Failures["bad-series"] == "" {
		t.Fatal("expected failure reason for bad-series")
	}
}

func TestBackfillManhwaChapterPages(t *testing.T) {
	t.Parallel()

	fetcher := fakeManhwaFetcher{
		catalog: map[int][]content.ManhwaSeries{
			1: {
				{Slug: "solo-leveling"},
				{Slug: "omniscient-reader"},
			},
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
			"omniscient-reader-chapter-1": {Slug: "omniscient-reader-chapter-1", SeriesSlug: "omniscient-reader"},
		},
	}
	seriesWriter := &fakeManhwaWriter{}
	chapterWriter := &fakeManhwaChapterWriter{}

	report, err := backfillManhwaChapterPages(context.Background(), fetcher, seriesWriter, chapterWriter, 1, 1, 1)
	if err != nil {
		t.Fatalf("backfillManhwaChapterPages returned error: %v", err)
	}

	if report.DiscoveredSeries != 2 {
		t.Fatalf("DiscoveredSeries = %d, want 2", report.DiscoveredSeries)
	}
	if report.AttemptedChapters != 2 {
		t.Fatalf("AttemptedChapters = %d, want 2", report.AttemptedChapters)
	}
	if report.SucceededChapters != 2 {
		t.Fatalf("SucceededChapters = %d, want 2", report.SucceededChapters)
	}
	if report.FailedChapters != 0 {
		t.Fatalf("FailedChapters = %d, want 0", report.FailedChapters)
	}
	if len(seriesWriter.slugs) != 2 {
		t.Fatalf("series writes = %d, want 2", len(seriesWriter.slugs))
	}
	if len(chapterWriter.slugs) != 2 {
		t.Fatalf("chapter writes = %d, want 2", len(chapterWriter.slugs))
	}
}

func TestBackfillManhwaChapterPagesContinuesOnChapterFailure(t *testing.T) {
	t.Parallel()

	fetcher := fakeManhwaFetcher{
		catalog: map[int][]content.ManhwaSeries{
			1: {{Slug: "solo-leveling"}},
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
			"chapter:solo-leveling-chapter-2": fmt.Errorf("blocked"),
		},
	}
	seriesWriter := &fakeManhwaWriter{}
	chapterWriter := &fakeManhwaChapterWriter{}

	report, err := backfillManhwaChapterPages(context.Background(), fetcher, seriesWriter, chapterWriter, 1, 1, 2)
	if err != nil {
		t.Fatalf("backfillManhwaChapterPages returned error: %v", err)
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
	if report.Failures["solo-leveling-chapter-2"] == "" {
		t.Fatal("expected failure reason for chapter")
	}
}
