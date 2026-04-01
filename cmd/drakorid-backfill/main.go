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
	"github.com/dwirijal/dwizzySCRAPE/internal/drakorid"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
)

type drakoridFetcher interface {
	FetchOngoingCatalog(ctx context.Context, page int) ([]drakorid.CatalogItem, error)
	FetchMovieCatalog(ctx context.Context, page int) ([]drakorid.CatalogItem, error)
	FetchDetail(ctx context.Context, slug string) (drakorid.Detail, error)
	FetchEpisodeDetail(ctx context.Context, item drakorid.Detail, ref drakorid.EpisodeRef) (drakorid.EpisodeDetail, error)
}

type drakoridWriter interface {
	UpsertCatalogItems(ctx context.Context, items []drakorid.CatalogItem) (int, error)
	UpsertDetail(ctx context.Context, detail drakorid.Detail) error
	UpsertEpisodeDetail(ctx context.Context, detail drakorid.EpisodeDetail) error
	HasDetail(ctx context.Context, mediaType, slug string) (bool, error)
	HasEpisodeDetail(ctx context.Context, slug string) (bool, error)
}

type backfillOptions struct {
	MaxOngoingPages    int
	MaxMoviePages      int
	MaxItems           int
	MaxEpisodesPerItem int
	SkipExisting       bool
}

type fullBackfillReport struct {
	OngoingPages      int
	MoviePages        int
	DiscoveredItems   int
	SucceededItems    int
	FailedItems       int
	AttemptedEpisodes int
	SucceededEpisodes int
	FailedEpisodes    int
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

	timeout, err := parseTimeoutOverride("DRAKORID_HTTP_TIMEOUT", cfg.HTTPTimeout)
	if err != nil {
		return err
	}
	fetcher := drakorid.NewService(
		drakorid.NewClient(cfg.DrakoridUserAgent, cfg.DrakoridCookie, timeout),
		cfg.DrakoridBaseURL,
		time.Time{},
	)
	writer := store.NewDrakoridStoreWithDB(db)

	report, err := runFullBackfill(ctx, fetcher, writer, backfillOptions{
		MaxOngoingPages:    parsePositiveInt("DRAKORID_MAX_ONGOING_PAGES", 10),
		MaxMoviePages:      parsePositiveInt("DRAKORID_MAX_MOVIE_PAGES", 10),
		MaxItems:           parseNonNegativeInt("DRAKORID_MAX_ITEMS", 0),
		MaxEpisodesPerItem: parseNonNegativeInt("DRAKORID_MAX_EPISODES_PER_ITEM", 0),
		SkipExisting:       parseBoolDefault("DRAKORID_SKIP_EXISTING", true),
	})
	if err != nil {
		return err
	}

	log.Printf(
		"drakorid full backfill done: ongoing_pages=%d movie_pages=%d discovered_items=%d succeeded_items=%d failed_items=%d attempted_episodes=%d succeeded_episodes=%d failed_episodes=%d",
		report.OngoingPages,
		report.MoviePages,
		report.DiscoveredItems,
		report.SucceededItems,
		report.FailedItems,
		report.AttemptedEpisodes,
		report.SucceededEpisodes,
		report.FailedEpisodes,
	)
	if len(report.Failures) > 0 {
		log.Printf("drakorid failures recorded: %d", len(report.Failures))
	}
	return nil
}

