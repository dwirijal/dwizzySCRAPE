package content

type PageAsset struct {
	Position int    `json:"position"`
	URL      string `json:"url"`
}

type ManhwaChapterRef struct {
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Label        string `json:"label"`
	Number       string `json:"number"`
	CanonicalURL string `json:"canonical_url"`
	PublishedAt  string `json:"published_at"`
}

type ManhwaChapter struct {
	Source       string      `json:"source"`
	SeriesSlug   string      `json:"series_slug"`
	SeriesTitle  string      `json:"series_title"`
	Slug         string      `json:"slug"`
	Title        string      `json:"title"`
	Label        string      `json:"label"`
	Number       string      `json:"number"`
	CanonicalURL string      `json:"canonical_url"`
	PrevURL      string      `json:"prev_url"`
	NextURL      string      `json:"next_url"`
	Pages        []PageAsset `json:"pages"`
}

type ManhwaSeries struct {
	Source        string             `json:"source"`
	MediaType     string             `json:"media_type"`
	Slug          string             `json:"slug"`
	Title         string             `json:"title"`
	AltTitle      string             `json:"alt_title"`
	CanonicalURL  string             `json:"canonical_url"`
	CoverURL      string             `json:"cover_url"`
	Status        string             `json:"status"`
	Type          string             `json:"type"`
	ReleasedYear  string             `json:"released_year"`
	Author        string             `json:"author"`
	Synopsis      string             `json:"synopsis"`
	Genres        []string           `json:"genres"`
	LatestChapter *ManhwaChapterRef  `json:"latest_chapter"`
	Chapters      []ManhwaChapterRef `json:"chapters"`
}
