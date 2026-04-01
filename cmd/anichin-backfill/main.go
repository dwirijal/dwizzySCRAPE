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

	"github.com/dwirijal/dwizzySCRAPE/internal/anichin"
	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
)

type anichinFetcher interface {
	FetchCatalog(ctx context.Context, section string, page int) ([]anichin.CatalogItem, error)
	FetchAnimeDetail(ctx context.Context, slug string) (anichin.AnimeDetail, error)
	FetchEpisodeDetail(ctx context.Context, animeSlug string, ref anichin.EpisodeRef) (anichin.EpisodeDetail, error)
}

type anichinWriter interface {
	UpsertCatalogItems(ctx context.Context, items []anichin.CatalogItem) (int, error)
	UpsertAnimeDetail(ctx context.Context, detail anichin.AnimeDetail) error
	UpsertEpisodeDetail(ctx context.Context, detail anichin.EpisodeDetail) error
	HasAnimeDetail(ctx context.Context, slug string) (bool, error)
	HasEpisodeDetail(ctx context.Context, slug string) (bool, error)
}

type backfillOptions struct {
	Sections             []string
	MaxPagesPerSection   int
	MaxEpisodesPerSeries int
	MaxSeries            int
	SkipExisting         bool
}

type fullBackfillReport struct {
	CatalogPages      int
	DiscoveredSeries  int
	SucceededSeries   int
	FailedSeries      int
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

	timeout := cfg.HTTPTimeout
	if raw := strings.TrimSpace(os.Getenv("ANICHIN_HTTP_TIMEOUT")); raw != "" {
		timeout, err = time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("parse ANICHIN_HTTP_TIMEOUT: %w", err)
		}
	}
	fetcher := anichin.NewService(
		anichin.NewClient(cfg.AnichinUserAgent, cfg.AnichinCookie, timeout),
		cfg.AnichinBaseURL,
		time.Time{},
	)
	writer := store.NewAnichinStoreWithDB(db)

	report, err := runFullBackfill(ctx, fetcher, writer, backfillOptions{
		Sections:             parseCSV(os.Getenv("ANICHIN_SECTIONS"), []string{"ongoing", "completed"}),
		MaxPagesPerSection:   parsePositiveInt("ANICHIN_MAX_PAGES_PER_SECTION", 100),
		MaxEpisodesPerSeries: parseNonNegativeInt("ANICHIN_MAX_EPISODES_PER_SERIES", 0),
		MaxSeries:            parseNonNegativeInt("ANICHIN_MAX_SERIES", 0),
		SkipExisting:         parseBoolDefault("ANICHIN_SKIP_EXISTING", true),
	})
	if err != nil {
		return err
	}

	log.Printf(
		"anichin full backfill done: catalog_pages=%d discovered_series=%d succeeded_series=%d failed_series=%d attempted_episodes=%d succeeded_episodes=%d failed_episodes=%d",
		report.CatalogPages,
		report.DiscoveredSeries,
		report.SucceededSeries,
		report.FailedSeries,
		report.AttemptedEpisodes,
		report.SucceededEpisodes,
		report.FailedEpisodes,
	)
	if len(report.Failures) > 0 {
		log.Printf("anichin failures recorded: %d", len(report.Failures))
	}
	return nil
}

func runFullBackfill(ctx context.Context, fetcher anichinFetcher, writer anichinWriter, options backfillOptions) (fullBackfillReport, error) {
	if fetcher == nil {
		return fullBackfillReport{}, fmt.Errorf("anichin fetcher is required")
	}
	if writer == nil {
		return fullBackfillReport{}, fmt.Errorf("anichin writer is required")
	}
	if len(options.Sections) == 0 {
		options.Sections = []string{"ongoing", "completed"}
	}
	if options.MaxPagesPerSection <= 0 {
		options.MaxPagesPerSection = 100
	}

	report := fullBackfillReport{Failures: make(map[string]string)}
	seen := make(map[string]struct{})

	for _, section := range options.Sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		for page := 1; page <= options.MaxPagesPerSection; page++ {
			items, err := fetcher.FetchCatalog(ctx, section, page)
			if err != nil {
				return report, fmt.Errorf("fetch catalog section %s page %d: %w", section, page, err)
			}
			if len(items) == 0 {
				break
			}
			report.CatalogPages++
			log.Printf("anichin catalog page discovered: section=%s page=%d items=%d", section, page, len(items))

			if _, err := writer.UpsertCatalogItems(ctx, items); err != nil {
				return report, fmt.Errorf("upsert catalog section %s page %d: %w", section, page, err)
			}

			for _, item := range items {
				if options.MaxSeries > 0 && report.DiscoveredSeries >= options.MaxSeries {
					return report, nil
				}
				slug := strings.TrimSpace(item.Slug)
				if slug == "" {
					continue
				}
				if _, ok := seen[slug]; ok {
					continue
				}
				seen[slug] = struct{}{}
				report.DiscoveredSeries++

				detailExists := false
				if options.SkipExisting {
					exists, err := writer.HasAnimeDetail(ctx, slug)
					if err != nil {
						return report, err
					}
					detailExists = exists
				}

				detail, err := fetcher.FetchAnimeDetail(ctx, slug)
				if err != nil {
					report.FailedSeries++
					report.Failures[slug] = err.Error()
					log.Printf("anichin detail fetch failed: section=%s page=%d slug=%s error=%s", section, page, slug, err)
					continue
				}
				if detailExists {
					log.Printf("anichin detail skip existing: section=%s page=%d slug=%s", section, page, slug)
				} else {
					if err := writer.UpsertAnimeDetail(ctx, detail); err != nil {
						report.FailedSeries++
						report.Failures[slug] = err.Error()
						log.Printf("anichin detail store failed: section=%s page=%d slug=%s error=%s", section, page, slug, err)
						continue
					}
					report.SucceededSeries++
					log.Printf("anichin series synced: section=%s page=%d slug=%s episodes=%d", section, page, slug, len(detail.EpisodeRefs))
				}

				episodes := detail.EpisodeRefs
				if options.MaxEpisodesPerSeries > 0 && len(episodes) > options.MaxEpisodesPerSeries {
					episodes = episodes[:options.MaxEpisodesPerSeries]
				}
				for _, ref := range episodes {
					if strings.TrimSpace(ref.Slug) == "" {
						continue
					}
					report.AttemptedEpisodes++
					if options.SkipExisting {
						exists, err := writer.HasEpisodeDetail(ctx, ref.Slug)
						if err != nil {
							return report, err
						}
						if exists {
							log.Printf("anichin episode skip existing: series=%s episode=%s", detail.Slug, ref.Slug)
							continue
						}
					}
					episode, err := fetcher.FetchEpisodeDetail(ctx, detail.Slug, ref)
					if err != nil {
						report.FailedEpisodes++
						report.Failures[ref.Slug] = err.Error()
						log.Printf("anichin episode fetch failed: series=%s episode=%s error=%s", detail.Slug, ref.Slug, err)
						continue
					}
					if err := writer.UpsertEpisodeDetail(ctx, episode); err != nil {
						report.FailedEpisodes++
						report.Failures[ref.Slug] = err.Error()
						log.Printf("anichin episode store failed: series=%s episode=%s error=%s", detail.Slug, ref.Slug, err)
						continue
					}
					report.SucceededEpisodes++
					log.Printf("anichin episode synced: series=%s episode=%s", detail.Slug, episode.EpisodeSlug)
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

func parseCSV(raw string, fallback []string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	if len(values) == 0 {
		return fallback
	}
	return values
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
