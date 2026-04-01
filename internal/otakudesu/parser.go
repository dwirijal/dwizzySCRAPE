package otakudesu

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var episodePattern = regexp.MustCompile(`(?i)\bepisode\s+(\d+(?:\.\d+)?)\b`)

func ParseSearchHTML(raw []byte) ([]SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse search html: %w", err)
	}

	results := make([]SearchResult, 0)
	seen := make(map[string]struct{})
	doc.Find("ul.chivsrc li h2 a[href]").Each(func(_ int, sel *goquery.Selection) {
		href, _ := sel.Attr("href")
		href = strings.TrimSpace(href)
		if href == "" {
			return
		}
		if _, ok := seen[href]; ok {
			return
		}
		seen[href] = struct{}{}
		results = append(results, SearchResult{
			Title: cleanAnimeTitle(sel.Text()),
			URL:   href,
		})
	})
	return results, nil
}

func ParseAnimeHTML(raw []byte) (AnimePage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return AnimePage{}, fmt.Errorf("parse anime html: %w", err)
	}

	canonicalURL, _ := doc.Find(`link[rel="canonical"]`).First().Attr("href")
	title := cleanAnimeTitle(doc.Find("title").First().Text())

	episodes := make([]AnimeEpisodeRef, 0)
	doc.Find(".episodelist ul li a[href]").Each(func(_ int, sel *goquery.Selection) {
		href, _ := sel.Attr("href")
		href = strings.TrimSpace(href)
		if !strings.Contains(href, "/episode/") {
			return
		}
		label := normalizeSpace(sel.Text())
		number := extractEpisodeNumber(label)
		if number == "" {
			number = extractEpisodeNumber(href)
		}
		episodes = append(episodes, AnimeEpisodeRef{
			Title:  cleanEpisodeTitle(label),
			Number: number,
			URL:    href,
		})
	})

	return AnimePage{
		Title:    title,
		URL:      strings.TrimSpace(canonicalURL),
		Episodes: episodes,
	}, nil
}

func ParseEpisodeHTML(raw []byte, rawURL string) (EpisodePage, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return EpisodePage{}, fmt.Errorf("parse episode html: %w", err)
	}

	streamURL := parseFirstIframeSrc(doc)
	title := cleanEpisodeTitle(doc.Find("title").First().Text())
	number := extractEpisodeNumber(doc.Find("title").First().Text())
	if number == "" {
		number = extractEpisodeNumber(rawURL)
	}

	mirrorRequests := parseMirrorRequests(doc)
	downloadLinks, downloadURLs := parseDownloadLinks(doc)

	return EpisodePage{
		Title:          title,
		Number:         number,
		URL:            strings.TrimSpace(rawURL),
		StreamURL:      strings.TrimSpace(streamURL),
		StreamMirrors:  make(map[string]string),
		DownloadLinks:  downloadLinks,
		DownloadURLs:   downloadURLs,
		MirrorRequests: mirrorRequests,
	}, nil
}

func ParseEmbedHTML(raw []byte) string {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parseFirstIframeSrc(doc))
}

func cleanAnimeTitle(value string) string {
	value = normalizeSpace(value)
	value = strings.TrimSuffix(value, "| Otaku Desu")
	value = strings.ReplaceAll(value, "Subtitle Indonesia", "")
	value = strings.ReplaceAll(value, "Sub Indo", "")
	if idx := strings.Index(strings.ToLower(value), "(episode"); idx >= 0 {
		value = value[:idx]
	}
	return normalizeSpace(value)
}

func cleanEpisodeTitle(value string) string {
	value = normalizeSpace(value)
	value = strings.TrimSuffix(value, "| Otaku Desu")
	if match := episodePattern.FindStringIndex(value); match != nil {
		value = value[:match[0]]
	}
	value = strings.ReplaceAll(value, "Subtitle Indonesia", "")
	return normalizeSpace(value)
}

func extractEpisodeNumber(value string) string {
	matches := episodePattern.FindStringSubmatch(value)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func normalizeSpace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func parseFirstIframeSrc(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	iframe := doc.Find(".responsive-embed-stream iframe").First()
	if src, ok := iframe.Attr("src"); ok && strings.TrimSpace(src) != "" {
		return src
	}
	iframe = doc.Find("#pembed iframe").First()
	if src, ok := iframe.Attr("src"); ok && strings.TrimSpace(src) != "" {
		return src
	}
	iframe = doc.Find("iframe").First()
	if src, ok := iframe.Attr("src"); ok && strings.TrimSpace(src) != "" {
		return src
	}
	return ""
}

func parseMirrorRequests(doc *goquery.Document) []EpisodeMirrorRequest {
	requests := make([]EpisodeMirrorRequest, 0)
	seen := make(map[string]struct{})
	doc.Find(".mirrorstream a[data-content]").Each(func(_ int, sel *goquery.Selection) {
		payload, _ := sel.Attr("data-content")
		payload = strings.TrimSpace(payload)
		if payload == "" {
			return
		}
		label := normalizeSpace(sel.Text())
		if label == "" {
			label = "mirror"
		}
		key := label + "\x00" + payload
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		requests = append(requests, EpisodeMirrorRequest{
			Label:          label,
			EncodedContent: payload,
		})
	})
	return requests
}

func parseDownloadLinks(doc *goquery.Document) (map[string]map[string]string, []string) {
	downloadLinks := make(map[string]map[string]string)
	downloadURLs := make([]string, 0)
	seenURLs := make(map[string]struct{})
	doc.Find(".download li").Each(func(_ int, li *goquery.Selection) {
		quality := normalizeSpace(li.Find("strong").First().Text())
		quality = strings.TrimSuffix(quality, ":")
		if quality == "" {
			quality = "unknown"
		}
		if _, ok := downloadLinks[quality]; !ok {
			downloadLinks[quality] = make(map[string]string)
		}
		li.Find("a[href]").Each(func(_ int, anchor *goquery.Selection) {
			href, _ := anchor.Attr("href")
			href = strings.TrimSpace(href)
			if href == "" {
				return
			}
			host := normalizeSpace(anchor.Text())
			if host == "" {
				host = "link"
			}
			downloadLinks[quality][host] = href
			if _, ok := seenURLs[href]; !ok {
				seenURLs[href] = struct{}{}
				downloadURLs = append(downloadURLs, href)
			}
		})
	})
	return downloadLinks, downloadURLs
}
