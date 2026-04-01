package hanime

import "time"

type CatalogItem struct {
	SourceDomain       string    `json:"source_domain"`
	CatalogSource      string    `json:"catalog_source"`
	Title              string    `json:"title"`
	NormalizedTitle    string    `json:"normalized_title"`
	EntryKind          string    `json:"entry_kind"`
	EpisodeNumber      int       `json:"episode_number"`
	SeriesCandidate    bool      `json:"series_candidate"`
	CanonicalURL       string    `json:"canonical_url"`
	Slug               string    `json:"slug"`
	CoverURL           string    `json:"cover_url"`
	Description        string    `json:"description"`
	DescriptionExcerpt string    `json:"description_excerpt"`
	Tags               []string  `json:"tags"`
	Brand              string    `json:"brand"`
	BrandSlug          string    `json:"brand_slug"`
	AlternateTitles    []string  `json:"alternate_titles"`
	DownloadPresent    bool      `json:"download_present"`
	ManifestPresent    bool      `json:"manifest_present"`
	ReleasedAt         time.Time `json:"released_at"`
	ScrapedAt          time.Time `json:"scraped_at"`
}

type DetailMetadata struct {
	Title              string
	CoverURL           string
	Description        string
	DescriptionExcerpt string
	Tags               []string
	Brand              string
	BrandSlug          string
	AlternateTitles    []string
	DownloadPresent    bool
	ManifestPresent    bool
	ReleasedAt         time.Time
}
