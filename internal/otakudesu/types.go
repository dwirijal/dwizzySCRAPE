package otakudesu

import "context"

type SearchResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type AnimeEpisodeRef struct {
	Title  string `json:"title"`
	Number string `json:"number"`
	URL    string `json:"url"`
}

type AnimePage struct {
	Title    string            `json:"title"`
	URL      string            `json:"url"`
	Episodes []AnimeEpisodeRef `json:"episodes"`
}

type EpisodePage struct {
	Title          string                       `json:"title"`
	Number         string                       `json:"number"`
	URL            string                       `json:"url"`
	StreamURL      string                       `json:"stream_url"`
	StreamMirrors  map[string]string            `json:"stream_mirrors"`
	DownloadLinks  map[string]map[string]string `json:"download_links"`
	DownloadURLs   []string                     `json:"download_urls"`
	MirrorRequests []EpisodeMirrorRequest       `json:"-"`
}

type EpisodeMirrorRequest struct {
	Label          string `json:"label"`
	EncodedContent string `json:"encoded_content"`
}

type SamehadakuEpisode struct {
	EpisodeSlug     string `json:"episode_slug"`
	Number          string `json:"number"`
	StreamPresent   bool   `json:"stream_present"`
	DownloadPresent bool   `json:"download_present"`
}

type SamehadakuAnime struct {
	AnimeSlug   string              `json:"anime_slug"`
	Title       string              `json:"title"`
	SourceTitle string              `json:"source_title"`
	Episodes    []SamehadakuEpisode `json:"episodes"`
}

type EpisodeReview struct {
	DBEpisodeSlug             string                       `json:"db_episode_slug"`
	EpisodeNumber             string                       `json:"episode_number"`
	OtakudesuEpisodeURL       string                       `json:"otakudesu_episode_url"`
	StreamURL                 string                       `json:"stream_url"`
	StreamMirrors             map[string]string            `json:"stream_mirrors"`
	DownloadLinks             map[string]map[string]string `json:"download_links"`
	DownloadURL               string                       `json:"download_url"`
	SamehadakuStreamPresent   bool                         `json:"samehadaku_stream_present"`
	SamehadakuDownloadPresent bool                         `json:"samehadaku_download_present"`
}

type ReviewResult struct {
	DBAnimeSlug   string          `json:"db_anime_slug"`
	DBTitle       string          `json:"db_title"`
	DBSourceTitle string          `json:"db_source_title"`
	Query         string          `json:"query"`
	MatchStatus   string          `json:"match_status"`
	MatchScore    float64         `json:"match_score"`
	NeedsReview   bool            `json:"needs_review"`
	MatchedTitle  string          `json:"matched_title"`
	MatchedURL    string          `json:"matched_url"`
	Notes         string          `json:"notes"`
	Episodes      []EpisodeReview `json:"episodes"`
}

type ReviewOptions struct {
	MinMatchScore float64
	MaxEpisodes   int
}

type Fetcher interface {
	SearchAnime(ctx context.Context, query string) ([]SearchResult, error)
	FetchAnimePage(ctx context.Context, rawURL string) (AnimePage, error)
	FetchEpisodePage(ctx context.Context, rawURL string) (EpisodePage, error)
}
