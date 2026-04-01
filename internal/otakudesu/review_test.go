package otakudesu

import (
	"context"
	"fmt"
	"testing"
)

type fakeFetcher struct {
	searchResults map[string][]SearchResult
	animePages    map[string]AnimePage
	episodePages  map[string]EpisodePage
}

func (f fakeFetcher) SearchAnime(_ context.Context, query string) ([]SearchResult, error) {
	return f.searchResults[query], nil
}

func (f fakeFetcher) FetchAnimePage(_ context.Context, rawURL string) (AnimePage, error) {
	page, ok := f.animePages[rawURL]
	if !ok {
		return AnimePage{}, fmt.Errorf("missing anime page: %s", rawURL)
	}
	return page, nil
}

func (f fakeFetcher) FetchEpisodePage(_ context.Context, rawURL string) (EpisodePage, error) {
	page, ok := f.episodePages[rawURL]
	if !ok {
		return EpisodePage{}, fmt.Errorf("missing episode page: %s", rawURL)
	}
	return page, nil
}

func TestBuildReviews(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		searchResults: map[string][]SearchResult{
			"Kusuriya no Hitorigoto": {
				{Title: "Kusuriya no Hitorigoto Season 2", URL: "https://otakudesu.blog/anime/kusuriya-hitorigoto-s2-sub-indo/"},
				{Title: "Kusuriya no Hitorigoto", URL: "https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/"},
			},
		},
		animePages: map[string]AnimePage{
			"https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/": {
				Title: "Kusuriya no Hitorigoto",
				URL:   "https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/",
				Episodes: []AnimeEpisodeRef{
					{Number: "2", URL: "https://otakudesu.blog/episode/knh-episode-2-sub-indo/"},
					{Number: "1", URL: "https://otakudesu.blog/episode/knh-episode-1-sub-indo/"},
				},
			},
		},
		episodePages: map[string]EpisodePage{
			"https://otakudesu.blog/episode/knh-episode-1-sub-indo/": {
				Title:        "Kusuriya no Hitorigoto",
				Number:       "1",
				StreamURL:    "https://desustream.info/stream/one",
				DownloadURLs: []string{"https://link.desustream.com/?id=one"},
			},
			"https://otakudesu.blog/episode/knh-episode-2-sub-indo/": {
				Title:        "Kusuriya no Hitorigoto",
				Number:       "2",
				StreamURL:    "https://desustream.info/stream/two",
				DownloadURLs: []string{"https://link.desustream.com/?id=two"},
			},
		},
	}

	results, err := BuildReviews(context.Background(), fetcher, []SamehadakuAnime{
		{
			AnimeSlug:   "kusuriya-no-hitorigoto",
			Title:       "Kusuriya no Hitorigoto",
			SourceTitle: "Kusuriya no Hitorigoto",
			Episodes: []SamehadakuEpisode{
				{EpisodeSlug: "kusuriya-episode-1", Number: "1", StreamPresent: true, DownloadPresent: true},
				{EpisodeSlug: "kusuriya-episode-2", Number: "2", StreamPresent: true, DownloadPresent: false},
			},
		},
	}, ReviewOptions{})
	if err != nil {
		t.Fatalf("BuildReviews returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].MatchStatus != "matched" {
		t.Fatalf("unexpected match status %q", results[0].MatchStatus)
	}
	if results[0].MatchedTitle != "Kusuriya no Hitorigoto" {
		t.Fatalf("unexpected matched title %q", results[0].MatchedTitle)
	}
	if len(results[0].Episodes) != 2 {
		t.Fatalf("len(results[0].Episodes) = %d, want 2", len(results[0].Episodes))
	}
	if results[0].Episodes[0].SamehadakuDownloadPresent != true {
		t.Fatalf("expected samehadaku download presence on first episode")
	}
	if results[0].Episodes[1].DownloadURL != "https://link.desustream.com/?id=two" {
		t.Fatalf("unexpected second download url %q", results[0].Episodes[1].DownloadURL)
	}
}

