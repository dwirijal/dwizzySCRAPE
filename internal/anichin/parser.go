package anichin

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var numberPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)

func ParseCatalogHTML(raw []byte, sourceURL, section string) ([]CatalogItem, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse catalog html: %w", err)
	}

	sourceDomain := extractDomain(sourceURL)
	items := make([]CatalogItem, 0)
	doc.Find(".bsx").Each(func(_ int, selection *goquery.Selection) {
		link := selection.Find("a[href]").First()
		href, ok := link.Attr("href")
		if !ok {
			return
		}
		title := normalizeSpace(selection.Find(".tt").First().Text())
		if title == "" {
			title = normalizeSpace(link.AttrOr("title", ""))
		}
		if title == "" {
			title = normalizeSpace(selection.Find("img").First().AttrOr("alt", ""))
		}
		if title == "" {
			return
		}

		item := CatalogItem{
			SourceDomain: sourceDomain,
			Section:      strings.TrimSpace(section),
			Title:        title,
			CanonicalURL: strings.TrimSpace(href),
			Slug:         slugFromURL(href),
			PosterURL:    extractImageURL(selection),
			AnimeType:    normalizeSpace(selection.Find(".typez").First().Text()),
			Status:       normalizeSpace(selection.Find(".status").First().Text()),
			ScrapedAt:    time.Now().UTC(),
		}
		if item.Status == "" {
			item.Status = titleCase(section)
		}
		if item.Slug != "" {
			items = append(items, item)
		}
	})

	return items, nil
}

func ParseSeriesHTML(raw []byte, canonicalURL string) (AnimeDetail, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return AnimeDetail{}, fmt.Errorf("parse series html: %w", err)
	}

	info := parseInfoMap(doc)
	detail := AnimeDetail{
		Slug:         slugFromURL(canonicalURL),
		CanonicalURL: strings.TrimSpace(canonicalURL),
		Title:        normalizeSpace(doc.Find(".entry-title").First().Text()),
		AltTitle:     normalizeSpace(doc.Find(".alter").First().Text()),
		PosterURL:    extractImageURL(doc.Find(".thumb").First()),
		Synopsis:     normalizeSpace(doc.Find(".bixbox.synp .entry-content").First().Text()),
		Status:       info["status"],
		AnimeType:    firstNonBlank(info["type"], "Donghua"),
		Season:       info["season"],
		ReleasedYear: info["released"],
		Network:      info["network"],
		ScrapedAt:    time.Now().UTC(),
	}
	if detail.Synopsis == "" {
		detail.Synopsis = normalizeSpace(doc.Find(".entry-content").First().Text())
	}
	if studio := strings.TrimSpace(info["studio"]); studio != "" {
		detail.StudioNames = []string{studio}
	}
	doc.Find(".genxed a").Each(func(_ int, item *goquery.Selection) {
		value := normalizeSpace(item.Text())
		if value != "" {
			detail.GenreNames = append(detail.GenreNames, value)
		}
	})

	sourceMeta := map[string]any{}
	for _, key := range []string{"network", "released", "season", "duration", "episodes"} {
		if value := strings.TrimSpace(info[key]); value != "" {
			sourceMeta[key] = value
		}
	}
	if len(sourceMeta) > 0 {
		detail.SourceMetaJSON, _ = json.Marshal(sourceMeta)
	}

	doc.Find(".eplister li").Each(func(_ int, item *goquery.Selection) {
		link := item.Find("a[href]").First()
		href, ok := link.Attr("href")
		if !ok {
			return
		}
		label := normalizeSpace(item.Find(".epl-title").First().Text())
		number := normalizeSpace(item.Find(".epl-num").First().Text())
		ref := EpisodeRef{
			CanonicalURL: strings.TrimSpace(href),
			Slug:         slugFromURL(href),
			Title:        firstNonBlank(label, normalizeSpace(link.Text())),
			Number:       parseEpisodeNumberText(number),
			ReleaseLabel: normalizeSpace(item.Find(".epl-date").First().Text()),
		}
		if ref.Slug != "" {
			detail.EpisodeRefs = append(detail.EpisodeRefs, ref)
		}
	})

	if detail.Title == "" {
		return AnimeDetail{}, fmt.Errorf("series page missing title")
	}
	return detail, nil
}

