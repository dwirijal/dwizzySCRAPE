package komiku

import (
	"context"
	"testing"
)

type stubFetcher struct {
	pages map[string][]byte
}

func (s stubFetcher) FetchPage(_ context.Context, rawURL string) ([]byte, error) {
	return s.pages[rawURL], nil
}

func TestServiceFetchSeries(t *testing.T) {
	t.Parallel()

	service := NewService(
		stubFetcher{
			pages: map[string][]byte{
				"https://komiku.org/manga/standard-of-reincarnation-id/": mustReadFixture(t, "series_sample.html"),
			},
		},
		"https://komiku.org",
	)

	series, err := service.FetchSeries(context.Background(), "standard-of-reincarnation-id")
	if err != nil {
		t.Fatalf("FetchSeries returned error: %v", err)
	}
	if series.Slug != "standard-of-reincarnation-id" {
		t.Fatalf("unexpected slug: %q", series.Slug)
	}
	if len(series.Chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(series.Chapters))
	}
}

func TestServiceFetchChapter(t *testing.T) {
	t.Parallel()

	service := NewService(
		stubFetcher{
			pages: map[string][]byte{
				"https://komiku.org/standard-of-reincarnation-id-chapter-173/": mustReadFixture(t, "chapter_sample.html"),
			},
		},
		"https://komiku.org",
	)

	chapter, err := service.FetchChapter(context.Background(), "standard-of-reincarnation-id-chapter-173")
	if err != nil {
		t.Fatalf("FetchChapter returned error: %v", err)
	}
	if chapter.Slug != "standard-of-reincarnation-id-chapter-173" {
		t.Fatalf("unexpected slug: %q", chapter.Slug)
	}
	if len(chapter.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(chapter.Pages))
	}
}
