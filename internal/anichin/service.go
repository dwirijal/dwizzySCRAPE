package anichin

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

func (s *Service) FetchCatalog(ctx context.Context, section string, page int) ([]CatalogItem, error) {
	target := buildCatalogPageURL(s.baseURL, section, page)
	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return nil, err
	}
	items, err := ParseCatalogHTML(raw, target, section)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].PageNumber = page
		items[i].ScrapedAt = s.now().UTC()
	}
	return items, nil
}

func (s *Service) FetchAnimeDetail(ctx context.Context, slug string) (AnimeDetail, error) {
	target := buildSeriesURL(s.baseURL, slug)
	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return AnimeDetail{}, err
	}
	detail, err := ParseSeriesHTML(raw, target)
	if err != nil {
		return AnimeDetail{}, err
	}
	detail.ScrapedAt = s.now().UTC()
	return detail, nil
}

func (s *Service) FetchEpisodeDetail(ctx context.Context, animeSlug string, ref EpisodeRef) (EpisodeDetail, error) {
	target := strings.TrimSpace(ref.CanonicalURL)
	if target == "" {
		target = buildEpisodeURL(s.baseURL, ref.Slug)
	}
	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return EpisodeDetail{}, err
	}
	detail, err := ParseEpisodeHTML(raw, target)
	if err != nil {
		return EpisodeDetail{}, err
	}
	detail.AnimeSlug = strings.TrimSpace(animeSlug)
	if detail.ReleaseLabel == "" {
		detail.ReleaseLabel = strings.TrimSpace(ref.ReleaseLabel)
	}
	if detail.EpisodeNumber == 0 {
		detail.EpisodeNumber = parseEpisodeNumberFloat(ref.Number, ref.Title, ref.Slug)
	}
	detail.ScrapedAt = s.now().UTC()
	return detail, nil
}

func buildCatalogPageURL(baseURL, section string, page int) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	section = strings.Trim(strings.TrimSpace(section), "/")
	if page <= 1 {
		return fmt.Sprintf("%s/%s/", baseURL, section)
	}
	return fmt.Sprintf("%s/%s/page/%d/", baseURL, section, page)
}

func buildSeriesURL(baseURL, slug string) string {
	slug = strings.TrimSpace(slug)
	if strings.HasPrefix(slug, "http://") || strings.HasPrefix(slug, "https://") {
		return slug
	}
	return strings.TrimRight(baseURL, "/") + "/seri/" + strings.Trim(slug, "/") + "/"
}

func buildEpisodeURL(baseURL, slug string) string {
	slug = strings.TrimSpace(slug)
	if strings.HasPrefix(slug, "http://") || strings.HasPrefix(slug, "https://") {
		return slug
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.Trim(slug, "/") + "/"
}
