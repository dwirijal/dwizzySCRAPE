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

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/content"
	"github.com/dwirijal/dwizzySCRAPE/internal/mangasusuku"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
)

type mangasusukuFetcher interface {
	FetchCatalog(ctx context.Context, letter string, page int) ([]content.ManhwaSeries, error)
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
	StartLetter          string
	MaxLetters           int
	MaxPagesPerLetter    int
	MaxChaptersPerSeries int
	MaxSeries            int
}

type fullBackfillReport struct {
	StartLetter       string
	LastLetter        string
	LettersProcessed  int
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

	timeout, err := parseTimeoutOverride("MANGASUSUKU_HTTP_TIMEOUT", cfg.HTTPTimeout)
	if err != nil {
		return err
	}
	fetcher := mangasusuku.NewService(
		mangasusuku.NewClient(cfg.MangasusukuUserAgent, cfg.MangasusukuCookie, timeout),
		cfg.MangasusukuBaseURL,
	)
	contentStore := store.NewContentStore(db)

	report, err := runFullBackfill(ctx, fetcher, contentStore, contentStore, backfillOptions{
		StartLetter:          parseStringEnv("MANGASUSUKU_START_LETTER", "."),
		MaxLetters:           parseNonNegativeInt("MANGASUSUKU_MAX_LETTERS", 0),
		MaxPagesPerLetter:    parsePositiveInt("MANGASUSUKU_MAX_PAGES_PER_LETTER", 100),
		MaxChaptersPerSeries: parseNonNegativeInt("MANGASUSUKU_MAX_CHAPTERS_PER_SERIES", 0),
		MaxSeries:            parseNonNegativeInt("MANGASUSUKU_MAX_SERIES", 0),
	})
	if err != nil {
		return err
	}

	log.Printf(
		"mangasusuku full backfill done: start_letter=%s last_letter=%s letters=%d catalog_pages=%d discovered_series=%d succeeded_series=%d failed_series=%d attempted_chapters=%d succeeded_chapters=%d failed_chapters=%d",
		report.StartLetter,
		report.LastLetter,
		report.LettersProcessed,
		report.CatalogPages,
		report.DiscoveredSeries,
		report.SucceededSeries,
		report.FailedSeries,
		report.AttemptedChapters,
		report.SucceededChapters,
		report.FailedChapters,
	)
	if len(report.Failures) > 0 {
		log.Printf("mangasusuku failures recorded: %d", len(report.Failures))
	}
	return nil
}

func runFullBackfill(
	ctx context.Context,
	fetcher mangasusukuFetcher,
	seriesWriter seriesWriter,
	chapterWriter chapterWriter,
	options backfillOptions,
) (fullBackfillReport, error) {
	if fetcher == nil {
		return fullBackfillReport{}, fmt.Errorf("mangasusuku fetcher is required")
	}
	if seriesWriter == nil {
		return fullBackfillReport{}, fmt.Errorf("mangasusuku series writer is required")
	}
	if chapterWriter == nil {
		return fullBackfillReport{}, fmt.Errorf("mangasusuku chapter writer is required")
	}

	letters := catalogLetters()
	startIndex := indexOfLetter(letters, options.StartLetter)
	if startIndex < 0 {
		startIndex = 0
	}
	letters = letters[startIndex:]
	if options.MaxLetters > 0 && options.MaxLetters < len(letters) {
		letters = letters[:options.MaxLetters]
	}
	if options.MaxPagesPerLetter <= 0 {
		options.MaxPagesPerLetter = 100
	}

	report := fullBackfillReport{
		StartLetter: parseStringFallback(options.StartLetter, letters),
		Failures:    make(map[string]string),
	}
	seenSeries := make(map[string]struct{})

	for _, letter := range letters {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		report.LettersProcessed++
		report.LastLetter = letter

		for page := 1; page <= options.MaxPagesPerLetter; page++ {
			items, err := fetcher.FetchCatalog(ctx, letter, page)
			if err != nil {
				return report, fmt.Errorf("fetch catalog letter %s page %d: %w", letter, page, err)
			}
			if len(items) == 0 {
				break
			}

			report.CatalogPages++
			log.Printf("mangasusuku catalog page discovered: letter=%s page=%d items=%d", letter, page, len(items))

			for _, item := range items {
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
					log.Printf("mangasusuku series fetch failed: letter=%s page=%d slug=%s error=%s", letter, page, slug, err)
					continue
				}
				if err := seriesWriter.UpsertManhwaSeries(ctx, series); err != nil {
					report.FailedSeries++
					report.Failures[slug] = err.Error()
					log.Printf("mangasusuku series store failed: letter=%s page=%d slug=%s error=%s", letter, page, slug, err)
					continue
				}

				report.SucceededSeries++
				log.Printf("mangasusuku series synced: letter=%s page=%d slug=%s chapters=%d", letter, page, series.Slug, len(series.Chapters))

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
						log.Printf("mangasusuku chapter fetch failed: series=%s chapter=%s error=%s", series.Slug, chapterSlug, err)
						continue
					}
					if err := chapterWriter.UpsertManhwaChapter(ctx, chapter); err != nil {
						report.FailedChapters++
						report.Failures[chapterSlug] = err.Error()
						log.Printf("mangasusuku chapter store failed: series=%s chapter=%s error=%s", series.Slug, chapterSlug, err)
						continue
					}

					report.SucceededChapters++
					log.Printf("mangasusuku chapter synced: series=%s chapter=%s pages=%d", series.Slug, chapter.Slug, len(chapter.Pages))
				}
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

func parseStringEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
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

func catalogLetters() []string {
	letters := []string{".", "0-9"}
	for letter := 'A'; letter <= 'Z'; letter++ {
		letters = append(letters, string(letter))
	}
	return letters
}

func indexOfLetter(letters []string, target string) int {
	target = strings.TrimSpace(target)
	if target == "" {
		return -1
	}
	for idx, letter := range letters {
		if strings.EqualFold(letter, target) {
			return idx
		}
	}
	return -1
}

func parseStringFallback(raw string, fallback []string) string {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		return raw
	}
	if len(fallback) == 0 {
		return ""
	}
	return fallback[0]
}
