package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DonghuaCollector struct {
	dsn string
}

func NewDonghuaCollector(dsn string) *DonghuaCollector {
	return &DonghuaCollector{dsn: strings.TrimSpace(dsn)}
}

func (c *DonghuaCollector) Domain() string {
	return "donghua"
}

func (c *DonghuaCollector) Build(ctx Context, writer *Writer, options BuildOptions) error {
	db, err := c.open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	catalog, err := c.writeDomainDocs(ctx, db, writer, options)
	if err != nil {
		return err
	}
	hot := limitDonghuaHot(catalog, options.HotLimit)
	for _, item := range hot {
		if err := c.writeTitleAndPlayback(ctx, db, writer, item.Slug); err != nil {
			continue
		}
	}
	return nil
}

func (c *DonghuaCollector) Patch(ctx Context, writer *Writer, slug string, _ BuildOptions) error {
	db, err := c.open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := c.writeDomainDocs(ctx, db, writer, BuildOptions{}); err != nil {
		return err
	}
	return c.writeTitleAndPlayback(ctx, db, writer, strings.TrimSpace(slug))
}

func (c *DonghuaCollector) writeDomainDocs(ctx context.Context, db *pgxpool.Pool, writer *Writer, options BuildOptions) ([]donghuaCatalogItem, error) {
	options = normalizeOptions(options)
	page := options.CatalogPage
	if page <= 0 {
		page = 1
	}
	limit := maxInt(options.HotLimit*3, 24)
	catalog, err := c.listCatalog(ctx, db, page, limit)
	if err != nil {
		return nil, err
	}
	hot := limitDonghuaHot(catalog, options.HotLimit)
	if _, err := writer.Write(c.Domain(), KindHome, "hot", map[string]any{
		"latest_updates": hot,
		"ongoing_series": hot,
	}); err != nil {
		return nil, err
	}
	if _, err := writer.Write(c.Domain(), KindCatalog, fmt.Sprintf("page-%d", page), catalog); err != nil {
		return nil, err
	}
	return catalog, nil
}

type donghuaCatalogItem struct {
	Type          string         `json:"type"`
	Slug          string         `json:"slug"`
	Title         string         `json:"title"`
	CanonicalURL  string         `json:"canonical_url"`
	CoverURL      string         `json:"cover_url,omitempty"`
	Status        string         `json:"status,omitempty"`
	LatestEpisode *episodeRecord `json:"latest_episode,omitempty"`
}

type donghuaSeries struct {
	Type          string          `json:"type"`
	Slug          string          `json:"slug"`
	Title         string          `json:"title"`
	AltTitle      string          `json:"alt_title,omitempty"`
	CanonicalURL  string          `json:"canonical_url"`
	CoverURL      string          `json:"cover_url,omitempty"`
	Status        string          `json:"status,omitempty"`
	ReleaseYear   string          `json:"release_year,omitempty"`
	Studio        string          `json:"studio,omitempty"`
	Synopsis      string          `json:"synopsis,omitempty"`
	Genres        []string        `json:"genres,omitempty"`
	LatestEpisode *episodeRecord  `json:"latest_episode,omitempty"`
	Episodes      []episodeRecord `json:"episodes,omitempty"`
}

type donghuaEpisode struct {
	SeriesSlug   string      `json:"series_slug,omitempty"`
	SeriesTitle  string      `json:"series_title,omitempty"`
	Slug         string      `json:"slug"`
	Title        string      `json:"title"`
	Label        string      `json:"label"`
	Number       string      `json:"number"`
	CanonicalURL string      `json:"canonical_url"`
	PrevURL      string      `json:"prev_url,omitempty"`
	NextURL      string      `json:"next_url,omitempty"`
	Assets       []pageAsset `json:"assets,omitempty"`
}

type episodeRecord struct {
	Slug         string `json:"slug"`
	Title        string `json:"title,omitempty"`
	Label        string `json:"label"`
	Number       string `json:"number"`
	CanonicalURL string `json:"canonical_url"`
	PublishedAt  string `json:"published_at,omitempty"`
}

