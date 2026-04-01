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

	"github.com/dwirijal/dwizzySCRAPE/internal/anilist"
	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
)

type backfillArgs struct {
	scope     store.AniListEnrichmentScope
	limit     int
	batchSize int
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	parsed, err := parseArgs(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	aniListClient := anilist.NewClient(cfg.AniListBaseURL, httpClient)

	db, err := store.NewPgxContentDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	service := store.NewAniListEnrichmentBackfillService(
		store.NewAniListEnrichmentCandidateStoreWithDB(db),
		store.NewMediaItemEnrichmentStoreWithDB(db),
		store.NewAniListPromotionStoreWithDB(db),
		aniListClient,
	)

	report, err := service.Backfill(ctx, store.AniListEnrichmentBackfillOptions{
		Scope:        parsed.scope,
		Limit:        parsed.limit,
		BatchSize:    parsed.batchSize,
		SkipExisting: true,
		DelayBetween: 500 * time.Millisecond,
	})
	if err != nil {
		return err
	}

	log.Printf(
		"anilist enrichment backfill done: scope=%s discovered=%d attempted=%d skipped=%d succeeded=%d failed=%d",
		parsed.scope,
		report.Discovered,
		report.Attempted,
		report.Skipped,
		report.Succeeded,
		report.Failed,
	)
	return nil
}

func parseArgs(args []string) (backfillArgs, error) {
	out := backfillArgs{
		scope:     store.AniListEnrichmentScopeAll,
		limit:     0,
		batchSize: 25,
	}

	if len(args) >= 1 && strings.TrimSpace(args[0]) != "" {
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "all":
			out.scope = store.AniListEnrichmentScopeAll
		case "video":
			out.scope = store.AniListEnrichmentScopeVideo
		case "comic":
			out.scope = store.AniListEnrichmentScopeComic
		default:
			return backfillArgs{}, fmt.Errorf("usage: anilist-enrichment-backfill [all|video|comic] [limit] [batchSize]")
		}
	}

	if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil {
			return backfillArgs{}, fmt.Errorf("parse limit: %w", err)
		}
		if parsed > 0 {
			out.limit = parsed
		}
	}

	if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(args[2]))
		if err != nil {
			return backfillArgs{}, fmt.Errorf("parse batch size: %w", err)
		}
		if parsed > 0 {
			out.batchSize = parsed
		}
	}

	return out, nil
}
