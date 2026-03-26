package samehadaku

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/jikan"
)

type fakeCatalogLookup struct {
	item CatalogItem
	err  error
}

func (f *fakeCatalogLookup) GetCatalogBySlug(ctx context.Context, slug string) (CatalogItem, error) {
	if f.err != nil {
		return CatalogItem{}, f.err
	}
	return f.item, nil
}

type fakeAnimeDetailWriter struct {
	detail AnimeDetail
	called bool
	err    error
}

func (f *fakeAnimeDetailWriter) UpsertAnimeDetail(ctx context.Context, detail AnimeDetail) error {
	f.called = true
	f.detail = detail
	return f.err
}

type fakeJikanClient struct {
	searchResults []jikan.AnimeSearchHit
	searchErr     error
	full          jikan.AnimeFull
	fullErr       error
	characters    []jikan.AnimeCharacter
	charactersErr error
}

func (f *fakeJikanClient) SearchAnime(ctx context.Context, query string, limit int) ([]jikan.AnimeSearchHit, error) {
	return f.searchResults, f.searchErr
}

func (f *fakeJikanClient) GetAnimeFull(ctx context.Context, malID int) (jikan.AnimeFull, error) {
	return f.full, f.fullErr
}

func (f *fakeJikanClient) GetAnimeCharacters(ctx context.Context, malID int) ([]jikan.AnimeCharacter, error) {
	return f.characters, f.charactersErr
}

func TestDetailServiceSyncAnimeDetailContinuesWhenJikanFails(t *testing.T) {
	catalog := &fakeCatalogLookup{
		item: CatalogItem{
			Slug:            "demo-anime",
			Title:           "Demo Anime",
			CanonicalURL:    "https://v2.samehadaku.how/anime/demo-anime/",
			Source:          "samehadaku",
			SourceDomain:    "v2.samehadaku.how",
			ContentType:     "anime",
			PosterURL:       "https://v2.samehadaku.how/poster.jpg",
			SynopsisExcerpt: "Demo synopsis",
			Genres:          []string{"Action"},
			AnimeType:       "TV",
			Status:          "Ongoing",
			PageNumber:      1,
		},
	}
	writer := &fakeAnimeDetailWriter{}
	jikanClient := &fakeJikanClient{
		searchErr: errors.New("jikan request failed with status 429"),
	}
	service := NewDetailService(catalog, writer, jikanClient, nil, time.Date(2026, 3, 23, 1, 30, 0, 0, time.UTC))

	report, err := service.SyncAnimeDetail(context.Background(), "demo-anime")
	if err != nil {
		t.Fatalf("SyncAnimeDetail returned error: %v", err)
	}
	if !writer.called {
		t.Fatal("expected writer to be called")
	}
	if report.Slug != "demo-anime" {
		t.Fatalf("unexpected report slug %q", report.Slug)
	}
	if writer.detail.MALID != 0 {
		t.Fatalf("expected MALID to remain empty, got %d", writer.detail.MALID)
	}
	if string(writer.detail.JikanMetaJSON) == "{}" {
		t.Fatalf("expected jikan_meta_json to contain error context, got %s", string(writer.detail.JikanMetaJSON))
	}
	if !strings.Contains(string(writer.detail.JikanMetaJSON), "429") {
		t.Fatalf("expected jikan_meta_json to contain rate limit error, got %s", string(writer.detail.JikanMetaJSON))
	}
}
