package drakorid

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	drakoridTokenPattern = regexp.MustCompile(`var token_now = "([^"]+)"`)
	detailTokenPattern   = regexp.MustCompile(`var token = "([^"]+)"`)
	detailMIDPattern     = regexp.MustCompile(`var mId = (\d+);`)
	detailMTypePattern   = regexp.MustCompile(`var mTipe = (\d+);`)
	detailLinkPattern    = regexp.MustCompile(`var link = "([^"]+)"`)
	lineBreakPattern     = regexp.MustCompile(`(?i)<br\s*/?>`)
	tagPattern           = regexp.MustCompile(`(?s)<[^>]+>`)
	numberPattern        = regexp.MustCompile(`\d+(?:\.\d+)?`)
)

func ParsePageToken(raw []byte) (string, error) {
	match := drakoridTokenPattern.FindSubmatch(raw)
	if len(match) < 2 {
		return "", fmt.Errorf("page token not found")
	}
	return strings.TrimSpace(string(match[1])), nil
}

func ParseOngoingHTML(raw []byte, sourceURL string, page int) ([]CatalogItem, error) {
	return parseCatalogCards(raw, sourceURL, page, "ongoing", "drama")
}

func ParseMovieCatalogHTML(raw []byte, sourceURL string, page int) ([]CatalogItem, error) {
	return parseCatalogCards(raw, sourceURL, page, "movie", "movie")
}

func parseCatalogCards(raw []byte, sourceURL string, page int, category string, mediaType string) ([]CatalogItem, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse catalog html: %w", err)
	}
	sourceDomain := extractDomain(sourceURL)
	items := make([]CatalogItem, 0)
	doc.Find(".card").Each(func(_ int, card *goquery.Selection) {
		link := card.Find("a[href]").First()
		href, ok := link.Attr("href")
		if !ok {
			return
		}
		title := normalizeSpace(card.Find("[data-original-title]").First().AttrOr("data-original-title", ""))
		if title == "" {
			title = normalizeSpace(card.Find("h5, h4, h3").First().Text())
		}
		label := normalizeSpace(card.Find(".badge").First().Text())
		item := CatalogItem{
			MediaType:    mediaType,
			Title:        trimEpisodeSuffix(title),
			Slug:         slugFromURL(href),
			CanonicalURL: strings.TrimSpace(href),
			PosterURL:    extractImageURL(card),
			Status:       catalogStatus(category, label, mediaType),
			LatestLabel:  label,
			LatestNumber: parseNumber(label),
			SourceDomain: sourceDomain,
			PageNumber:   page,
			Category:     category,
			ScrapedAt:    time.Now().UTC(),
		}
		if item.Slug != "" && item.Title != "" {
			items = append(items, item)
		}
	})
	return items, nil
}

