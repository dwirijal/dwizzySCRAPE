package content

import "testing"

func TestChapterPageAssetsPreserveOrder(t *testing.T) {
	t.Parallel()

	chapter := ManhwaChapter{
		Slug: "solo-leveling-chapter-100",
		Pages: []PageAsset{
			{Position: 1, URL: "https://img.example/1.jpg"},
			{Position: 2, URL: "https://img.example/2.jpg"},
			{Position: 3, URL: "https://img.example/3.jpg"},
		},
	}

	if len(chapter.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(chapter.Pages))
	}
	if chapter.Pages[0].URL != "https://img.example/1.jpg" {
		t.Fatalf("unexpected first page url: %q", chapter.Pages[0].URL)
	}
	if chapter.Pages[2].Position != 3 {
		t.Fatalf("unexpected third page position: %d", chapter.Pages[2].Position)
	}
}

func TestSeriesCarriesCanonicalMetadata(t *testing.T) {
	t.Parallel()

	series := ManhwaSeries{
		Source:       "manhwaindo",
		MediaType:    "manhwa",
		Slug:         "solo-leveling",
		Title:        "Solo Leveling",
		CanonicalURL: "https://www.manhwaindo.my/series/solo-leveling/",
		Genres:       []string{"Action", "Adventure", "Fantasy"},
		LatestChapter: &ManhwaChapterRef{
			Slug:   "solo-leveling-chapter-179-2",
			Label:  "Chapter 179.2",
			Number: "179.2",
		},
	}

	if series.Source != "manhwaindo" {
		t.Fatalf("unexpected source: %q", series.Source)
	}
	if series.MediaType != "manhwa" {
		t.Fatalf("unexpected media type: %q", series.MediaType)
	}
	if series.LatestChapter == nil {
		t.Fatal("expected latest chapter metadata")
	}
	if series.LatestChapter.Slug != "solo-leveling-chapter-179-2" {
		t.Fatalf("unexpected latest chapter slug: %q", series.LatestChapter.Slug)
	}
}
