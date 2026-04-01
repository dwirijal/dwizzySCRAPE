package nekopoi

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string        `xml:"title"`
	Items []rssFeedItem `xml:"item"`
}

type rssFeedItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	PublishedAt string   `xml:"pubDate"`
	Description string   `xml:"description"`
	Content     string   `xml:"encoded"`
	Categories  []string `xml:"category"`
}

var (
	nekopoiImagePattern  = regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["']`)
	nekopoiTagPattern    = regexp.MustCompile(`(?s)<[^>]+>`)
	nekopoiItemPattern   = regexp.MustCompile(`(?is)<item\b[^>]*>(.*?)</item>`)
	titleLabelPattern    = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*`)
	episodeNumberPattern = regexp.MustCompile(`(?i)\bepisode\s+(\d{1,4})\b`)
	partNumberPattern    = regexp.MustCompile(`(?i)\bpart\s+(\d{1,4})\b`)
)

func ParseFeedXML(raw []byte, feedURL string) ([]FeedItem, error) {
	sourceDomain := hostFromURL(feedURL)
	itemBlocks := nekopoiItemPattern.FindAllSubmatch(raw, -1)
	if len(itemBlocks) == 0 {
		return nil, fmt.Errorf("decode rss feed: no item blocks found")
	}

	items := make([]FeedItem, 0, len(itemBlocks))
	for _, block := range itemBlocks {
		entry := parseRSSItemBlock(block[1])
		item, ok := parseFeedItem(entry, sourceDomain)
		if !ok {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func parseRSSItemBlock(raw []byte) rssFeedItem {
	body := string(raw)
	return rssFeedItem{
		Title:       decodeRSSTag(extractTagValue(body, "title")),
		Link:        decodeRSSTag(extractTagValue(body, "link")),
		PublishedAt: decodeRSSTag(extractTagValue(body, "pubDate")),
		Description: decodeRSSTag(extractTagValue(body, "description")),
		Content:     decodeRSSTag(extractTagValue(body, "content:encoded")),
		Categories:  decodeRSSValues(extractTagValues(body, "category")),
	}
}

func extractTagValue(body, tag string) string {
	values := extractTagValues(body, tag)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func extractTagValues(body, tag string) []string {
	pattern := regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(tag) + `(?:\b[^>]*)?>(.*?)</` + regexp.QuoteMeta(tag) + `>`)
	matches := pattern.FindAllStringSubmatch(body, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		values = append(values, unwrapCDATA(strings.TrimSpace(match[1])))
	}
	return values
}

func unwrapCDATA(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "<![CDATA[")
	value = strings.TrimSuffix(value, "]]>")
	return strings.TrimSpace(value)
}

func decodeRSSTag(value string) string {
	return strings.TrimSpace(html.UnescapeString(value))
}

func decodeRSSValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if decoded := decodeRSSTag(value); decoded != "" {
			out = append(out, decoded)
		}
	}
	return out
}

func parseFeedItem(entry rssFeedItem, sourceDomain string) (FeedItem, bool) {
	canonicalURL := strings.TrimSpace(entry.Link)
	slug := slugFromURL(canonicalURL)
	title := strings.TrimSpace(html.UnescapeString(entry.Title))
	if canonicalURL == "" || slug == "" || title == "" {
		return FeedItem{}, false
	}

	publishedAt := parsePublishedAt(entry.PublishedAt)
	contentHTML := strings.TrimSpace(entry.Content)
	descriptionHTML := strings.TrimSpace(entry.Description)
	categoryNames := normalizeList(entry.Categories)
	contentFormat := inferContentFormat(categoryNames, title)
	contentFields := parseContentFields(contentHTML, descriptionHTML)
	return HydrateDerivedMetadata(FeedItem{
		SourceDomain:       sourceDomain,
		Title:              title,
		CanonicalURL:       canonicalURL,
		Slug:               slug,
		CoverURL:           firstNonEmpty(contentFields.CoverURL, extractImageURL(contentHTML), extractImageURL(descriptionHTML)),
		Categories:         categoryNames,
		Genres:             normalizeList(contentFields.Genres),
		ContentFormat:      contentFormat,
		DescriptionHTML:    descriptionHTML,
		ContentHTML:        contentHTML,
		DescriptionExcerpt: firstNonEmpty(contentFields.DescriptionExcerpt, excerptFromHTML(descriptionHTML), excerptFromHTML(contentHTML)),
		OriginalTitle:      contentFields.OriginalTitle,
		NuclearCode:        contentFields.NuclearCode,
		Actress:            contentFields.Actress,
		Parody:             contentFields.Parody,
		Producers:          normalizeList(contentFields.Producers),
		Duration:           contentFields.Duration,
		Size:               contentFields.Size,
		PublishedAt:        publishedAt,
	}), true
}

type titleMetadata struct {
	NormalizedTitle string
	TitleLabels     []string
	EntryKind       string
	EpisodeNumber   int
	PartNumber      int
}

func deriveTitleMetadata(rawTitle string) titleMetadata {
	title := strings.TrimSpace(html.UnescapeString(rawTitle))
	labels := make([]string, 0, 2)
	for {
		match := titleLabelPattern.FindStringSubmatch(title)
		if len(match) < 2 {
			break
		}
		label := strings.ToUpper(strings.TrimSpace(match[1]))
		if label != "" {
			labels = append(labels, label)
		}
		title = strings.TrimSpace(title[len(match[0]):])
	}
	title = strings.Join(strings.Fields(title), " ")

	meta := titleMetadata{
		NormalizedTitle: title,
		TitleLabels:     normalizeList(labels),
		EntryKind:       "standalone",
		EpisodeNumber:   firstPatternInt(title, episodeNumberPattern),
		PartNumber:      firstPatternInt(title, partNumberPattern),
	}

	lower := strings.ToLower(title)
	switch {
	case hasTitleLabel(meta.TitleLabels, "PV") || strings.Contains(lower, "preview"):
		meta.EntryKind = "preview"
	case strings.Contains(lower, "compilation"):
		meta.EntryKind = "compilation"
	case meta.EpisodeNumber > 0:
		meta.EntryKind = "episode"
	case meta.PartNumber > 0:
		meta.EntryKind = "part"
	}
	return meta
}

func shouldTreatTitleAsSeriesCandidate(meta titleMetadata, contentFormat string) bool {
	if meta.EntryKind != "episode" || meta.EpisodeNumber <= 0 {
		return false
	}
	if hasTitleLabel(meta.TitleLabels, "PV") {
		return false
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentFormat)), "animation")
}

func HydrateDerivedMetadata(item FeedItem) FeedItem {
	titleMeta := deriveTitleMetadata(item.Title)
	item.NormalizedTitle = titleMeta.NormalizedTitle
	item.TitleLabels = titleMeta.TitleLabels
	item.EntryKind = titleMeta.EntryKind
	item.EpisodeNumber = titleMeta.EpisodeNumber
	item.PartNumber = titleMeta.PartNumber
	item.SeriesCandidate = shouldTreatTitleAsSeriesCandidate(titleMeta, item.ContentFormat)
	return item
}

func hasTitleLabel(labels []string, target string) bool {
	target = strings.ToUpper(strings.TrimSpace(target))
	for _, label := range labels {
		if strings.ToUpper(strings.TrimSpace(label)) == target {
			return true
		}
	}
	return false
}

func firstPatternInt(value string, pattern *regexp.Regexp) int {
	match := pattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0
	}
	var out int
	fmt.Sscanf(match[1], "%d", &out)
	return out
}

type contentFields struct {
	CoverURL           string
	DescriptionExcerpt string
	OriginalTitle      string
	NuclearCode        string
	Actress            string
	Parody             string
	Producers          []string
	Duration           string
	Genres             []string
	Size               string
}

type DetailMetadata struct {
	PostID         string
	CoverURL       string
	Excerpt        string
	PlayerCount    int
	PlayerHosts    []string
	DownloadCount  int
	DownloadLabels []string
	DownloadHosts  []string
}

func parseContentFields(contentHTML, descriptionHTML string) contentFields {
	htmlSource := strings.TrimSpace(contentHTML)
	if htmlSource == "" {
		htmlSource = strings.TrimSpace(descriptionHTML)
	}
	fields := contentFields{
		CoverURL: extractImageURL(htmlSource),
	}
	lines := htmlLines(htmlSource)
	if len(lines) == 0 {
		lines = htmlLines(descriptionHTML)
	}
	for _, line := range lines {
		key, value, ok := splitLabeledLine(line)
		if !ok {
			if fields.DescriptionExcerpt == "" && len(line) >= 24 {
				fields.DescriptionExcerpt = line
			}
			continue
		}
		switch normalizeLabelKey(key) {
		case "originaltitle":
			fields.OriginalTitle = value
		case "nuclearcode", "code":
			fields.NuclearCode = value
		case "actress", "performer":
			fields.Actress = value
		case "parody":
			fields.Parody = value
		case "producer", "producers", "studio":
			fields.Producers = splitValueList(value)
		case "duration", "runtime":
			fields.Duration = value
		case "genre", "genres":
			fields.Genres = splitValueList(value)
		case "size":
			fields.Size = value
		}
	}
	return fields
}

func htmlLines(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	replacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n",
		"</div>", "\n",
		"</li>", "\n",
	)
	text := replacer.Replace(raw)
	text = nekopoiTagPattern.ReplaceAllString(text, "")
	text = html.UnescapeString(text)

	parts := strings.Split(text, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

func splitLabeledLine(line string) (string, string, bool) {
	for _, sep := range []string{":", " : "} {
		if idx := strings.Index(line, sep); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+len(sep):])
			if key != "" && value != "" {
				return key, value, true
			}
		}
	}
	return "", "", false
}

func normalizeLabelKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.ReplaceAll(value, " ", "")
}

func splitValueList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '/' || r == '|'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func extractImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	match := nekopoiImagePattern.FindStringSubmatch(raw)
	if len(match) >= 2 {
		return strings.TrimSpace(html.UnescapeString(match[1]))
	}
	return ""
}

func excerptFromHTML(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewBufferString("<div>" + raw + "</div>"))
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(doc.Text())
	if text == "" {
		return ""
	}
	fields := strings.Fields(text)
	return strings.Join(fields, " ")
}

func parsePublishedAt(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Hostname())
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
	return strings.TrimSpace(parts[len(parts)-1])
}

func normalizeList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(html.UnescapeString(value))
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func inferContentFormat(categories []string, title string) string {
	for _, category := range categories {
		switch strings.ToLower(strings.TrimSpace(category)) {
		case "jav", "live action":
			return "live_action"
		case "2d animation", "2d hentai", "l2d":
			return "animation_2d"
		case "3d hentai", "3d animation":
			return "animation_3d"
		}
	}

	lowerTitle := strings.ToLower(strings.TrimSpace(title))
	switch {
	case strings.Contains(lowerTitle, "live action"):
		return "live_action"
	case strings.Contains(lowerTitle, "[3d]"):
		return "animation_3d"
	case strings.Contains(lowerTitle, "[l2d]"), strings.Contains(lowerTitle, "[2d]"):
		return "animation_2d"
	default:
		return "live_action"
	}
}

func ParseDetailHTML(raw []byte) (DetailMetadata, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return DetailMetadata{}, fmt.Errorf("parse detail html: %w", err)
	}

	meta := DetailMetadata{
		PostID: strings.TrimSpace(extractPostID(doc)),
		CoverURL: strings.TrimSpace(firstNonEmpty(
			metaContent(doc, `meta[property="og:image"]`),
			metaContent(doc, `meta[name="twitter:image"]`),
		)),
		Excerpt: strings.TrimSpace(firstNonEmpty(
			metaContent(doc, `meta[name="description"]`),
			metaContent(doc, `meta[property="og:description"]`),
		)),
		PlayerCount:    doc.Find(".nk-player-frame iframe").Length(),
		PlayerHosts:    collectPlayerHosts(doc),
		DownloadCount:  doc.Find(".nk-download-row").Length(),
		DownloadLabels: collectDownloadLabels(doc),
		DownloadHosts:  collectDownloadHosts(doc),
	}
	return meta, nil
}

func metaContent(doc *goquery.Document, selector string) string {
	content, ok := doc.Find(selector).First().Attr("content")
	if !ok {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(content))
}

func extractPostID(doc *goquery.Document) string {
	className, ok := doc.Find("body").First().Attr("class")
	if !ok {
		return ""
	}
	for _, field := range strings.Fields(className) {
		if strings.HasPrefix(field, "postid-") {
			return strings.TrimPrefix(field, "postid-")
		}
	}
	return ""
}

func collectPlayerHosts(doc *goquery.Document) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	doc.Find(".nk-player-frame iframe").Each(func(_ int, frame *goquery.Selection) {
		src, ok := frame.Attr("src")
		if !ok {
			return
		}
		host := normalizedHost(src)
		if host == "" {
			return
		}
		if _, exists := seen[host]; exists {
			return
		}
		seen[host] = struct{}{}
		out = append(out, host)
	})
	return out
}

func collectDownloadLabels(doc *goquery.Document) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	doc.Find(".nk-download-row .nk-download-name").Each(func(_ int, row *goquery.Selection) {
		text := strings.TrimSpace(strings.Join(strings.Fields(row.Text()), " "))
		if text == "" {
			return
		}
		if _, exists := seen[text]; exists {
			return
		}
		seen[text] = struct{}{}
		out = append(out, text)
	})
	return out
}

func collectDownloadHosts(doc *goquery.Document) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	doc.Find(".nk-download-row .nk-download-links a[href]").Each(func(_ int, link *goquery.Selection) {
		label := strings.TrimSpace(strings.Join(strings.Fields(link.Text()), " "))
		host := normalizedHost(attrOrEmpty(link, "href"))
		value := firstNonEmpty(label, host)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	})
	return out
}

func normalizedHost(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}

func attrOrEmpty(selection *goquery.Selection, name string) string {
	value, ok := selection.Attr(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
