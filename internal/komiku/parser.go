package komiku

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

func ParseCatalogHTML(raw []byte, sourceURL string) ([]content.ManhwaSeries, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse catalog html: %w", err)
	}

	items := make([]content.ManhwaSeries, 0)
	doc.Find("article.manga-card").Each(func(_ int, selection *goquery.Selection) {
		link := selection.Find("a[href]").First()
		href, ok := link.Attr("href")
		if !ok {
			return
		}
		canonical := resolveURL(sourceURL, href)
		if canonical == "" {
			return
		}

		title := normalizeSpace(selection.Find("h4 a").First().Text())
		if title == "" {
			title = normalizeSpace(selection.Find("img").First().AttrOr("alt", ""))
		}
		if title == "" {
			return
		}

		meta := normalizeSpace(selection.Find("p.meta").First().Text())
		typeLabel, genreLabel, status := parseCatalogMeta(meta)
		series := content.ManhwaSeries{
			Source:       "komiku",
			MediaType:    normalizeMediaType(typeLabel),
			Slug:         slugFromURL(canonical),
			Title:        title,
			CanonicalURL: canonical,
			CoverURL:     extractImageURL(selection),
			Status:       status,
			Type:         typeLabel,
		}
		if genreLabel != "" {
			series.Genres = []string{genreLabel}
		}
		items = append(items, series)
	})

	return items, nil
}

