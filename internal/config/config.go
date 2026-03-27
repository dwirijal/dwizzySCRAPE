package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultCatalogURL = "https://v2.samehadaku.how/daftar-anime-2/"
const DefaultKanataMovieTubeBaseURL = "https://api.kanata.web.id/movietube"
const DefaultTMDBBaseURL = "https://api.themoviedb.org/3"
const DefaultManhwaindoBaseURL = "https://www.manhwaindo.my"
const DefaultKomikuBaseURL = "https://komiku.org"

type Config struct {
	CatalogURL             string
	KanataMovieTubeBaseURL string
	ManhwaindoBaseURL      string
	ManhwaindoUserAgent    string
	ManhwaindoCookie       string
	KomikuBaseURL          string
	KomikuUserAgent        string
	KomikuCookie           string
	SnapshotOutputDir      string
	SnapshotHotLimit       int
	SnapshotCatalogPage    int
	SnapshotMovieGenres    []string
	SnapshotMovieQueries   []string
	PostgresURL            string
	UserAgent              string
	Cookie                 string
	HTTPTimeout            time.Duration
	JikanBaseURL           string
	TMDBBaseURL            string
	TMDBReadToken          string
	TMDBAPIKey             string
}

func Load() (Config, error) {
	cfg := Config{
		CatalogURL:             strings.TrimSpace(os.Getenv("SAMEHADAKU_CATALOG_URL")),
		KanataMovieTubeBaseURL: strings.TrimSpace(os.Getenv("KANATA_MOVIETUBE_BASE_URL")),
		ManhwaindoBaseURL:      strings.TrimSpace(os.Getenv("MANHWAINDO_BASE_URL")),
		ManhwaindoUserAgent:    strings.TrimSpace(os.Getenv("MANHWAINDO_USER_AGENT")),
		ManhwaindoCookie:       strings.TrimSpace(os.Getenv("MANHWAINDO_COOKIE")),
		KomikuBaseURL:          strings.TrimSpace(os.Getenv("KOMIKU_BASE_URL")),
		KomikuUserAgent:        strings.TrimSpace(os.Getenv("KOMIKU_USER_AGENT")),
		KomikuCookie:           strings.TrimSpace(os.Getenv("KOMIKU_COOKIE")),
		SnapshotOutputDir:      strings.TrimSpace(os.Getenv("SNAPSHOT_OUTPUT_DIR")),
		SnapshotHotLimit:       parsePositiveInt(os.Getenv("SNAPSHOT_HOT_LIMIT"), 8),
		SnapshotCatalogPage:    parsePositiveInt(os.Getenv("SNAPSHOT_CATALOG_PAGE"), 1),
		SnapshotMovieGenres:    splitCSV(os.Getenv("SNAPSHOT_MOVIE_GENRES")),
		SnapshotMovieQueries:   splitCSV(os.Getenv("SNAPSHOT_MOVIE_SEARCH_QUERIES")),
		PostgresURL:            strings.TrimSpace(firstNonEmpty("POSTGRES_URL", "DATABASE_URL", "NEON_DATABASE_URL")),
		UserAgent:              strings.TrimSpace(os.Getenv("SAMEHADAKU_USER_AGENT")),
		Cookie:                 strings.TrimSpace(os.Getenv("SAMEHADAKU_COOKIE")),
		HTTPTimeout:            30 * time.Second,
		JikanBaseURL:           strings.TrimSpace(os.Getenv("JIKAN_BASE_URL")),
		TMDBBaseURL:            strings.TrimSpace(os.Getenv("TMDB_BASE_URL")),
		TMDBReadToken:          strings.TrimSpace(os.Getenv("TMDB_READ_TOKEN")),
		TMDBAPIKey:             strings.TrimSpace(os.Getenv("TMDB_API_KEY")),
	}
	if cfg.CatalogURL == "" {
		cfg.CatalogURL = DefaultCatalogURL
	}
	if cfg.KanataMovieTubeBaseURL == "" {
		cfg.KanataMovieTubeBaseURL = DefaultKanataMovieTubeBaseURL
	}
	if cfg.ManhwaindoBaseURL == "" {
		cfg.ManhwaindoBaseURL = DefaultManhwaindoBaseURL
	}
	if cfg.KomikuBaseURL == "" {
		cfg.KomikuBaseURL = DefaultKomikuBaseURL
	}
	if cfg.SnapshotOutputDir == "" {
		cfg.SnapshotOutputDir = "snapshots"
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	}
	if cfg.ManhwaindoUserAgent == "" {
		cfg.ManhwaindoUserAgent = cfg.UserAgent
	}
	if cfg.KomikuUserAgent == "" {
		cfg.KomikuUserAgent = cfg.UserAgent
	}
	if cfg.JikanBaseURL == "" {
		cfg.JikanBaseURL = "https://api.jikan.moe/v4"
	}
	if cfg.TMDBBaseURL == "" {
		cfg.TMDBBaseURL = DefaultTMDBBaseURL
	}
	if raw := strings.TrimSpace(os.Getenv("SAMEHADAKU_HTTP_TIMEOUT")); raw != "" {
		timeout, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse SAMEHADAKU_HTTP_TIMEOUT: %w", err)
		}
		cfg.HTTPTimeout = timeout
	}
	return cfg, nil
}

func firstNonEmpty(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func parsePositiveInt(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}
