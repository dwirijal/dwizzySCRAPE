package nekopoi

import "time"

type FeedItem struct {
	SourceDomain       string    `json:"source_domain"`
	Title              string    `json:"title"`
	NormalizedTitle    string    `json:"normalized_title"`
	TitleLabels        []string  `json:"title_labels"`
	EntryKind          string    `json:"entry_kind"`
	EpisodeNumber      int       `json:"episode_number"`
	PartNumber         int       `json:"part_number"`
	SeriesCandidate    bool      `json:"series_candidate"`
	CanonicalURL       string    `json:"canonical_url"`
	Slug               string    `json:"slug"`
	CoverURL           string    `json:"cover_url"`
	Categories         []string  `json:"categories"`
	Genres             []string  `json:"genres"`
	ContentFormat      string    `json:"content_format"`
	DescriptionHTML    string    `json:"description_html"`
	ContentHTML        string    `json:"content_html"`
	DescriptionExcerpt string    `json:"description_excerpt"`
	OriginalTitle      string    `json:"original_title"`
	NuclearCode        string    `json:"nuclear_code"`
	Actress            string    `json:"actress"`
	Parody             string    `json:"parody"`
	Producers          []string  `json:"producers"`
	Duration           string    `json:"duration"`
	Size               string    `json:"size"`
	PostID             string    `json:"post_id"`
	PlayerCount        int       `json:"player_count"`
	PlayerHosts        []string  `json:"player_hosts"`
	DownloadCount      int       `json:"download_count"`
	DownloadLabels     []string  `json:"download_labels"`
	DownloadHosts      []string  `json:"download_hosts"`
	PublishedAt        time.Time `json:"published_at"`
	ScrapedAt          time.Time `json:"scraped_at"`
}
