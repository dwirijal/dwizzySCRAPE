package kanata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/store"
	"github.com/dwirijal/dwizzySCRAPE/internal/tmdb"
)

var nonSlugRunesPattern = regexp.MustCompile(`[^a-z0-9]+`)
var trailingYearPattern = regexp.MustCompile(`\s*\((19|20)\d{2}\)\s*$`)
var trailingAliasPattern = regexp.MustCompile(`(?i)\s*\(([^)]*\baka\b[^)]*)\)\s*$`)

type MovieV3Service struct {
	client     *Client
	tmdbClient *tmdb.Client
	store      movieV3Store
	fixedNow   time.Time
}

type movieV3Store interface {
	UpsertMovies(ctx context.Context, rows []store.MovieCoreRow) (int, error)
	UpsertMovieMeta(ctx context.Context, rows []store.MovieMetaRow) (int, error)
	UpsertProviderRecords(ctx context.Context, rows []store.MovieProviderRecordRow) ([]store.MovieProviderRecordRow, error)
	UpsertWatchOptions(ctx context.Context, rows []store.MovieWatchOptionRow) (int, error)
	UpsertDownloadOptions(ctx context.Context, rows []store.MovieDownloadOptionRow) (int, error)
}

type SyncHomeReport struct {
	Discovered     int
	Matched        int
	Upserted       int
	Failed         int
	Failures       map[string]string
	FailureCodes   map[string]string
	FailureDetails map[string]MovieSyncFailure
}

type MovieSyncFailure struct {
	Code    string
	Message string
}

type movieSyncError struct {
	code string
	err  error
}

