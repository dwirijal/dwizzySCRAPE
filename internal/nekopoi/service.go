package nekopoi

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	fetcher Fetcher
	baseURL string
	now     func() time.Time
}

func NewService(fetcher Fetcher, baseURL string, fixedNow time.Time) *Service {
	nowFn := time.Now
	if !fixedNow.IsZero() {
		nowFn = func() time.Time { return fixedNow }
	}
	return &Service{
		fetcher: fetcher,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		now:     nowFn,
	}
}

func (s *Service) FetchFeed(ctx context.Context) ([]FeedItem, error) {
	target := s.baseURL + "/feed/"
	raw, err := s.fetcher.FetchFeed(ctx, target)
	if err != nil {
		return nil, err
	}
	items, err := ParseFeedXML(raw, target)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	for i := range items {
		items[i].ScrapedAt = now
	}
	return items, nil
}

func (s *Service) SyncFeed(ctx context.Context, sink interface {
	UpsertFeedItems(context.Context, []FeedItem) (int, error)
}) (int, error) {
	if sink == nil {
		return 0, fmt.Errorf("feed sink is required")
	}
	items, err := s.FetchFeed(ctx)
	if err != nil {
		return 0, err
	}
	items = s.EnrichFeedItems(ctx, items)
	return sink.UpsertFeedItems(ctx, items)
}

func (s *Service) EnrichFeedItems(ctx context.Context, items []FeedItem) []FeedItem {
	for i := range items {
		enriched, err := s.EnrichItem(ctx, items[i])
		if err != nil {
			continue
		}
		items[i] = enriched
	}
	return items
}

func (s *Service) EnrichItem(ctx context.Context, item FeedItem) (FeedItem, error) {
	canonicalURL := strings.TrimSpace(item.CanonicalURL)
	if canonicalURL == "" {
		return item, fmt.Errorf("canonical url is required")
	}
	raw, err := s.fetcher.FetchPage(ctx, canonicalURL)
	if err != nil {
		return item, err
	}
	meta, err := ParseDetailHTML(raw)
	if err != nil {
		return item, err
	}
	if strings.TrimSpace(item.CoverURL) == "" {
		item.CoverURL = strings.TrimSpace(meta.CoverURL)
	}
	if strings.TrimSpace(item.DescriptionExcerpt) == "" {
		item.DescriptionExcerpt = strings.TrimSpace(meta.Excerpt)
	}
	item.PostID = strings.TrimSpace(meta.PostID)
	item.PlayerCount = meta.PlayerCount
	item.PlayerHosts = meta.PlayerHosts
	item.DownloadCount = meta.DownloadCount
	item.DownloadLabels = meta.DownloadLabels
	item.DownloadHosts = meta.DownloadHosts
	return item, nil
}
