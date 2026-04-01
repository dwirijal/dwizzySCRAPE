package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/content"
	"github.com/dwirijal/dwizzySCRAPE/internal/manhwaindo"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
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

type backfillOptions struct {
	StartPage            int
	MaxCatalogPages      int
	MaxChaptersPerSeries int
}

type fullBackfillReport struct {
	StartPage         int
	LastPage          int
	CatalogPages      int
	DiscoveredSeries  int
	SucceededSeries   int
	FailedSeries      int
	AttemptedChapters int
	SucceededChapters int
	FailedChapters    int
	Failures          map[string]string
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openContentDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := applyMigrations(ctx, db); err != nil {
		return err
	}

	fetcher := manhwaindo.NewService(
		manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout),
		cfg.ManhwaindoBaseURL,
	)
	contentStore := store.NewContentStore(db)

	report, err := runFullBackfill(ctx, fetcher, contentStore, contentStore, backfillOptions{
		StartPage:            parsePositiveInt("MANHWAINDO_START_PAGE", 1),
		MaxCatalogPages:      parsePositiveInt("MANHWAINDO_MAX_CATALOG_PAGES", 100),
		MaxChaptersPerSeries: parseNonNegativeInt("MANHWAINDO_MAX_CHAPTERS_PER_SERIES", 0),
	})
	if err != nil {
		return err
	}

	log.Printf(
		"manhwaindo full backfill done: pages=%d last_page=%d discovered_series=%d succeeded_series=%d failed_series=%d attempted_chapters=%d succeeded_chapters=%d failed_chapters=%d",
		report.CatalogPages,
		report.LastPage,
		report.DiscoveredSeries,
		report.SucceededSeries,
		report.FailedSeries,
		report.AttemptedChapters,
		report.SucceededChapters,
		report.FailedChapters,
	)
	if len(report.Failures) > 0 {
		log.Printf("manhwaindo failures recorded: %d", len(report.Failures))
	}

	return nil
}

func runFullBackfill(
	ctx context.Context,
	fetcher manhwaCatalogFetcher,
	seriesWriter manhwaSeriesWriter,
	chapterWriter manhwaChapterWriter,
	options backfillOptions,
) (fullBackfillReport, error) {
	if fetcher == nil {
		return fullBackfillReport{}, fmt.Errorf("manhwa fetcher is required")
	}
	if seriesWriter == nil {
		return fullBackfillReport{}, fmt.Errorf("manhwa series writer is required")
	}
	if chapterWriter == nil {
		return fullBackfillReport{}, fmt.Errorf("manhwa chapter writer is required")
	}
	if options.StartPage <= 0 {
		options.StartPage = 1
	}
	if options.MaxCatalogPages <= 0 {
		options.MaxCatalogPages = 100
	}

	report := fullBackfillReport{
		StartPage: options.StartPage,
		LastPage:  options.StartPage - 1,
		Failures:  make(map[string]string),
	}
	seenSeries := make(map[string]struct{})

	for page := options.StartPage; page < options.StartPage+options.MaxCatalogPages; page++ {
		if err := ctx.Err(); err != nil {
			return report, err
		}

		items, err := fetcher.FetchCatalog(ctx, page)
		if err != nil {
			return report, fmt.Errorf("fetch catalog page %d: %w", page, err)
		}
		if len(items) == 0 {
			break
		}

		report.CatalogPages++
		report.LastPage = page
		log.Printf("manhwaindo catalog page discovered: page=%d items=%d", page, len(items))

		for _, item := range items {
			slug := strings.TrimSpace(item.Slug)
			if slug == "" {
				continue
			}
			if _, ok := seenSeries[slug]; ok {
				continue
			}
			seenSeries[slug] = struct{}{}
			report.DiscoveredSeries++

			series, err := fetcher.FetchSeries(ctx, slug)
			if err != nil {
				report.FailedSeries++
				report.Failures[slug] = err.Error()
				log.Printf("manhwaindo series fetch failed: page=%d slug=%s error=%s", page, slug, err)
				continue
			}
			if err := seriesWriter.UpsertManhwaSeries(ctx, series); err != nil {
				report.FailedSeries++
				report.Failures[slug] = err.Error()
				log.Printf("manhwaindo series store failed: page=%d slug=%s error=%s", page, slug, err)
				continue
			}

			report.SucceededSeries++
			log.Printf("manhwaindo series synced: page=%d slug=%s chapters=%d", page, series.Slug, len(series.Chapters))

			chapters := series.Chapters
			if options.MaxChaptersPerSeries > 0 && len(chapters) > options.MaxChaptersPerSeries {
				chapters = chapters[:options.MaxChaptersPerSeries]
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
					log.Printf("manhwaindo chapter fetch failed: series=%s chapter=%s error=%s", series.Slug, chapterSlug, err)
					continue
				}
				if err := chapterWriter.UpsertManhwaChapter(ctx, chapter); err != nil {
					report.FailedChapters++
					report.Failures[chapterSlug] = err.Error()
					log.Printf("manhwaindo chapter store failed: series=%s chapter=%s error=%s", series.Slug, chapterSlug, err)
					continue
				}

				report.SucceededChapters++
				log.Printf("manhwaindo chapter synced: series=%s chapter=%s pages=%d", series.Slug, chapter.Slug, len(chapter.Pages))
			}
		}
	}

	return report, nil
}

func applyMigrations(ctx context.Context, db *store.PgxContentDB) error {
	paths, err := filepath.Glob("sql/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no sql migrations found")
	}
	for _, migrationPath := range paths {
		info, err := os.Stat(migrationPath)
		if err != nil {
			return fmt.Errorf("stat migration %s: %w", migrationPath, err)
		}
		if info.IsDir() || info.Mode()&fs.ModeType != 0 {
			continue
		}
		query, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", migrationPath, err)
		}
		if err := db.Exec(ctx, string(query)); err != nil {
			return fmt.Errorf("apply migration %s: %w", migrationPath, err)
		}
	}
	return nil
}

func openContentDB(ctx context.Context, cfg config.Config) (*store.PgxContentDB, error) {
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return store.NewPgxContentDB(ctx, cfg.DatabaseURL)
}

func parsePositiveInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseNonNegativeInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}
