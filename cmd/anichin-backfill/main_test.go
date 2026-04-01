package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/anichin"
)

type fakeFetcher struct {
	catalog  map[string]map[int][]anichin.CatalogItem
	details  map[string]anichin.AnimeDetail
	episodes map[string]anichin.EpisodeDetail
	errors   map[string]error
}

func (f fakeFetcher) FetchCatalog(ctx context.Context, section string, page int) ([]anichin.CatalogItem, error) {
	if err := f.errors[fmt.Sprintf("catalog:%s:%d", section, page)]; err != nil {
		return nil, err
	}
	if pages, ok := f.catalog[section]; ok {
		return pages[page], nil
	}
	return nil, nil
}

func (f fakeFetcher) FetchAnimeDetail(ctx context.Context, slug string) (anichin.AnimeDetail, error) {
	if err := f.errors["detail:"+slug]; err != nil {
		return anichin.AnimeDetail{}, err
	}
	if detail, ok := f.details[slug]; ok {
		return detail, nil
	}
	return anichin.AnimeDetail{}, fmt.Errorf("missing detail: %s", slug)
}

func (f fakeFetcher) FetchEpisodeDetail(ctx context.Context, animeSlug string, ref anichin.EpisodeRef) (anichin.EpisodeDetail, error) {
	if err := f.errors["episode:"+ref.Slug]; err != nil {
		return anichin.EpisodeDetail{}, err
	}
	if detail, ok := f.episodes[ref.Slug]; ok {
		return detail, nil
	}
	return anichin.EpisodeDetail{}, fmt.Errorf("missing episode: %s", ref.Slug)
}

type fakeWriter struct {
	series     []string
	episodes   []string
	hasSeries  map[string]bool
	hasEpisode map[string]bool
}

func (w *fakeWriter) UpsertCatalogItems(ctx context.Context, items []anichin.CatalogItem) (int, error) {
	return len(items), nil
}

func (w *fakeWriter) UpsertAnimeDetail(ctx context.Context, detail anichin.AnimeDetail) error {
	w.series = append(w.series, detail.Slug)
	return nil
}

func (w *fakeWriter) UpsertEpisodeDetail(ctx context.Context, detail anichin.EpisodeDetail) error {
	w.episodes = append(w.episodes, detail.EpisodeSlug)
	return nil
}

func (w *fakeWriter) HasAnimeDetail(ctx context.Context, slug string) (bool, error) {
	return w.hasSeries[slug], nil
}

func (w *fakeWriter) HasEpisodeDetail(ctx context.Context, slug string) (bool, error) {
	return w.hasEpisode[slug], nil
}

func TestRunFullBackfillStopsOnEmptyCatalogPage(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		catalog: map[string]map[int][]anichin.CatalogItem{
			"ongoing": {
				1: {
					{Slug: "a", CanonicalURL: "https://anichin.cafe/seri/a/"},
					{Slug: "b", CanonicalURL: "https://anichin.cafe/seri/b/"},
				},
				2: {},
			},
			"completed": {
				1: {},
			},
		},
		details: map[string]anichin.AnimeDetail{
			"a": {Slug: "a", EpisodeRefs: []anichin.EpisodeRef{{Slug: "a-ep-1"}}},
			"b": {Slug: "b", EpisodeRefs: []anichin.EpisodeRef{{Slug: "b-ep-1"}}},
		},
		episodes: map[string]anichin.EpisodeDetail{
			"a-ep-1": {EpisodeSlug: "a-ep-1"},
			"b-ep-1": {EpisodeSlug: "b-ep-1"},
		},
	}
	writer := &fakeWriter{}

	report, err := runFullBackfill(context.Background(), fetcher, writer, backfillOptions{
		Sections:             []string{"ongoing", "completed"},
		MaxPagesPerSection:   10,
		MaxEpisodesPerSeries: 0,
	})
	if err != nil {
		t.Fatalf("runFullBackfill returned error: %v", err)
	}
	if report.CatalogPages != 1 {
		t.Fatalf("CatalogPages = %d, want 1", report.CatalogPages)
	}
	if report.DiscoveredSeries != 2 {
		t.Fatalf("DiscoveredSeries = %d, want 2", report.DiscoveredSeries)
	}
	if report.SucceededEpisodes != 2 {
		t.Fatalf("SucceededEpisodes = %d, want 2", report.SucceededEpisodes)
	}
}

func TestRunFullBackfillSkipsExistingDetailsAndEpisodes(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		catalog: map[string]map[int][]anichin.CatalogItem{
			"ongoing": {1: {{Slug: "a", CanonicalURL: "https://anichin.cafe/seri/a/"}}},
		},
		details: map[string]anichin.AnimeDetail{
			"a": {Slug: "a", EpisodeRefs: []anichin.EpisodeRef{{Slug: "a-ep-1"}}},
		},
		episodes: map[string]anichin.EpisodeDetail{
			"a-ep-1": {EpisodeSlug: "a-ep-1"},
		},
	}
	writer := &fakeWriter{
		hasSeries:  map[string]bool{"a": true},
		hasEpisode: map[string]bool{"a-ep-1": true},
	}

	report, err := runFullBackfill(context.Background(), fetcher, writer, backfillOptions{
		Sections:           []string{"ongoing"},
		MaxPagesPerSection: 2,
		SkipExisting:       true,
	})
	if err != nil {
		t.Fatalf("runFullBackfill returned error: %v", err)
	}
	if report.SucceededSeries != 0 {
		t.Fatalf("SucceededSeries = %d, want 0", report.SucceededSeries)
	}
	if report.SucceededEpisodes != 0 {
		t.Fatalf("SucceededEpisodes = %d, want 0", report.SucceededEpisodes)
	}
	if len(writer.series) != 0 {
		t.Fatalf("unexpected series upserts %#v", writer.series)
	}
	if len(writer.episodes) != 0 {
		t.Fatalf("unexpected episode upserts %#v", writer.episodes)
	}
}
