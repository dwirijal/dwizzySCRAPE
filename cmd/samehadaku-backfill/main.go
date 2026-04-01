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
	"github.com/dwirijal/dwizzySCRAPE/internal/jikan"
	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	command := "full"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		command = strings.TrimSpace(args[0])
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := openContentDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	switch command {
	case "migrate":
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
		log.Printf("samehadaku migrate done")
		return nil
	case "catalog":
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
		return runCatalogSync(ctx, cfg, db)
	case "details":
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
		return runDetailBackfill(ctx, cfg, db)
	case "episodes":
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
		return runEpisodeBackfill(ctx, cfg, db)
	case "full":
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
		if err := runCatalogSync(ctx, cfg, db); err != nil {
			return err
		}
		if err := runDetailBackfill(ctx, cfg, db); err != nil {
			return err
		}
		return runEpisodeBackfill(ctx, cfg, db)
	default:
		return fmt.Errorf("usage: samehadaku-backfill [migrate|catalog|details|episodes|full]")
	}
}

func runCatalogSync(ctx context.Context, cfg config.Config, db *store.PgxContentDB) error {
	fetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
	catalogStore := store.NewCatalogStoreWithDB(db)
	service := samehadaku.NewService(fetcher, catalogStore, time.Time{})

	report, err := service.SyncCatalog(ctx, cfg.CatalogURL)
	if err != nil {
		return err
	}
	log.Printf("samehadaku catalog sync done: parsed=%d upserted=%d", report.Parsed, report.Upserted)
	return nil
}

func runDetailBackfill(ctx context.Context, cfg config.Config, db *store.PgxContentDB) error {
	fetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
	catalogStore := store.NewCatalogStoreWithDB(db)
	detailStore := store.NewAnimeDetailStoreWithDB(db)
	jikanClient := jikan.NewClient(cfg.JikanBaseURL, nil)
	detailService := samehadaku.NewDetailService(catalogStore, detailStore, jikanClient, fetcher, time.Time{})
	backfillService := samehadaku.NewDetailBackfillService(catalogStore, detailStore, detailService, time.Time{})

	report, err := backfillService.Backfill(ctx, samehadaku.DetailBackfillOptions{
		BatchSize:    parsePositiveInt("SAMEHADAKU_DETAIL_BATCH_SIZE", 100),
		Limit:        parsePositiveInt("SAMEHADAKU_DETAIL_LIMIT", 0),
		IncludeSlugs: parseCSVList(os.Getenv("SAMEHADAKU_DETAIL_SLUGS")),
		SkipExisting: parseBoolDefault("SAMEHADAKU_DETAIL_SKIP_EXISTING", true),
		DelayBetween: parseDurationDefault("SAMEHADAKU_DETAIL_DELAY", 1250*time.Millisecond),
		Progress: func(progress samehadaku.DetailBackfillProgress) {
			log.Printf(
				"samehadaku detail backfill %s: slug=%s page=%d discovered=%d attempted=%d skipped=%d succeeded=%d failed=%d reason=%s",
				progress.Action,
				progress.Slug,
				progress.PageNumber,
				progress.Counts.Discovered,
				progress.Counts.Attempted,
				progress.Counts.Skipped,
				progress.Counts.Succeeded,
				progress.Counts.Failed,
				progress.Reason,
			)
		},
	})
	if err != nil {
		return err
	}

	log.Printf(
		"samehadaku detail backfill done: discovered=%d attempted=%d skipped=%d succeeded=%d failed=%d",
		report.Discovered,
		report.Attempted,
		report.Skipped,
		report.Succeeded,
		report.Failed,
	)
	return nil
}

func runEpisodeBackfill(ctx context.Context, cfg config.Config, db *store.PgxContentDB) error {
	fetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
	catalogStore := store.NewCatalogStoreWithDB(db)
	episodeStore := store.NewEpisodeDetailStoreWithDB(db)
	episodeService := samehadaku.NewEpisodeService(fetcher, episodeStore, time.Time{})
	backfillService := samehadaku.NewEpisodeBackfillService(catalogStore, episodeStore, episodeService, time.Time{})

	report, err := backfillService.Backfill(ctx, samehadaku.EpisodeBackfillOptions{
		BatchSize:    parsePositiveInt("SAMEHADAKU_EPISODE_BATCH_SIZE", 100),
		Limit:        parsePositiveInt("SAMEHADAKU_EPISODE_LIMIT", 0),
		IncludeSlugs: parseCSVList(os.Getenv("SAMEHADAKU_EPISODE_SLUGS")),
		SkipExisting: parseBoolDefault("SAMEHADAKU_EPISODE_SKIP_EXISTING", true),
		DelayBetween: parseDurationDefault("SAMEHADAKU_EPISODE_DELAY", 250*time.Millisecond),
		Progress: func(progress samehadaku.EpisodeBackfillProgress) {
			log.Printf(
				"samehadaku episode backfill %s: slug=%s page=%d discovered=%d attempted=%d skipped=%d succeeded=%d failed=%d reason=%s",
				progress.Action,
				progress.Slug,
				progress.PageNumber,
				progress.Counts.Discovered,
				progress.Counts.Attempted,
				progress.Counts.Skipped,
				progress.Counts.Succeeded,
				progress.Counts.Failed,
				progress.Reason,
			)
		},
	})
	if err != nil {
		return err
	}

	log.Printf(
		"samehadaku episode backfill done: discovered=%d attempted=%d skipped=%d succeeded=%d failed=%d",
		report.Discovered,
		report.Attempted,
		report.Skipped,
		report.Succeeded,
		report.Failed,
	)
	return nil
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

func parseCSVList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		values = append(values, part)
	}
	return values
}

func parseDurationDefault(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
