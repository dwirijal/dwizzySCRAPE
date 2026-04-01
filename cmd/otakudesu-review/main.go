package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/otakudesu"
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

	limit := parsePositiveInt("OTAKUDESU_REVIEW_LIMIT", 20)
	minMatchScore := parseFloatDefault("OTAKUDESU_REVIEW_MIN_MATCH_SCORE", 0.55)
	slugFilter := parseCSVList(os.Getenv("OTAKUDESU_REVIEW_SLUGS"))
	outputDir := strings.TrimSpace(os.Getenv("OTAKUDESU_OUTPUT_DIR"))
	if outputDir == "" {
		outputDir = "artifacts"
	}

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
	})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	jsonPath := filepath.Join(outputDir, "otakudesu-review.json")
	csvPath := filepath.Join(outputDir, "otakudesu-review.csv")
	if err := writeJSON(jsonPath, results); err != nil {
		return err
	}
	if err := writeCSV(csvPath, results); err != nil {
		return err
	}

	fmt.Printf("otakudesu review done: anime=%d json=%s csv=%s\n", len(results), jsonPath, csvPath)
	return nil
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

func writeJSON(path string, results []otakudesu.ReviewResult) error {
	body, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal review json: %w", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write review json: %w", err)
	}
	return nil
}

func writeCSV(path string, results []otakudesu.ReviewResult) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create review csv: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"db_anime_slug",
		"db_title",
		"db_source_title",
		"query",
		"match_status",
		"match_score",
		"needs_review",
		"matched_title",
		"matched_url",
		"notes",
		"db_episode_slug",
		"episode_number",
		"otakudesu_episode_url",
		"stream_url",
		"download_url",
		"samehadaku_stream_present",
		"samehadaku_download_present",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, result := range results {
		if len(result.Episodes) == 0 {
			if err := writer.Write([]string{
				result.DBAnimeSlug,
				result.DBTitle,
				result.DBSourceTitle,
				result.Query,
				result.MatchStatus,
				formatScore(result.MatchScore),
				formatBool(result.NeedsReview),
				result.MatchedTitle,
				result.MatchedURL,
				result.Notes,
				"", "", "", "", "", "", "",
			}); err != nil {
				return fmt.Errorf("write csv row: %w", err)
			}
			continue
		}

		for _, episode := range result.Episodes {
			if err := writer.Write([]string{
				result.DBAnimeSlug,
				result.DBTitle,
				result.DBSourceTitle,
				result.Query,
				result.MatchStatus,
				formatScore(result.MatchScore),
				formatBool(result.NeedsReview),
				result.MatchedTitle,
				result.MatchedURL,
				result.Notes,
				episode.DBEpisodeSlug,
				episode.EpisodeNumber,
				episode.OtakudesuEpisodeURL,
				episode.StreamURL,
				episode.DownloadURL,
				formatBool(episode.SamehadakuStreamPresent),
				formatBool(episode.SamehadakuDownloadPresent),
			}); err != nil {
				return fmt.Errorf("write csv row: %w", err)
			}
		}
	}

	return nil
}

func formatEpisodeNumber(value float64) string {
	if value == float64(int64(value)) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 3, 64), "0"), ".")
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

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func formatScore(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func parseCSVList(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
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
