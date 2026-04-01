package hanime

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	hanimeExplicitEpisodePattern = regexp.MustCompile(`(?i)^(.*?)(?:\s+episode\s+)(\d{1,3})$`)
	hanimeTrailingNumberPattern  = regexp.MustCompile(`(?i)^(.*?)(?:\s+)(\d{1,3})$`)
	hanimeWhitespacePattern      = regexp.MustCompile(`\s+`)
	hanimeManifestPattern        = regexp.MustCompile(`(?i)(application/x-mpegurl|kind\s*:\s*["']hls["']|\.m3u8\b)`)
	hanimeAlternateSplitPattern  = regexp.MustCompile(`\s*(?:\r?\n|/|\|)\s*`)
	hanimeReleasedAtLayoutShort  = "Jan 2, 2006"
	hanimeReleasedAtLayoutLong   = "January 2, 2006"
)

func ParseCatalogHTML(raw []byte, pageURL string) ([]CatalogItem, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse catalog html: %w", err)
	}

	sourceDomain := hostFromURL(pageURL)
	catalogSource := pathFromURL(pageURL)
	seen := make(map[string]struct{})
	items := make([]CatalogItem, 0, 24)

	doc.Find(`a.no-touch[href^="/videos/hentai/"]`).Each(func(_ int, sel *goquery.Selection) {
		href, ok := sel.Attr("href")
		if !ok {
			return
		}
		canonicalURL := resolveURL(pageURL, href)
		slug := slugFromURL(canonicalURL)
		if slug == "" {
			return
		}
		if _, ok := seen[slug]; ok {
			return
		}
		title := normalizeSpace(firstNonEmpty(
			strings.TrimSpace(sel.AttrOr("alt", "")),
			sel.Find(".hv-title").First().Text(),
		))
		if title == "" {
			return
		}
		seen[slug] = struct{}{}
		item := HydrateDerivedMetadata(CatalogItem{
			SourceDomain:  sourceDomain,
			CatalogSource: catalogSource,
			Title:         title,
			CanonicalURL:  canonicalURL,
			Slug:          slug,
		})
		items = append(items, item)
	})

	if len(items) == 0 {
		return nil, nil
	}
	promoteCatalogSeriesCandidates(items)
	return items, nil
}

func ParseDetailHTML(raw []byte, pageURL string) (DetailMetadata, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return DetailMetadata{}, fmt.Errorf("parse detail html: %w", err)
	}

	title := normalizeSpace(doc.Find("h1.tv-title").First().Text())
	if title == "" {
		return DetailMetadata{}, fmt.Errorf("parse detail html: missing title")
	}

	coverURL := resolveURL(pageURL, strings.TrimSpace(doc.Find("img.hvpi-cover[src]").First().AttrOr("src", "")))
	description := normalizeSpace(doc.Find(".hvpist-description").First().Text())
	brand, brandSlug := findInfoBlockLink(doc, "Brand")
	alternateTitles := splitAlternateTitles(findInfoBlockText(doc, "Alternate Titles"))
	downloadPath := strings.TrimSpace(doc.Find(`.htv-video-page-action-bar a[href^="/downloads/"]`).First().AttrOr("href", ""))
	releasedAt := parseReleasedAt(findInfoBlockText(doc, "Released"))

	tags := make([]string, 0, 8)
	doc.Find(`.hvpi-summary a[href^="/browse/tags/"]`).Each(func(_ int, sel *goquery.Selection) {
		if tag := normalizeSpace(sel.Text()); tag != "" {
			tags = append(tags, tag)
		}
	})

	return DetailMetadata{
		Title:              title,
		CoverURL:           coverURL,
		Description:        description,
		DescriptionExcerpt: truncateExcerpt(description, 220),
		Tags:               normalizeList(tags),
		Brand:              brand,
		BrandSlug:          brandSlug,
		AlternateTitles:    alternateTitles,
		DownloadPresent:    downloadPath != "",
		ManifestPresent:    hanimeManifestPattern.Match(raw),
		ReleasedAt:         releasedAt,
	}, nil
}

func HydrateDerivedMetadata(item CatalogItem) CatalogItem {
	meta := deriveTitleMetadata(item.Title)
	item.NormalizedTitle = meta.NormalizedTitle
	item.EntryKind = meta.EntryKind
	item.EpisodeNumber = meta.EpisodeNumber
	item.SeriesCandidate = meta.SeriesCandidate
	return item
}

type titleMetadata struct {
	NormalizedTitle string
	EntryKind       string
	EpisodeNumber   int
	SeriesCandidate bool
	ExplicitEpisode bool
}

