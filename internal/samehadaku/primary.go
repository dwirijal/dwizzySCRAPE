package samehadaku

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type PrimaryAnimePage struct {
	CanonicalURL  string
	Title         string
	PosterURL     string
	Synopsis      string
	Genres        []string
	Studios       []string
	BatchLinks    map[string]string
	Episodes      []MirrorEpisodeRef
	Metadata      map[string]string
	LatestEpisode string
}

type PrimaryServerOption struct {
	Label  string `json:"label"`
	PostID string `json:"post_id"`
	Number string `json:"number"`
	Type   string `json:"type"`
}

type PrimaryEpisodePage struct {
	CanonicalURL    string
	Title           string
	EpisodeSlug     string
	EpisodeNumber   float64
	PosterURL       string
	AnimeTitle      string
	AnimeURL        string
	PublishedAt     string
	StreamOptions   []PrimaryServerOption
	DirectDownloads map[string]map[string]map[string]string
	PreviousEpisode string
	NextEpisode     string
	AllEpisodesURL  string
	SeriesGenres    []string
	SeriesSynopsis  string
}

func ParsePrimaryAnimeHTML(raw []byte, sourceURL string) (PrimaryAnimePage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return PrimaryAnimePage{}, fmt.Errorf("parse primary anime html: %w", err)
	}

	page := PrimaryAnimePage{
		CanonicalURL:  strings.TrimSpace(sourceURL),
		Title:         normalizeSpace(doc.Find(".infoanime.widget_senction h2.entry-title").First().Text()),
		PosterURL:     firstImageURL(doc.Find(".infoanime.widget_senction .thumb img").First()),
		Synopsis:      normalizeSpace(doc.Find(".infoanime.widget_senction .infox .desc .entry-content").First().Text()),
		Genres:        collectSelectionTexts(doc.Find(".infoanime.widget_senction .genre-info a")),
		Metadata:      extractMetadataMap(doc.Find(".anim-senct .right-senc .spe").First()),
		BatchLinks:    parseBatchLinks(doc, extractDomain(sourceURL)),
		LatestEpisode: strings.TrimSpace(attrOrEmpty(doc.Find(".play-new-episode").First(), "href")),
	}
	page.Studios = extractLinkedMetadata(doc.Find(".anim-senct .right-senc .spe").First(), "Studio")
	page.Episodes = parsePrimaryEpisodeRefs(doc)
	if canonical := strings.TrimSpace(attrOrEmpty(doc.Find("link[rel='canonical']").First(), "href")); canonical != "" {
		page.CanonicalURL = canonical
	}

	if page.Title == "" && len(page.Episodes) == 0 {
		return PrimaryAnimePage{}, fmt.Errorf("primary anime page missing expected selectors")
	}
	return page, nil
}

func ParsePrimaryEpisodeHTML(raw []byte, sourceURL string) (PrimaryEpisodePage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return PrimaryEpisodePage{}, fmt.Errorf("parse primary episode html: %w", err)
	}

	page := PrimaryEpisodePage{
		CanonicalURL:    strings.TrimSpace(sourceURL),
		Title:           normalizeSpace(doc.Find(".player-area .entry-title").First().Text()),
		EpisodeSlug:     slugFromURL(sourceURL),
		PosterURL:       firstNonEmpty(primaryImageFromMeta(doc), firstImageURL(doc.Find(".episodeinf .thumb img").First())),
		AnimeURL:        strings.TrimSpace(attrOrEmpty(doc.Find(".naveps .nvsc a").First(), "href")),
		PublishedAt:     strings.TrimSpace(attrOrEmpty(doc.Find("meta[property='article:published_time']").First(), "content")),
		StreamOptions:   parsePrimaryServerOptions(doc),
		DirectDownloads: parsePrimaryDownloadGroups(doc),
		PreviousEpisode: strings.TrimSpace(attrOrEmpty(doc.Find(".naveps .nvs a").First(), "href")),
		NextEpisode:     strings.TrimSpace(attrOrEmpty(doc.Find(".naveps .nvs.rght a").First(), "href")),
		SeriesGenres:    collectSelectionTexts(doc.Find(".episodeinf .genre-info a")),
		SeriesSynopsis:  normalizeSpace(doc.Find(".episodeinf .desc .entry-content").First().Text()),
	}

	page.EpisodeNumber = parseEpisodeNumber(
		normalizeSpace(doc.Find("[itemprop='episodeNumber']").First().Text()),
		page.Title,
		page.EpisodeSlug,
	)
	page.AllEpisodesURL = page.AnimeURL
	page.AnimeTitle = inferAnimeTitle(page.Title, page.AnimeURL)
	if canonical := strings.TrimSpace(attrOrEmpty(doc.Find("link[rel='canonical']").First(), "href")); canonical != "" {
		page.CanonicalURL = canonical
		page.EpisodeSlug = slugFromURL(canonical)
	}

	if page.Title == "" && len(page.StreamOptions) == 0 && len(page.DirectDownloads) == 0 {
		return PrimaryEpisodePage{}, fmt.Errorf("primary episode page missing expected selectors")
	}
	return page, nil
}

