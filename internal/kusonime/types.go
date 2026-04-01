package kusonime

import "context"

type SearchResult struct {
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	PosterURL string   `json:"poster_url"`
	Genres    []string `json:"genres"`
}

type BatchLinkGroup struct {
	Label     string                       `json:"label"`
	Downloads map[string]map[string]string `json:"downloads"`
}

type AnimePage struct {
	Title         string           `json:"title"`
	URL           string           `json:"url"`
	PosterURL     string           `json:"poster_url"`
	JapaneseTitle string           `json:"japanese_title"`
	Synopsis      string           `json:"synopsis"`
	Genres        []string         `json:"genres"`
	Producers     []string         `json:"producers"`
	Season        string           `json:"season"`
	BatchType     string           `json:"batch_type"`
	Status        string           `json:"status"`
	TotalEpisodes string           `json:"total_episodes"`
	Duration      string           `json:"duration"`
	ReleasedOn    string           `json:"released_on"`
	PublishedAt   string           `json:"published_at"`
	ModifiedAt    string           `json:"modified_at"`
	Score         float64          `json:"score"`
	Batches       []BatchLinkGroup `json:"batches"`
}

type SamehadakuAnime struct {
	AnimeSlug   string `json:"anime_slug"`
	Title       string `json:"title"`
	SourceTitle string `json:"source_title"`
}

type ReviewResult struct {
	DBAnimeSlug   string    `json:"db_anime_slug"`
	DBTitle       string    `json:"db_title"`
	DBSourceTitle string    `json:"db_source_title"`
	Query         string    `json:"query"`
	MatchStatus   string    `json:"match_status"`
	MatchScore    float64   `json:"match_score"`
	NeedsReview   bool      `json:"needs_review"`
	MatchedTitle  string    `json:"matched_title"`
	MatchedURL    string    `json:"matched_url"`
	Notes         string    `json:"notes"`
	Page          AnimePage `json:"page"`
}

type ReviewOptions struct {
	MinMatchScore float64
}

type Fetcher interface {
	SearchAnime(ctx context.Context, query string) ([]SearchResult, error)
	FetchAnimePage(ctx context.Context, rawURL string) (AnimePage, error)
}
