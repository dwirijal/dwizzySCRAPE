package samehadaku

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const DefaultMirrorBaseURL = "https://samehadaku.li"

var episodeNumberPattern = regexp.MustCompile(`(?i)episode\s+(\d+(?:\.\d+)?)`)

type MirrorAnimePage struct {
	CanonicalURL  string
	Title         string
	PosterURL     string
	TrailerURL    string
	Synopsis      string
	Genres        []string
	Studios       []string
	BatchLinks    map[string]string
	Cast          []MirrorCastEntry
	Episodes      []MirrorEpisodeRef
	Metadata      map[string]string
	FirstEpisode  string
	LatestEpisode string
}

type MirrorCastEntry struct {
	CharacterName  string
	CharacterRole  string
	CharacterImage string
	ActorName      string
	ActorRole      string
	ActorImage     string
}

type MirrorEpisodeRef struct {
	CanonicalURL  string
	Title         string
	EpisodeSlug   string
	EpisodeNumber float64
	ReleaseDate   string
}

type MirrorEpisodePage struct {
	CanonicalURL    string
	Title           string
	EpisodeSlug     string
	EpisodeNumber   float64
	PosterURL       string
	PrimaryStream   string
	AnimeTitle      string
	AnimeURL        string
	PublishedAt     string
	Summary         string
	StreamMirrors   map[string]string
	DirectDownloads map[string]string
	PreviousEpisode string
	NextEpisode     string
	AllEpisodesURL  string
	SeriesMetadata  map[string]string
	SeriesGenres    []string
	SeriesSynopsis  string
}

func BuildMirrorAnimeURL(slug string) string {
	return strings.TrimRight(DefaultMirrorBaseURL, "/") + "/anime/" + strings.Trim(strings.TrimSpace(slug), "/") + "/"
}

func ParseMirrorAnimeHTML(raw []byte, sourceURL string) (MirrorAnimePage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return MirrorAnimePage{}, fmt.Errorf("parse mirror anime html: %w", err)
	}

	page := MirrorAnimePage{
		CanonicalURL: strings.TrimSpace(sourceURL),
		Title:        normalizeSpace(doc.Find(".animefull .infox .entry-title").First().Text()),
		PosterURL:    firstImageURL(doc.Find(".animefull .thumb img").First()),
		TrailerURL:   strings.TrimSpace(attrOrEmpty(doc.Find(".animefull .trailerbutton").First(), "href")),
		Synopsis:     normalizeSpace(doc.Find(".bixbox.synp .entry-content").First().Text()),
		Genres:       collectSelectionTexts(doc.Find(".animefull .genxed a")),
		Studios:      extractLinkedMetadata(doc.Find(".animefull .spe").First(), "Studio"),
		BatchLinks:   parseBatchLinks(doc, extractDomain(sourceURL)),
		Metadata:     extractMetadataMap(doc.Find(".animefull .spe").First()),
	}
	page.Cast = parseMirrorCast(doc)
	page.Episodes = parseMirrorEpisodeRefs(doc)
	page.FirstEpisode = strings.TrimSpace(attrOrEmpty(doc.Find(".epcurfirst").First().Parent(), "href"))
	page.LatestEpisode = strings.TrimSpace(attrOrEmpty(doc.Find(".epcurlast").First().Parent(), "href"))
	if canonical := strings.TrimSpace(attrOrEmpty(doc.Find("link[rel='canonical']").First(), "href")); canonical != "" {
		page.CanonicalURL = canonical
	}

	return page, nil
}

