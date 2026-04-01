package mangasusuku

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

var readerRunPattern = regexp.MustCompile(`(?s)ts_reader\.run\((\{.*?\})\);`)

type readerPayload struct {
	PrevURL string `json:"prevUrl"`
	NextURL string `json:"nextUrl"`
	Sources []struct {
		Source string   `json:"source"`
		Images []string `json:"images"`
	} `json:"sources"`
}

func ParseCatalogHTML(raw []byte, sourceURL string) ([]content.ManhwaSeries, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse catalog html: %w", err)
	}

	items := make([]content.ManhwaSeries, 0)
	doc.Find(".bsx").Each(func(_ int, selection *goquery.Selection) {
		link := selection.Find("a[href]").First()
		href, ok := link.Attr("href")
		if !ok {
			return
		}
		canonical := strings.TrimSpace(href)
		title := normalizeSpace(selection.Find(".tt").First().Text())
		if title == "" {
			title = normalizeSpace(link.AttrOr("title", ""))
		}
		if title == "" {
			return
		}

		label := normalizeSpace(selection.Find(".epxs").First().Text())
		text := normalizeSpace(selection.Text())
		status := ""
		lowerText := strings.ToLower(text)
		if strings.Contains(lowerText, "completed") || strings.Contains(strings.ToLower(label), "end") {
			status = "Completed"
		}

		items = append(items, content.ManhwaSeries{
			Source:       "mangasusuku",
			Slug:         slugFromURL(canonical),
			Title:        title,
			CanonicalURL: canonical,
			CoverURL:     extractImageURL(selection),
			Status:       status,
			LatestChapter: buildChapterRef(
				canonical,
				label,
				"",
			),
		})
	})

	return items, nil
}

func ParseSeriesHTML(raw []byte, canonicalURL string) (content.ManhwaSeries, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return content.ManhwaSeries{}, fmt.Errorf("parse series html: %w", err)
	}

	info := parseInfoMap(doc)
	series := content.ManhwaSeries{
		Source:       "mangasusuku",
		MediaType:    normalizeMediaType(info["type"]),
		Slug:         slugFromURL(canonicalURL),
		Title:        normalizeSpace(doc.Find(".entry-title").First().Text()),
		CanonicalURL: strings.TrimSpace(canonicalURL),
		CoverURL:     extractImageURL(doc.Find(".thumb").First()),
		Status:       info["status"],
		Type:         info["type"],
		ReleasedYear: info["released"],
		Author:       firstNonEmpty(info["author"], info["artist"]),
		Synopsis:     normalizeSpace(doc.Find(".entry-content").First().Text()),
	}

	doc.Find(".seriestugenre a").Each(func(_ int, genre *goquery.Selection) {
		value := normalizeSpace(genre.Text())
		if value != "" {
			series.Genres = append(series.Genres, value)
		}
	})

	doc.Find(".bxcl ul li").Each(func(_ int, item *goquery.Selection) {
		link := item.Find("a[href]").First()
		href, ok := link.Attr("href")
		if !ok {
			return
		}
		label := normalizeSpace(link.Text())
		if label == "" {
			return
		}
		canonical := strings.TrimSpace(href)
		series.Chapters = append(series.Chapters, content.ManhwaChapterRef{
			Slug:         slugFromURL(canonical),
			Title:        normalizeSpace(series.Title + " " + label),
			Label:        label,
			Number:       chapterNumberFromLabel(label),
			CanonicalURL: canonical,
			PublishedAt:  normalizeSpace(item.Find("span, time, .eph-date, .chapterdate").First().Text()),
		})
	})
	if len(series.Chapters) > 0 {
		latest := series.Chapters[0]
		series.LatestChapter = &latest
	}

	return series, nil
}