func ParseSeriesHTML(raw []byte, canonicalURL string) (content.ManhwaSeries, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return content.ManhwaSeries{}, fmt.Errorf("parse series html: %w", err)
	}

	info := parseInfoTable(doc)
	series := content.ManhwaSeries{
		Source:       "komiku",
		Slug:         slugFromURL(canonicalURL),
		Title:        firstNonEmpty(info["judul komik"], normalizeSpace(doc.Find("h1").First().Text())),
		AltTitle:     info["judul indonesia"],
		CanonicalURL: strings.TrimSpace(canonicalURL),
		CoverURL:     extractImageURL(doc.Find(".ims").First()),
		Status:       info["status"],
		Type:         info["jenis komik"],
		MediaType:    normalizeMediaType(info["jenis komik"]),
		Author:       info["pengarang"],
		Synopsis:     normalizeSpace(doc.Find("p.desc").First().Text()),
	}

	doc.Find("ul.genre li.genre span").Each(func(_ int, g *goquery.Selection) {
		value := normalizeSpace(g.Text())
		if value != "" {
			series.Genres = append(series.Genres, value)
		}
	})

	doc.Find("#daftarChapter tr[itemprop='itemListElement']").Each(func(_ int, item *goquery.Selection) {
		link := item.Find("td.judulseries a[href]").First()
		href, ok := link.Attr("href")
		if !ok {
			return
		}
		canonical := resolveURL(canonicalURL, href)
		label := normalizeSpace(link.Find("span").First().Text())
		if label == "" {
			label = normalizeSpace(link.Text())
		}
		if label == "" {
			return
		}
		series.Chapters = append(series.Chapters, content.ManhwaChapterRef{
			Slug:         slugFromURL(canonical),
			Title:        normalizeSpace(series.Title + " " + label),
			Label:        label,
			Number:       chapterNumberFromLabel(label),
			CanonicalURL: canonical,
			PublishedAt:  normalizeSpace(item.Find("td.tanggalseries").First().Text()),
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
		Source:       "komiku",
		Slug:         slugFromURL(canonicalURL),
		CanonicalURL: strings.TrimSpace(canonicalURL),
		Title:        normalizeSpace(doc.Find("#Judul h1").First().Text()),
	}
	chapter.Label = extractChapterLabel(chapter.Title)
	chapter.Number = chapterNumberFromLabel(chapter.Label)

	if href, ok := doc.Find("#Description a[rel='tag'][href]").First().Attr("href"); ok {
		seriesURL := resolveURL(canonicalURL, href)
		chapter.SeriesSlug = slugFromURL(seriesURL)
		chapter.SeriesTitle = normalizeSpace(doc.Find("#Description a[rel='tag']").First().Text())
	}
	if chapter.SeriesSlug == "" {
		if href, ok := doc.Find(".toolbar a[aria-label='List'][href]").First().Attr("href"); ok {
			seriesURL := resolveURL(canonicalURL, href)
			chapter.SeriesSlug = slugFromURL(seriesURL)
		}
	}

	chapter.PrevURL = extractNavURL(doc, canonicalURL, []string{
		".toolbar a[aria-label='Prev']",
		"a.prev",
		"a.prevch",
	})
	chapter.NextURL = extractNavURL(doc, canonicalURL, []string{
		".toolbar a[aria-label='Next']",
		"a.next",
		"a.nextch",
	})
	if chapter.NextURL == "" {
		if rawNext, ok := doc.Find(".nextch").First().Attr("data"); ok {
			chapter.NextURL = strings.TrimSpace(rawNext)
		}
	}

	doc.Find("#Baca_Komik img").Each(func(_ int, img *goquery.Selection) {
		imageURL := strings.TrimSpace(img.AttrOr("data-src", img.AttrOr("src", "")))
		if imageURL == "" || !strings.HasPrefix(imageURL, "http") {
			return
		}
		if strings.Contains(imageURL, "/asset/img/") {
			return
		}
		chapter.Pages = append(chapter.Pages, content.PageAsset{
			Position: len(chapter.Pages) + 1,
			URL:      imageURL,
		})
	})

	return chapter, nil
}

func parseInfoTable(doc *goquery.Document) map[string]string {
	info := make(map[string]string)
	doc.Find("table.inftable tr").Each(func(_ int, row *goquery.Selection) {
		key := normalizeSpace(row.Find("td").First().Text())
		value := normalizeSpace(row.Find("td").Eq(1).Text())
		if key == "" || value == "" {
			return
		}
		info[strings.ToLower(key)] = value
	})
	return info
}

func parseCatalogMeta(raw string) (typeLabel, genreLabel, status string) {
	raw = normalizeSpace(raw)
	if raw == "" {
		return "", "", ""
	}
	parts := strings.Split(raw, "Status:")
	left := strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		status = normalizeSpace(parts[1])
	}
	metaParts := strings.Split(left, "•")
	if len(metaParts) > 0 {
		typeLabel = normalizeSpace(metaParts[0])
	}
	if len(metaParts) > 1 {
		genreLabel = normalizeSpace(metaParts[1])
	}
	return typeLabel, genreLabel, status
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

func chapterNumberFromLabel(label string) string {
	label = strings.ToLower(normalizeSpace(label))
	label = strings.TrimPrefix(label, "chapter ")
	label = strings.TrimPrefix(label, "chapter")
	label = strings.TrimPrefix(label, "ch. ")
	label = strings.TrimPrefix(label, "ch ")
	return strings.TrimSpace(label)
}

func normalizeMediaType(raw string) string {
	raw = strings.ToLower(normalizeSpace(raw))
	switch {
	case strings.Contains(raw, "manhwa"):
		return "manhwa"
	case strings.Contains(raw, "manhua"):
		return "manhua"
	case strings.Contains(raw, "manga"):
		return "manga"
	default:
		return "manga"
	}
}

func extractImageURL(selection *goquery.Selection) string {
	if selection == nil {
		return ""
	}
	img := selection.Find("img").First()
	for _, attr := range []string{"data-src", "data-lazy-src", "src"} {
		value := strings.TrimSpace(img.AttrOr(attr, ""))
		if value != "" && !strings.HasPrefix(value, "data:image") {
			return value
		}
	}
	return ""
}

func extractNavURL(doc *goquery.Document, baseURL string, selectors []string) string {
	for _, selector := range selectors {
		if href, ok := doc.Find(selector).First().Attr("href"); ok {
			return resolveURL(baseURL, href)
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

func resolveURL(baseURL, raw string) string {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return ""
	}
	ref, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(base.ResolveReference(ref).String())
}

func normalizeSpace(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
