package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
	"github.com/dwirijal/dwizzySCRAPE/internal/tmdb"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	tmdbClient := tmdb.NewClient(cfg.TMDBBaseURL, cfg.TMDBReadToken, cfg.TMDBAPIKey, httpClient)
	if !tmdbClient.Enabled() {
		return fmt.Errorf("tmdb credentials are not configured")
	}

	scope := store.TMDBEnrichmentScopeAll
	if len(args) >= 1 && strings.TrimSpace(args[0]) != "" {
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "all":
			scope = store.TMDBEnrichmentScopeAll
		case "movie":
			scope = store.TMDBEnrichmentScopeMovie
		case "series":
			scope = store.TMDBEnrichmentScopeSeries
		default:
			return fmt.Errorf("usage: tmdb-enrichment-backfill [all|movie|series] [limit] [batchSize]")
		}
	}

	limit := 0
	if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil {
			return fmt.Errorf("parse limit: %w", err)
		}
		if parsed > 0 {
			limit = parsed
		}
	}

	batchSize := 25
	if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(args[2]))
		if err != nil {
			return fmt.Errorf("parse batch size: %w", err)
		}
		if parsed > 0 {
			batchSize = parsed
		}
	}

	db, err := store.NewPgxContentDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	service := store.NewTMDBEnrichmentBackfillService(
		store.NewTMDBEnrichmentCandidateStoreWithDB(db),
		store.NewMediaItemEnrichmentStoreWithDB(db),
		tmdbClient,
	)

	report, err := service.Backfill(ctx, store.TMDBEnrichmentBackfillOptions{
		Scope:        scope,
		Limit:        limit,
		BatchSize:    batchSize,
		SkipExisting: true,
		DelayBetween: 250 * time.Millisecond,
		Progress: func(progress store.TMDBEnrichmentBackfillProgress) {
			switch progress.Action {
			case "success":
				log.Printf(
					"tmdb enrichment success: slug=%s attempted=%d succeeded=%d failed=%d",
					progress.Slug,
					progress.Counts.Attempted,
					progress.Counts.Succeeded,
					progress.Counts.Failed,
				)
			case "fail":
				log.Printf(
					"tmdb enrichment fail: slug=%s attempted=%d succeeded=%d failed=%d error=%s",
					progress.Slug,
					progress.Counts.Attempted,
					progress.Counts.Succeeded,
					progress.Counts.Failed,
					progress.Reason,
				)
			}
		},
	})
	if err != nil {
		return err
	}

	log.Printf(
		"tmdb enrichment backfill done: scope=%s discovered=%d attempted=%d skipped=%d succeeded=%d failed=%d",
		scope,
		report.Discovered,
		report.Attempted,
		report.Skipped,
		report.Succeeded,
		report.Failed,
	)
	if len(report.Failures) > 0 {
		for slug, reason := range report.Failures {
			log.Printf("tmdb enrichment backfill failed: slug=%s error=%s", slug, reason)
		}
	}
	return nil
}
