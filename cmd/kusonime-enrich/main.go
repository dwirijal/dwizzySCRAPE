package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/kusonime"
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

	limit := parsePositiveInt("KUSONIME_ENRICH_LIMIT", 20)
	minMatchScore := parseFloatDefault("KUSONIME_ENRICH_MIN_MATCH_SCORE", 0.60)
	slugFilter := parseCSVList(os.Getenv("KUSONIME_ENRICH_SLUGS"))
	skipExisting := parseBoolDefault("KUSONIME_ENRICH_SKIP_EXISTING", true)

	anime, err := loadSamehadakuAnime(ctx, pool, limit, slugFilter, skipExisting)
	if err != nil {
		return err
	}

	client := kusonime.NewClient(
		firstNonEmpty(os.Getenv("KUSONIME_BASE_URL"), "https://kusonime.com"),
		firstNonEmpty(os.Getenv("KUSONIME_USER_AGENT"), cfg.UserAgent),
		parseDurationDefault("KUSONIME_HTTP_TIMEOUT", cfg.HTTPTimeout),
	)
	results, err := kusonime.BuildReviews(ctx, client, anime, kusonime.ReviewOptions{
		MinMatchScore: minMatchScore,
	})
	if err != nil {
		return err
	}

	db, err := store.NewPgxContentDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open content db: %w", err)
	}
	defer db.Close()

	writer := store.NewKusonimeEnrichmentStoreWithDB(db)
	scrapedAt := time.Now().UTC()
	report := enrichReport{}

	for _, result := range results {
		if result.MatchStatus != "matched" {
			report.AnimeSkipped++
			fmt.Printf("kusonime enrich skipped: anime=%s reason=%s\n", result.DBAnimeSlug, result.MatchStatus)
			continue
		}
		if len(result.Page.Batches) == 0 {
			report.AnimeSkipped++
			fmt.Printf("kusonime enrich skipped: anime=%s reason=no_batches\n", result.DBAnimeSlug)
			continue
		}
		if skipExisting {
			exists, err := writer.HasAnimeEnrichment(ctx, result.DBAnimeSlug)
			if err != nil {
				return err
			}
			if exists {
				report.AnimeSkipped++
				fmt.Printf("kusonime enrich skipped: anime=%s reason=already_enriched\n", result.DBAnimeSlug)
				continue
			}
		}
		if err := writer.UpsertAnimeEnrichment(ctx, result, scrapedAt); err != nil {
			report.AnimeFailed++
			fmt.Printf("kusonime enrich failed: anime=%s error=%s\n", result.DBAnimeSlug, err)
			continue
		}
		report.AnimeEnriched++
		fmt.Printf("kusonime enrich synced: anime=%s\n", result.DBAnimeSlug)
	}

	fmt.Printf(
		"kusonime enrich done: anime_enriched=%d anime_skipped=%d anime_failed=%d\n",
		report.AnimeEnriched,
		report.AnimeSkipped,
		report.AnimeFailed,
	)
	return nil
}

type enrichReport struct {
	AnimeEnriched int
	AnimeSkipped  int
	AnimeFailed   int
}

func loadSamehadakuAnime(ctx context.Context, pool *pgxpool.Pool, limit int, slugFilter []string, skipExisting bool) ([]kusonime.SamehadakuAnime, error) {
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
	} else if skipExisting {
		query += `
  AND COALESCE(i.detail->'batch_sources'->'kusonime', '{}'::jsonb) = '{}'::jsonb
`
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

	anime := make([]kusonime.SamehadakuAnime, 0, limit)
	for rows.Next() {
		var entry kusonime.SamehadakuAnime
		if err := rows.Scan(&entry.AnimeSlug, &entry.Title, &entry.SourceTitle); err != nil {
			return nil, fmt.Errorf("scan samehadaku anime: %w", err)
		}
		anime = append(anime, entry)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate samehadaku anime: %w", rows.Err())
	}
	return anime, nil
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

func parseCSVList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		values = append(values, part)
	}
	return values
}

func parseDurationDefault(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
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
