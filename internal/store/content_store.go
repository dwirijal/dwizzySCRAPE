package store

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type contentDB interface {
	Exec(ctx context.Context, query string, args ...any) error
	QueryRow(ctx context.Context, query string, args ...any) rowScanner
}

type ContentStore struct {
	db contentDB
}

var embeddedDatePattern = regexp.MustCompile(`\b((?:19|20)\d{2}-\d{2}-\d{2})\b`)
var embeddedTimestampPattern = regexp.MustCompile(`\b((?:19|20)\d{2}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+\-]\d{2}:\d{2}))\b`)

func NewContentStore(db contentDB) *ContentStore {
	return &ContentStore{db: db}
}

func (s *ContentStore) UpsertManhwaSeries(ctx context.Context, series content.ManhwaSeries) error {
	if s.db == nil {
		return fmt.Errorf("content db is required")
	}
	if strings.TrimSpace(series.Slug) == "" {
		return fmt.Errorf("series slug is required")
	}
	sourceKey := normalizeSourceKey(series.Source)
	mediaType := normalizeReadingMediaType(sourceKey, series.MediaType)
	itemKey := mediaItemKey(sourceKey, mediaType, series.Slug)
	genres := normalizeSeriesGenres(sourceKey, series.Genres)
	detailJSON, err := json.Marshal(map[string]any{
		"alt_title":            strings.TrimSpace(series.AltTitle),
		"canonical_url":        series.CanonicalURL,
		"author":               strings.TrimSpace(series.Author),
		"synopsis":             strings.TrimSpace(series.Synopsis),
		"genres":               genres,
		"latest_unit_slug":     latestSlug(series.LatestChapter),
		"latest_chapter_label": latestChapterLabel(series.LatestChapter),
		"type":                 strings.TrimSpace(series.Type),
		"source":               sourceKey,
	})
	if err != nil {
		return fmt.Errorf("marshal series detail: %w", err)
	}
	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_item(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb
)
`, itemKey, sourceKey, mediaType, series.Slug, series.Title, strings.TrimSpace(series.CoverURL), normalizeStatus(series.Status), parseReleaseYear(series.ReleasedYear), float32(0), nil, nil, detailJSON); err != nil {
		return fmt.Errorf("upsert media_items: %w", err)
	}

	for idx, chapter := range series.Chapters {
		if err := s.upsertChapterSummary(ctx, itemKey, sourceKey, idx, chapter); err != nil {
			return err
		}
	}

	return nil
}

func (s *ContentStore) UpsertManhwaChapter(ctx context.Context, chapter content.ManhwaChapter) error {
	if s.db == nil {
		return fmt.Errorf("content db is required")
	}
	if strings.TrimSpace(chapter.SeriesSlug) == "" {
		return fmt.Errorf("series slug is required")
	}
	if strings.TrimSpace(chapter.Slug) == "" {
		return fmt.Errorf("chapter slug is required")
	}
	sourceKey := normalizeSourceKey(chapter.Source)

	pagesJSON, err := json.Marshal(map[string]any{
		"pages":        chapter.Pages,
		"series_slug":  strings.TrimSpace(chapter.SeriesSlug),
		"series_title": strings.TrimSpace(chapter.SeriesTitle),
	})
	if err != nil {
		return fmt.Errorf("marshal pages json: %w", err)
	}

	itemKey, err := s.resolveSeriesItemKey(ctx, sourceKey, chapter.SeriesSlug)
	if err != nil {
		return err
	}
	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_unit(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb
)
`, mediaUnitKey(sourceKey, "chapter", chapter.Slug), itemKey, sourceKey, "chapter", chapter.Slug, chapter.Title, chapter.Label, chapterSequenceIndex(chapter.Number, 0), chapter.CanonicalURL, nil, unitSlugFromURL(chapter.PrevURL), unitSlugFromURL(chapter.NextURL), pagesJSON); err != nil {
		return fmt.Errorf("upsert media_units chapter detail: %w", err)
	}

	return nil
}

func (s *ContentStore) resolveSeriesItemKey(ctx context.Context, sourceKey, seriesSlug string) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("content db is required")
	}
	seriesSlug = strings.TrimSpace(seriesSlug)
	if seriesSlug == "" {
		return "", fmt.Errorf("series slug is required")
	}

	var itemKey string
	err := s.db.QueryRow(ctx, `
SELECT item_key
FROM public.media_items
WHERE source = $1 AND slug = $2
ORDER BY updated_at DESC
LIMIT 1
`, sourceKey, seriesSlug).Scan(&itemKey)
	if err == nil && strings.TrimSpace(itemKey) != "" {
		return strings.TrimSpace(itemKey), nil
	}

	mediaType := normalizeReadingMediaType(sourceKey, "")
	return mediaItemKey(sourceKey, mediaType, seriesSlug), nil
}

