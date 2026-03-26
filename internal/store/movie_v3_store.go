package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type MovieV3Store struct {
	client      httpDoer
	supabaseURL string
	secretKey   string
	db          contentDB
}

type MovieCoreRow struct {
	TMDBID           int64
	Slug             string
	Title            string
	OriginalTitle    string
	PosterPath       string
	BackdropPath     string
	Year             *int16
	RuntimeMinutes   *int16
	Rating           float32
	StatusCode       string
	LanguageCode     string
	GenreCodes       []int16
	CountryCodes     []int16
	Overview         string
	Tagline          string
	TrailerYouTubeID string
	MetaSourceCode   string
	UpdatedAt        time.Time
}

type MovieMetaRow struct {
	TMDBID        int64
	CastJSON      []byte
	DirectorNames []string
	AltTitlesJSON []byte
	UpdatedAt     time.Time
}

type MovieProviderRecordRow struct {
	ID                 int64      `json:"id"`
	TMDBID             int64      `json:"tmdb_id"`
	ProviderCode       string     `json:"provider_code"`
	ProviderMovieSlug  string     `json:"provider_movie_slug"`
	ProviderTitle      string     `json:"provider_title"`
	ProviderPosterPath string     `json:"provider_poster_path"`
	ProviderYear       *int16     `json:"provider_year,omitempty"`
	ProviderRating     float32    `json:"provider_rating"`
	QualityCode        string     `json:"quality_code"`
	ScrapeStatusCode   string     `json:"scrape_status_code"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
}

type MovieWatchOptionRow struct {
	TMDBID         int64
	ProviderRecord int64
	ProviderCode   string
	HostCode       string
	Label          string
	EmbedURL       string
	LangCode       string
	QualityCode    string
	Priority       int16
	StatusCode     string
	LastVerifiedAt *time.Time
	UpdatedAt      time.Time
}

type MovieDownloadOptionRow struct {
	TMDBID         int64
	ProviderRecord int64
	ProviderCode   string
	HostCode       string
	Label          string
	DownloadURL    string
	QualityCode    string
	FormatCode     string
	SizeLabel      string
	StatusCode     string
	LastVerifiedAt *time.Time
	UpdatedAt      time.Time
}

type movieCorePayload struct {
	TMDBID           int64   `json:"tmdb_id"`
	Slug             string  `json:"slug"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	Year             *int16  `json:"year,omitempty"`
	RuntimeMinutes   *int16  `json:"runtime_minutes,omitempty"`
	Rating           float32 `json:"rating"`
	StatusCode       string  `json:"status_code"`
	LanguageCode     string  `json:"language_code"`
	GenreCodes       []int16 `json:"genre_codes"`
	CountryCodes     []int16 `json:"country_codes"`
	Overview         string  `json:"overview"`
	Tagline          string  `json:"tagline"`
	TrailerYouTubeID string  `json:"trailer_youtube_id"`
	MetaSourceCode   string  `json:"meta_source_code"`
	UpdatedAt        string  `json:"updated_at"`
}

type movieMetaPayload struct {
	TMDBID        int64           `json:"tmdb_id"`
	CastJSON      json.RawMessage `json:"cast_json"`
	DirectorNames []string        `json:"director_names"`
	AltTitlesJSON json.RawMessage `json:"alt_titles_json"`
	UpdatedAt     string          `json:"updated_at"`
}

