package samehadaku

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseCatalogHTML(t *testing.T) {
	fixturePath := filepath.Join("testdata", "catalog_sample.html")
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	scrapedAt := time.Date(2026, 3, 22, 15, 0, 0, 0, time.UTC)
	items, err := ParseCatalogHTML(raw, "https://v2.samehadaku.how/daftar-anime-2/", scrapedAt)
	if err != nil {
		t.Fatalf("ParseCatalogHTML returned error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	first := items[0]
	if first.Title != "#Compass2.0 Animation Project" {
		t.Fatalf("unexpected first title: %q", first.Title)
	}
	if first.CanonicalURL != "https://v2.samehadaku.how/anime/compass2-0-animation-project/" {
		t.Fatalf("unexpected first canonical url: %q", first.CanonicalURL)
	}
	if first.Slug != "compass2-0-animation-project" {
		t.Fatalf("unexpected first slug: %q", first.Slug)
	}
	if first.PosterURL != "https://cdn.samehadaku.example/posters/compass2.webp" {
		t.Fatalf("unexpected first poster url: %q", first.PosterURL)
	}
	if first.AnimeType != "TV" {
		t.Fatalf("unexpected first anime type: %q", first.AnimeType)
	}
	if first.Status != "Completed" {
		t.Fatalf("unexpected first status: %q", first.Status)
	}
	if first.Score != 6.01 {
		t.Fatalf("unexpected first score: %v", first.Score)
	}
	if first.Views != 884361 {
		t.Fatalf("unexpected first views: %d", first.Views)
	}
	if len(first.Genres) != 2 || first.Genres[0] != "Action" || first.Genres[1] != "Strategy Game" {
		t.Fatalf("unexpected first genres: %#v", first.Genres)
	}
	if first.Source != "samehadaku" {
		t.Fatalf("unexpected first source: %q", first.Source)
	}
	if first.SourceDomain != "v2.samehadaku.how" {
		t.Fatalf("unexpected first source domain: %q", first.SourceDomain)
	}
	if first.ContentType != "anime" {
		t.Fatalf("unexpected first content type: %q", first.ContentType)
	}
	if first.ScrapedAt != scrapedAt {
		t.Fatalf("unexpected first scrapedAt: %v", first.ScrapedAt)
	}
	if first.SynopsisExcerpt == "" {
		t.Fatal("expected synopsis excerpt to be populated")
	}
}