func (s *ContentStore) upsertChapterSummary(ctx context.Context, itemKey, sourceKey string, index int, chapter content.ManhwaChapterRef) error {
	if strings.TrimSpace(chapter.Slug) == "" {
		return nil
	}
	sequenceIndex := chapterSequenceIndex(chapter.Number, index)
	publishedAt := normalizePublishedAt(chapter.PublishedAt)
	if err := s.db.Exec(ctx, `
SELECT public.upsert_media_unit(
	$1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, '')::timestamptz, NULL, NULL, $11::jsonb
)
`, mediaUnitKey(sourceKey, "chapter", chapter.Slug), itemKey, sourceKey, "chapter", chapter.Slug, stringValue(chapter.Title), chapter.Label, sequenceIndex, chapter.CanonicalURL, publishedAt, []byte(`{}`)); err != nil {
		return fmt.Errorf("upsert media_units chapter summary: %w", err)
	}
	return nil
}

func latestSlug(chapter *content.ManhwaChapterRef) any {
	if chapter == nil {
		return nil
	}
	return emptyToNil(chapter.Slug)
}

func latestChapterLabel(chapter *content.ManhwaChapterRef) any {
	if chapter == nil {
		return nil
	}
	return emptyToNil(chapter.Label)
}

func emptyToNil(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return ""
	}
}

func normalizeStatus(status string) any {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return nil
	}
	return status
}

func parseReleaseYear(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1800 || value > 9999 {
		return nil
	}
	year := int16(value)
	return year
}

func slugifyGenre(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "&", "and")
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	return strings.Join(fields, "-")
}

func unitSlugFromURL(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.Trim(raw, "/")
	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		return raw[idx+1:]
	}
	return raw
}

func chapterSequenceIndex(number string, fallbackIndex int) float64 {
	number = strings.TrimSpace(number)
	if number == "" {
		return float64(fallbackIndex)
	}
	normalized := strings.ReplaceAll(number, ",", ".")
	if value, err := strconv.ParseFloat(normalized, 64); err == nil {
		return value
	}
	var builder strings.Builder
	dotUsed := false
	for _, r := range normalized {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.' && !dotUsed:
			builder.WriteRune(r)
			dotUsed = true
		}
	}
	if builder.Len() == 0 {
		return float64(fallbackIndex)
	}
	value, err := strconv.ParseFloat(builder.String(), 64)
	if err != nil {
		return float64(fallbackIndex)
	}
	return value
}

func normalizePublishedAt(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	monthMap := map[string]string{
		"januari":   "January",
		"februari":  "February",
		"maret":     "March",
		"april":     "April",
		"mei":       "May",
		"juni":      "June",
		"juli":      "July",
		"agustus":   "August",
		"september": "September",
		"oktober":   "October",
		"november":  "November",
		"desember":  "December",
	}
	normalized := raw
	for from, to := range monthMap {
		normalized = strings.ReplaceAll(normalized, from, to)
		normalized = strings.ReplaceAll(normalized, strings.Title(from), to)
	}

	layouts := []string{
		"2006-01-02",
		time.RFC3339,
		"02/01/2006",
		"2/1/2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2 January 2006",
		"02 January 2006",
		"2 Jan 2006",
		"02 Jan 2006",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, normalized)
		if err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}

	return ""
}

func normalizePublishedAtFromEmbeddedDate(raw string) string {
	match := embeddedDatePattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(match) < 2 {
		return ""
	}
	return normalizePublishedAt(match[1])
}

func normalizePublishedAtFromEmbeddedTimestamp(raw string) string {
	match := embeddedTimestampPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if len(match) < 2 {
		return ""
	}
	return normalizePublishedAt(match[1])
}

func normalizeSourceKey(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "bacaman":
		return "bacaman"
	case "mangasusuku":
		return "mangasusuku"
	case "kanzenin":
		return "kanzenin"
	case "komiku":
		return "komiku"
	case "manhwaindo":
		return "manhwaindo"
	default:
		return "manhwaindo"
	}
}

func normalizeSeriesGenres(sourceKey string, genres []string) []string {
	normalized := make([]string, 0, len(genres)+1)
	seen := make(map[string]struct{}, len(genres)+1)

	for _, genre := range genres {
		genre = strings.TrimSpace(genre)
		if genre == "" {
			continue
		}
		key := strings.ToLower(genre)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, genre)
	}

	if sourceKey == "kanzenin" || sourceKey == "mangasusuku" {
		if _, ok := seen["nsfw"]; !ok {
			normalized = append(normalized, "nsfw")
		}
	}

	return normalized
}

func normalizeReadingMediaType(sourceKey, raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch {
	case raw != "":
		return raw
	case sourceKey == "komiku":
		return "komiku"
	default:
		return "manhwa"
	}
}