type movieProviderPayload struct {
	TMDBID             int64   `json:"tmdb_id"`
	ProviderCode       string  `json:"provider_code"`
	ProviderMovieSlug  string  `json:"provider_movie_slug"`
	ProviderTitle      string  `json:"provider_title"`
	ProviderPosterPath string  `json:"provider_poster_path"`
	ProviderYear       *int16  `json:"provider_year,omitempty"`
	ProviderRating     float32 `json:"provider_rating"`
	QualityCode        string  `json:"quality_code"`
	ScrapeStatusCode   string  `json:"scrape_status_code"`
	LastSeenAt         string  `json:"last_seen_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type movieWatchPayload struct {
	TMDBID         int64   `json:"tmdb_id"`
	ProviderRecord int64   `json:"provider_record_id"`
	ProviderCode   string  `json:"provider_code"`
	HostCode       string  `json:"host_code"`
	Label          string  `json:"label"`
	EmbedURL       string  `json:"embed_url"`
	LangCode       string  `json:"lang_code"`
	QualityCode    string  `json:"quality_code"`
	Priority       int16   `json:"priority"`
	StatusCode     string  `json:"status_code"`
	LastVerifiedAt *string `json:"last_verified_at,omitempty"`
	UpdatedAt      string  `json:"updated_at"`
}

type movieDownloadPayload struct {
	TMDBID         int64   `json:"tmdb_id"`
	ProviderRecord int64   `json:"provider_record_id"`
	ProviderCode   string  `json:"provider_code"`
	HostCode       string  `json:"host_code"`
	Label          string  `json:"label"`
	DownloadURL    string  `json:"download_url"`
	QualityCode    string  `json:"quality_code"`
	FormatCode     string  `json:"format_code"`
	SizeLabel      string  `json:"size_label"`
	StatusCode     string  `json:"status_code"`
	LastVerifiedAt *string `json:"last_verified_at,omitempty"`
	UpdatedAt      string  `json:"updated_at"`
}

func NewMovieV3Store(client httpDoer, supabaseURL, secretKey string) *MovieV3Store {
	if client == nil {
		client = http.DefaultClient
	}
	return &MovieV3Store{
		client:      client,
		supabaseURL: strings.TrimRight(strings.TrimSpace(supabaseURL), "/"),
		secretKey:   strings.TrimSpace(secretKey),
	}
}

func NewMovieV3StoreWithDB(db contentDB) *MovieV3Store {
	return &MovieV3Store{db: db}
}

func (s *MovieV3Store) UpsertMovies(ctx context.Context, rows []MovieCoreRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	payload := make([]movieCorePayload, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, movieCorePayload{
			TMDBID:           row.TMDBID,
			Slug:             normalizeMovieSlug(row.Slug, row.TMDBID),
			Title:            row.Title,
			OriginalTitle:    row.OriginalTitle,
			PosterPath:       row.PosterPath,
			BackdropPath:     row.BackdropPath,
			Year:             row.Year,
			RuntimeMinutes:   row.RuntimeMinutes,
			Rating:           row.Rating,
			StatusCode:       defaultCode(row.StatusCode, "r"),
			LanguageCode:     row.LanguageCode,
			GenreCodes:       defaultSmallintSlice(row.GenreCodes),
			CountryCodes:     defaultSmallintSlice(row.CountryCodes),
			Overview:         row.Overview,
			Tagline:          row.Tagline,
			TrailerYouTubeID: row.TrailerYouTubeID,
			MetaSourceCode:   defaultCode(row.MetaSourceCode, "t"),
			UpdatedAt:        normalizeTimestamp(row.UpdatedAt),
		})
	}

	if s.db != nil {
		return s.upsertMoviesWithDB(ctx, payload)
	}
	if _, err := s.upsert(ctx, "/rest/v1/movies", "tmdb_id", payload, "resolution=merge-duplicates,return=minimal"); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (s *MovieV3Store) UpsertMovieMeta(ctx context.Context, rows []MovieMetaRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	payload := make([]movieMetaPayload, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, movieMetaPayload{
			TMDBID:        row.TMDBID,
			CastJSON:      rawJSONOrFallback(row.CastJSON, []byte("[]")),
			DirectorNames: defaultStringSlice(row.DirectorNames),
			AltTitlesJSON: rawJSONOrFallback(row.AltTitlesJSON, []byte("[]")),
			UpdatedAt:     normalizeTimestamp(row.UpdatedAt),
		})
	}

	if s.db != nil {
		return s.upsertMovieMetaWithDB(ctx, payload)
	}
	if _, err := s.upsert(ctx, "/rest/v1/movie_meta", "tmdb_id", payload, "resolution=merge-duplicates,return=minimal"); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (s *MovieV3Store) UpsertProviderRecords(ctx context.Context, rows []MovieProviderRecordRow) ([]MovieProviderRecordRow, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	payload := make([]movieProviderPayload, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, movieProviderPayload{
			TMDBID:             row.TMDBID,
			ProviderCode:       defaultCode(row.ProviderCode, "u"),
			ProviderMovieSlug:  row.ProviderMovieSlug,
			ProviderTitle:      row.ProviderTitle,
			ProviderPosterPath: row.ProviderPosterPath,
			ProviderYear:       row.ProviderYear,
			ProviderRating:     row.ProviderRating,
			QualityCode:        defaultCode(row.QualityCode, "u"),
			ScrapeStatusCode:   defaultCode(row.ScrapeStatusCode, "x"),
			LastSeenAt:         normalizeTimestamp(derefTime(row.LastSeenAt)),
			UpdatedAt:          normalizeTimestamp(derefTime(row.UpdatedAt)),
		})
	}

	if s.db != nil {
		return s.upsertProviderRecordsWithDB(ctx, payload)
	}
	body, err := s.upsert(ctx, "/rest/v1/movie_provider_records", "provider_code,provider_movie_slug", payload, "resolution=merge-duplicates,return=representation")
	if err != nil {
		return nil, err
	}

	var records []MovieProviderRecordRow
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("decode provider upsert response: %w", err)
	}
	return records, nil
}

func (s *MovieV3Store) UpsertWatchOptions(ctx context.Context, rows []MovieWatchOptionRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	payload := make([]movieWatchPayload, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, movieWatchPayload{
			TMDBID:         row.TMDBID,
			ProviderRecord: row.ProviderRecord,
			ProviderCode:   defaultCode(row.ProviderCode, "u"),
			HostCode:       defaultCode(row.HostCode, "u"),
			Label:          row.Label,
			EmbedURL:       row.EmbedURL,
			LangCode:       row.LangCode,
			QualityCode:    defaultCode(row.QualityCode, "u"),
			Priority:       row.Priority,
			StatusCode:     defaultCode(row.StatusCode, "a"),
			LastVerifiedAt: normalizeOptionalTimestamp(row.LastVerifiedAt),
			UpdatedAt:      normalizeTimestamp(row.UpdatedAt),
		})
	}

	if s.db != nil {
		return s.upsertWatchOptionsWithDB(ctx, payload)
	}
	if _, err := s.upsert(ctx, "/rest/v1/movie_watch_options", "provider_record_id,label,embed_url", payload, "resolution=merge-duplicates,return=minimal"); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (s *MovieV3Store) UpsertDownloadOptions(ctx context.Context, rows []MovieDownloadOptionRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	payload := make([]movieDownloadPayload, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, movieDownloadPayload{
			TMDBID:         row.TMDBID,
			ProviderRecord: row.ProviderRecord,
			ProviderCode:   defaultCode(row.ProviderCode, "u"),
			HostCode:       defaultCode(row.HostCode, "u"),
			Label:          row.Label,
			DownloadURL:    row.DownloadURL,
			QualityCode:    defaultCode(row.QualityCode, "u"),
			FormatCode:     defaultCode(row.FormatCode, "u"),
			SizeLabel:      row.SizeLabel,
			StatusCode:     defaultCode(row.StatusCode, "a"),
			LastVerifiedAt: normalizeOptionalTimestamp(row.LastVerifiedAt),
			UpdatedAt:      normalizeTimestamp(row.UpdatedAt),
		})
	}

	if s.db != nil {
		return s.upsertDownloadOptionsWithDB(ctx, payload)
	}
	if _, err := s.upsert(ctx, "/rest/v1/movie_download_options", "provider_record_id,label,download_url", payload, "resolution=merge-duplicates,return=minimal"); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (s *MovieV3Store) upsertMoviesWithDB(ctx context.Context, payload []movieCorePayload) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("encode upsert payload: %w", err)
	}

	var affected int
	if err := s.db.QueryRow(ctx, `
WITH input AS (
    SELECT *
    FROM jsonb_to_recordset($1::jsonb) AS item(
        tmdb_id bigint,
        slug text,
        title text,
        original_title text,
        poster_path text,
        backdrop_path text,
        year smallint,
        runtime_minutes smallint,
        rating real,
        status_code char(1),
        language_code char(2),
        genre_codes smallint[],
        country_codes smallint[],
        overview text,
        tagline text,
        trailer_youtube_id text,
        meta_source_code char(1),
        updated_at timestamptz
    )
), upserted AS (
    INSERT INTO public.movies (
        tmdb_id, slug, title, original_title, poster_path, backdrop_path, year, runtime_minutes, rating,
        status_code, language_code, genre_codes, country_codes, overview, tagline, trailer_youtube_id,
        meta_source_code, updated_at
    )
    SELECT
        tmdb_id,
        COALESCE(NULLIF(BTRIM(slug), ''), 'movie-' || tmdb_id::text),
        COALESCE(title, ''),
        COALESCE(original_title, ''),
        COALESCE(poster_path, ''),
        COALESCE(backdrop_path, ''),
        year,
        runtime_minutes,
        COALESCE(rating, 0),
        COALESCE(status_code, 'r'),
        COALESCE(language_code, ''),
        COALESCE(genre_codes, '{}'::smallint[]),
        COALESCE(country_codes, '{}'::smallint[]),
        COALESCE(overview, ''),
        COALESCE(tagline, ''),
        COALESCE(trailer_youtube_id, ''),
        COALESCE(meta_source_code, 't'),
        COALESCE(updated_at, now())
    FROM input
    ON CONFLICT (tmdb_id) DO UPDATE
    SET
        slug = CASE
            WHEN NULLIF(BTRIM(public.movies.slug), '') IS NULL THEN EXCLUDED.slug
            WHEN public.movies.slug = 'movie-' || EXCLUDED.tmdb_id::text AND EXCLUDED.slug <> public.movies.slug THEN EXCLUDED.slug
            ELSE public.movies.slug
        END,
        title = EXCLUDED.title,
        original_title = EXCLUDED.original_title,
        poster_path = EXCLUDED.poster_path,
        backdrop_path = EXCLUDED.backdrop_path,
        year = EXCLUDED.year,
        runtime_minutes = EXCLUDED.runtime_minutes,
        rating = EXCLUDED.rating,
        status_code = EXCLUDED.status_code,
        language_code = EXCLUDED.language_code,
        genre_codes = EXCLUDED.genre_codes,
        country_codes = EXCLUDED.country_codes,
        overview = EXCLUDED.overview,
        tagline = EXCLUDED.tagline,
        trailer_youtube_id = EXCLUDED.trailer_youtube_id,
        meta_source_code = EXCLUDED.meta_source_code,
        updated_at = EXCLUDED.updated_at
    RETURNING 1
)
SELECT COUNT(*)::int FROM upserted
`, body).Scan(&affected); err != nil {
		return 0, fmt.Errorf("upsert movies via postgres: %w", err)
	}
	return affected, nil
}

func (s *MovieV3Store) upsertMovieMetaWithDB(ctx context.Context, payload []movieMetaPayload) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("encode upsert payload: %w", err)
	}

	var affected int
	if err := s.db.QueryRow(ctx, `
WITH input AS (
    SELECT *
    FROM jsonb_to_recordset($1::jsonb) AS item(
        tmdb_id bigint,
        cast_json jsonb,
        director_names text[],
        alt_titles_json jsonb,
        updated_at timestamptz
    )
), upserted AS (
    INSERT INTO public.movie_meta (
        tmdb_id, cast_json, director_names, alt_titles_json, updated_at
    )
    SELECT
        tmdb_id,
        COALESCE(cast_json, '[]'::jsonb),
        COALESCE(director_names, '{}'::text[]),
        COALESCE(alt_titles_json, '[]'::jsonb),
        COALESCE(updated_at, now())
    FROM input
    ON CONFLICT (tmdb_id) DO UPDATE
    SET
        cast_json = EXCLUDED.cast_json,
        director_names = EXCLUDED.director_names,
        alt_titles_json = EXCLUDED.alt_titles_json,
        updated_at = EXCLUDED.updated_at
    RETURNING 1
)
SELECT COUNT(*)::int FROM upserted
`, body).Scan(&affected); err != nil {
		return 0, fmt.Errorf("upsert movie meta via postgres: %w", err)
	}
	return affected, nil
}

func normalizeMovieSlug(raw string, tmdbID int64) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed != "" {
		return trimmed
	}
	if tmdbID > 0 {
		return fmt.Sprintf("movie-%d", tmdbID)
	}
	return "movie"
}

func (s *MovieV3Store) upsertProviderRecordsWithDB(ctx context.Context, payload []movieProviderPayload) ([]MovieProviderRecordRow, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode upsert payload: %w", err)
	}

	var response []byte
	if err := s.db.QueryRow(ctx, `
WITH input AS (
    SELECT *
    FROM jsonb_to_recordset($1::jsonb) AS item(
        tmdb_id bigint,
        provider_code char(1),
        provider_movie_slug text,
        provider_title text,
        provider_poster_path text,
        provider_year smallint,
        provider_rating real,
        quality_code char(1),
        scrape_status_code char(1),
        last_seen_at timestamptz,
        updated_at timestamptz
    )
), upserted AS (
    INSERT INTO public.movie_provider_records (
        tmdb_id, provider_code, provider_movie_slug, provider_title, provider_poster_path, provider_year,
        provider_rating, quality_code, scrape_status_code, last_seen_at, updated_at
    )
    SELECT
        tmdb_id,
        COALESCE(provider_code, 'u'),
        provider_movie_slug,
        COALESCE(provider_title, ''),
        COALESCE(provider_poster_path, ''),
        provider_year,
        COALESCE(provider_rating, 0),
        COALESCE(quality_code, 'u'),
        COALESCE(scrape_status_code, 'x'),
        COALESCE(last_seen_at, now()),
        COALESCE(updated_at, now())
    FROM input
    ON CONFLICT (provider_code, provider_movie_slug) DO UPDATE
    SET
        tmdb_id = EXCLUDED.tmdb_id,
        provider_title = EXCLUDED.provider_title,
        provider_poster_path = EXCLUDED.provider_poster_path,
        provider_year = EXCLUDED.provider_year,
        provider_rating = EXCLUDED.provider_rating,
        quality_code = EXCLUDED.quality_code,
        scrape_status_code = EXCLUDED.scrape_status_code,
        last_seen_at = EXCLUDED.last_seen_at,
        updated_at = EXCLUDED.updated_at
    RETURNING id, tmdb_id, provider_code, provider_movie_slug, provider_title, provider_poster_path, provider_year,
        provider_rating, quality_code, scrape_status_code, last_seen_at, updated_at
)
SELECT COALESCE(json_agg(row_to_json(upserted)), '[]'::json)::text FROM upserted
`, body).Scan(&response); err != nil {
		return nil, fmt.Errorf("upsert provider records via postgres: %w", err)
	}

	var rows []MovieProviderRecordRow
	if err := json.Unmarshal(response, &rows); err != nil {
		return nil, fmt.Errorf("decode provider upsert response: %w", err)
	}
	return rows, nil
}

func (s *MovieV3Store) upsertWatchOptionsWithDB(ctx context.Context, payload []movieWatchPayload) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("encode upsert payload: %w", err)
	}

	var affected int
	if err := s.db.QueryRow(ctx, `
WITH input AS (
    SELECT *
    FROM jsonb_to_recordset($1::jsonb) AS item(
        tmdb_id bigint,
        provider_record_id bigint,
        provider_code char(1),
        host_code char(1),
        label text,
        embed_url text,
        lang_code char(2),
        quality_code char(1),
        priority smallint,
        status_code char(1),
        last_verified_at timestamptz,
        updated_at timestamptz
    )
), upserted AS (
    INSERT INTO public.movie_watch_options (
        tmdb_id, provider_record_id, provider_code, host_code, label, embed_url, lang_code,
        quality_code, priority, status_code, last_verified_at, updated_at
    )
    SELECT
        tmdb_id,
        provider_record_id,
        COALESCE(provider_code, 'u'),
        COALESCE(host_code, 'u'),
        COALESCE(label, ''),
        COALESCE(embed_url, ''),
        COALESCE(lang_code, ''),
        COALESCE(quality_code, 'u'),
        COALESCE(priority, 0),
        COALESCE(status_code, 'a'),
        last_verified_at,
        COALESCE(updated_at, now())
    FROM input
    ON CONFLICT (provider_record_id, label, embed_url) DO UPDATE
    SET
        tmdb_id = EXCLUDED.tmdb_id,
        provider_code = EXCLUDED.provider_code,
        host_code = EXCLUDED.host_code,
        lang_code = EXCLUDED.lang_code,
        quality_code = EXCLUDED.quality_code,
        priority = EXCLUDED.priority,
        status_code = EXCLUDED.status_code,
        last_verified_at = EXCLUDED.last_verified_at,
        updated_at = EXCLUDED.updated_at
    RETURNING 1
)
SELECT COUNT(*)::int FROM upserted
`, body).Scan(&affected); err != nil {
		return 0, fmt.Errorf("upsert watch options via postgres: %w", err)
	}
	return affected, nil
}

func (s *MovieV3Store) upsertDownloadOptionsWithDB(ctx context.Context, payload []movieDownloadPayload) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("encode upsert payload: %w", err)
	}

	var affected int
	if err := s.db.QueryRow(ctx, `
WITH input AS (
    SELECT *
    FROM jsonb_to_recordset($1::jsonb) AS item(
        tmdb_id bigint,
        provider_record_id bigint,
        provider_code char(1),
        host_code char(1),
        label text,
        download_url text,
        quality_code char(1),
        format_code char(1),
        size_label text,
        status_code char(1),
        last_verified_at timestamptz,
        updated_at timestamptz
    )
), upserted AS (
    INSERT INTO public.movie_download_options (
        tmdb_id, provider_record_id, provider_code, host_code, label, download_url, quality_code,
        format_code, size_label, status_code, last_verified_at, updated_at
    )
    SELECT
        tmdb_id,
        provider_record_id,
        COALESCE(provider_code, 'u'),
        COALESCE(host_code, 'u'),
        COALESCE(label, ''),
        COALESCE(download_url, ''),
        COALESCE(quality_code, 'u'),
        COALESCE(format_code, 'u'),
        COALESCE(size_label, ''),
        COALESCE(status_code, 'a'),
        last_verified_at,
        COALESCE(updated_at, now())
    FROM input
    ON CONFLICT (provider_record_id, label, download_url) DO UPDATE
    SET
        tmdb_id = EXCLUDED.tmdb_id,
        provider_code = EXCLUDED.provider_code,
        host_code = EXCLUDED.host_code,
        quality_code = EXCLUDED.quality_code,
        format_code = EXCLUDED.format_code,
        size_label = EXCLUDED.size_label,
        status_code = EXCLUDED.status_code,
        last_verified_at = EXCLUDED.last_verified_at,
        updated_at = EXCLUDED.updated_at
    RETURNING 1
)
SELECT COUNT(*)::int FROM upserted
`, body).Scan(&affected); err != nil {
		return 0, fmt.Errorf("upsert download options via postgres: %w", err)
	}
	return affected, nil
}

func (s *MovieV3Store) upsert(ctx context.Context, path, conflictTarget string, payload any, prefer string) ([]byte, error) {
	if s.db != nil {
		return nil, fmt.Errorf("http upsert path is unavailable when postgres db is configured")
	}
	if s.client == nil {
		return nil, fmt.Errorf("http client is required")
	}
	if s.supabaseURL == "" {
		return nil, fmt.Errorf("supabase url is required")
	}
	if s.secretKey == "" {
		return nil, fmt.Errorf("supabase secret key is required")
	}

	endpoint, err := url.Parse(s.supabaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("build upsert endpoint: %w", err)
	}
	query := endpoint.Query()
	if strings.TrimSpace(conflictTarget) != "" {
		query.Set("on_conflict", conflictTarget)
	}
	endpoint.RawQuery = query.Encode()

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode upsert payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build upsert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", s.secretKey)
	req.Header.Set("Authorization", "Bearer "+s.secretKey)
	req.Header.Set("Prefer", prefer)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform upsert request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upsert response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("supabase upsert failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func defaultCode(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func defaultSmallintSlice(value []int16) []int16 {
	if len(value) == 0 {
		return []int16{}
	}
	return value
}

func defaultStringSlice(value []string) []string {
	if len(value) == 0 {
		return []string{}
	}
	return value
}

func normalizeTimestamp(value time.Time) string {
	if value.IsZero() {
		value = time.Now().UTC()
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func normalizeOptionalTimestamp(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
