package manhwaindo

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
				"https://www.manhwaindo.my/series/solo-leveling/": mustReadFixture(t, "detail_sample.html"),
			},
		},
		"https://www.manhwaindo.my",
	)

	series, err := service.FetchSeries(context.Background(), "solo-leveling")
	if err != nil {
		t.Fatalf("FetchSeries returned error: %v", err)
	}
	if series.Slug != "solo-leveling" {
		t.Fatalf("unexpected slug: %q", series.Slug)
	}
	if len(series.Chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %d", len(series.Chapters))
	}
}

func TestServiceFetchChapter(t *testing.T) {
	t.Parallel()

	service := NewService(
		stubFetcher{
			pages: map[string][]byte{
				"https://www.manhwaindo.my/solo-leveling-chapter-100/": mustReadFixture(t, "chapter_sample.html"),
			},
		},
		"https://www.manhwaindo.my",
	)

	chapter, err := service.FetchChapter(context.Background(), "solo-leveling-chapter-100")
	if err != nil {
		t.Fatalf("FetchChapter returned error: %v", err)
	}
	if chapter.Slug != "solo-leveling-chapter-100" {
		t.Fatalf("unexpected slug: %q", chapter.Slug)
	}
	if len(chapter.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(chapter.Pages))
	}
}
