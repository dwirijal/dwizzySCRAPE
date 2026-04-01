package samehadaku

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	scorePattern = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	viewsPattern = regexp.MustCompile(`(\d[\d.,]*)\s*Views?`)
)

type CatalogItem struct {
	Source          string    `json:"source"`
	SourceDomain    string    `json:"source_domain"`
	ContentType     string    `json:"content_type"`
	Title           string    `json:"title"`
	CanonicalURL    string    `json:"canonical_url"`
	Slug            string    `json:"slug"`
	PageNumber      int       `json:"page_number"`
	PosterURL       string    `json:"poster_url"`
	AnimeType       string    `json:"anime_type,omitempty"`
	Status          string    `json:"status,omitempty"`
	Score           float64   `json:"score"`
	Views           int64     `json:"views"`
	SynopsisExcerpt string    `json:"synopsis_excerpt"`
	Genres          []string  `json:"genres"`
	ScrapedAt       time.Time `json:"scraped_at"`
}

type AnimeDetail struct {
	Slug                  string    `json:"slug"`
	CanonicalURL          string    `json:"canonical_url"`
	PrimarySourceURL      string    `json:"primary_source_url"`
	PrimarySourceDomain   string    `json:"primary_source_domain"`
	SecondarySourceURL    string    `json:"secondary_source_url"`
	SecondarySourceDomain string    `json:"secondary_source_domain"`
	EffectiveSourceURL    string    `json:"effective_source_url"`
	EffectiveSourceDomain string    `json:"effective_source_domain"`
	EffectiveSourceKind   string    `json:"effective_source_kind"`
	SourceTitle           string    `json:"source_title"`
	MALID                 int       `json:"mal_id,omitempty"`
	MALURL                string    `json:"mal_url"`
	MALThumbnailURL       string    `json:"mal_thumbnail_url"`
	SynopsisSource        string    `json:"synopsis_source"`
	SynopsisEnriched      string    `json:"synopsis_enriched"`
	AnimeType             string    `json:"anime_type,omitempty"`
	Status                string    `json:"status,omitempty"`
	Season                string    `json:"season,omitempty"`
	StudioNames           []string  `json:"studio_names"`
	GenreNames            []string  `json:"genre_names"`
	BatchLinksJSON        []byte    `json:"batch_links_json"`
	CastJSON              []byte    `json:"cast_json"`
	SourceMetaJSON        []byte    `json:"source_meta_json"`
	JikanMetaJSON         []byte    `json:"jikan_meta_json"`
	SourceFetchStatus     string    `json:"source_fetch_status"`
	SourceFetchError      string    `json:"source_fetch_error"`
	ScrapedAt             time.Time `json:"scraped_at"`
}

type EpisodeDetail struct {
	AnimeSlug             string    `json:"anime_slug"`
	EpisodeSlug           string    `json:"episode_slug"`
	CanonicalURL          string    `json:"canonical_url"`
	PrimarySourceURL      string    `json:"primary_source_url"`
	PrimarySourceDomain   string    `json:"primary_source_domain"`
	SecondarySourceURL    string    `json:"secondary_source_url"`
	SecondarySourceDomain string    `json:"secondary_source_domain"`
	EffectiveSourceURL    string    `json:"effective_source_url"`
	EffectiveSourceDomain string    `json:"effective_source_domain"`
	EffectiveSourceKind   string    `json:"effective_source_kind"`
	Title                 string    `json:"title"`
	EpisodeNumber         float64   `json:"episode_number"`
	ReleaseLabel          string    `json:"release_label"`
	StreamLinksJSON       []byte    `json:"stream_links_json"`
	DownloadLinksJSON     []byte    `json:"download_links_json"`
	SourceMetaJSON        []byte    `json:"source_meta_json"`
	FetchStatus           string    `json:"fetch_status"`
	FetchError            string    `json:"fetch_error"`
	ScrapedAt             time.Time `json:"scraped_at"`
}

