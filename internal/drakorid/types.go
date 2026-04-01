package drakorid

import "time"

type CatalogItem struct {
	MediaType    string
	Title        string
	Slug         string
	CanonicalURL string
	PosterURL    string
	Status       string
	LatestLabel  string
	LatestNumber float64
	SourceDomain string
	PageNumber   int
	Category     string
	ScrapedAt    time.Time
}

type EpisodeRef struct {
	Number       string
	Title        string
	Label        string
	CanonicalURL string
}

type Detail struct {
	MediaType      string
	Title          string
	Slug           string
	CanonicalURL   string
	PosterURL      string
	Synopsis       string
	Status         string
	ReleaseYear    string
	Country        string
	Language       string
	Aired          string
	Runtime        string
	Format         string
	SourceItemID   int
	SourceTypeID   int
	NativeTitle    string
	AltTitle       string
	Genres         []string
	Categories     []string
	Director       string
	Network        string
	EpisodesText   string
	DetailToken    string
	SourceLinkSlug string
	EpisodeRefs    []EpisodeRef
	SourceMetaJSON []byte
	ScrapedAt      time.Time
}

type EpisodeDetail struct {
	MediaType         string
	ItemSlug          string
	EpisodeSlug       string
	CanonicalURL      string
	Title             string
	Label             string
	EpisodeNumber     float64
	StreamURL         string
	StreamLinksJSON   []byte
	DownloadLinksJSON []byte
	SourceMetaJSON    []byte
	ScrapedAt         time.Time
}
