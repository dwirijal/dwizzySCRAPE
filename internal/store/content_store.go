package store

import (
	"context"
	"encoding/json"
	"fmt"
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

	var titleID int64
	if err := s.db.QueryRow(ctx, `
INSERT INTO content_titles (
	media_type, slug, title, alt_title, canonical_url, cover_url, status, release_year, author, synopsis, latest_unit_slug, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())
ON CONFLICT (media_type, slug)
DO UPDATE SET
	title = EXCLUDED.title,
	alt_title = EXCLUDED.alt_title,
	canonical_url = EXCLUDED.canonical_url,
	cover_url = EXCLUDED.cover_url,
	status = EXCLUDED.status,
	release_year = EXCLUDED.release_year,
	author = EXCLUDED.author,
	synopsis = EXCLUDED.synopsis,
	latest_unit_slug = EXCLUDED.latest_unit_slug,
	updated_at = NOW()
RETURNING id
`, "manhwa", series.Slug, series.Title, emptyToNil(series.AltTitle), series.CanonicalURL, emptyToNil(series.CoverURL), normalizeStatus(series.Status), emptyToNil(series.ReleasedYear), emptyToNil(series.Author), emptyToNil(series.Synopsis), latestSlug(series.LatestChapter)).Scan(&titleID); err != nil {
		return fmt.Errorf("upsert content_titles: %w", err)
	}

	if err := s.db.Exec(ctx, `
INSERT INTO content_source_links (title_id, source_key, source_slug, canonical_url, last_scraped_at)
VALUES ($1,$2,$3,$4,NOW())
ON CONFLICT (source_key, source_slug)
DO UPDATE SET
	title_id = EXCLUDED.title_id,
	canonical_url = EXCLUDED.canonical_url,
	last_scraped_at = NOW()
`, titleID, sourceKey, series.Slug, series.CanonicalURL); err != nil {
		return fmt.Errorf("upsert content_source_links: %w", err)
	}

	if err := s.db.Exec(ctx, `DELETE FROM content_title_genres WHERE title_id = $1`, titleID); err != nil {
		return fmt.Errorf("clear title genres: %w", err)
	}
	for _, genre := range series.Genres {
		genre = strings.TrimSpace(genre)
		if genre == "" {
			continue
		}
		var genreID int64
		if err := s.db.QueryRow(ctx, `
INSERT INTO content_genres (slug, label)
VALUES ($1,$2)
ON CONFLICT (slug)
DO UPDATE SET label = EXCLUDED.label
RETURNING id
`, slugifyGenre(genre), genre).Scan(&genreID); err != nil {
			return fmt.Errorf("upsert content_genres: %w", err)
		}
		if err := s.db.Exec(ctx, `
INSERT INTO content_title_genres (title_id, genre_id)
VALUES ($1,$2)
ON CONFLICT (title_id, genre_id) DO NOTHING
`, titleID, genreID); err != nil {
			return fmt.Errorf("insert content_title_genres: %w", err)
		}
	}

	for idx, chapter := range series.Chapters {
		if err := s.upsertChapterSummary(ctx, titleID, idx, chapter); err != nil {
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

	pagesJSON, err := json.Marshal(chapter.Pages)
	if err != nil {
		return fmt.Errorf("marshal pages json: %w", err)
	}

	var unitID int64
	if err := s.db.QueryRow(ctx, `
INSERT INTO content_units (
	title_id, unit_type, slug, title, label, number_label, canonical_url, prev_unit_slug, next_unit_slug, pages_json, updated_at
)
SELECT source_links.title_id, 'chapter', $3, $4, $5, $6, $7, $8, $9, $10::jsonb, NOW()
FROM content_source_links AS source_links
WHERE source_links.source_key = $1
  AND source_links.source_slug = $2
ON CONFLICT (slug)
DO UPDATE SET
	title = EXCLUDED.title,
	label = EXCLUDED.label,
	number_label = EXCLUDED.number_label,
	canonical_url = EXCLUDED.canonical_url,
	prev_unit_slug = EXCLUDED.prev_unit_slug,
	next_unit_slug = EXCLUDED.next_unit_slug,
	pages_json = EXCLUDED.pages_json,
	updated_at = NOW()
RETURNING id
`, sourceKey, chapter.SeriesSlug, chapter.Slug, chapter.Title, chapter.Label, chapter.Number, chapter.CanonicalURL, unitSlugFromURL(chapter.PrevURL), unitSlugFromURL(chapter.NextURL), pagesJSON).Scan(&unitID); err != nil {
		return fmt.Errorf("upsert content_units chapter detail: %w", err)
	}
	if unitID == 0 {
		return fmt.Errorf("upsert content_units chapter detail: no matching title for source=%q slug=%q", sourceKey, chapter.SeriesSlug)
	}

	return nil
}

func (s *ContentStore) upsertChapterSummary(ctx context.Context, titleID int64, index int, chapter content.ManhwaChapterRef) error {
	if strings.TrimSpace(chapter.Slug) == "" {
		return nil
	}
	sequenceIndex := chapterSequenceIndex(chapter.Number, index)
	publishedAt := normalizePublishedAt(chapter.PublishedAt)
	if err := s.db.Exec(ctx, `
INSERT INTO content_units (
	title_id, unit_type, slug, title, label, number_label, sequence_index, canonical_url, published_at, updated_at
) VALUES ($1,'chapter',$2,$3,$4,$5,$6,$7,NULLIF($8, '')::timestamptz,NOW())
ON CONFLICT (slug)
DO UPDATE SET
	title_id = EXCLUDED.title_id,
	title = EXCLUDED.title,
	label = EXCLUDED.label,
	number_label = EXCLUDED.number_label,
	sequence_index = EXCLUDED.sequence_index,
	canonical_url = EXCLUDED.canonical_url,
	published_at = EXCLUDED.published_at,
	updated_at = NOW()
`, titleID, chapter.Slug, emptyToNil(chapter.Title), chapter.Label, chapter.Number, sequenceIndex, chapter.CanonicalURL, publishedAt); err != nil {
		return fmt.Errorf("upsert content_units chapter summary: %w", err)
	}
	return nil
}

func latestSlug(chapter *content.ManhwaChapterRef) any {
	if chapter == nil {
		return nil
	}
	return emptyToNil(chapter.Slug)
}

func emptyToNil(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func normalizeStatus(status string) any {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return nil
	}
	return status
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

func normalizeSourceKey(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "komiku":
		return "komiku"
	case "manhwaindo":
		return "manhwaindo"
	default:
		return "manhwaindo"
	}
}
