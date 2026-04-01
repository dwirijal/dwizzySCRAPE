package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/otakudesu"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(context.Background()); err != nil {
		panic(err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database pool: %w", err)
	}
	defer pool.Close()

	limit := parsePositiveInt("OTAKUDESU_ENRICH_LIMIT", 20)
	minMatchScore := parseFloatDefault("OTAKUDESU_ENRICH_MIN_MATCH_SCORE", 0.55)
	slugFilter := parseCSVList(os.Getenv("OTAKUDESU_ENRICH_SLUGS"))
	skipExisting := parseBoolDefault("OTAKUDESU_ENRICH_SKIP_EXISTING", true)
	maxEpisodes := parsePositiveInt("OTAKUDESU_ENRICH_MAX_EPISODES_PER_ANIME", 0)

	anime, err := loadSamehadakuAnime(ctx, pool, limit, slugFilter)
	if err != nil {
		return err
	}

	client := otakudesu.NewClient(
		firstNonEmpty(os.Getenv("OTAKUDESU_BASE_URL"), "https://otakudesu.blog"),
		firstNonEmpty(os.Getenv("OTAKUDESU_USER_AGENT"), cfg.UserAgent),
		parseDurationDefault("OTAKUDESU_HTTP_TIMEOUT", cfg.HTTPTimeout),
	)
	results, err := otakudesu.BuildReviews(ctx, client, anime, otakudesu.ReviewOptions{
		MinMatchScore: minMatchScore,
		MaxEpisodes:   maxEpisodes,
	})
	if err != nil {
		return err
	}

	db, err := store.NewPgxContentDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open content db: %w", err)
	}
	defer db.Close()

	writer := store.NewOtakudesuEnrichmentStoreWithDB(db)
	scrapedAt := time.Now().UTC()
	report := enrichReport{}

	for _, result := range results {
		if result.MatchStatus != "matched" {
			report.AnimeSkipped++
			continue
		}
		report.AnimeMatched++
		for _, episode := range result.Episodes {
			if strings.TrimSpace(episode.DBEpisodeSlug) == "" {
				report.EpisodesSkipped++
				continue
			}
			if !hasEpisodeEnrichmentPayload(episode) {
				report.EpisodesSkipped++
				continue
			}
			if skipExisting {
				exists, err := writer.HasEpisodeEnrichment(ctx, episode.DBEpisodeSlug)
				if err != nil {
					return err
				}
				if exists {
					report.EpisodesSkipped++
					continue
				}
			}
			if err := writer.UpsertEpisodeEnrichment(ctx, result, episode, scrapedAt); err != nil {
				report.EpisodesFailed++
				fmt.Printf("otakudesu enrich failed: anime=%s episode=%s error=%s\n", result.DBAnimeSlug, episode.DBEpisodeSlug, err)
				continue
			}
			report.EpisodesEnriched++
			fmt.Printf("otakudesu enrich synced: anime=%s episode=%s\n", result.DBAnimeSlug, episode.DBEpisodeSlug)
		}
	}

	fmt.Printf(
		"otakudesu enrich done: anime_matched=%d anime_skipped=%d episodes_enriched=%d episodes_skipped=%d episodes_failed=%d\n",
		report.AnimeMatched,
		report.AnimeSkipped,
		report.EpisodesEnriched,
		report.EpisodesSkipped,
		report.EpisodesFailed,
	)
	return nil
}

type enrichReport struct {
	AnimeMatched     int
	AnimeSkipped     int
	EpisodesEnriched int
	EpisodesSkipped  int
	EpisodesFailed   int
}

func hasEpisodeEnrichmentPayload(episode otakudesu.EpisodeReview) bool {
	return strings.TrimSpace(episode.StreamURL) != "" || len(episode.StreamMirrors) > 0 || len(episode.DownloadLinks) > 0
}

func loadSamehadakuAnime(ctx context.Context, pool *pgxpool.Pool, limit int, slugFilter []string) ([]otakudesu.SamehadakuAnime, error) {
	query := `
SELECT
    i.slug,
    i.title,
    COALESCE(i.detail->>'source_title', '') AS source_title
FROM public.media_items i
WHERE i.source = 'samehadaku'
  AND i.media_type = 'anime'
`
	args := make([]any, 0, 2)
	if len(slugFilter) > 0 {
		query += `
  AND i.slug = ANY($1)
`
		args = append(args, slugFilter)
	}
	query += `
ORDER BY i.updated_at DESC NULLS LAST, i.slug ASC
`
	if len(slugFilter) == 0 {
		query += `
LIMIT $1
`
		args = append(args, limit)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query samehadaku anime: %w", err)
	}
	defer rows.Close()

	anime := make([]otakudesu.SamehadakuAnime, 0, limit)
	slugs := make([]string, 0, limit)
	indexBySlug := make(map[string]int)
	for rows.Next() {
		var entry otakudesu.SamehadakuAnime
		if err := rows.Scan(&entry.AnimeSlug, &entry.Title, &entry.SourceTitle); err != nil {
			return nil, fmt.Errorf("scan samehadaku anime: %w", err)
		}
		indexBySlug[entry.AnimeSlug] = len(anime)
		anime = append(anime, entry)
		slugs = append(slugs, entry.AnimeSlug)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate samehadaku anime: %w", rows.Err())
	}
	if len(slugs) == 0 {
		return nil, nil
	}

	episodeRows, err := pool.Query(ctx, `
SELECT
    i.slug AS anime_slug,
    u.slug AS episode_slug,
    u.number,
    COALESCE(u.detail->'stream_links_json', '{}'::jsonb) <> '{}'::jsonb AS stream_present,
    COALESCE(u.detail->'download_links_json', '{}'::jsonb) <> '{}'::jsonb AS download_present
FROM public.media_items i
JOIN public.media_units u ON u.item_key = i.item_key
WHERE i.source = 'samehadaku'
  AND i.media_type = 'anime'
  AND i.slug = ANY($1)
  AND u.source = 'samehadaku'
  AND u.unit_type = 'episode'
  AND u.number IS NOT NULL
ORDER BY i.slug ASC, u.number ASC
`, slugs)
	if err != nil {
		return nil, fmt.Errorf("query samehadaku episodes: %w", err)
	}
	defer episodeRows.Close()

	for episodeRows.Next() {
		var (
			animeSlug       string
			episodeSlug     string
			number          float64
			streamPresent   bool
			downloadPresent bool
		)
		if err := episodeRows.Scan(&animeSlug, &episodeSlug, &number, &streamPresent, &downloadPresent); err != nil {
			return nil, fmt.Errorf("scan samehadaku episode: %w", err)
		}
		idx, ok := indexBySlug[animeSlug]
		if !ok {
			continue
		}
		anime[idx].Episodes = append(anime[idx].Episodes, otakudesu.SamehadakuEpisode{
			EpisodeSlug:     episodeSlug,
			Number:          formatEpisodeNumber(number),
			StreamPresent:   streamPresent,
			DownloadPresent: downloadPresent,
		})
	}
	if episodeRows.Err() != nil {
		return nil, fmt.Errorf("iterate samehadaku episodes: %w", episodeRows.Err())
	}

	return anime, nil
}

func formatEpisodeNumber(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func parsePositiveInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseFloatDefault(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseCSVList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func parseBoolDefault(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func parseDurationDefault(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