func ParseCatalogHTML(raw []byte, sourceURL string, scrapedAt time.Time) ([]CatalogItem, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	sourceDomain := extractDomain(sourceURL)
	items := make([]CatalogItem, 0)

	cardSelection := doc.Find("article")
	if cardSelection.Length() == 0 {
		doc.Find("h4").Each(func(_ int, selection *goquery.Selection) {
			root := selection.Parent()
			if root != nil {
				item, ok := parseCard(root.Parent(), sourceDomain, scrapedAt)
				if ok {
					items = append(items, item)
				}
			}
		})
		return dedupeCatalog(items), nil
	}

	cardSelection.Each(func(_ int, selection *goquery.Selection) {
		item, ok := parseCard(selection, sourceDomain, scrapedAt)
		if ok {
			items = append(items, item)
		}
	})

	return dedupeCatalog(items), nil
}

func parseCard(selection *goquery.Selection, sourceDomain string, scrapedAt time.Time) (CatalogItem, bool) {
	title := normalizeSpace(selection.Find("h4").First().Text())
	href, ok := selection.Find("a[href]").First().Attr("href")
	if !ok || title == "" {
		return CatalogItem{}, false
	}

	canonicalURL := strings.TrimSpace(href)
	metaText := normalizeSpace(selection.Find(".meta").First().Text())
	if metaText == "" {
		metaText = normalizeSpace(selection.Text())
	}

	score := extractScore(metaText)
	views := extractViews(metaText)
	animeType := extractMetaValue(selection, ".type", []string{"TV", "OVA", "ONA", "Special", "Movie"})
	status := extractMetaValue(selection, ".status", []string{"Ongoing", "Completed", "Finished Airing", "Currently Airing"})
	excerpt := normalizeSpace(selection.Find("p").First().Text())
	genres := extractGenres(selection)
	posterURL := extractPosterURL(selection)

	return CatalogItem{
		Source:          "samehadaku",
		SourceDomain:    sourceDomain,
		ContentType:     contentTypeFromAnimeType(animeType),
		Title:           title,
		CanonicalURL:    canonicalURL,
		Slug:            slugFromURL(canonicalURL),
		PosterURL:       posterURL,
		AnimeType:       animeType,
		Status:          status,
		Score:           score,
		Views:           views,
		SynopsisExcerpt: excerpt,
		Genres:          genres,
		ScrapedAt:       scrapedAt,
	}, true
}

func contentTypeFromAnimeType(animeType string) string {
	if strings.EqualFold(strings.TrimSpace(animeType), "movie") {
		return "movie"
	}
	return "anime"
}

func dedupeCatalog(items []CatalogItem) []CatalogItem {
	seen := make(map[string]struct{}, len(items))
	out := make([]CatalogItem, 0, len(items))
	for _, item := range items {
		if item.CanonicalURL == "" {
			continue
		}
		if _, ok := seen[item.CanonicalURL]; ok {
			continue
		}
		seen[item.CanonicalURL] = struct{}{}
		out = append(out, item)
	}
	return out
}

func extractMetaValue(selection *goquery.Selection, directSelector string, fallbacks []string) string {
	if direct := normalizeSpace(selection.Find(directSelector).First().Text()); direct != "" {
		return direct
	}
	text := normalizeSpace(selection.Text())
	for _, candidate := range fallbacks {
		if strings.Contains(strings.ToLower(text), strings.ToLower(candidate)) {
			return candidate
		}
	}
	return ""
}

func extractScore(meta string) float64 {
	match := scorePattern.FindString(meta)
	if match == "" {
		return 0
	}
	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return value
}

func extractViews(meta string) int64 {
	match := viewsPattern.FindStringSubmatch(meta)
	if len(match) < 2 {
		return 0
	}
	clean := strings.NewReplacer(".", "", ",", "").Replace(match[1])
	value, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func extractGenres(selection *goquery.Selection) []string {
	genres := make([]string, 0)
	selection.Find(".genres a").Each(func(_ int, genre *goquery.Selection) {
		text := normalizeSpace(genre.Text())
		if text != "" {
			genres = append(genres, text)
		}
	})
	return genres
}

func extractPosterURL(selection *goquery.Selection) string {
	for _, attr := range []string{"data-src", "src", "data-lazy-src"} {
		if value, ok := selection.Find("img").First().Attr(attr); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func extractDomain(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func slugFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func normalizeSpace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}
