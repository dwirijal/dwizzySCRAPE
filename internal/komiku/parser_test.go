package komiku

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCatalogHTML(t *testing.T) {
	t.Parallel()

	raw := mustReadFixture(t, "catalog_sample.html")
	items, err := ParseCatalogHTML(raw, "https://komiku.org/daftar-komik/")
	if err != nil {
		t.Fatalf("ParseCatalogHTML returned error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Source != "komiku" {
		t.Fatalf("unexpected source: %q", items[0].Source)
	}
	if items[0].Slug != "standard-of-reincarnation-id" {
		t.Fatalf("unexpected slug: %q", items[0].Slug)
	}
	if items[0].MediaType != "manhwa" {
		t.Fatalf("unexpected media type: %q", items[0].MediaType)
	}
	if items[0].Status != "Ongoing" {
		t.Fatalf("unexpected status: %q", items[0].Status)
	}
}

func TestParseSeriesHTML(t *testing.T) {
	t.Parallel()

	raw := mustReadFixture(t, "series_sample.html")
	series, err := ParseSeriesHTML(raw, "https://komiku.org/manga/standard-of-reincarnation-id/")
	if err != nil {
		t.Fatalf("ParseSeriesHTML returned error: %v", err)
	}

	if series.Source != "komiku" {
		t.Fatalf("unexpected source: %q", series.Source)
	}
	if series.Slug != "standard-of-reincarnation-id" {
		t.Fatalf("unexpected slug: %q", series.Slug)
	}
	if series.Title != "Standard of Reincarnation" {
		t.Fatalf("unexpected title: %q", series.Title)
	}
	if series.MediaType != "manhwa" {
		t.Fatalf("unexpected media type: %q", series.MediaType)
	}
	if len(series.Genres) != 2 {
		t.Fatalf("expected 2 genres, got %d", len(series.Genres))
	}
	if len(series.Chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(series.Chapters))
	}
	if series.LatestChapter == nil || series.LatestChapter.Slug != "standard-of-reincarnation-id-chapter-173" {
		t.Fatalf("unexpected latest chapter: %#v", series.LatestChapter)
	}
}

func TestParseChapterHTML(t *testing.T) {
	t.Parallel()

	raw := mustReadFixture(t, "chapter_sample.html")
	chapter, err := ParseChapterHTML(raw, "https://komiku.org/standard-of-reincarnation-id-chapter-173/")
	if err != nil {
		t.Fatalf("ParseChapterHTML returned error: %v", err)
	}

	if chapter.Source != "komiku" {
		t.Fatalf("unexpected source: %q", chapter.Source)
	}
	if chapter.SeriesSlug != "standard-of-reincarnation-id" {
		t.Fatalf("unexpected series slug: %q", chapter.SeriesSlug)
	}
	if chapter.Slug != "standard-of-reincarnation-id-chapter-173" {
		t.Fatalf("unexpected chapter slug: %q", chapter.Slug)
	}
	if chapter.NextURL != "https://komiku.org/standard-of-reincarnation-id-chapter-174/" {
		t.Fatalf("unexpected next url: %q", chapter.NextURL)
	}
	if chapter.PrevURL != "https://komiku.org/standard-of-reincarnation-id-chapter-172/" {
		t.Fatalf("unexpected prev url: %q", chapter.PrevURL)
	}
	if len(chapter.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(chapter.Pages))
	}
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}
