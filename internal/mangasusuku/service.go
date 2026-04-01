package mangasusuku

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

type Service struct {
	fetcher Fetcher
	baseURL string
}

func NewService(fetcher Fetcher, baseURL string) *Service {
	return &Service{
		fetcher: fetcher,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
	}
}

func (s *Service) FetchCatalog(ctx context.Context, letter string, page int) ([]content.ManhwaSeries, error) {
	target := normalizeCatalogURL(s.baseURL, letter, page)
	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return nil, err
	}
	return ParseCatalogHTML(raw, target)
}

func (s *Service) FetchSeries(ctx context.Context, slug string) (content.ManhwaSeries, error) {
	target := normalizeSeriesURL(s.baseURL, slug)
	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return content.ManhwaSeries{}, err
	}
	return ParseSeriesHTML(raw, target)
}

func (s *Service) FetchChapter(ctx context.Context, slug string) (content.ManhwaChapter, error) {
	target := normalizeChapterURL(s.baseURL, slug)
	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return content.ManhwaChapter{}, err
	}
	return ParseChapterHTML(raw, target)
}

func normalizeCatalogURL(baseURL, letter string, page int) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	letter = strings.TrimSpace(letter)
	if page <= 1 {
		if letter == "" {
			return baseURL + "/az-list/"
		}
		return fmt.Sprintf("%s/az-list/?show=%s", baseURL, url.QueryEscape(letter))
	}
	if letter == "" {
		return fmt.Sprintf("%s/az-list/page/%d/", baseURL, page)
	}
	return fmt.Sprintf("%s/az-list/page/%d/?show=%s", baseURL, page, url.QueryEscape(letter))
}

func normalizeSeriesURL(baseURL, slug string) string {
	slug = strings.TrimSpace(slug)
	if strings.HasPrefix(slug, "http://") || strings.HasPrefix(slug, "https://") {
		return slug
	}
	return strings.TrimRight(baseURL, "/") + "/komik/" + strings.Trim(slug, "/") + "/"
}

func normalizeChapterURL(baseURL, slug string) string {
	slug = strings.TrimSpace(slug)
	if strings.HasPrefix(slug, "http://") || strings.HasPrefix(slug, "https://") {
		return slug
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.Trim(slug, "/") + "/"
}
