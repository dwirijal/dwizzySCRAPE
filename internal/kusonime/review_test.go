package kusonime

import (
	"context"
	"fmt"
	"testing"
)

type fakeFetcher struct {
	searchResults map[string][]SearchResult
	animePages    map[string]AnimePage
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

func TestBuildReviews(t *testing.T) {
	t.Parallel()

	fetcher := fakeFetcher{
		searchResults: map[string][]SearchResult{
			"Kusuriya no Hitorigoto": {
				{Title: "Kusuriya no Hitorigoto Season 2", URL: "https://kusonime.com/kusuriyanohitorigoto-s2-batch-subtitle-indonesia/"},
				{Title: "Kusuriya no Hitorigoto", URL: "https://kusonime.com/kusuriyanohitorigoto-batch-subtitle-indonesia/"},
			},
		},
		animePages: map[string]AnimePage{
			"https://kusonime.com/kusuriyanohitorigoto-batch-subtitle-indonesia/": {
				Title: "Kusuriya no Hitorigoto",
				URL:   "https://kusonime.com/kusuriyanohitorigoto-batch-subtitle-indonesia/",
				Batches: []BatchLinkGroup{
					{
						Label: "Download Kusuriya no Hitorigoto Episode 01-12 Batch BD Subtitle Indonesia",
						Downloads: map[string]map[string]string{
							"360P": {"Google Drive": "https://drive.example/360"},
						},
					},
				},
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

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].MatchStatus != "matched" {
		t.Fatalf("unexpected match status %q", results[0].MatchStatus)
	}
	if results[0].MatchedTitle != "Kusuriya no Hitorigoto" {
		t.Fatalf("unexpected matched title %q", results[0].MatchedTitle)
	}
	if len(results[0].Page.Batches) != 1 {
		t.Fatalf("len(results[0].Page.Batches) = %d, want 1", len(results[0].Page.Batches))
	}
}