func ParseChapterHTML(raw []byte, canonicalURL string) (content.ManhwaChapter, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return content.ManhwaChapter{}, fmt.Errorf("parse chapter html: %w", err)
	}

	chapter := content.ManhwaChapter{
		Source:       "mangasusuku",
		Slug:         slugFromURL(canonicalURL),
		CanonicalURL: strings.TrimSpace(canonicalURL),
		Title:        normalizeSpace(doc.Find(".entry-title").First().Text()),
	}
	chapter.Label = extractChapterLabel(chapter.Title)
	chapter.Number = chapterNumberFromLabel(chapter.Label)

	if href, ok := doc.Find(".allc a[href]").First().Attr("href"); ok {
		seriesURL := strings.TrimSpace(href)
		chapter.SeriesSlug = slugFromURL(seriesURL)
		chapter.SeriesTitle = normalizeSpace(doc.Find(".allc a").First().Text())
	}

	if payload, ok := extractReaderPayload(raw); ok {
		chapter.PrevURL = normalizeNavURL(payload.PrevURL)
		chapter.NextURL = normalizeNavURL(payload.NextURL)
		for _, source := range payload.Sources {
			for _, imageURL := range source.Images {
				imageURL = strings.TrimSpace(imageURL)
				if imageURL == "" {
					continue
				}
				chapter.Pages = append(chapter.Pages, content.PageAsset{
					Position: len(chapter.Pages) + 1,
					URL:      imageURL,
				})
			}
			if len(chapter.Pages) > 0 {
				break
			}
		}
	}

	if len(chapter.Pages) == 0 {
		doc.Find("#readerarea img").Each(func(_ int, img *goquery.Selection) {
			imageURL := strings.TrimSpace(img.AttrOr("data-src", img.AttrOr("src", "")))
			if imageURL == "" || strings.HasPrefix(imageURL, "data:image/svg+xml") {
				return
			}
			chapter.Pages = append(chapter.Pages, content.PageAsset{
				Position: len(chapter.Pages) + 1,
				URL:      imageURL,
			})
		})
	}

	return chapter, nil
}

func parseInfoMap(doc *goquery.Document) map[string]string {
	info := make(map[string]string)
	doc.Find(".infotable tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 2 {
			return
		}
		key := normalizeSpace(cells.First().Text())
		value := normalizeSpace(cells.Eq(1).Text())
		if key == "" || value == "" {
			return
		}
		info[strings.ToLower(key)] = value
	})
	return info
}

func extractReaderPayload(raw []byte) (readerPayload, bool) {
	matches := readerRunPattern.FindSubmatch(raw)
	if len(matches) < 2 {
		return readerPayload{}, false
	}
	var payload readerPayload
	if err := json.Unmarshal(matches[1], &payload); err != nil {
		return readerPayload{}, false
	}
	return payload, true
}

func normalizeSpace(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func normalizeMediaType(raw string) string {
	switch strings.ToLower(normalizeSpace(raw)) {
	case "manhwa":
		return "manhwa"
	case "manhua":
		return "manhua"
	case "manga":
		return "manga"
	default:
		return ""
	}
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

func chapterNumberFromLabel(label string) string {
	label = normalizeSpace(label)
	lower := strings.ToLower(label)
	if strings.HasPrefix(lower, "chapter ") {
		return strings.TrimSpace(label[len("Chapter "):])
	}
	if strings.HasPrefix(lower, "chapter") {
		return strings.TrimSpace(label[len("Chapter"):])
	}
	return label
}

func extractChapterLabel(title string) string {
	title = normalizeSpace(title)
	lower := strings.ToLower(title)
	idx := strings.Index(lower, "chapter")
	if idx < 0 {
		return title
	}
	return normalizeSpace(title[idx:])
}

func normalizeNavURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "#/") {
		return ""
	}
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = normalizeSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func buildChapterRef(canonical, label, publishedAt string) *content.ManhwaChapterRef {
	label = normalizeSpace(label)
	if label == "" {
		return nil
	}
	return &content.ManhwaChapterRef{
		Slug:         slugFromURL(canonical),
		Title:        label,
		Label:        label,
		Number:       chapterNumberFromLabel(label),
		CanonicalURL: strings.TrimSpace(canonical),
		PublishedAt:  normalizeSpace(publishedAt),
	}
}