func ParseDetailHTML(raw []byte, canonicalURL string) (Detail, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return Detail{}, fmt.Errorf("parse detail html: %w", err)
	}

	detail := Detail{
		Slug:         slugFromURL(canonicalURL),
		CanonicalURL: strings.TrimSpace(canonicalURL),
		PosterURL:    strings.TrimSpace(doc.Find(`meta[property="og:image"]`).AttrOr("content", "")),
		ScrapedAt:    time.Now().UTC(),
	}
	if detail.PosterURL == "" {
		detail.PosterURL = strings.TrimSpace(doc.Find(`link[rel="image_src"]`).AttrOr("href", ""))
	}

	title := normalizeSpace(extractMetaTitle(doc))
	if title == "" {
		title = normalizeSpace(extractDetailValue(raw, "Title"))
	}
	if title == "" {
		title = normalizeSpace(extractDetailValue(raw, "Movie"))
	}
	detail.Title = trimSuffixes(title, " Sub Indo - Drakor.id", " - Drakor.id")
	detail.NativeTitle = normalizeSpace(extractDetailValue(raw, "Native Title"))
	detail.AltTitle = normalizeSpace(extractDetailValue(raw, "Also Known As"))
	detail.Director = normalizeSpace(firstNonBlank(extractDetailValue(raw, "Director"), extractDetailValue(raw, "Writer")))
	detail.Country = normalizeSpace(extractDetailValue(raw, "Country"))
	detail.Language = normalizeSpace(extractDetailValue(raw, "Language"))
	detail.Aired = normalizeSpace(firstNonBlank(extractDetailValue(raw, "Aired"), extractDetailValue(raw, "Release Date")))
	detail.Runtime = normalizeSpace(firstNonBlank(extractDetailValue(raw, "Duration"), extractDetailValue(raw, "Runtime")))
	detail.Format = normalizeSpace(firstNonBlank(extractDetailValue(raw, "Format"), extractDetailValue(raw, "Type")))
	detail.Network = normalizeSpace(firstNonBlank(extractDetailValue(raw, "Original Network"), extractDetailValue(raw, "Distributor")))
	detail.EpisodesText = normalizeSpace(extractDetailValue(raw, "Episodes"))
	detail.Genres = splitValues(extractDetailValue(raw, "Genres"))
	detail.Categories = extractCategories(doc)
	detail.Synopsis = normalizeSpace(extractSynopsis(raw))
	detail.SourceItemID = parseIntMatch(detailMIDPattern, raw)
	detail.SourceTypeID = parseIntMatch(detailMTypePattern, raw)
	detail.DetailToken = normalizeSpace(extractStringMatch(detailTokenPattern, raw))
	detail.SourceLinkSlug = normalizeSpace(extractStringMatch(detailLinkPattern, raw))
	detail.MediaType = detectMediaType(raw, detail)
	detail.Status = detectStatus(detail)
	detail.ReleaseYear = releaseYear(detail)

	doc.Find("#formPilihEpisode option").Each(func(_ int, option *goquery.Selection) {
		value := normalizeSpace(option.AttrOr("value", ""))
		if value == "" || value == "0" {
			return
		}
		label := normalizeSpace(option.Text())
		detail.EpisodeRefs = append(detail.EpisodeRefs, EpisodeRef{
			Number:       value,
			Label:        label,
			Title:        episodeTitle(detail.Title, value),
			CanonicalURL: strings.TrimRight(detail.CanonicalURL, "/") + "/episode-" + value,
		})
	})
	if len(detail.EpisodeRefs) == 0 {
		detail.EpisodeRefs = append(detail.EpisodeRefs, EpisodeRef{
			Number:       "1",
			Label:        "Episode 1",
			Title:        episodeTitle(detail.Title, "1"),
			CanonicalURL: strings.TrimRight(detail.CanonicalURL, "/") + "/episode-1",
		})
	}

	sourceMeta, _ := json.Marshal(map[string]any{
		"detail_token_present": detailTokenPattern.Match(raw),
		"detail_token":         detail.DetailToken,
		"source_item_id":       detail.SourceItemID,
		"source_type_id":       detail.SourceTypeID,
		"source_link_slug":     detail.SourceLinkSlug,
		"categories":           detail.Categories,
		"country":              detail.Country,
		"language":             detail.Language,
		"format":               detail.Format,
		"aired":                detail.Aired,
		"runtime":              detail.Runtime,
		"network":              detail.Network,
		"episodes_text":        detail.EpisodesText,
		"detail_canonical_url": detail.CanonicalURL,
	})
	detail.SourceMetaJSON = sourceMeta

	if detail.Title == "" {
		return Detail{}, fmt.Errorf("detail page missing title")
	}
	return detail, nil
}

func ParseWatchHTML(raw []byte) (string, map[string]string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return "", nil, fmt.Errorf("parse watch html: %w", err)
	}
	iframe := strings.TrimSpace(doc.Find("iframe[src]").First().AttrOr("src", ""))
	if iframe == "" {
		return "", nil, fmt.Errorf("watch page missing player iframe")
	}
	streamURL := decodeBunnyVideoURL(iframe)
	if streamURL == "" {
		streamURL = iframe
	}
	return streamURL, map[string]string{"player": iframe}, nil
}

func ParseDownloadHTML(raw []byte) (map[string]map[string]string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse download html: %w", err)
	}
	downloads := make(map[string]map[string]string)
	doc.Find(`a[target="_blank"][href]`).Each(func(_ int, link *goquery.Selection) {
		href := strings.TrimSpace(link.AttrOr("href", ""))
		label := normalizeSpace(link.Text())
		if href == "" || label == "" {
			return
		}
		quality := firstNonBlank(extractQuality(label), "unknown")
		if _, ok := downloads[quality]; !ok {
			downloads[quality] = make(map[string]string)
		}
		downloads[quality]["direct"] = href
	})
	if len(downloads) == 0 {
		return nil, fmt.Errorf("download page missing direct links")
	}
	return downloads, nil
}

func extractMetaTitle(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	title := normalizeSpace(doc.Find(`meta[property="og:title"]`).AttrOr("content", ""))
	if title != "" {
		return title
	}
	return normalizeSpace(doc.Find("title").First().Text())
}