func parsePrimaryEpisodeRefs(doc *goquery.Document) []MirrorEpisodeRef {
	out := make([]MirrorEpisodeRef, 0)
	doc.Find(".whites.lsteps.widget_senction .lstepsiode.listeps ul li").Each(func(_ int, item *goquery.Selection) {
		link := item.Find(".epsleft .lchx a").First()
		href := strings.TrimSpace(attrOrEmpty(link, "href"))
		title := normalizeSpace(link.Text())
		dateText := normalizeSpace(item.Find(".epsleft .date").First().Text())
		numberText := normalizeSpace(item.Find(".epsright .eps a").First().Text())
		ref := MirrorEpisodeRef{
			CanonicalURL:  href,
			Title:         title,
			EpisodeSlug:   slugFromURL(href),
			EpisodeNumber: parseEpisodeNumber(numberText, title, href),
			ReleaseDate:   dateText,
		}
		if ref.CanonicalURL != "" {
			out = append(out, ref)
		}
	})
	return dedupeEpisodeRefs(out)
}

func parsePrimaryServerOptions(doc *goquery.Document) []PrimaryServerOption {
	out := make([]PrimaryServerOption, 0)
	doc.Find(".east_player_option").Each(func(_ int, item *goquery.Selection) {
		option := PrimaryServerOption{
			Label:  normalizeSpace(item.Find("span").First().Text()),
			PostID: strings.TrimSpace(attrOrEmpty(item, "data-post")),
			Number: strings.TrimSpace(attrOrEmpty(item, "data-nume")),
			Type:   strings.TrimSpace(attrOrEmpty(item, "data-type")),
		}
		if option.Label != "" {
			out = append(out, option)
		}
	})
	return out
}

func parsePrimaryDownloadGroups(doc *goquery.Document) map[string]map[string]map[string]string {
	out := make(map[string]map[string]map[string]string)
	doc.Find(".download-eps").Each(func(_ int, group *goquery.Selection) {
		containerName := normalizeSpace(group.Find("p b").First().Text())
		if containerName == "" {
			return
		}
		if _, ok := out[containerName]; !ok {
			out[containerName] = make(map[string]map[string]string)
		}
		group.Find("ul > li").Each(func(_ int, item *goquery.Selection) {
			quality := normalizeSpace(item.Find("strong").First().Text())
			if quality == "" {
				return
			}
			if _, ok := out[containerName][quality]; !ok {
				out[containerName][quality] = make(map[string]string)
			}
			item.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
				label := normalizeSpace(link.Text())
				href := strings.TrimSpace(attrOrEmpty(link, "href"))
				if label == "" || href == "" {
					return
				}
				out[containerName][quality][label] = href
			})
		})
	})
	return out
}

func primaryImageFromMeta(doc *goquery.Document) string {
	return strings.TrimSpace(attrOrEmpty(doc.Find("meta[property='og:image']").First(), "content"))
}

func inferAnimeTitle(episodeTitle, animeURL string) string {
	title := normalizeSpace(episodeTitle)
	if title == "" {
		return titleFromAnimeURL(animeURL)
	}
	markers := []string{" Episode ", " [END]"}
	for _, marker := range markers {
		if idx := strings.Index(title, marker); idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
	}
	if parsed := titleFromAnimeURL(animeURL); parsed != "" {
		return parsed
	}
	return title
}

func titleFromAnimeURL(animeURL string) string {
	slug := slugFromURL(animeURL)
	if slug == "" {
		return ""
	}
	parts := strings.Fields(strings.ReplaceAll(slug, "-", " "))
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func dedupeEpisodeRefs(items []MirrorEpisodeRef) []MirrorEpisodeRef {
	seen := make(map[string]struct{}, len(items))
	out := make([]MirrorEpisodeRef, 0, len(items))
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

func parsePrimaryDownloadCount(groups map[string]map[string]map[string]string) int {
	count := 0
	for _, qualities := range groups {
		for _, hosts := range qualities {
			count += len(hosts)
		}
	}
	return count
}

func parsePrimaryEpisodeNumber(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}