func ParseMirrorEpisodeHTML(raw []byte, sourceURL string) (MirrorEpisodePage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return MirrorEpisodePage{}, fmt.Errorf("parse mirror episode html: %w", err)
	}

	page := MirrorEpisodePage{
		CanonicalURL:    strings.TrimSpace(sourceURL),
		Title:           normalizeSpace(doc.Find(".entry-title").First().Text()),
		EpisodeSlug:     slugFromURL(sourceURL),
		PosterURL:       firstImageURL(doc.Find(".item.meta .tb img").First()),
		AnimeTitle:      normalizeSpace(doc.Find(".single-info .infolimit h2").First().Text()),
		AnimeURL:        strings.TrimSpace(attrOrEmpty(doc.Find(".item.meta .year a[href*='/anime/']").First(), "href")),
		PublishedAt:     strings.TrimSpace(attrOrEmpty(doc.Find("meta[property='article:published_time']").First(), "content")),
		Summary:         normalizeSpace(doc.Find(".bixbox.infx").First().Text()),
		StreamMirrors:   parseMirrorOptions(doc.Find("select.mirror option")),
		DirectDownloads: parseDirectDownloads(doc),
		SeriesMetadata:  extractMetadataMap(doc.Find(".single-info .spe").First()),
		SeriesGenres:    collectSelectionTexts(doc.Find(".single-info .genxed a")),
		SeriesSynopsis:  normalizeSpace(doc.Find(".single-info .desc").First().Text()),
	}

	page.EpisodeNumber = parseEpisodeNumber(
		strings.TrimSpace(attrOrEmpty(doc.Find("meta[itemprop='episodeNumber']").First(), "content")),
		page.Title,
		page.EpisodeSlug,
	)
	page.PreviousEpisode = strings.TrimSpace(attrOrEmpty(doc.Find(".naveps a[rel='prev']").First(), "href"))
	page.NextEpisode = strings.TrimSpace(attrOrEmpty(doc.Find(".naveps a[rel='next']").First(), "href"))
	page.AllEpisodesURL = strings.TrimSpace(attrOrEmpty(doc.Find(".naveps .nvsc a").First(), "href"))
	if page.AllEpisodesURL == "" {
		page.AllEpisodesURL = strings.TrimSpace(attrOrEmpty(doc.Find(".item.meta .year a[href*='/anime/']").First(), "href"))
	}
	if iframe := strings.TrimSpace(attrOrEmpty(doc.Find(".player-embed iframe").First(), "src")); iframe != "" {
		page.PrimaryStream = iframe
	}
	if canonical := strings.TrimSpace(attrOrEmpty(doc.Find("link[rel='canonical']").First(), "href")); canonical != "" {
		page.CanonicalURL = canonical
		page.EpisodeSlug = slugFromURL(canonical)
	}

	return page, nil
}

func parseMirrorCast(doc *goquery.Document) []MirrorCastEntry {
	out := make([]MirrorCastEntry, 0)
	doc.Find(".cvlist .cvitem").Each(func(_ int, item *goquery.Selection) {
		charSel := item.Find(".cvsubitem.cvchar").First()
		actorSel := item.Find(".cvsubitem.cvactor").First()
		entry := MirrorCastEntry{
			CharacterName:  normalizeSpace(charSel.Find(".charname").First().Text()),
			CharacterRole:  normalizeSpace(charSel.Find(".charrole").First().Text()),
			CharacterImage: firstImageURL(charSel.Find("img").First()),
			ActorName:      normalizeSpace(actorSel.Find(".charname").First().Text()),
			ActorRole:      normalizeSpace(actorSel.Find(".charrole").First().Text()),
			ActorImage:     firstImageURL(actorSel.Find("img").First()),
		}
		if entry.CharacterName != "" {
			out = append(out, entry)
		}
	})
	return out
}

func parseMirrorEpisodeRefs(doc *goquery.Document) []MirrorEpisodeRef {
	out := make([]MirrorEpisodeRef, 0)
	doc.Find(".eplister ul li a").Each(func(_ int, item *goquery.Selection) {
		href := strings.TrimSpace(attrOrEmpty(item, "href"))
		title := normalizeSpace(item.Find(".epl-title").First().Text())
		ref := MirrorEpisodeRef{
			CanonicalURL:  href,
			Title:         title,
			EpisodeSlug:   slugFromURL(href),
			EpisodeNumber: parseEpisodeNumber(normalizeSpace(item.Find(".epl-num").First().Text()), title, href),
			ReleaseDate:   normalizeSpace(item.Find(".epl-date").First().Text()),
		}
		if ref.CanonicalURL != "" {
			out = append(out, ref)
		}
	})
	return out
}