func extractCategories(doc *goquery.Document) []string {
	values := make([]string, 0)
	doc.Find(".chip-outline, .chip").Each(func(_ int, item *goquery.Selection) {
		value := normalizeSpace(item.Text())
		if value == "" {
			return
		}
		if strings.EqualFold(value, "2026") || numberPattern.MatchString(value) && len(strings.Fields(value)) == 1 {
			return
		}
		values = append(values, value)
	})
	return uniqueStrings(values)
}

func extractSynopsis(raw []byte) string {
	text := string(raw)
	idx := strings.Index(text, "Sinopsis<br")
	if idx < 0 {
		idx = strings.Index(text, ">Sinopsis<br")
	}
	if idx < 0 {
		return ""
	}
	segment := text[idx:]
	end := strings.Index(segment, "</p>")
	if end >= 0 {
		segment = segment[:end]
	}
	segment = lineBreakPattern.ReplaceAllString(segment, "\n")
	segment = tagPattern.ReplaceAllString(segment, "")
	segment = html.UnescapeString(segment)
	segment = strings.TrimSpace(strings.TrimPrefix(segment, "Sinopsis"))
	return normalizeSpace(segment)
}

func extractDetailValue(raw []byte, label string) string {
	text := html.UnescapeString(string(raw))
	pattern := regexp.MustCompile(regexp.QuoteMeta(label) + `:\s*([^<\n]+)`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return normalizeSpace(match[1])
}

func decodeBunnyVideoURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	value := parsed.Query().Get("v")
	if value == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(padBase64(value))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(decoded))
}

func padBase64(value string) string {
	for len(value)%4 != 0 {
		value += "="
	}
	return value
}

func detectMediaType(raw []byte, detail Detail) string {
	if detail.SourceTypeID == 1 {
		return "movie"
	}
	if strings.TrimSpace(extractDetailValue(raw, "Movie")) != "" {
		return "movie"
	}
	return "drama"
}

func detectStatus(detail Detail) string {
	if detail.MediaType == "movie" {
		return "completed"
	}
	if strings.Contains(detail.Aired, "?") {
		return "ongoing"
	}
	return "completed"
}

func releaseYear(detail Detail) string {
	values := []string{detail.Aired, detail.Title, detail.CanonicalURL}
	for _, value := range values {
		match := regexp.MustCompile(`(?:19|20)\d{2}`).FindString(value)
		if match != "" {
			return match
		}
	}
	return ""
}

func catalogStatus(category, latestLabel, mediaType string) string {
	if mediaType == "movie" {
		return "completed"
	}
	if strings.Contains(strings.ToLower(latestLabel), "episode") {
		return "ongoing"
	}
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "ongoing":
		return "ongoing"
	default:
		return ""
	}
}

func extractImageURL(selection *goquery.Selection) string {
	if selection == nil {
		return ""
	}
	img := selection.Find("img").First()
	for _, attr := range []string{"data-src", "data-lazy-src", "src"} {
		value := strings.TrimSpace(img.AttrOr(attr, ""))
		if value != "" {
			return value
		}
	}
	return ""
}

func extractDomain(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Host))
}

func slugFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func normalizeSpace(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func trimEpisodeSuffix(raw string) string {
	re := regexp.MustCompile(`(?i)\s+episode\s+\d+(?:\.\d+)?$`)
	return normalizeSpace(re.ReplaceAllString(strings.TrimSpace(raw), ""))
}

func episodeTitle(itemTitle, number string) string {
	number = normalizeSpace(number)
	if number == "" {
		return strings.TrimSpace(itemTitle)
	}
	return normalizeSpace(itemTitle + " Episode " + number)
}

func parseNumber(raw string) float64 {
	match := numberPattern.FindString(raw)
	if match == "" {
		return 0
	}
	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return value
}

func parseIntMatch(pattern *regexp.Regexp, raw []byte) int {
	value := extractStringMatch(pattern, raw)
	if value == "" {
		return 0
	}
	n, _ := strconv.Atoi(value)
	return n
}

func extractStringMatch(pattern *regexp.Regexp, raw []byte) string {
	match := pattern.FindSubmatch(raw)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(string(match[1]))
}

func splitValues(raw string) []string {
	raw = normalizeSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := normalizeSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return uniqueStrings(values)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, strings.TrimSpace(value))
	}
	return out
}

func extractQuality(raw string) string {
	match := regexp.MustCompile(`\b(?:240|360|480|540|720|1080|2160)p\b`).FindString(strings.ToLower(raw))
	return strings.ToUpper(match)
}

func trimSuffixes(raw string, suffixes ...string) string {
	raw = strings.TrimSpace(raw)
	for _, suffix := range suffixes {
		raw = strings.TrimSpace(strings.TrimSuffix(raw, suffix))
	}
	return raw
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := normalizeSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
