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
const DefaultAniListBaseURL = "https://graphql.anilist.co"
const DefaultManhwaindoBaseURL = "https://www.manhwaindo.my"
const DefaultKomikuBaseURL = "https://komiku.org"
const DefaultKanzeninBaseURL = "https://kanzenin.info"
const DefaultAnichinBaseURL = "https://anichin.cafe"
const DefaultBacamanBaseURL = "https://bacaman.id"
const DefaultMangasusukuBaseURL = "https://mangasusuku.com"
const DefaultDrakoridBaseURL = "https://drakorid.co"
const DefaultNekopoiBaseURL = "https://nekopoi.care"
const DefaultHanimeBaseURL = "https://hanime.tv"

type Config struct {
	CatalogURL             string
	KanataMovieTubeBaseURL string
	ManhwaindoBaseURL      string
	ManhwaindoUserAgent    string
	ManhwaindoCookie       string
	KomikuBaseURL          string
	KomikuUserAgent        string
	KomikuCookie           string
	KanzeninBaseURL        string
	KanzeninUserAgent      string
	KanzeninCookie         string
	AnichinBaseURL         string
	AnichinUserAgent       string
	AnichinCookie          string
	DrakoridBaseURL        string
	DrakoridUserAgent      string
	DrakoridCookie         string
	NekopoiBaseURL         string
	NekopoiUserAgent       string
	NekopoiCookie          string
	HanimeBaseURL          string
	HanimeUserAgent        string
	HanimeCookie           string
	HanimeBrowsePaths      []string
	BacamanBaseURL         string
	BacamanUserAgent       string
	BacamanCookie          string
	MangasusukuBaseURL     string
	MangasusukuUserAgent   string
	MangasusukuCookie      string
	SnapshotOutputDir      string
	SnapshotHotLimit       int
	SnapshotCatalogPage    int
	SnapshotMovieGenres    []string
	SnapshotMovieQueries   []string
	DatabaseURL            string
	UserAgent              string
	Cookie                 string
	HTTPTimeout            time.Duration
	JikanBaseURL           string
	TMDBBaseURL            string
	TMDBReadToken          string
	TMDBAPIKey             string
	AniListBaseURL         string
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
		KanzeninBaseURL:        strings.TrimSpace(os.Getenv("KANZENIN_BASE_URL")),
		KanzeninUserAgent:      strings.TrimSpace(os.Getenv("KANZENIN_USER_AGENT")),
		KanzeninCookie:         strings.TrimSpace(os.Getenv("KANZENIN_COOKIE")),
		AnichinBaseURL:         strings.TrimSpace(os.Getenv("ANICHIN_BASE_URL")),
		AnichinUserAgent:       strings.TrimSpace(os.Getenv("ANICHIN_USER_AGENT")),
		AnichinCookie:          strings.TrimSpace(os.Getenv("ANICHIN_COOKIE")),
		DrakoridBaseURL:        strings.TrimSpace(os.Getenv("DRAKORID_BASE_URL")),
		DrakoridUserAgent:      strings.TrimSpace(os.Getenv("DRAKORID_USER_AGENT")),
		DrakoridCookie:         strings.TrimSpace(os.Getenv("DRAKORID_COOKIE")),
		NekopoiBaseURL:         strings.TrimSpace(os.Getenv("NEKOPOI_BASE_URL")),
		NekopoiUserAgent:       strings.TrimSpace(os.Getenv("NEKOPOI_USER_AGENT")),
		NekopoiCookie:          strings.TrimSpace(os.Getenv("NEKOPOI_COOKIE")),
		HanimeBaseURL:          strings.TrimSpace(os.Getenv("HANIME_BASE_URL")),
		HanimeUserAgent:        strings.TrimSpace(os.Getenv("HANIME_USER_AGENT")),
		HanimeCookie:           strings.TrimSpace(os.Getenv("HANIME_COOKIE")),
		HanimeBrowsePaths:      splitCSV(os.Getenv("HANIME_BROWSE_PATHS")),
		BacamanBaseURL:         strings.TrimSpace(os.Getenv("BACAMAN_BASE_URL")),
		BacamanUserAgent:       strings.TrimSpace(os.Getenv("BACAMAN_USER_AGENT")),
		BacamanCookie:          strings.TrimSpace(os.Getenv("BACAMAN_COOKIE")),
		MangasusukuBaseURL:     strings.TrimSpace(os.Getenv("MANGASUSUKU_BASE_URL")),
		MangasusukuUserAgent:   strings.TrimSpace(os.Getenv("MANGASUSUKU_USER_AGENT")),
		MangasusukuCookie:      strings.TrimSpace(os.Getenv("MANGASUSUKU_COOKIE")),
		SnapshotOutputDir:      strings.TrimSpace(os.Getenv("SNAPSHOT_OUTPUT_DIR")),
		SnapshotHotLimit:       parsePositiveInt(os.Getenv("SNAPSHOT_HOT_LIMIT"), 8),
		SnapshotCatalogPage:    parsePositiveInt(os.Getenv("SNAPSHOT_CATALOG_PAGE"), 1),
		SnapshotMovieGenres:    splitCSV(os.Getenv("SNAPSHOT_MOVIE_GENRES")),
		SnapshotMovieQueries:   splitCSV(os.Getenv("SNAPSHOT_MOVIE_SEARCH_QUERIES")),
		DatabaseURL:            strings.TrimSpace(firstNonEmpty("DATABASE_URL", "POSTGRES_URL", "NEON_DATABASE_URL")),
		UserAgent:              strings.TrimSpace(os.Getenv("SAMEHADAKU_USER_AGENT")),
		Cookie:                 strings.TrimSpace(os.Getenv("SAMEHADAKU_COOKIE")),
		HTTPTimeout:            30 * time.Second,
		JikanBaseURL:           strings.TrimSpace(os.Getenv("JIKAN_BASE_URL")),
		TMDBBaseURL:            strings.TrimSpace(os.Getenv("TMDB_BASE_URL")),
		TMDBReadToken:          strings.TrimSpace(os.Getenv("TMDB_READ_TOKEN")),
		TMDBAPIKey:             strings.TrimSpace(os.Getenv("TMDB_API_KEY")),
		AniListBaseURL:         strings.TrimSpace(os.Getenv("ANILIST_BASE_URL")),
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
	if cfg.KanzeninBaseURL == "" {
		cfg.KanzeninBaseURL = DefaultKanzeninBaseURL
	}
	if cfg.AnichinBaseURL == "" {
		cfg.AnichinBaseURL = DefaultAnichinBaseURL
	}
	if cfg.DrakoridBaseURL == "" {
		cfg.DrakoridBaseURL = DefaultDrakoridBaseURL
	}
	if cfg.NekopoiBaseURL == "" {
		cfg.NekopoiBaseURL = DefaultNekopoiBaseURL
	}
	if cfg.HanimeBaseURL == "" {
		cfg.HanimeBaseURL = DefaultHanimeBaseURL
	}
	if cfg.BacamanBaseURL == "" {
		cfg.BacamanBaseURL = DefaultBacamanBaseURL
	}
	if cfg.MangasusukuBaseURL == "" {
		cfg.MangasusukuBaseURL = DefaultMangasusukuBaseURL
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
	if cfg.KanzeninUserAgent == "" {
		cfg.KanzeninUserAgent = cfg.UserAgent
	}
	if cfg.AnichinUserAgent == "" {
		cfg.AnichinUserAgent = cfg.UserAgent
	}
	if cfg.DrakoridUserAgent == "" {
		cfg.DrakoridUserAgent = cfg.UserAgent
	}
	if cfg.NekopoiUserAgent == "" {
		cfg.NekopoiUserAgent = cfg.UserAgent
	}
	if cfg.HanimeUserAgent == "" {
		cfg.HanimeUserAgent = cfg.UserAgent
	}
	if len(cfg.HanimeBrowsePaths) == 0 {
		cfg.HanimeBrowsePaths = []string{"/home", "/browse/trending"}
	}
	if cfg.BacamanUserAgent == "" {
		cfg.BacamanUserAgent = cfg.UserAgent
	}
	if cfg.MangasusukuUserAgent == "" {
		cfg.MangasusukuUserAgent = cfg.UserAgent
	}
	if cfg.JikanBaseURL == "" {
		cfg.JikanBaseURL = "https://api.jikan.moe/v4"
	}
	if cfg.TMDBBaseURL == "" {
		cfg.TMDBBaseURL = DefaultTMDBBaseURL
	}
	if cfg.AniListBaseURL == "" {
		cfg.AniListBaseURL = DefaultAniListBaseURL
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
