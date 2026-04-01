package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/nekopoi"
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
		return fmt.Errorf("DATABASE_URL is required for nekopoi sync")
	}

	db, err := store.NewPgxContentDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	fetcher := nekopoi.NewClient(cfg.NekopoiUserAgent, cfg.NekopoiCookie, cfg.HTTPTimeout)
	service := nekopoi.NewService(fetcher, cfg.NekopoiBaseURL, time.Time{})
	sink := store.NewNekopoiStoreWithDB(db)

	upserted, err := service.SyncFeed(ctx, sink)
	if err != nil {
		return err
	}
	log.Printf("nekopoi feed sync done: upserted=%d", upserted)
	return nil
}
