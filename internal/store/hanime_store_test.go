package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/hanime"
)

func TestHanimeStoreUpsertCatalogItems(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
	store := NewHanimeStoreWithDB(db)

	_, err := store.UpsertCatalogItems(context.Background(), []hanime.CatalogItem{
		{
			SourceDomain:    "hanime.tv",
			CatalogSource:   "/browse/trending",
			Title:           "My Mother 1",
			NormalizedTitle: "My Mother",
			EntryKind:       "episode",
			EpisodeNumber:   1,
			SeriesCandidate: true,
			CanonicalURL:    "https://hanime.tv/videos/hentai/my-mother-1",
			Slug:            "my-mother-1",
			CoverURL:        "https://hanime-cdn.com/images/covers/my-mother-1.webp",
			Tags:            []string{"incest", "censored"},
			Brand:           "nur",
			BrandSlug:       "nur",
			ReleasedAt:      time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
			ScrapedAt:       time.Date(2026, time.March, 31, 1, 2, 3, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("UpsertCatalogItems returned error: %v", err)
	}

	got := strings.Join(db.execQueries, "\n")
	if !strings.Contains(got, "select public.upsert_media_item") {
		t.Fatalf("expected media item upsert query, got:\n%s", got)
	}
	if !strings.Contains(got, "update public.media_items") {
		t.Fatalf("expected taxonomy update query, got:\n%s", got)
	}
	if len(db.execArgs) == 0 || db.execArgs[0][2] != "anime" {
		t.Fatalf("expected anime media type, got %#v", db.execArgs)
	}
}

func TestHanimeStoreKeepsStandaloneAsMovie(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
	store := NewHanimeStoreWithDB(db)

	_, err := store.UpsertCatalogItems(context.Background(), []hanime.CatalogItem{
		{
			SourceDomain:    "hanime.tv",
			Title:           "Standalone OVA",
			NormalizedTitle: "Standalone OVA",
			CanonicalURL:    "https://hanime.tv/videos/hentai/standalone-ova",
			Slug:            "standalone-ova",
		},
	})
	if err != nil {
		t.Fatalf("UpsertCatalogItems returned error: %v", err)
	}
	if len(db.execArgs) == 0 || db.execArgs[0][2] != "movie" {
		t.Fatalf("expected movie media type, got %#v", db.execArgs)
	}
}