type pageAsset struct {
	Position int    `json:"position"`
	URL      string `json:"url"`
}

func (c *DonghuaCollector) open(ctx context.Context) (*pgxpool.Pool, error) {
	if c.dsn == "" {
		return nil, fmt.Errorf("donghua snapshot dsn is required")
	}
	return pgxpool.New(ctx, c.dsn)
}

func (c *DonghuaCollector) writeTitleAndPlayback(ctx context.Context, db *pgxpool.Pool, writer *Writer, slug string) error {
	if slug == "" {
		return fmt.Errorf("donghua slug is required")
	}
	series, err := c.getSeries(ctx, db, slug)
	if err != nil {
		return err
	}
	if _, err := writer.Write(c.Domain(), KindTitle, slug, series); err != nil {
		return err
	}
	playbackSlug := ""
	if series.LatestEpisode != nil {
		playbackSlug = series.LatestEpisode.Slug
	}
	if playbackSlug == "" && len(series.Episodes) > 0 {
		playbackSlug = series.Episodes[0].Slug
	}
	if playbackSlug == "" {
		return nil
	}
	playback, err := c.getEpisode(ctx, db, playbackSlug)
	if err != nil {
		return err
	}
	_, err = writer.Write(c.Domain(), KindPlayback, playbackSlug, playback)
	return err
}