func (e *movieSyncError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *movieSyncError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func withMovieSyncCode(code string, err error) error {
	if err == nil {
		return nil
	}
	var coded *movieSyncError
	if errors.As(err, &coded) {
		return err
	}
	return &movieSyncError{
		code: strings.TrimSpace(code),
		err:  err,
	}
}

func movieSyncFailureFromErr(err error) MovieSyncFailure {
	if err == nil {
		return MovieSyncFailure{
			Code:    "unknown_error",
			Message: "",
		}
	}
	var coded *movieSyncError
	if errors.As(err, &coded) && strings.TrimSpace(coded.code) != "" {
		return MovieSyncFailure{
			Code:    coded.code,
			Message: coded.Error(),
		}
	}
	return MovieSyncFailure{
		Code:    "unknown_error",
		Message: err.Error(),
	}
}

func NewMovieV3Service(client *Client, tmdbClient *tmdb.Client, store movieV3Store, fixedNow time.Time) *MovieV3Service {
	return &MovieV3Service{
		client:     client,
		tmdbClient: tmdbClient,
		store:      store,
		fixedNow:   fixedNow,
	}
}

func (s *MovieV3Service) SyncHome(ctx context.Context, limit int) (SyncHomeReport, error) {
	if s.client == nil {
		return SyncHomeReport{}, fmt.Errorf("kanata client is required")
	}
	if s.tmdbClient == nil || !s.tmdbClient.Enabled() {
		return SyncHomeReport{}, fmt.Errorf("tmdb client with credentials is required")
	}
	if s.store == nil {
		return SyncHomeReport{}, fmt.Errorf("movie v3 store is required")
	}

	items, err := s.client.GetHome(ctx)
	if err != nil {
		return SyncHomeReport{}, err
	}

	return s.syncItems(ctx, items, limit)
}

func (s *MovieV3Service) SyncGenre(ctx context.Context, genre string, page, limit int) (SyncHomeReport, error) {
	if s.client == nil {
		return SyncHomeReport{}, fmt.Errorf("kanata client is required")
	}
	items, err := s.client.GetGenre(ctx, genre, page)
	if err != nil {
		return SyncHomeReport{}, err
	}

	return s.syncItems(ctx, items, limit)
}

func (s *MovieV3Service) SyncSearch(ctx context.Context, query string, page, limit int) (SyncHomeReport, error) {
	if s.client == nil {
		return SyncHomeReport{}, fmt.Errorf("kanata client is required")
	}
	items, err := s.client.Search(ctx, query, page)
	if err != nil {
		return SyncHomeReport{}, err
	}

	return s.syncItems(ctx, items, limit)
}

func (s *MovieV3Service) syncItems(ctx context.Context, items []HomeMovie, limit int) (SyncHomeReport, error) {
	if s.client == nil {
		return SyncHomeReport{}, fmt.Errorf("kanata client is required")
	}
	if s.tmdbClient == nil || !s.tmdbClient.Enabled() {
		return SyncHomeReport{}, fmt.Errorf("tmdb client with credentials is required")
	}
	if s.store == nil {
		return SyncHomeReport{}, fmt.Errorf("movie v3 store is required")
	}

	items = dedupeHomeMovies(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	report := SyncHomeReport{
		Discovered:     len(items),
		Failures:       make(map[string]string),
		FailureCodes:   make(map[string]string),
		FailureDetails: make(map[string]MovieSyncFailure),
	}

	for _, item := range items {
		if err := s.syncHomeItem(ctx, item); err != nil {
			report.Failed++
			failure := movieSyncFailureFromErr(err)
			report.Failures[item.Slug] = failure.Message
			report.FailureCodes[item.Slug] = failure.Code
			report.FailureDetails[item.Slug] = failure
			continue
		}
		report.Matched++
		report.Upserted++
	}

	return report, nil
}

func (s *MovieV3Service) syncHomeItem(ctx context.Context, item HomeMovie) error {
	now := s.now()
	cleanTitle := cleanMovieTitle(item.Title)
	if cleanTitle == "" {
		return withMovieSyncCode("empty_title", fmt.Errorf("empty title"))
	}

	year := parseYear(item.Year)
	searchQueries := movieSearchQueries(item.Title)
	if len(searchQueries) == 0 {
		searchQueries = []string{cleanTitle}
	}

	var results []tmdb.SearchHit
	var err error
	for _, searchQuery := range searchQueries {
		results, err = s.tmdbClient.SearchMovies(ctx, searchQuery, year, 8)
		if err != nil {
			return withMovieSyncCode("tmdb_search_error", fmt.Errorf("tmdb search: %w", err))
		}
		if len(results) > 0 {
			break
		}
	}

	matchResult := tmdb.PickBestMovieMatchResult(cleanTitle, year, results)
	if !matchResult.Matched {
		return withMovieSyncCode("tmdb_match_"+string(matchResult.Reason), fmt.Errorf(
			"no tmdb match found: reason=%s query=%q year=%d candidates=%d best_score=%d",
			matchResult.Reason,
			cleanTitle,
			year,
			matchResult.CandidateCount,
			matchResult.BestScore,
		))
	}

	detail, err := s.tmdbClient.GetMovieDetail(ctx, matchResult.Hit.ID)
	if err != nil {
		return withMovieSyncCode("tmdb_detail_error", fmt.Errorf("tmdb detail: %w", err))
	}

	providerDetail, err := s.client.GetDetail(ctx, item.Slug)
	if err != nil {
		return withMovieSyncCode("provider_detail_error", fmt.Errorf("kanata detail: %w", err))
	}

	stream, err := s.client.GetStream(ctx, item.Slug)
	if err != nil {
		return withMovieSyncCode("provider_stream_error", fmt.Errorf("kanata stream: %w", err))
	}

	canonicalYear := yearFromTMDB(detail)
	if canonicalYear == 0 {
		canonicalYear = year
	}

	core := store.MovieCoreRow{
		TMDBID:           int64(detail.ID),
		Title:            strings.TrimSpace(detail.Title),
		OriginalTitle:    strings.TrimSpace(detail.OriginalTitle),
		PosterPath:       chooseFirstNonEmpty(detail.PosterPath, strings.TrimSpace(item.Poster)),
		BackdropPath:     strings.TrimSpace(detail.BackdropPath),
		Year:             smallintPtr(canonicalYear),
		RuntimeMinutes:   smallintPtr(firstPositive(detail.Runtime, parseDurationMinutes(item.Duration))),
		Rating:           float32(firstPositiveFloat(detail.VoteAverage, item.Rating)),
		StatusCode:       movieStatusCode(detail.Status),
		LanguageCode:     normalizeLanguageCode(detail.OriginalLanguage),
		GenreCodes:       genreIDs(detail),
		CountryCodes:     []int16{},
		Overview:         chooseFirstNonEmpty(strings.TrimSpace(detail.Overview), strings.TrimSpace(providerDetail.Synopsis)),
		Tagline:          strings.TrimSpace(detail.Tagline),
		TrailerYouTubeID: youtubeIDFromURL(tmdb.PickTrailerURL(detail)),
		MetaSourceCode:   "t",
		UpdatedAt:        now,
	}
	if core.Title == "" {
		core.Title = cleanTitle
	}
	if core.OriginalTitle == "" {
		core.OriginalTitle = core.Title
	}
	if core.Overview == "" {
		core.MetaSourceCode = "s"
	}
	core.Slug = buildCanonicalMovieSlug(core.Title, canonicalYear, int64(detail.ID))

	castJSON, err := json.Marshal(tmdb.CastNameObjects(detail, 10))
	if err != nil {
		return fmt.Errorf("encode cast json: %w", err)
	}

	meta := store.MovieMetaRow{
		TMDBID:        int64(detail.ID),
		CastJSON:      castJSON,
		DirectorNames: tmdb.PickDirectorNames(detail),
		AltTitlesJSON: []byte("[]"),
		UpdatedAt:     now,
	}

	_, err = s.store.UpsertMovies(ctx, []store.MovieCoreRow{core})
	if err != nil {
		return withMovieSyncCode("store_upsert_movies_error", fmt.Errorf("upsert movies: %w", err))
	}
	_, err = s.store.UpsertMovieMeta(ctx, []store.MovieMetaRow{meta})
	if err != nil {
		return withMovieSyncCode("store_upsert_movie_meta_error", fmt.Errorf("upsert movie meta: %w", err))
	}

	providers, err := s.store.UpsertProviderRecords(ctx, []store.MovieProviderRecordRow{
		{
			TMDBID:             int64(detail.ID),
			ProviderCode:       "k",
			ProviderMovieSlug:  strings.TrimSpace(item.Slug),
			ProviderTitle:      strings.TrimSpace(providerDetail.Title),
			ProviderPosterPath: strings.TrimSpace(item.Poster),
			ProviderYear:       smallintPtr(year),
			ProviderRating:     float32(item.Rating),
			QualityCode:        movieQualityCode(item.Quality),
			ScrapeStatusCode:   "a",
			LastSeenAt:         &now,
			UpdatedAt:          &now,
		},
	})
	if err != nil {
		return withMovieSyncCode("store_upsert_provider_record_error", fmt.Errorf("upsert provider record: %w", err))
	}
	if len(providers) == 0 {
		return withMovieSyncCode("store_provider_record_empty", fmt.Errorf("provider record response is empty"))
	}

	if strings.TrimSpace(stream.StreamURL) == "" {
		return nil
	}

	_, err = s.store.UpsertWatchOptions(ctx, []store.MovieWatchOptionRow{
		{
			TMDBID:         int64(detail.ID),
			ProviderRecord: providers[0].ID,
			ProviderCode:   "k",
			HostCode:       hostCode(stream.StreamURL),
			Label:          "Kanata Source",
			EmbedURL:       strings.TrimSpace(stream.StreamURL),
			LangCode:       "id",
			QualityCode:    movieQualityCode(item.Quality),
			Priority:       0,
			StatusCode:     "a",
			LastVerifiedAt: &now,
			UpdatedAt:      now,
		},
	})
	if err != nil {
		return withMovieSyncCode("store_upsert_watch_option_error", fmt.Errorf("upsert watch option: %w", err))
	}

	return nil
}

func (s *MovieV3Service) now() time.Time {
	if !s.fixedNow.IsZero() {
		return s.fixedNow.UTC()
	}
	return time.Now().UTC()
}

func cleanMovieTitle(value string) string {
	value = html.UnescapeString(strings.TrimSpace(value))
	value = trailingYearPattern.ReplaceAllString(value, "")
	return strings.TrimSpace(value)
}

func movieSearchQueries(rawTitle string) []string {
	primary := cleanMovieTitle(rawTitle)
	if primary == "" {
		return nil
	}

	queries := []string{primary}
	if stripped := stripTrailingAlias(primary); stripped != "" && stripped != primary {
		queries = append(queries, stripped)
	}
	return queries
}

func stripTrailingAlias(value string) string {
	matches := trailingAliasPattern.FindStringSubmatch(value)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(trailingAliasPattern.ReplaceAllString(value, ""))
}

func parseYear(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	year, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return year
}

func parseDurationMinutes(value string) int {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return hours*60 + minutes
}

func buildCanonicalMovieSlug(title string, year int, tmdbID int64) string {
	base := strings.ToLower(html.UnescapeString(strings.TrimSpace(title)))
	base = nonSlugRunesPattern.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		if tmdbID > 0 {
			return fmt.Sprintf("movie-%d", tmdbID)
		}
		return "movie"
	}
	if year > 0 {
		base = fmt.Sprintf("%s-%d", base, year)
	}
	if tmdbID > 0 {
		return fmt.Sprintf("%s-%d", base, tmdbID)
	}
	return base
}

func yearFromTMDB(detail tmdb.MovieDetail) int {
	if len(detail.ReleaseDate) < 4 {
		return 0
	}
	year, err := strconv.Atoi(detail.ReleaseDate[:4])
	if err != nil {
		return 0
	}
	return year
}

func smallintPtr(value int) *int16 {
	if value <= 0 {
		return nil
	}
	v := int16(value)
	return &v
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func movieStatusCode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "released", "returning series":
		return "r"
	case "post production", "in production":
		return "p"
	case "planned", "rumored":
		return "u"
	default:
		return "x"
	}
}