func ParseEpisodeHTML(raw []byte, canonicalURL string) (EpisodeDetail, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return EpisodeDetail{}, fmt.Errorf("parse episode html: %w", err)
	}

	detail := EpisodeDetail{
		EpisodeSlug:   slugFromURL(canonicalURL),
		CanonicalURL:  strings.TrimSpace(canonicalURL),
		Title:         normalizeSpace(doc.Find(".entry-title").First().Text()),
		EpisodeNumber: parseEpisodeNumberFloat(doc.Find(".entry-title").First().Text(), canonicalURL),
		StreamMirrors: make(map[string]string),
		DownloadLinks: make(map[string]map[string]string),
		ScrapedAt:     time.Now().UTC(),
	}
	if iframe, ok := doc.Find("#embed_holder iframe[src], .player-embed iframe[src], iframe[src]").First().Attr("src"); ok {
		detail.StreamURL = strings.TrimSpace(iframe)
	}

	doc.Find("select.mirror option[value]").Each(func(_ int, option *goquery.Selection) {
		label := normalizeSpace(option.Text())
		value, ok := option.Attr("value")
		if !ok || strings.TrimSpace(value) == "" || label == "" || strings.EqualFold(label, "Select Video Server") {
			return
		}
		if decoded := decodeMirrorValue(value); decoded != "" {
			detail.StreamMirrors[label] = decoded
		}
	})

	doc.Find(".soraurlx").Each(func(_ int, group *goquery.Selection) {
		quality := normalizeSpace(group.Find("strong").First().Text())
		if quality == "" {
			return
		}
		if _, ok := detail.DownloadLinks[quality]; !ok {
			detail.DownloadLinks[quality] = make(map[string]string)
		}
		group.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
			label := normalizeSpace(link.Text())
			href, ok := link.Attr("href")
			if !ok || label == "" {
				return
			}
			detail.DownloadLinks[quality][label] = strings.TrimSpace(href)
		})
	})

	if detail.Title == "" {
		return EpisodeDetail{}, fmt.Errorf("episode page missing title")
	}
	return detail, nil
}

func normalizeSpace(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func slugFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func extractImageURL(selection *goquery.Selection) string {
	if selection == nil {
		return ""
	}
	img := selection.Find("img").First()
	for _, attr := range []string{"data-src", "data-lazy-src", "src"} {
		if value, ok := img.Attr(attr); ok {
			value = strings.TrimSpace(value)
			if value != "" && !strings.HasPrefix(value, "data:image/svg+xml") {
				return value
			}
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

func parseInfoMap(doc *goquery.Document) map[string]string {
	info := make(map[string]string)
	doc.Find(".info-content .spe span").Each(func(_ int, item *goquery.Selection) {
		label := normalizeSpace(item.Find("b").First().Text())
		value := normalizeSpace(strings.TrimPrefix(item.Text(), item.Find("b").First().Text()))
		label = strings.ToLower(strings.TrimSuffix(label, ":"))
		if label == "" || value == "" {
			return
		}
		info[label] = value
	})
	return info
}

func parseEpisodeNumberText(raw string) string {
	match := numberPattern.FindString(normalizeSpace(raw))
	return strings.TrimSpace(match)
}

func parseEpisodeNumberFloat(values ...string) float64 {
	for _, value := range values {
		match := numberPattern.FindString(normalizeSpace(value))
		if match == "" {
			continue
		}
		parsed, err := strconv.ParseFloat(match, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func decodeMirrorValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	match := regexp.MustCompile(`src="([^"]+)"`).FindSubmatch(decoded)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(string(match[1]))
}

func titleCase(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.ToUpper(raw[:1]) + strings.ToLower(raw[1:])
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