func (c *DonghuaCollector) listCatalog(ctx context.Context, db *pgxpool.Pool, page, limit int) ([]donghuaCatalogItem, error) {
	offset := (page - 1) * limit
	rows, err := db.Query(ctx, `
SELECT
  t.slug,
  t.title,
  t.canonical_url,
  COALESCE(t.cover_url, ''),
  COALESCE(t.status, ''),
  COALESCE(u.slug, ''),
  COALESCE(u.title, ''),
  COALESCE(u.label, ''),
  COALESCE(u.number_label, ''),
  COALESCE(u.canonical_url, ''),
  COALESCE(to_char(u.published_at, 'YYYY-MM-DD'), '')
FROM content_titles t
LEFT JOIN content_units u ON u.slug = t.latest_unit_slug
WHERE t.media_type = 'donghua'
ORDER BY t.updated_at DESC NULLS LAST, t.title ASC
LIMIT $1 OFFSET $2
`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list donghua snapshot catalog: %w", err)
	}
	defer rows.Close()

	items := make([]donghuaCatalogItem, 0, limit)
	for rows.Next() {
		var item donghuaCatalogItem
		var latest episodeRecord
		if err := rows.Scan(
			&item.Slug,
			&item.Title,
			&item.CanonicalURL,
			&item.CoverURL,
			&item.Status,
			&latest.Slug,
			&latest.Title,
			&latest.Label,
			&latest.Number,
			&latest.CanonicalURL,
			&latest.PublishedAt,
		); err != nil {
			return nil, err
		}
		item.Type = "donghua"
		if latest.Slug != "" {
			item.LatestEpisode = &latest
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (c *DonghuaCollector) getSeries(ctx context.Context, db *pgxpool.Pool, slug string) (donghuaSeries, error) {
	var series donghuaSeries
	var latest episodeRecord
	err := db.QueryRow(ctx, `
SELECT
  t.slug,
  t.title,
  COALESCE(t.alt_title, ''),
  t.canonical_url,
  COALESCE(t.cover_url, ''),
  COALESCE(t.status, ''),
  COALESCE(t.release_year, ''),
  COALESCE(t.author, ''),
  COALESCE(t.synopsis, ''),
  COALESCE(u.slug, ''),
  COALESCE(u.title, ''),
  COALESCE(u.label, ''),
  COALESCE(u.number_label, ''),
  COALESCE(u.canonical_url, ''),
  COALESCE(to_char(u.published_at, 'YYYY-MM-DD'), '')
FROM content_titles t
LEFT JOIN content_units u ON u.slug = t.latest_unit_slug
WHERE t.media_type = 'donghua' AND t.slug = $1
`, slug).Scan(
		&series.Slug,
		&series.Title,
		&series.AltTitle,
		&series.CanonicalURL,
		&series.CoverURL,
		&series.Status,
		&series.ReleaseYear,
		&series.Studio,
		&series.Synopsis,
		&latest.Slug,
		&latest.Title,
		&latest.Label,
		&latest.Number,
		&latest.CanonicalURL,
		&latest.PublishedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return donghuaSeries{}, fmt.Errorf("donghua %q not found", slug)
		}
		return donghuaSeries{}, err
	}
	series.Type = "donghua"
	if latest.Slug != "" {
		series.LatestEpisode = &latest
	}

	genreRows, err := db.Query(ctx, `
SELECT g.label
FROM content_title_genres tg
JOIN content_genres g ON g.id = tg.genre_id
JOIN content_titles t ON t.id = tg.title_id
WHERE t.media_type = 'donghua' AND t.slug = $1
ORDER BY g.label ASC
`, slug)
	if err != nil {
		return donghuaSeries{}, err
	}
	defer genreRows.Close()
	for genreRows.Next() {
		var label string
		if err := genreRows.Scan(&label); err != nil {
			return donghuaSeries{}, err
		}
		series.Genres = append(series.Genres, label)
	}
	if err := genreRows.Err(); err != nil {
		return donghuaSeries{}, err
	}

	episodes, err := c.listEpisodes(ctx, db, slug)
	if err == nil {
		series.Episodes = episodes
	}
	return series, nil
}

func limitDonghuaHot(items []donghuaCatalogItem, limit int) []donghuaCatalogItem {
	normalizedLimit := maxInt(limit, 8)
	if len(items) <= normalizedLimit {
		return append([]donghuaCatalogItem(nil), items...)
	}
	return append([]donghuaCatalogItem(nil), items[:normalizedLimit]...)
}

func (c *DonghuaCollector) listEpisodes(ctx context.Context, db *pgxpool.Pool, slug string) ([]episodeRecord, error) {
	rows, err := db.Query(ctx, `
SELECT u.slug, COALESCE(u.title, ''), u.label, u.number_label, u.canonical_url, COALESCE(to_char(u.published_at, 'YYYY-MM-DD'), '')
FROM content_units u
JOIN content_titles t ON t.id = u.title_id
WHERE t.media_type = 'donghua' AND t.slug = $1 AND u.unit_type = 'episode'
ORDER BY u.sequence_index DESC NULLS LAST, u.published_at DESC NULLS LAST, u.id DESC
`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]episodeRecord, 0)
	for rows.Next() {
		var item episodeRecord
		if err := rows.Scan(&item.Slug, &item.Title, &item.Label, &item.Number, &item.CanonicalURL, &item.PublishedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (c *DonghuaCollector) getEpisode(ctx context.Context, db *pgxpool.Pool, unitSlug string) (donghuaEpisode, error) {
	var episode donghuaEpisode
	var assetsJSON []byte
	err := db.QueryRow(ctx, `
SELECT
  t.slug,
  t.title,
  u.slug,
  COALESCE(u.title, ''),
  u.label,
  u.number_label,
  u.canonical_url,
  COALESCE(u.prev_unit_slug, ''),
  COALESCE(u.next_unit_slug, ''),
  COALESCE(u.pages_json, '[]'::jsonb)
FROM content_units u
JOIN content_titles t ON t.id = u.title_id
WHERE u.slug = $1
`, unitSlug).Scan(
		&episode.SeriesSlug,
		&episode.SeriesTitle,
		&episode.Slug,
		&episode.Title,
		&episode.Label,
		&episode.Number,
		&episode.CanonicalURL,
		&episode.PrevURL,
		&episode.NextURL,
		&assetsJSON,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return donghuaEpisode{}, fmt.Errorf("donghua episode %q not found", unitSlug)
		}
		return donghuaEpisode{}, err
	}
	if len(assetsJSON) > 0 {
		if err := json.Unmarshal(assetsJSON, &episode.Assets); err != nil {
			return donghuaEpisode{}, fmt.Errorf("decode donghua assets %q: %w", unitSlug, err)
		}
	}
	return episode, nil
}
