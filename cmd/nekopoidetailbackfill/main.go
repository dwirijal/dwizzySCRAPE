package main

import (
	"context"
	"flag"
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
	limit := flag.Int("limit", 25, "maximum number of existing nekopoi items to inspect")
	missingOnly := flag.Bool("missing-only", true, "only inspect rows missing detail enrichment fields")
	pause := flag.Duration("pause", 1500*time.Millisecond, "sleep between detail page requests")
	skipFetch := flag.Bool("skip-fetch", false, "only backfill derived title metadata without fetching detail pages")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required for nekopoi detail backfill")
	}

	db, err := store.NewPgxContentDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	fetcher := nekopoi.NewClient(cfg.NekopoiUserAgent, cfg.NekopoiCookie, cfg.HTTPTimeout)
	service := nekopoi.NewService(fetcher, cfg.NekopoiBaseURL, time.Time{})
	sink := store.NewNekopoiStoreWithDB(db)

	items, err := sink.ListFeedItemsForDetailBackfill(ctx, *limit, *missingOnly)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		log.Printf("nekopoi detail backfill skipped: no existing items")
		return nil
	}

	enriched := make([]nekopoi.FeedItem, 0, len(items))
	failed := 0
	skipped := 0
	for i, item := range items {
		before := item
		next := nekopoi.HydrateDerivedMetadata(item)
		if !*skipFetch {
			var err error
			next, err = service.EnrichItem(ctx, next)
			if err != nil {
				if !hasMeaningfulChange(before, next) {
					failed++
					log.Printf("nekopoi detail backfill: %d/%d slug=%s failed: %v", i+1, len(items), item.Slug, err)
				} else {
					enriched = append(enriched, next)
					log.Printf("nekopoi detail backfill: %d/%d slug=%s derived-only update (detail fetch failed: %v)", i+1, len(items), next.Slug, err)
				}
			} else if !hasMeaningfulChange(before, next) {
				skipped++
				log.Printf("nekopoi detail backfill: %d/%d slug=%s unchanged", i+1, len(items), item.Slug)
			} else {
				enriched = append(enriched, next)
				log.Printf(
					"nekopoi detail backfill: %d/%d slug=%s post_id=%s players=%d downloads=%d",
					i+1, len(items), next.Slug, next.PostID, next.PlayerCount, next.DownloadCount,
				)
			}
		} else if !hasMeaningfulChange(before, next) {
			skipped++
			log.Printf("nekopoi detail backfill: %d/%d slug=%s unchanged", i+1, len(items), item.Slug)
		} else {
			enriched = append(enriched, next)
			log.Printf("nekopoi detail backfill: %d/%d slug=%s derived-only update (skip-fetch)", i+1, len(items), next.Slug)
		}
		if !*skipFetch && *pause > 0 && i < len(items)-1 {
			time.Sleep(*pause)
		}
	}
	if len(enriched) == 0 {
		log.Printf("nekopoi detail backfill done: inspected=%d enriched=0 skipped=%d failed=%d", len(items), skipped, failed)
		return nil
	}

	upserted, err := sink.UpsertFeedItems(ctx, enriched)
	if err != nil {
		return err
	}
	log.Printf("nekopoi detail backfill done: inspected=%d enriched=%d skipped=%d failed=%d upserted=%d", len(items), len(enriched), skipped, failed, upserted)
	return nil
}

func detailCompleteness(item nekopoi.FeedItem) int {
	score := 0
	if strings.TrimSpace(item.PostID) != "" {
		score++
	}
	if len(item.PlayerHosts) > 0 {
		score++
	}
	if len(item.DownloadHosts) > 0 {
		score++
	}
	return score
}

func hasMeaningfulChange(before, after nekopoi.FeedItem) bool {
	if before.NormalizedTitle != after.NormalizedTitle {
		return true
	}
	if before.EntryKind != after.EntryKind || before.EpisodeNumber != after.EpisodeNumber || before.PartNumber != after.PartNumber {
		return true
	}
	if before.SeriesCandidate != after.SeriesCandidate {
		return true
	}
	if !equalStringSlices(before.TitleLabels, after.TitleLabels) {
		return true
	}
	if strings.TrimSpace(before.CoverURL) == "" && strings.TrimSpace(after.CoverURL) != "" {
		return true
	}
	if strings.TrimSpace(before.DescriptionExcerpt) == "" && strings.TrimSpace(after.DescriptionExcerpt) != "" {
		return true
	}
	return detailCompleteness(after) > detailCompleteness(before)
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
