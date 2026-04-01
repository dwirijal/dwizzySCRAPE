package kusonime

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func ParseSearchHTML(raw []byte) ([]SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse search html: %w", err)
	}

	results := make([]SearchResult, 0)
	seen := make(map[string]struct{})
	doc.Find(".kover .detpost").Each(func(_ int, sel *goquery.Selection) {
		link := sel.Find("h2.episodeye a[href]").First()
		if link.Length() == 0 {
			link = sel.Find(".thumb a[href]").First()
		}
		href := strings.TrimSpace(link.AttrOr("href", ""))
		if href == "" {
			return
		}
		if _, ok := seen[href]; ok {
			return
		}
		seen[href] = struct{}{}

		result := SearchResult{
			Title:     cleanAnimeTitle(firstNonEmpty(normalizeSpace(link.Text()), normalizeSpace(link.AttrOr("title", "")))),
			URL:       href,
			PosterURL: extractImageURL(sel),
		}
		sel.Find(".content a[rel='tag']").Each(func(_ int, genre *goquery.Selection) {
			value := normalizeSpace(genre.Text())
			if value != "" {
				result.Genres = append(result.Genres, value)
			}
		})
		if result.Title != "" {
			results = append(results, result)
		}
	})
	return results, nil
}

func ParseAnimeHTML(raw []byte) (AnimePage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return AnimePage{}, fmt.Errorf("parse anime html: %w", err)
	}

	page := AnimePage{
		Title:       cleanAnimeTitle(firstNonEmpty(doc.Find("h1.jdlz").First().Text(), doc.Find("title").First().Text())),
		URL:         strings.TrimSpace(doc.Find(`link[rel="canonical"]`).First().AttrOr("href", "")),
		PosterURL:   extractImageURL(doc.Find(".post-thumb").First()),
		PublishedAt: strings.TrimSpace(doc.Find(`meta[property="article:published_time"]`).First().AttrOr("content", "")),
		ModifiedAt:  strings.TrimSpace(doc.Find(`meta[property="article:modified_time"]`).First().AttrOr("content", "")),
	}

	doc.Find(".lexot .info p").Each(func(_ int, item *goquery.Selection) {
		label := normalizeLabel(item.Find("b").First().Text())
		value := cleanInfoValue(strings.TrimPrefix(item.Text(), item.Find("b").First().Text()))
		switch label {
		case "japanese":
			page.JapaneseTitle = value
		case "genre":
			page.Genres = collectTexts(item.Find("a"))
		case "seasons":
			page.Season = firstNonEmpty(value, normalizeSpace(item.Find("a").First().Text()))
		case "producers":
			page.Producers = splitCSV(value)
		case "type":
			page.BatchType = value
		case "status":
			page.Status = value
		case "total episode":
			page.TotalEpisodes = value
		case "score":
			page.Score = parseFloat(value)
		case "duration":
			page.Duration = value
		case "released on":
			page.ReleasedOn = value
		}
	})

	synopsisParts := make([]string, 0)
	doc.Find(".lexot").First().ChildrenFiltered("p").Each(func(_ int, item *goquery.Selection) {
		text := normalizeSpace(item.Text())
		if text == "" {
			return
		}
		lower := strings.ToLower(text)
		switch {
		case strings.HasPrefix(lower, "subtitle :"),
			strings.HasPrefix(lower, "lirik "),
			strings.HasPrefix(lower, "retiming "),
			strings.HasPrefix(lower, "sumber video"),
			strings.HasPrefix(lower, "download "):
			return
		default:
			synopsisParts = append(synopsisParts, text)
		}
	})
	page.Synopsis = strings.Join(synopsisParts, "\n\n")

	doc.Find(".dlbodz .smokeddlrh").Each(func(_ int, block *goquery.Selection) {
		group := BatchLinkGroup{
			Label:     normalizeSpace(block.Find(".smokettlrh").First().Text()),
			Downloads: make(map[string]map[string]string),
		}
		block.Find(".smokeurlrh").Each(func(_ int, row *goquery.Selection) {
			quality := normalizeSpace(strings.TrimSuffix(row.Find("strong").First().Text(), ":"))
			if quality == "" {
				quality = inferQualityLabel(row)
			}
			if quality == "" {
				quality = "unknown"
			}
			if _, ok := group.Downloads[quality]; !ok {
				group.Downloads[quality] = make(map[string]string)
			}
			row.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
				href := strings.TrimSpace(link.AttrOr("href", ""))
				host := normalizeSpace(link.Text())
				if href == "" || host == "" {
					return
				}
				group.Downloads[quality][host] = href
			})
		})
		if group.Label != "" && len(group.Downloads) > 0 {
			page.Batches = append(page.Batches, group)
		}
	})

	if page.Title == "" {
		return AnimePage{}, fmt.Errorf("anime page missing title")
	}
	return page, nil
}

func cleanAnimeTitle(value string) string {
	value = normalizeSpace(value)
	value = strings.TrimSuffix(value, "| Kusonime")
	replacements := []string{
		"Batch Subtitle Indonesia",
		"Subtitle Indonesia",
		"Sub Indo",
		"Batch",
		"BD",
	}
	for _, replacement := range replacements {
		value = strings.ReplaceAll(value, replacement, "")
	}
	return normalizeSpace(value)
}

func normalizeSpace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normalizeLabel(value string) string {
	value = strings.ToLower(normalizeSpace(value))
	value = strings.TrimSuffix(value, ":")
	return strings.TrimSpace(value)
}

func cleanInfoValue(value string) string {
	value = normalizeSpace(value)
	value = strings.TrimLeft(value, ": ")
	return normalizeSpace(value)
}

func extractImageURL(selection *goquery.Selection) string {
	if selection == nil {
		return ""
	}
	img := selection.Find("img").First()
	for _, attr := range []string{"data-src", "data-lazy-src", "src"} {
		value := strings.TrimSpace(img.AttrOr(attr, ""))
		if value != "" && !strings.HasPrefix(value, "data:image/svg+xml") {
			return value
		}
	}
	return ""
}

func collectTexts(selection *goquery.Selection) []string {
	values := make([]string, 0)
	seen := make(map[string]struct{})
	selection.Each(func(_ int, item *goquery.Selection) {
		value := normalizeSpace(item.Text())
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	})
	return values
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = normalizeSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseFloat(value string) float64 {
	value = normalizeSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func inferQualityLabel(row *goquery.Selection) string {
	text := normalizeSpace(row.Text())
	row.Find("a").Each(func(_ int, link *goquery.Selection) {
		text = normalizeSpace(strings.ReplaceAll(text, normalizeSpace(link.Text()), ""))
	})
	text = strings.Trim(text, "|:- ")
	return normalizeSpace(text)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := normalizeSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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