func movieQualityCode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == "hd" || strings.Contains(normalized, "1080") || strings.Contains(normalized, "720"):
		return "h"
	case strings.Contains(normalized, "bluray"):
		return "b"
	case strings.Contains(normalized, "web"):
		return "w"
	default:
		return "u"
	}
}

func normalizeLanguageCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) == 2 {
		return value
	}
	return ""
}

func genreIDs(detail tmdb.MovieDetail) []int16 {
	if len(detail.Genres) == 0 {
		return []int16{}
	}
	result := make([]int16, 0, len(detail.Genres))
	for _, genre := range detail.Genres {
		if genre.ID <= 0 {
			continue
		}
		result = append(result, int16(genre.ID))
	}
	return result
}

func chooseFirstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func youtubeIDFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if strings.Contains(parsed.Host, "youtu.be") {
		return strings.Trim(strings.TrimSpace(parsed.Path), "/")
	}
	if strings.Contains(parsed.Host, "youtube.com") {
		return strings.TrimSpace(parsed.Query().Get("v"))
	}
	return ""
}

func hostCode(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "u"
	}
	host := strings.ToLower(parsed.Host)
	switch {
	case strings.Contains(host, "ngopi.web.id"):
		return "n"
	case strings.Contains(host, "youtube.com"), strings.Contains(host, "youtu.be"):
		return "y"
	default:
		return "u"
	}
}

func dedupeHomeMovies(items []HomeMovie) []HomeMovie {
	seen := make(map[string]struct{}, len(items))
	result := make([]HomeMovie, 0, len(items))
	for _, item := range items {
		slug := strings.TrimSpace(item.Slug)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		result = append(result, item)
	}
	return result
}