func parseMirrorOptions(options *goquery.Selection) map[string]string {
	out := make(map[string]string)
	options.Each(func(_ int, option *goquery.Selection) {
		label := normalizeSpace(option.Text())
		encoded, ok := option.Attr("value")
		if !ok || strings.TrimSpace(encoded) == "" {
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return
		}
		frameDoc, err := goquery.NewDocumentFromReader(bytes.NewReader(decoded))
		if err != nil {
			return
		}
		src := strings.TrimSpace(attrOrEmpty(frameDoc.Find("iframe").First(), "src"))
		if src == "" {
			return
		}
		if label == "" {
			label = fmt.Sprintf("mirror_%d", len(out)+1)
		}
		out[label] = src
	})
	return out
}

func parseDirectDownloads(doc *goquery.Document) map[string]string {
	out := make(map[string]string)
	doc.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
		href := strings.TrimSpace(attrOrEmpty(link, "href"))
		if href == "" {
			return
		}
		text := normalizeSpace(link.Text())
		aria := strings.ToLower(strings.TrimSpace(attrOrEmpty(link, "aria-label")))
		if strings.Contains(strings.ToLower(text), "download") || aria == "download" {
			label := "download"
			if text != "" {
				label = text
			}
			out[label] = href
		}
	})
	return out
}

func parseBatchLinks(doc *goquery.Document, sourceDomain string) map[string]string {
	out := make(map[string]string)
	doc.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
		href := strings.TrimSpace(attrOrEmpty(link, "href"))
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") || strings.HasPrefix(href, "#") {
			return
		}
		label := normalizeSpace(link.Text())
		lowerText := strings.ToLower(label)
		lowerHref := strings.ToLower(href)

		if !strings.Contains(lowerText, "batch") && !strings.Contains(lowerHref, "batch") {
			return
		}
		if strings.Contains(lowerHref, "/list") || strings.Contains(lowerHref, "/daftar") {
			return
		}

		host := extractDomain(href)
		if host != "" && sourceDomain != "" && strings.EqualFold(host, sourceDomain) {
			// Avoid internal navigation links that happen to contain "batch".
			if !(strings.Contains(lowerHref, "download") || strings.Contains(lowerHref, "/batch")) {
				return
			}
		}

		if label == "" {
			label = fmt.Sprintf("batch_%d", len(out)+1)
		}
		if _, exists := out[label]; exists {
			label = fmt.Sprintf("%s_%d", label, len(out)+1)
		}
		out[label] = href
	})
	return out
}

func extractMetadataMap(container *goquery.Selection) map[string]string {
	out := make(map[string]string)
	container.Find("span").Each(func(_ int, span *goquery.Selection) {
		label := normalizeSpace(strings.TrimSuffix(span.Find("b").First().Text(), ":"))
		if label == "" {
			return
		}
		clone := span.Clone()
		clone.Find("b").First().Remove()
		value := normalizeSpace(clone.Text())
		if value != "" {
			out[label] = value
		}
	})
	return out
}

func extractLinkedMetadata(container *goquery.Selection, label string) []string {
	var result []string
	container.Find("span").Each(func(_ int, span *goquery.Selection) {
		key := normalizeSpace(strings.TrimSuffix(span.Find("b").First().Text(), ":"))
		if !strings.EqualFold(key, label) {
			return
		}
		result = collectSelectionTexts(span.Find("a"))
	})
	return result
}

func collectSelectionTexts(selection *goquery.Selection) []string {
	out := make([]string, 0)
	selection.Each(func(_ int, item *goquery.Selection) {
		text := normalizeSpace(item.Text())
		if text != "" {
			out = append(out, text)
		}
	})
	return out
}

func firstImageURL(selection *goquery.Selection) string {
	for _, attr := range []string{"src", "data-src", "data-lazy-src"} {
		if value, ok := selection.Attr(attr); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func parseEpisodeNumber(values ...string) float64 {
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}
		if parsed, err := strconv.ParseFloat(clean, 64); err == nil {
			return parsed
		}
		match := episodeNumberPattern.FindStringSubmatch(clean)
		if len(match) > 1 {
			if parsed, err := strconv.ParseFloat(match[1], 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func attrOrEmpty(selection *goquery.Selection, attr string) string {
	if selection == nil {
		return ""
	}
	value, _ := selection.Attr(attr)
	return value
}

func parseDateOrZero(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	formats := []string{time.RFC3339, "January 2, 2006"}
	for _, format := range formats {
		if parsed, err := time.Parse(format, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func normalizeMirrorURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return parsed.String()
}
