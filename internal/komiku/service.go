package komiku

import (
	"context"
	"fmt"
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

func (s *Service) FetchCatalog(ctx context.Context, page int) ([]content.ManhwaSeries, error) {
	target := s.baseURL + "/daftar-komik/"
	if page > 1 {
		target = fmt.Sprintf("%s/daftar-komik/?halaman=%d", s.baseURL, page)
	}
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

func normalizeSeriesURL(baseURL, slug string) string {
	slug = strings.TrimSpace(slug)
	if strings.HasPrefix(slug, "http://") || strings.HasPrefix(slug, "https://") {
		return slug
	}
	return strings.TrimRight(baseURL, "/") + "/manga/" + strings.Trim(slug, "/") + "/"
}

func normalizeChapterURL(baseURL, slug string) string {
	slug = strings.TrimSpace(slug)
	if strings.HasPrefix(slug, "http://") || strings.HasPrefix(slug, "https://") {
		return slug
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.Trim(slug, "/") + "/"
}
