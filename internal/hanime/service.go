package hanime

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	fetcher     Fetcher
	baseURL     string
	defaultPath []string
	now         func() time.Time
}

type SyncReport struct {
	Upserted    int
	FailedPaths []string
}

func NewService(fetcher Fetcher, baseURL string, defaultPaths []string, fixedNow time.Time) *Service {
	nowFn := time.Now
	if !fixedNow.IsZero() {
		nowFn = func() time.Time { return fixedNow }
	}
	return &Service{
		fetcher:     fetcher,
		baseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		defaultPath: append([]string(nil), defaultPaths...),
		now:         nowFn,
	}
}

func (s *Service) FetchCatalog(ctx context.Context, paths []string) ([]CatalogItem, []string, error) {
	targets := s.catalogPaths(paths)
	seen := make(map[string]struct{})
	items := make([]CatalogItem, 0, 48)
	failures := make([]string, 0)
	now := s.now().UTC()
	for _, path := range targets {
		targetURL := s.absoluteURL(path)
		raw, err := s.fetcher.FetchPage(ctx, targetURL)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		pageItems, err := ParseCatalogHTML(raw, targetURL)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if len(pageItems) == 0 {
			failures = append(failures, fmt.Sprintf("%s: no video cards found", path))
			continue
		}
		for _, item := range pageItems {
			if _, ok := seen[item.Slug]; ok {
				continue
			}
			seen[item.Slug] = struct{}{}
			item.ScrapedAt = now
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		if len(failures) == 0 {
			return nil, nil, fmt.Errorf("hanime catalog returned no items")
		}
		return nil, failures, fmt.Errorf("hanime catalog returned no items (%s)", strings.Join(failures, "; "))
	}
	return items, failures, nil
}

func (s *Service) SyncCatalog(ctx context.Context, sink interface {
	UpsertCatalogItems(context.Context, []CatalogItem) (int, error)
}, paths []string) (SyncReport, error) {
	if sink == nil {
		return SyncReport{}, fmt.Errorf("catalog sink is required")
	}
	items, failures, err := s.FetchCatalog(ctx, paths)
	if err != nil {
		return SyncReport{}, err
	}
	upserted, err := sink.UpsertCatalogItems(ctx, items)
	if err != nil {
		return SyncReport{}, err
	}
	return SyncReport{
		Upserted:    upserted,
		FailedPaths: failures,
	}, nil
}

func (s *Service) EnrichItem(ctx context.Context, item CatalogItem) (CatalogItem, error) {
	target := strings.TrimSpace(item.CanonicalURL)
	if target == "" && strings.TrimSpace(item.Slug) != "" {
		target = s.absoluteURL("/videos/hentai/" + item.Slug)
	}
	if target == "" {
		return item, fmt.Errorf("canonical url is required")
	}

	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return item, err
	}
	meta, err := ParseDetailHTML(raw, target)
	if err != nil {
		return item, err
	}

	if strings.TrimSpace(meta.Title) != "" {
		item.Title = meta.Title
	}
	if strings.TrimSpace(meta.CoverURL) != "" {
		item.CoverURL = strings.TrimSpace(meta.CoverURL)
	}
	if strings.TrimSpace(meta.Description) != "" {
		item.Description = strings.TrimSpace(meta.Description)
	}
	if strings.TrimSpace(meta.DescriptionExcerpt) != "" {
		item.DescriptionExcerpt = strings.TrimSpace(meta.DescriptionExcerpt)
	}
	if len(meta.Tags) > 0 {
		item.Tags = append([]string(nil), meta.Tags...)
	}
	if strings.TrimSpace(meta.Brand) != "" {
		item.Brand = strings.TrimSpace(meta.Brand)
	}
	if strings.TrimSpace(meta.BrandSlug) != "" {
		item.BrandSlug = strings.TrimSpace(meta.BrandSlug)
	}
	if len(meta.AlternateTitles) > 0 {
		item.AlternateTitles = append([]string(nil), meta.AlternateTitles...)
	}
	item.DownloadPresent = meta.DownloadPresent
	item.ManifestPresent = meta.ManifestPresent
	if !meta.ReleasedAt.IsZero() {
		item.ReleasedAt = meta.ReleasedAt
	}
	previousEpisodeNumber := item.EpisodeNumber
	previousSeriesCandidate := item.SeriesCandidate
	previousNormalizedTitle := item.NormalizedTitle
	item = HydrateDerivedMetadata(item)
	if previousSeriesCandidate && item.EpisodeNumber == previousEpisodeNumber && item.NormalizedTitle == previousNormalizedTitle {
		item.SeriesCandidate = true
		if item.EntryKind == "numbered" {
			item.EntryKind = "episode"
		}
	}
	if item.EpisodeNumber > 1 {
		item.SeriesCandidate = true
		if item.EntryKind == "numbered" {
			item.EntryKind = "episode"
		}
	}
	return item, nil
}

func (s *Service) catalogPaths(paths []string) []string {
	if len(paths) > 0 {
		return normalizePaths(paths)
	}
	if len(s.defaultPath) > 0 {
		return normalizePaths(s.defaultPath)
	}
	return []string{"/home", "/browse/trending"}
}

func normalizePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{})
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "/") {
			trimmed = "/" + trimmed
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func (s *Service) absoluteURL(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if path == "" {
		return s.baseURL
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return s.baseURL + path
}