func runFullBackfill(ctx context.Context, fetcher drakoridFetcher, writer drakoridWriter, options backfillOptions) (fullBackfillReport, error) {
	if fetcher == nil {
		return fullBackfillReport{}, fmt.Errorf("drakorid fetcher is required")
	}
	if writer == nil {
		return fullBackfillReport{}, fmt.Errorf("drakorid writer is required")
	}
	if options.MaxOngoingPages <= 0 {
		options.MaxOngoingPages = 10
	}
	if options.MaxMoviePages <= 0 {
		options.MaxMoviePages = 10
	}

	report := fullBackfillReport{Failures: make(map[string]string)}
	seen := make(map[string]struct{})

	processItems := func(items []drakorid.CatalogItem, kind string, page int) error {
		if len(items) == 0 {
			return nil
		}
		if options.MaxItems > 0 {
			remaining := options.MaxItems - report.DiscoveredItems
			if remaining <= 0 {
				return nil
			}
			if len(items) > remaining {
				items = items[:remaining]
			}
		}
		if _, err := writer.UpsertCatalogItems(ctx, items); err != nil {
			return fmt.Errorf("upsert %s catalog page %d: %w", kind, page, err)
		}
		for _, item := range items {
			if options.MaxItems > 0 && report.DiscoveredItems >= options.MaxItems {
				return nil
			}
			slug := strings.TrimSpace(item.Slug)
			if slug == "" {
				continue
			}
			if _, ok := seen[slug]; ok {
				continue
			}
			seen[slug] = struct{}{}
			report.DiscoveredItems++

			detailExists := false
			if options.SkipExisting {
				exists, err := writer.HasDetail(ctx, item.MediaType, slug)
				if err != nil {
					return err
				}
				detailExists = exists
			}
			detail, err := fetcher.FetchDetail(ctx, slug)
			if err != nil {
				report.FailedItems++
				report.Failures[slug] = err.Error()
				log.Printf("drakorid detail fetch failed: kind=%s page=%d slug=%s error=%s", kind, page, slug, err)
				continue
			}
			if detailExists {
				log.Printf("drakorid detail skip existing: kind=%s page=%d slug=%s", kind, page, slug)
			} else {
				if err := writer.UpsertDetail(ctx, detail); err != nil {
					report.FailedItems++
					report.Failures[slug] = err.Error()
					log.Printf("drakorid detail store failed: kind=%s page=%d slug=%s error=%s", kind, page, slug, err)
					continue
				}
				report.SucceededItems++
				log.Printf("drakorid item synced: kind=%s page=%d slug=%s media_type=%s episodes=%d", kind, page, slug, detail.MediaType, len(detail.EpisodeRefs))
			}

			episodes := detail.EpisodeRefs
			if options.MaxEpisodesPerItem > 0 && len(episodes) > options.MaxEpisodesPerItem {
				episodes = episodes[:options.MaxEpisodesPerItem]
			}
			for _, ref := range episodes {
				epSlug := strings.TrimSpace(ref.Number)
				if epSlug == "" {
					continue
				}
				report.AttemptedEpisodes++
				unitSlug := detail.Slug + "-episode-" + strings.ReplaceAll(ref.Number, ".", "-")
				if options.SkipExisting {
					exists, err := writer.HasEpisodeDetail(ctx, unitSlug)
					if err != nil {
						return err
					}
					if exists {
						log.Printf("drakorid episode skip existing: series=%s episode=%s", detail.Slug, unitSlug)
						continue
					}
				}
				episode, err := fetcher.FetchEpisodeDetail(ctx, detail, ref)
				if err != nil {
					report.FailedEpisodes++
					report.Failures[unitSlug] = err.Error()
					log.Printf("drakorid episode fetch failed: series=%s episode=%s error=%s", detail.Slug, unitSlug, err)
					continue
				}
				if err := writer.UpsertEpisodeDetail(ctx, episode); err != nil {
					report.FailedEpisodes++
					report.Failures[unitSlug] = err.Error()
					log.Printf("drakorid episode store failed: series=%s episode=%s error=%s", detail.Slug, unitSlug, err)
					continue
				}
				report.SucceededEpisodes++
				log.Printf("drakorid episode synced: series=%s episode=%s", detail.Slug, episode.EpisodeSlug)
			}
		}
		return nil
	}

	for page := 1; page <= options.MaxOngoingPages; page++ {
		items, err := fetcher.FetchOngoingCatalog(ctx, page)
		if err != nil {
			return report, fmt.Errorf("fetch ongoing page %d: %w", page, err)
		}
		if len(items) == 0 {
			break
		}
		report.OngoingPages++
		log.Printf("drakorid ongoing page discovered: page=%d items=%d", page, len(items))
		if err := processItems(items, "ongoing", page); err != nil {
			if options.MaxItems > 0 && report.DiscoveredItems >= options.MaxItems {
				return report, nil
			}
			return report, err
		}
		if options.MaxItems > 0 && report.DiscoveredItems >= options.MaxItems {
			return report, nil
		}
	}

	for page := 1; page <= options.MaxMoviePages; page++ {
		items, err := fetcher.FetchMovieCatalog(ctx, page)
		if err != nil {
			return report, fmt.Errorf("fetch movie page %d: %w", page, err)
		}
		if len(items) == 0 {
			break
		}
		report.MoviePages++
		log.Printf("drakorid movie page discovered: page=%d items=%d", page, len(items))
		if err := processItems(items, "movie", page); err != nil {
			if options.MaxItems > 0 && report.DiscoveredItems >= options.MaxItems {
				return report, nil
			}
			return report, err
		}
		if options.MaxItems > 0 && report.DiscoveredItems >= options.MaxItems {
			return report, nil
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

func parseBoolDefault(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
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
