package manhwaindo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCatalogHTML(t *testing.T) {
	t.Parallel()

	raw := mustReadFixture(t, "catalog_sample.html")
	items, err := ParseCatalogHTML(raw, "https://www.manhwaindo.my/")
	if err != nil {
		t.Fatalf("ParseCatalogHTML returned error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	first := items[0]
	if first.Title != "Solo Leveling" {
		t.Fatalf("unexpected first title: %q", first.Title)
	}
	if first.Slug != "solo-leveling" {
		t.Fatalf("unexpected first slug: %q", first.Slug)
	}
	if first.LatestChapter == nil || first.LatestChapter.Number != "179.2" {
		t.Fatalf("unexpected latest chapter: %#v", first.LatestChapter)
	}
	if first.CoverURL == "" {
		t.Fatal("expected cover url")
	}
}

func TestParseSeriesHTML(t *testing.T) {
	t.Parallel()

	raw := mustReadFixture(t, "detail_sample.html")
	series, err := ParseSeriesHTML(raw, "https://www.manhwaindo.my/series/solo-leveling/")
	if err != nil {
		t.Fatalf("ParseSeriesHTML returned error: %v", err)
	}

	if series.Title != "Solo Leveling" {
		t.Fatalf("unexpected title: %q", series.Title)
	}
	if series.AltTitle != "나 혼자만 레벨업" {
		t.Fatalf("unexpected alt title: %q", series.AltTitle)
	}
	if series.Status != "Ongoing" {
		t.Fatalf("unexpected status: %q", series.Status)
	}
	if len(series.Genres) != 3 {
		t.Fatalf("expected 3 genres, got %d", len(series.Genres))
	}
	if len(series.Chapters) != 3 {
		t.Fatalf("expected 3 chapters, got %d", len(series.Chapters))
	}
	if series.Chapters[0].Slug != "solo-leveling-chapter-179-2" {
		t.Fatalf("unexpected first chapter slug: %q", series.Chapters[0].Slug)
	}
}

func TestParseChapterHTML(t *testing.T) {
	t.Parallel()

	raw := mustReadFixture(t, "chapter_sample.html")
	chapter, err := ParseChapterHTML(raw, "https://www.manhwaindo.my/solo-leveling-chapter-100/")
	if err != nil {
		t.Fatalf("ParseChapterHTML returned error: %v", err)
	}

	if chapter.SeriesSlug != "solo-leveling" {
		t.Fatalf("unexpected series slug: %q", chapter.SeriesSlug)
	}
	if chapter.Slug != "solo-leveling-chapter-100" {
		t.Fatalf("unexpected chapter slug: %q", chapter.Slug)
	}
	if chapter.PrevURL != "https://www.manhwaindo.my/solo-leveling-chapter-99/" {
		t.Fatalf("unexpected prev url: %q", chapter.PrevURL)
	}
	if chapter.NextURL != "https://www.manhwaindo.my/solo-leveling-chapter-101/" {
		t.Fatalf("unexpected next url: %q", chapter.NextURL)
	}
	if len(chapter.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(chapter.Pages))
	}
	if chapter.Pages[0].URL != "http://kacu.gmbr.pro/uploads/manga-images/s/solo-leveling/chapter-100/1.jpg" {
		t.Fatalf("unexpected first page url: %q", chapter.Pages[0].URL)
	}
	if chapter.Pages[2].Position != 3 {
		t.Fatalf("unexpected third page position: %d", chapter.Pages[2].Position)
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
