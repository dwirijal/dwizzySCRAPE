package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/linkresolver"
	"github.com/jackc/pgx/v5/pgxpool"
)

var urlPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

type sourceURL struct {
	SourceType string `json:"source_type"`
	Slug       string `json:"slug"`
	URL        string `json:"url"`
}

type auditEntry struct {
	SourceType  string   `json:"source_type"`
	Slug        string   `json:"slug"`
	OriginalURL string   `json:"original_url"`
	FinalURL    string   `json:"final_url"`
	FinalHost   string   `json:"final_host"`
	StatusCode  int      `json:"status_code"`
	Hops        []string `json:"hops"`
	Error       string   `json:"error,omitempty"`
}

type report struct {
	GeneratedAt string                    `json:"generated_at"`
	Summary     map[string]any            `json:"summary"`
	Entries     []auditEntry              `json:"entries"`
	Hosts       map[string]map[string]int `json:"hosts"`
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	limitPerSource := parsePositiveIntEnv("DOWNLOAD_RESOLVER_AUDIT_LIMIT_PER_SOURCE", 10)
	maxURLs := parsePositiveIntEnv("DOWNLOAD_RESOLVER_AUDIT_MAX_URLS", 50)
	outputDir := strings.TrimSpace(os.Getenv("DOWNLOAD_RESOLVER_AUDIT_OUTPUT_DIR"))
	if outputDir == "" {
		outputDir = "artifacts"
	}

	samehadakuURLs, err := loadURLSamples(ctx, pool, "samehadaku", limitPerSource)
	if err != nil {
		return err
	}
	kusonimeURLs, err := loadURLSamples(ctx, pool, "kusonime", limitPerSource)
	if err != nil {
		return err
	}

	samples := append(samehadakuURLs, kusonimeURLs...)
	if len(samples) > maxURLs {
		samples = samples[:maxURLs]
	}

	resolver := linkresolver.New(cfg.UserAgent, cfg.HTTPTimeout)
	entries := make([]auditEntry, 0, len(samples))
	hostSummary := map[string]map[string]int{}
	for _, sample := range samples {
		resolution := resolver.Resolve(ctx, sample.URL)
		entry := auditEntry{
			SourceType:  sample.SourceType,
			Slug:        sample.Slug,
			OriginalURL: sample.URL,
			FinalURL:    resolution.FinalURL,
			FinalHost:   resolution.FinalHost,
			StatusCode:  resolution.StatusCode,
			Hops:        resolution.Hops,
			Error:       resolution.Error,
		}
		entries = append(entries, entry)

		host := resolution.FinalHost
		if host == "" {
			host = "unresolved"
		}
		if _, ok := hostSummary[sample.SourceType]; !ok {
			hostSummary[sample.SourceType] = map[string]int{}
		}
		hostSummary[sample.SourceType][host]++
	}

	report := report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Summary: map[string]any{
			"sample_count":       len(entries),
			"limit_per_source":   limitPerSource,
			"max_urls":           maxURLs,
			"samehadaku_samples": len(samehadakuURLs),
			"kusonime_samples":   len(kusonimeURLs),
		},
		Entries: entries,
		Hosts:   hostSummary,
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := writeJSON(filepath.Join(outputDir, "download-resolver-audit.json"), report); err != nil {
		return err
	}
	if err := writeCSV(filepath.Join(outputDir, "download-resolver-audit.csv"), entries); err != nil {
		return err
	}

	fmt.Printf("download resolver audit written: %s\n", filepath.Join(outputDir, "download-resolver-audit.json"))
	fmt.Printf("download resolver audit written: %s\n", filepath.Join(outputDir, "download-resolver-audit.csv"))
	return nil
}

func loadURLSamples(ctx context.Context, pool *pgxpool.Pool, sourceType string, limit int) ([]sourceURL, error) {
	switch sourceType {
	case "samehadaku":
		return loadSamehadakuURLSamples(ctx, pool, limit)
	case "kusonime":
		return loadKusonimeURLSamples(ctx, pool, limit)
	default:
		return nil, fmt.Errorf("unknown source type %q", sourceType)
	}
}

func loadSamehadakuURLSamples(ctx context.Context, pool *pgxpool.Pool, limit int) ([]sourceURL, error) {
	rows, err := pool.Query(ctx, `
SELECT slug, COALESCE((detail->'download_links_json')::text, '{}'::text)
FROM public.media_units
WHERE source = 'samehadaku'
  AND unit_type = 'episode'
  AND COALESCE(detail->'download_links_json', '{}'::jsonb) <> '{}'::jsonb
ORDER BY updated_at DESC, slug ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("query samehadaku samples: %w", err)
	}
	defer rows.Close()
	return collectURLs(rows, "samehadaku")
}

func loadKusonimeURLSamples(ctx context.Context, pool *pgxpool.Pool, limit int) ([]sourceURL, error) {
	rows, err := pool.Query(ctx, `
SELECT slug, COALESCE((detail->'batch_sources'->'kusonime')::text, '{}'::text)
FROM public.media_items
WHERE source = 'samehadaku'
  AND media_type = 'anime'
  AND COALESCE(detail->'batch_sources'->'kusonime', '{}'::jsonb) <> '{}'::jsonb
ORDER BY updated_at DESC, slug ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, fmt.Errorf("query kusonime samples: %w", err)
	}
	defer rows.Close()
	return collectURLs(rows, "kusonime")
}

type simpleRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func collectURLs(rows simpleRows, sourceType string) ([]sourceURL, error) {
	var samples []sourceURL
	for rows.Next() {
		var (
			slug    string
			payload string
		)
		if err := rows.Scan(&slug, &payload); err != nil {
			return nil, err
		}
		urls := extractURLs(payload)
		sort.Strings(urls)
		for _, rawURL := range urls {
			samples = append(samples, sourceURL{
				SourceType: sourceType,
				Slug:       slug,
				URL:        rawURL,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return samples, nil
}

func extractURLs(raw string) []string {
	matches := urlPattern.FindAllString(raw, -1)
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		match = strings.TrimSpace(match)
		match = strings.TrimRight(match, `\`)
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		out = append(out, match)
	}
	return out
}

func writeJSON(path string, payload any) error {
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json report: %w", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write json report: %w", err)
	}
	return nil
}

func writeCSV(path string, entries []auditEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv report: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"source_type", "slug", "original_url", "final_url", "final_host", "status_code", "hop_count", "error"}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}
	for _, entry := range entries {
		row := []string{
			entry.SourceType,
			entry.Slug,
			entry.OriginalURL,
			entry.FinalURL,
			entry.FinalHost,
			fmt.Sprintf("%d", entry.StatusCode),
			fmt.Sprintf("%d", len(entry.Hops)),
			entry.Error,
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	return nil
}

func parsePositiveIntEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil || value <= 0 {
		return fallback
	}
	return value
}
