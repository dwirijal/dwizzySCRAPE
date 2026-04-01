package anichin

import "time"

type CatalogItem struct {
	SourceDomain string    `json:"source_domain"`
	Section      string    `json:"section"`
	Title        string    `json:"title"`
	CanonicalURL string    `json:"canonical_url"`
	Slug         string    `json:"slug"`
	PageNumber   int       `json:"page_number"`
	PosterURL    string    `json:"poster_url"`
	AnimeType    string    `json:"anime_type"`
	Status       string    `json:"status"`
	ScrapedAt    time.Time `json:"scraped_at"`
}

type EpisodeRef struct {
	CanonicalURL string `json:"canonical_url"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Number       string `json:"number"`
	ReleaseLabel string `json:"release_label"`
}

type AnimeDetail struct {
	Slug           string       `json:"slug"`
	CanonicalURL   string       `json:"canonical_url"`
	Title          string       `json:"title"`
	AltTitle       string       `json:"alt_title"`
	PosterURL      string       `json:"poster_url"`
	Synopsis       string       `json:"synopsis"`
	Status         string       `json:"status"`
	AnimeType      string       `json:"anime_type"`
	Season         string       `json:"season"`
	ReleasedYear   string       `json:"released_year"`
	Network        string       `json:"network"`
	StudioNames    []string     `json:"studio_names"`
	GenreNames     []string     `json:"genre_names"`
	SourceMetaJSON []byte       `json:"source_meta_json"`
	EpisodeRefs    []EpisodeRef `json:"episode_refs"`
	ScrapedAt      time.Time    `json:"scraped_at"`
}

type EpisodeDetail struct {
	AnimeSlug      string                       `json:"anime_slug"`
	EpisodeSlug    string                       `json:"episode_slug"`
	CanonicalURL   string                       `json:"canonical_url"`
	Title          string                       `json:"title"`
	EpisodeNumber  float64                      `json:"episode_number"`
	ReleaseLabel   string                       `json:"release_label"`
	StreamURL      string                       `json:"stream_url"`
	StreamMirrors  map[string]string            `json:"stream_mirrors"`
	DownloadLinks  map[string]map[string]string `json:"download_links"`
	SourceMetaJSON []byte                       `json:"source_meta_json"`
	ScrapedAt      time.Time                    `json:"scraped_at"`
}
