package manhwaindo

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

		title := normalizeSpace(selection.Find(".tt").First().Text())
		if title == "" {
			title = normalizeSpace(link.AttrOr("title", ""))
		}
		if title == "" {
			return
		}

		latestLabel := normalizeSpace(selection.Find(".epxs").First().Text())
		items = append(items, content.ManhwaSeries{
			Source:       "manhwaindo",
			MediaType:    normalizeMediaType(selection.Find(".typename").First().Text()),
			Slug:         slugFromURL(href),
			Title:        title,
			CanonicalURL: strings.TrimSpace(href),
			CoverURL:     extractImageURL(selection),
			LatestChapter: buildChapterRef(
				strings.TrimSpace(href),
				latestLabel,
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

	series := content.ManhwaSeries{
		Source:       "manhwaindo",
		MediaType:    "manhwa",
		Slug:         slugFromURL(canonicalURL),
		CanonicalURL: strings.TrimSpace(canonicalURL),
		Title:        normalizeSpace(doc.Find(".entry-title").First().Text()),
		AltTitle:     normalizeSpace(doc.Find(".alternative").First().Text()),
		CoverURL:     extractImageURL(doc.Find(".thumb").First()),
		Synopsis:     normalizeSpace(doc.Find(".entry-content-single").First().Text()),
	}

	info := parseInfoMap(doc)
	if status := info["status"]; status != "" {
		series.Status = status
	}
	if mediaType := normalizeMediaType(info["type"]); mediaType != "" {
		series.MediaType = mediaType
		series.Type = info["type"]
	}
	series.ReleasedYear = info["released"]
	series.Author = info["author"]

	doc.Find(".mgen a").Each(func(_ int, genre *goquery.Selection) {
		value := normalizeSpace(genre.Text())
		if value != "" {
			series.Genres = append(series.Genres, value)
		}
	})

	doc.Find(".eplister li").Each(func(_ int, item *goquery.Selection) {
		link := item.Find("a[href]").First()
		href, ok := link.Attr("href")
		if !ok {
			return
		}
		label := normalizeSpace(item.Find(".chapternum").First().Text())
		if label == "" {
			label = normalizeSpace(link.Text())
		}
		series.Chapters = append(series.Chapters, content.ManhwaChapterRef{
			Slug:         slugFromURL(href),
			Title:        normalizeSpace(series.Title + " " + label),
			Label:        label,
			Number:       chapterNumberFromLabel(label),
			CanonicalURL: strings.TrimSpace(href),
			PublishedAt:  normalizeSpace(item.Find(".chapterdate").First().Text()),
		})
	})

	if latestHref, ok := doc.Find(".epcurlast").First().Parent().Attr("href"); ok {
		series.LatestChapter = buildChapterRef(
			strings.TrimSpace(latestHref),
			normalizeSpace(doc.Find(".epcurlast").First().Text()),
			normalizeSpace(doc.Find(".epcurlast").First().Closest("a").Find(".chapterdate").First().Text()),
		)
	}
	if series.LatestChapter == nil && len(series.Chapters) > 0 {
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
		Source:       "manhwaindo",
		Slug:         slugFromURL(canonicalURL),
		CanonicalURL: strings.TrimSpace(canonicalURL),
		Title:        normalizeSpace(doc.Find(".entry-title").First().Text()),
	}
	chapter.Label = extractChapterLabel(chapter.Title)
	chapter.Number = chapterNumberFromLabel(chapter.Label)

	if href, ok := doc.Find(".allc a[href]").First().Attr("href"); ok {
		chapter.SeriesSlug = slugFromURL(href)
		chapter.SeriesTitle = normalizeSpace(doc.Find(".allc a").First().Text())
	}

	payload, ok := extractReaderPayload(raw)
	if ok {
		chapter.PrevURL = strings.TrimSpace(payload.PrevURL)
		chapter.NextURL = strings.TrimSpace(payload.NextURL)
		if len(payload.Sources) > 0 {
			for idx, imageURL := range payload.Sources[0].Images {
				if strings.TrimSpace(imageURL) == "" {
					continue
				}
				chapter.Pages = append(chapter.Pages, content.PageAsset{
					Position: idx + 1,
					URL:      strings.TrimSpace(imageURL),
				})
			}
		}
	}

	if len(chapter.Pages) == 0 {
		doc.Find("#readerarea img").Each(func(index int, img *goquery.Selection) {
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
		return "manhwa"
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
	label = normalizeSpace(strings.TrimPrefix(strings.TrimSpace(label), "Chapter"))
	return strings.TrimSpace(label)
}

func buildChapterRef(href, label, publishedAt string) *content.ManhwaChapterRef {
	if strings.TrimSpace(href) == "" && strings.TrimSpace(label) == "" {
		return nil
	}
	return &content.ManhwaChapterRef{
		Slug:         slugFromURL(href),
		Label:        normalizeSpace(label),
		Number:       chapterNumberFromLabel(label),
		CanonicalURL: strings.TrimSpace(href),
		PublishedAt:  strings.TrimSpace(publishedAt),
	}
}

func parseInfoMap(doc *goquery.Document) map[string]string {
	info := make(map[string]string)
	doc.Find(".tsinfo .imptdt").Each(func(_ int, item *goquery.Selection) {
		text := normalizeSpace(item.Text())
		for _, key := range []string{"Status", "Type", "Released", "Author"} {
			if strings.HasPrefix(strings.ToLower(text), strings.ToLower(key+" ")) {
				info[strings.ToLower(key)] = strings.TrimSpace(strings.TrimPrefix(text, key))
				return
			}
		}
	})
	return info
}

func extractChapterLabel(title string) string {
	title = normalizeSpace(title)
	index := strings.LastIndex(strings.ToLower(title), "chapter ")
	if index < 0 {
		return title
	}
	return normalizeSpace(title[index:])
}

func extractReaderPayload(raw []byte) (readerPayload, bool) {
	match := readerRunPattern.FindSubmatch(raw)
	if len(match) < 2 {
		return readerPayload{}, false
	}

	var payload readerPayload
	if err := json.Unmarshal(match[1], &payload); err != nil {
		return readerPayload{}, false
	}
	return payload, true
}
