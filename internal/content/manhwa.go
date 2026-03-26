package content

type ManhwaSeries struct {
	Source        string             `json:"source"`
	MediaType     string             `json:"media_type"`
	Slug          string             `json:"slug"`
	Title         string             `json:"title"`
	AltTitle      string             `json:"alt_title,omitempty"`
	CanonicalURL  string             `json:"canonical_url"`
	CoverURL      string             `json:"cover_url,omitempty"`
	Status        string             `json:"status,omitempty"`
	Type          string             `json:"type,omitempty"`
	ReleasedYear  string             `json:"released_year,omitempty"`
	Author        string             `json:"author,omitempty"`
	Synopsis      string             `json:"synopsis,omitempty"`
	Genres        []string           `json:"genres,omitempty"`
	LatestChapter *ManhwaChapterRef  `json:"latest_chapter,omitempty"`
	Chapters      []ManhwaChapterRef `json:"chapters,omitempty"`
}

type ManhwaChapterRef struct {
	Slug         string `json:"slug"`
	Title        string `json:"title,omitempty"`
	Label        string `json:"label"`
	Number       string `json:"number"`
	CanonicalURL string `json:"canonical_url,omitempty"`
	PublishedAt  string `json:"published_at,omitempty"`
}

type ManhwaChapter struct {
	Source       string      `json:"source,omitempty"`
	SeriesSlug   string      `json:"series_slug,omitempty"`
	SeriesTitle  string      `json:"series_title,omitempty"`
	Slug         string      `json:"slug"`
	Title        string      `json:"title"`
	Label        string      `json:"label"`
	Number       string      `json:"number"`
	CanonicalURL string      `json:"canonical_url"`
	PrevURL      string      `json:"prev_url,omitempty"`
	NextURL      string      `json:"next_url,omitempty"`
	Pages        []PageAsset `json:"pages,omitempty"`
}

type PageAsset struct {
	Position int    `json:"position"`
	URL      string `json:"url"`
}