func TestBuildReviewsMarksNoMatch(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		searchResults: map[string][]SearchResult{
			"Zenshuu": {
				{Title: "Completely Different Anime", URL: "https://otakudesu.blog/anime/different/"},
			},
		},
	}

	results, err := BuildReviews(context.Background(), fetcher, []SamehadakuAnime{
		{AnimeSlug: "zenshuu", Title: "Zenshuu"},
	}, ReviewOptions{})
	if err != nil {
		t.Fatalf("BuildReviews returned error: %v", err)
	}
	if results[0].MatchStatus != "no_match" {
		t.Fatalf("unexpected match status %q", results[0].MatchStatus)
	}
}

func TestBuildReviewsKeepsOtakudesuEpisodesWithoutSamehadakuEpisodeRows(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		searchResults: map[string][]SearchResult{
			"Kusuriya no Hitorigoto": {
				{Title: "Kusuriya no Hitorigoto", URL: "https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/"},
			},
		},
		animePages: map[string]AnimePage{
			"https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/": {
				Title: "Kusuriya no Hitorigoto",
				URL:   "https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/",
				Episodes: []AnimeEpisodeRef{
					{Number: "1", URL: "https://otakudesu.blog/episode/knh-episode-1-sub-indo/"},
				},
			},
		},
		episodePages: map[string]EpisodePage{
			"https://otakudesu.blog/episode/knh-episode-1-sub-indo/": {
				Title:        "Kusuriya no Hitorigoto",
				Number:       "1",
				StreamURL:    "https://desustream.info/stream/one",
				DownloadURLs: []string{"https://link.desustream.com/?id=one"},
			},
		},
	}

	results, err := BuildReviews(context.Background(), fetcher, []SamehadakuAnime{
		{
			AnimeSlug:   "kusuriya-no-hitorigoto",
			Title:       "Kusuriya no Hitorigoto",
			SourceTitle: "Kusuriya no Hitorigoto",
		},
	}, ReviewOptions{})
	if err != nil {
		t.Fatalf("BuildReviews returned error: %v", err)
	}

	if len(results[0].Episodes) != 1 {
		t.Fatalf("len(results[0].Episodes) = %d, want 1", len(results[0].Episodes))
	}
	if results[0].Episodes[0].SamehadakuStreamPresent {
		t.Fatal("expected no samehadaku stream presence for missing episode rows")
	}
	if results[0].Episodes[0].StreamURL == "" {
		t.Fatal("expected otakudesu stream url to be captured")
	}
}

func TestBuildReviewsSortsEpisodeNumbersNumerically(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		searchResults: map[string][]SearchResult{
			"Kusuriya no Hitorigoto": {
				{Title: "Kusuriya no Hitorigoto", URL: "https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/"},
			},
		},
		animePages: map[string]AnimePage{
			"https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/": {
				Title: "Kusuriya no Hitorigoto",
				URL:   "https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/",
				Episodes: []AnimeEpisodeRef{
					{Number: "10", URL: "https://otakudesu.blog/episode/knh-episode-10-sub-indo/"},
					{Number: "2", URL: "https://otakudesu.blog/episode/knh-episode-2-sub-indo/"},
					{Number: "1", URL: "https://otakudesu.blog/episode/knh-episode-1-sub-indo/"},
				},
			},
		},
		episodePages: map[string]EpisodePage{
			"https://otakudesu.blog/episode/knh-episode-10-sub-indo/": {Number: "10"},
			"https://otakudesu.blog/episode/knh-episode-2-sub-indo/":  {Number: "2"},
			"https://otakudesu.blog/episode/knh-episode-1-sub-indo/":  {Number: "1"},
		},
	}

	results, err := BuildReviews(context.Background(), fetcher, []SamehadakuAnime{
		{AnimeSlug: "kusuriya-no-hitorigoto", Title: "Kusuriya no Hitorigoto"},
	}, ReviewOptions{})
	if err != nil {
		t.Fatalf("BuildReviews returned error: %v", err)
	}

	if results[0].Episodes[0].EpisodeNumber != "1" {
		t.Fatalf("unexpected first episode %q", results[0].Episodes[0].EpisodeNumber)
	}
	if results[0].Episodes[1].EpisodeNumber != "2" {
		t.Fatalf("unexpected second episode %q", results[0].Episodes[1].EpisodeNumber)
	}
	if results[0].Episodes[2].EpisodeNumber != "10" {
		t.Fatalf("unexpected third episode %q", results[0].Episodes[2].EpisodeNumber)
	}
}
