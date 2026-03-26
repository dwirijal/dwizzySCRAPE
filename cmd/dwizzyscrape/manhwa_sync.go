package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

type manhwaCatalogFetcher interface {
	FetchCatalog(ctx context.Context, page int) ([]content.ManhwaSeries, error)
	FetchSeries(ctx context.Context, slug string) (content.ManhwaSeries, error)
	FetchChapter(ctx context.Context, slug string) (content.ManhwaChapter, error)
}

type manhwaSeriesWriter interface {
	UpsertManhwaSeries(ctx context.Context, series content.ManhwaSeries) error
}

type manhwaChapterWriter interface {
	UpsertManhwaChapter(ctx context.Context, chapter content.ManhwaChapter) error
}

type ManhwaBackfillReport struct {
	StartPage  int
	EndPage    int
	Discovered int
	Succeeded  int
	Failed     int
	Failures   map[string]string
}

type ManhwaChapterBackfillReport struct {
	StartPage          int
	EndPage            int
	MaxChaptersPerSlug int
	DiscoveredSeries   int
	AttemptedChapters  int
	SucceededChapters  int
	FailedChapters     int
	Failures           map[string]string
}

func backfillManhwaSeriesPages(
	ctx context.Context,
	fetcher manhwaCatalogFetcher,
	writer manhwaSeriesWriter,
	startPage int,
	endPage int,
) (ManhwaBackfillReport, error) {
	if fetcher == nil {
		return ManhwaBackfillReport{}, fmt.Errorf("manhwa fetcher is required")
	}
	if writer == nil {
		return ManhwaBackfillReport{}, fmt.Errorf("manhwa writer is required")
	}
	if startPage <= 0 {
		return ManhwaBackfillReport{}, fmt.Errorf("start page must be greater than zero")
	}
	if endPage < startPage {
		return ManhwaBackfillReport{}, fmt.Errorf("end page must be greater than or equal to start page")
	}

	report := ManhwaBackfillReport{
		StartPage: startPage,
		EndPage:   endPage,
		Failures:  make(map[string]string),
	}
	seen := make(map[string]struct{})

	for page := startPage; page <= endPage; page++ {
		items, err := fetcher.FetchCatalog(ctx, page)
		if err != nil {
			return report, fmt.Errorf("fetch catalog page %d: %w", page, err)
		}
		log.Printf("series backfill page discovered: page=%d items=%d", page, len(items))

		for _, item := range items {
			slug := strings.TrimSpace(item.Slug)
			if slug == "" {
				continue
			}
			if _, ok := seen[slug]; ok {
				continue
			}
			seen[slug] = struct{}{}
			report.Discovered++

			series, err := fetcher.FetchSeries(ctx, slug)
			if err != nil {
				report.Failed++
				report.Failures[slug] = err.Error()
				log.Printf("series backfill fetch failed: slug=%s page=%d error=%s", slug, page, err)
				continue
			}
			if err := writer.UpsertManhwaSeries(ctx, series); err != nil {
				report.Failed++
				report.Failures[slug] = err.Error()
				log.Printf("series backfill store failed: slug=%s page=%d error=%s", slug, page, err)
				continue
			}

			report.Succeeded++
			log.Printf("series backfill synced: slug=%s page=%d chapters=%d", series.Slug, page, len(series.Chapters))
		}
	}

	return report, nil
}

func backfillManhwaChapterPages(
	ctx context.Context,
	fetcher manhwaCatalogFetcher,
	seriesWriter manhwaSeriesWriter,
	chapterWriter manhwaChapterWriter,
	startPage int,
	endPage int,
	maxChaptersPerSeries int,
) (ManhwaChapterBackfillReport, error) {
	if fetcher == nil {
		return ManhwaChapterBackfillReport{}, fmt.Errorf("manhwa fetcher is required")
	}
	if seriesWriter == nil {
		return ManhwaChapterBackfillReport{}, fmt.Errorf("manhwa series writer is required")
	}
	if chapterWriter == nil {
		return ManhwaChapterBackfillReport{}, fmt.Errorf("manhwa chapter writer is required")
	}
	if startPage <= 0 {
		return ManhwaChapterBackfillReport{}, fmt.Errorf("start page must be greater than zero")
	}
	if endPage < startPage {
		return ManhwaChapterBackfillReport{}, fmt.Errorf("end page must be greater than or equal to start page")
	}
	if maxChaptersPerSeries <= 0 {
		return ManhwaChapterBackfillReport{}, fmt.Errorf("max chapters per series must be greater than zero")
	}

	report := ManhwaChapterBackfillReport{
		StartPage:          startPage,
		EndPage:            endPage,
		MaxChaptersPerSlug: maxChaptersPerSeries,
		Failures:           make(map[string]string),
	}
	seen := make(map[string]struct{})

	for page := startPage; page <= endPage; page++ {
		items, err := fetcher.FetchCatalog(ctx, page)
		if err != nil {
			return report, fmt.Errorf("fetch catalog page %d: %w", page, err)
		}
		log.Printf("chapter backfill page discovered: page=%d items=%d", page, len(items))

		for _, item := range items {
			slug := strings.TrimSpace(item.Slug)
			if slug == "" {
				continue
			}
			if _, ok := seen[slug]; ok {
				continue
			}
			seen[slug] = struct{}{}
			report.DiscoveredSeries++

			series, err := fetcher.FetchSeries(ctx, slug)
			if err != nil {
				report.Failures[slug] = err.Error()
				log.Printf("chapter backfill fetch series failed: slug=%s page=%d error=%s", slug, page, err)
				continue
			}
			if err := seriesWriter.UpsertManhwaSeries(ctx, series); err != nil {
				report.Failures[slug] = err.Error()
				log.Printf("chapter backfill store series failed: slug=%s page=%d error=%s", slug, page, err)
				continue
			}

			chapters := series.Chapters
			if len(chapters) > maxChaptersPerSeries {
				chapters = chapters[:maxChaptersPerSeries]
			}

			for _, chapterRef := range chapters {
				chapterSlug := strings.TrimSpace(chapterRef.Slug)
				if chapterSlug == "" {
					continue
				}
				report.AttemptedChapters++

				chapter, err := fetcher.FetchChapter(ctx, chapterSlug)
				if err != nil {
					report.FailedChapters++
					report.Failures[chapterSlug] = err.Error()
					log.Printf("chapter backfill fetch failed: series=%s chapter=%s error=%s", slug, chapterSlug, err)
					continue
				}
				if err := chapterWriter.UpsertManhwaChapter(ctx, chapter); err != nil {
					report.FailedChapters++
					report.Failures[chapterSlug] = err.Error()
					log.Printf("chapter backfill store failed: series=%s chapter=%s error=%s", slug, chapterSlug, err)
					continue
				}

				report.SucceededChapters++
				log.Printf("chapter backfill synced: series=%s chapter=%s pages=%d", slug, chapter.Slug, len(chapter.Pages))
			}
		}
	}

	return report, nil
}
