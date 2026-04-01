package main

import (
	"context"
	"flag"
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
	limit := flag.Int("limit", 25, "maximum number of existing hanime items to inspect")
	missingOnly := flag.Bool("missing-only", false, "only inspect rows missing detail enrichment fields")
	pause := flag.Duration("pause", 1500*time.Millisecond, "sleep between detail page requests")
	flag.Parse()

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

	items, err := sink.ListCatalogItemsForDetailBackfill(ctx, *limit, *missingOnly)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		log.Printf("hanime detail backfill skipped: no existing items")
		return nil
	}

	enriched := make([]hanime.CatalogItem, 0, len(items))
	failed := 0
	skipped := 0
	for i, item := range items {
		before := item
		next, err := service.EnrichItem(ctx, item)
		if err != nil {
			failed++
			log.Printf("hanime detail backfill: %d/%d slug=%s failed: %v", i+1, len(items), item.Slug, err)
		} else if !hasMeaningfulChange(before, next) {
			skipped++
			log.Printf("hanime detail backfill: %d/%d slug=%s unchanged", i+1, len(items), item.Slug)
		} else {
			enriched = append(enriched, next)
			log.Printf(
				"hanime detail backfill: %d/%d slug=%s brand=%s tags=%d download=%t manifest=%t",
				i+1, len(items), next.Slug, next.Brand, len(next.Tags), next.DownloadPresent, next.ManifestPresent,
			)
		}
		if *pause > 0 && i < len(items)-1 {
			time.Sleep(*pause)
		}
	}
	if len(enriched) == 0 {
		log.Printf("hanime detail backfill done: inspected=%d enriched=0 skipped=%d failed=%d", len(items), skipped, failed)
		return nil
	}

	upserted, err := sink.UpsertCatalogItems(ctx, enriched)
	if err != nil {
		return err
	}
	log.Printf("hanime detail backfill done: inspected=%d enriched=%d skipped=%d failed=%d upserted=%d", len(items), len(enriched), skipped, failed, upserted)
	return nil
}

func detailCompleteness(item hanime.CatalogItem) int {
	score := 0
	if strings.TrimSpace(item.CoverURL) != "" {
		score++
	}
	if strings.TrimSpace(item.DescriptionExcerpt) != "" {
		score++
	}
	if strings.TrimSpace(item.Brand) != "" {
		score++
	}
	if len(item.Tags) > 0 {
		score++
	}
	if item.DownloadPresent {
		score++
	}
	if item.ManifestPresent {
		score++
	}
	return score
}

func hasMeaningfulChange(before, after hanime.CatalogItem) bool {
	if before.NormalizedTitle != after.NormalizedTitle {
		return true
	}
	if before.EntryKind != after.EntryKind || before.EpisodeNumber != after.EpisodeNumber || before.SeriesCandidate != after.SeriesCandidate {
		return true
	}
	if before.Title != after.Title {
		return true
	}
	if before.CoverURL != after.CoverURL {
		return true
	}
	if before.Description != after.Description {
		return true
	}
	if before.DescriptionExcerpt != after.DescriptionExcerpt {
		return true
	}
	if before.Brand != after.Brand || before.BrandSlug != after.BrandSlug {
		return true
	}
	if !equalStringSlices(before.Tags, after.Tags) {
		return true
	}
	if !equalStringSlices(before.AlternateTitles, after.AlternateTitles) {
		return true
	}
	if before.DownloadPresent != after.DownloadPresent || before.ManifestPresent != after.ManifestPresent {
		return true
	}
	if !before.ReleasedAt.Equal(after.ReleasedAt) {
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
