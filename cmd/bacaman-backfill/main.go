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
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/bacaman"
	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/content"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
)

type bacamanFetcher interface {
	FetchCatalog(ctx context.Context) ([]content.ManhwaSeries, error)
	FetchSeries(ctx context.Context, slug string) (content.ManhwaSeries, error)
	FetchChapter(ctx context.Context, slug string) (content.ManhwaChapter, error)
}

type seriesWriter interface {
	UpsertManhwaSeries(ctx context.Context, series content.ManhwaSeries) error
}

type chapterWriter interface {
	UpsertManhwaChapter(ctx context.Context, chapter content.ManhwaChapter) error
}

type backfillOptions struct {
	MaxSeries            int
	MaxChaptersPerSeries int
}

type fullBackfillReport struct {
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

	timeout, err := parseTimeoutOverride("BACAMAN_HTTP_TIMEOUT", cfg.HTTPTimeout)
	if err != nil {
		return err
	}
	fetcher := bacaman.NewService(
		bacaman.NewClient(cfg.BacamanUserAgent, cfg.BacamanCookie, timeout),
		cfg.BacamanBaseURL,
	)
	contentStore := store.NewContentStore(db)

	report, err := runFullBackfill(ctx, fetcher, contentStore, contentStore, backfillOptions{
		MaxSeries:            parseNonNegativeInt("BACAMAN_MAX_SERIES", 0),
		MaxChaptersPerSeries: parseNonNegativeInt("BACAMAN_MAX_CHAPTERS_PER_SERIES", 0),
	})
	if err != nil {
		return err
	}

	log.Printf(
		"bacaman full backfill done: discovered_series=%d succeeded_series=%d failed_series=%d attempted_chapters=%d succeeded_chapters=%d failed_chapters=%d",
		report.DiscoveredSeries,
		report.SucceededSeries,
		report.FailedSeries,
		report.AttemptedChapters,
		report.SucceededChapters,
		report.FailedChapters,
	)
	if len(report.Failures) > 0 {
		log.Printf("bacaman failures recorded: %d", len(report.Failures))
	}
	return nil
}

func runFullBackfill(
	ctx context.Context,
	fetcher bacamanFetcher,
	seriesWriter seriesWriter,
	chapterWriter chapterWriter,
	options backfillOptions,
) (fullBackfillReport, error) {
	if fetcher == nil {
		return fullBackfillReport{}, fmt.Errorf("bacaman fetcher is required")
	}
	if seriesWriter == nil {
		return fullBackfillReport{}, fmt.Errorf("bacaman series writer is required")
	}
	if chapterWriter == nil {
		return fullBackfillReport{}, fmt.Errorf("bacaman chapter writer is required")
	}

	report := fullBackfillReport{
		Failures: make(map[string]string),
	}
	items, err := fetcher.FetchCatalog(ctx)
	if err != nil {
		return report, fmt.Errorf("fetch catalog: %w", err)
	}

	seenSeries := make(map[string]struct{})
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if options.MaxSeries > 0 && report.DiscoveredSeries >= options.MaxSeries {
			return report, nil
		}
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
			log.Printf("bacaman series fetch failed: slug=%s error=%s", slug, err)
			continue
		}
		if err := seriesWriter.UpsertManhwaSeries(ctx, series); err != nil {
			report.FailedSeries++
			report.Failures[slug] = err.Error()
			log.Printf("bacaman series store failed: slug=%s error=%s", slug, err)
			continue
		}

		report.SucceededSeries++
		log.Printf("bacaman series synced: slug=%s chapters=%d", series.Slug, len(series.Chapters))

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
				log.Printf("bacaman chapter fetch failed: series=%s chapter=%s error=%s", series.Slug, chapterSlug, err)
				continue
			}
			if err := chapterWriter.UpsertManhwaChapter(ctx, chapter); err != nil {
				report.FailedChapters++
				report.Failures[chapterSlug] = err.Error()
				log.Printf("bacaman chapter store failed: series=%s chapter=%s error=%s", series.Slug, chapterSlug, err)
				continue
			}

			report.SucceededChapters++
			log.Printf("bacaman chapter synced: series=%s chapter=%s pages=%d", series.Slug, chapter.Slug, len(chapter.Pages))
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

func parseTimeoutOverride(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return timeout, nil
}
