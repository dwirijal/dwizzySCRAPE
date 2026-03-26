package samehadaku

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const maxCatalogPages = 500

type CatalogStore interface {
	UpsertCatalog(ctx context.Context, items []CatalogItem) (int, error)
}

type SyncReport struct {
	Parsed   int
	Upserted int
}

type Service struct {
	fetcher Fetcher
	store   CatalogStore
	now     func() time.Time
}

func NewService(fetcher Fetcher, store CatalogStore, fixedNow time.Time) *Service {
	nowFn := time.Now
	if !fixedNow.IsZero() {
		nowFn = func() time.Time { return fixedNow }
	}
	return &Service{
		fetcher: fetcher,
		store:   store,
		now:     nowFn,
	}
}

func (s *Service) SyncCatalog(ctx context.Context, sourceURL string) (SyncReport, error) {
	if s.fetcher == nil {
		return SyncReport{}, fmt.Errorf("fetcher is required")
	}
	if s.store == nil {
		return SyncReport{}, fmt.Errorf("catalog store is required")
	}

	report := SyncReport{}
	seen := make(map[string]struct{})

	for page := 1; page <= maxCatalogPages; page++ {
		pageURL := buildCatalogPageURL(sourceURL, page)

		raw, err := s.fetcher.FetchCatalogPage(ctx, pageURL)
		if err != nil {
			return report, fmt.Errorf("fetch catalog page %d: %w", page, err)
		}

		items, err := ParseCatalogHTML(raw, pageURL, s.now().UTC())
		if err != nil {
			return report, fmt.Errorf("parse catalog page %d: %w", page, err)
		}
		if len(items) == 0 {
			return report, nil
		}

		freshItems := filterNewCatalogItems(items, seen)
		if len(freshItems) == 0 {
			return report, nil
		}
		for i := range freshItems {
			freshItems[i].PageNumber = page
		}

		upserted, err := s.store.UpsertCatalog(ctx, freshItems)
		if err != nil {
			return report, fmt.Errorf("upsert catalog page %d: %w", page, err)
		}
		report.Parsed += len(freshItems)
		report.Upserted += upserted
	}

	return report, fmt.Errorf("catalog pagination exceeded max pages (%d)", maxCatalogPages)
}

func buildCatalogPageURL(sourceURL string, page int) string {
	sourceURL = strings.TrimRight(strings.TrimSpace(sourceURL), "/")
	if page <= 1 {
		return sourceURL + "/"
	}
	return fmt.Sprintf("%s/page/%d/", sourceURL, page)
}

func filterNewCatalogItems(items []CatalogItem, seen map[string]struct{}) []CatalogItem {
	freshItems := make([]CatalogItem, 0, len(items))
	for _, item := range items {
		if item.CanonicalURL == "" {
			continue
		}
		if _, ok := seen[item.CanonicalURL]; ok {
			continue
		}
		seen[item.CanonicalURL] = struct{}{}
		freshItems = append(freshItems, item)
	}
	return freshItems
}