func deriveTitleMetadata(rawTitle string) titleMetadata {
	title := normalizeSpace(rawTitle)
	meta := titleMetadata{
		NormalizedTitle: title,
		EntryKind:       "standalone",
	}
	match := hanimeExplicitEpisodePattern.FindStringSubmatch(title)
	if len(match) >= 3 {
		baseTitle := normalizeSpace(strings.Trim(match[1], "-: "))
		var episode int
		fmt.Sscanf(match[2], "%d", &episode)
		if baseTitle != "" && episode > 0 {
			meta.NormalizedTitle = baseTitle
			meta.EntryKind = "episode"
			meta.EpisodeNumber = episode
			meta.SeriesCandidate = true
			meta.ExplicitEpisode = true
		}
		return meta
	}
	match = hanimeTrailingNumberPattern.FindStringSubmatch(title)
	if len(match) >= 3 {
		baseTitle := normalizeSpace(strings.Trim(match[1], "-: "))
		var episode int
		fmt.Sscanf(match[2], "%d", &episode)
		if baseTitle != "" && episode > 0 {
			meta.NormalizedTitle = baseTitle
			meta.EntryKind = "numbered"
			meta.EpisodeNumber = episode
		}
	}
	return meta
}

func promoteCatalogSeriesCandidates(items []CatalogItem) {
	counts := make(map[string]int)
	for _, item := range items {
		if item.EntryKind != "numbered" || item.NormalizedTitle == "" {
			continue
		}
		counts[item.NormalizedTitle]++
	}
	for i := range items {
		if items[i].SeriesCandidate {
			continue
		}
		if items[i].EntryKind != "numbered" || items[i].EpisodeNumber <= 0 {
			continue
		}
		if items[i].EpisodeNumber > 1 || counts[items[i].NormalizedTitle] > 1 {
			items[i].EntryKind = "episode"
			items[i].SeriesCandidate = true
		}
	}
}

func findInfoBlockText(doc *goquery.Document, header string) string {
	var value string
	doc.Find(".hvpimbc-item").EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		currentHeader := normalizeSpace(sel.Find(".hvpimbc-header").First().Text())
		if !strings.EqualFold(currentHeader, header) {
			return true
		}
		value = normalizeSpace(sel.Find(".hvpimbc-text").First().Text())
		if value == "" {
			value = normalizeSpace(sel.Text())
			value = strings.TrimPrefix(value, currentHeader)
			value = normalizeSpace(value)
		}
		return false
	})
	return value
}

func findInfoBlockLink(doc *goquery.Document, header string) (string, string) {
	var text string
	var slug string
	doc.Find(".hvpimbc-item").EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		currentHeader := normalizeSpace(sel.Find(".hvpimbc-header").First().Text())
		if !strings.EqualFold(currentHeader, header) {
			return true
		}
		link := sel.Find("a").First()
		text = normalizeSpace(link.Text())
		slug = slugFromURL(link.AttrOr("href", ""))
		if text == "" {
			text = normalizeSpace(sel.Find(".hvpimbc-text").First().Text())
		}
		return false
	})
	return text, slug
}

func splitAlternateTitles(raw string) []string {
	raw = normalizeSpace(raw)
	if raw == "" {
		return nil
	}
	parts := hanimeAlternateSplitPattern.Split(raw, -1)
	return normalizeList(parts)
}

func parseReleasedAt(raw string) time.Time {
	raw = normalizeSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{hanimeReleasedAtLayoutShort, hanimeReleasedAtLayoutLong, "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func truncateExcerpt(raw string, limit int) string {
	raw = normalizeSpace(raw)
	runes := []rune(raw)
	if raw == "" || limit <= 0 || len(runes) <= limit {
		return raw
	}
	trimmed := strings.TrimSpace(string(runes[:limit]))
	if idx := strings.LastIndex(trimmed, " "); idx > len([]rune(trimmed))/2 {
		trimmed = trimmed[:idx]
	}
	return trimmed + "..."
}

func normalizeList(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeSpace(value)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeSpace(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return hanimeWhitespacePattern.ReplaceAllString(value, " ")
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Host)
}

func pathFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	path := strings.TrimSpace(parsed.Path)
	if path == "" {
		return "/"
	}
	return path
}

func resolveURL(baseRaw, refRaw string) string {
	refRaw = strings.TrimSpace(refRaw)
	if refRaw == "" {
		return ""
	}
	ref, err := url.Parse(refRaw)
	if err != nil {
		return refRaw
	}
	base, err := url.Parse(strings.TrimSpace(baseRaw))
	if err != nil {
		return refRaw
	}
	return base.ResolveReference(ref).String()
}

func slugFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	trimmed := strings.Trim(parsed.Path, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	return strings.TrimSpace(parts[len(parts)-1])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
