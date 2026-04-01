package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/hanime"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
)

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
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return fmt.Errorf("database DSN is required (DATABASE_URL, POSTGRES_URL, or NEON_DATABASE_URL)")
	}

	db, err := store.NewPgxContentDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	fetcher := hanime.NewClient(cfg.HanimeUserAgent, cfg.HanimeCookie, cfg.HTTPTimeout)
	service := hanime.NewService(fetcher, cfg.HanimeBaseURL, cfg.HanimeBrowsePaths, time.Time{})
	sink := store.NewHanimeStoreWithDB(db)

	report, err := service.SyncCatalog(ctx, sink, cfg.HanimeBrowsePaths)
	if err != nil {
		return err
	}
	if len(report.FailedPaths) > 0 {
		log.Printf(
			"hanime catalog sync done with partial path failures: upserted=%d paths=%s failed=%s",
			report.Upserted,
			strings.Join(cfg.HanimeBrowsePaths, ","),
			strings.Join(report.FailedPaths, " | "),
		)
		return nil
	}
	log.Printf("hanime catalog sync done: upserted=%d paths=%s", report.Upserted, strings.Join(cfg.HanimeBrowsePaths, ","))
	return nil
}
