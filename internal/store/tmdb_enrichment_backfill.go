package store

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/tmdb"
)

var (
	yearSuffixPattern  = regexp.MustCompile(`\s*\((19|20)\d{2}\)\s*$`)
	dramaPrefixPattern = regexp.MustCompile(`^(?i:drama\s+(korea|china|jepang|japan)\s+)`)
)

type tmdbEnrichmentClient interface {
	SearchMovies(ctx context.Context, query string, year, limit int) ([]tmdb.SearchHit, error)
	GetMovieDetail(ctx context.Context, movieID int) (tmdb.MovieDetail, error)
	SearchTV(ctx context.Context, query string, year, limit int) ([]tmdb.SeriesSearchHit, error)
	GetTVDetail(ctx context.Context, tvID int) (tmdb.SeriesDetail, error)
}

type tmdbEnrichmentCandidateReader interface {
	ListTMDBEnrichmentCandidates(ctx context.Context, offset, limit int, options TMDBEnrichmentCandidateOptions) ([]TMDBEnrichmentCandidate, error)
}

type tmdbEnrichmentWriter interface {
	UpsertItemEnrichment(ctx context.Context, record MediaItemEnrichmentRecord) error
}

type TMDBEnrichmentBackfillOptions struct {
	Scope        TMDBEnrichmentScope
	BatchSize    int
	Limit        int
	SkipExisting bool
	DelayBetween time.Duration
	Progress     func(TMDBEnrichmentBackfillProgress)
}

type TMDBEnrichmentBackfillReport struct {
	Discovered int
	Attempted  int
	Skipped    int
	Succeeded  int
	Failed     int
	Failures   map[string]string
}

type TMDBEnrichmentBackfillProgress struct {
	ItemKey string
	Slug    string
	Action  string
	Reason  string
	Counts  TMDBEnrichmentBackfillReport
}

type TMDBEnrichmentBackfillService struct {
	reader tmdbEnrichmentCandidateReader
	writer tmdbEnrichmentWriter
	client tmdbEnrichmentClient
	sleep  func(time.Duration)
}

func NewTMDBEnrichmentBackfillService(
	reader tmdbEnrichmentCandidateReader,
	writer tmdbEnrichmentWriter,
	client tmdbEnrichmentClient,
) *TMDBEnrichmentBackfillService {
	return &TMDBEnrichmentBackfillService{
		reader: reader,
		writer: writer,
		client: client,
		sleep:  time.Sleep,
	}
}

func (s *TMDBEnrichmentBackfillService) Backfill(
	ctx context.Context,
	options TMDBEnrichmentBackfillOptions,
) (TMDBEnrichmentBackfillReport, error) {
	if s.reader == nil {
		return TMDBEnrichmentBackfillReport{}, fmt.Errorf("tmdb enrichment candidate reader is required")
	}
	if s.writer == nil {
		return TMDBEnrichmentBackfillReport{}, fmt.Errorf("tmdb enrichment writer is required")
	}
	if s.client == nil {
		return TMDBEnrichmentBackfillReport{}, fmt.Errorf("tmdb enrichment client is required")
	}

	scope := options.Scope
	if scope == "" {
		scope = TMDBEnrichmentScopeAll
	}
	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	report := TMDBEnrichmentBackfillReport{
		Failures: make(map[string]string),
	}
	processed := 0
	attemptedKeys := make(map[string]struct{})

	for offset := 0; ; {
		if err := ctx.Err(); err != nil {
			return report, err
		}

		batchOffset := offset
		if options.SkipExisting {
			batchOffset = 0
		}
		candidates, err := s.reader.ListTMDBEnrichmentCandidates(ctx, batchOffset, batchSize, TMDBEnrichmentCandidateOptions{
			Scope:        scope,
			SkipExisting: options.SkipExisting,
		})
		if err != nil {
			return report, err
		}
		if len(candidates) == 0 {
			break
		}

		freshCandidates := make([]TMDBEnrichmentCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			if _, seen := attemptedKeys[candidate.ItemKey]; seen {
				report.Skipped++
				continue
			}
			freshCandidates = append(freshCandidates, candidate)
		}
		if len(freshCandidates) == 0 {
			break
		}

		for _, candidate := range freshCandidates {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			if options.Limit > 0 && processed >= options.Limit {
				return report, nil
			}

			attemptedKeys[candidate.ItemKey] = struct{}{}
			report.Discovered++
			processed++
			report.Attempted++

			if err := s.enrichCandidate(ctx, candidate); err != nil {
				report.Failed++
				report.Failures[candidate.Slug] = err.Error()
				if options.Progress != nil {
					options.Progress(TMDBEnrichmentBackfillProgress{
						ItemKey: candidate.ItemKey,
						Slug:    candidate.Slug,
						Action:  "fail",
						Reason:  err.Error(),
						Counts:  report,
					})
				}
				continue
			}

			report.Succeeded++
			if options.Progress != nil {
				options.Progress(TMDBEnrichmentBackfillProgress{
					ItemKey: candidate.ItemKey,
					Slug:    candidate.Slug,
					Action:  "success",
					Counts:  report,
				})
			}
			if options.DelayBetween > 0 && s.sleep != nil {
				s.sleep(options.DelayBetween)
			}
		}

		if len(freshCandidates) < batchSize {
			break
		}
		if !options.SkipExisting {
			offset += batchSize
		}
	}

	return report, nil
}

func (s *TMDBEnrichmentBackfillService) enrichCandidate(ctx context.Context, candidate TMDBEnrichmentCandidate) error {
	queries := buildTMDBSearchQueries(candidate)
	if len(queries) == 0 {
		recordFailure := MediaItemEnrichmentRecord{
			ItemKey:     candidate.ItemKey,
			Provider:    "tmdb",
			MatchStatus: "missing_query",
			MatchedTitle: firstPresent(
				strings.TrimSpace(candidate.Title),
				strings.TrimSpace(candidate.Slug),
			),
			Payload: map[string]any{
				"kind":   candidate.SurfaceType,
				"source": candidate.Source,
			},
		}
		if err := s.writer.UpsertItemEnrichment(ctx, recordFailure); err != nil {
			return fmt.Errorf("record tmdb enrichment failure: %w", err)
		}
		return fmt.Errorf("no tmdb search query candidates for %s", candidate.Slug)
	}

	switch strings.TrimSpace(candidate.SurfaceType) {
	case "movie":
		return s.enrichMovie(ctx, candidate, queries)
	case "series":
		return s.enrichSeries(ctx, candidate, queries)
	default:
		return fmt.Errorf("unsupported tmdb enrichment surface type %q", candidate.SurfaceType)
	}
}

func (s *TMDBEnrichmentBackfillService) enrichMovie(ctx context.Context, candidate TMDBEnrichmentCandidate, queries []string) error {
	bestScore := -1
	bestReason := tmdb.MatchReasonSearchEmpty
	var bestHit tmdb.SearchHit
	var matchedQuery string

	for _, query := range queries {
		results, err := s.client.SearchMovies(ctx, query, candidate.ReleaseYear, 5)
		if err != nil {
			return fmt.Errorf("search tmdb movie for %s: %w", candidate.Slug, err)
		}
		result := tmdb.PickBestMovieMatchResult(query, candidate.ReleaseYear, results)
		if result.BestScore > bestScore {
			bestScore = result.BestScore
			bestReason = result.Reason
			bestHit = result.Hit
			matchedQuery = query
		}
		if result.Matched {
			break
		}
	}

	if strings.TrimSpace(bestHit.Title) == "" || bestScore <= 0 {
		recordFailure := MediaItemEnrichmentRecord{
			ItemKey:      candidate.ItemKey,
			Provider:     "tmdb",
			MatchStatus:  string(bestReason),
			MatchScore:   clampTMDBMatchScore(bestScore),
			MatchedTitle: firstPresent(strings.TrimSpace(bestHit.Title), candidate.Title, candidate.Slug),
			Payload: map[string]any{
				"kind":    "movie",
				"queries": queries,
			},
		}
		if err := s.writer.UpsertItemEnrichment(ctx, recordFailure); err != nil {
			return fmt.Errorf("record tmdb movie mismatch: %w", err)
		}
		return fmt.Errorf("tmdb movie match failed for %s: %s", candidate.Slug, bestReason)
	}

	detail, err := s.client.GetMovieDetail(ctx, bestHit.ID)
	if err != nil {
		return fmt.Errorf("get tmdb movie detail for %s: %w", candidate.Slug, err)
	}

	record := MediaItemEnrichmentRecord{
		ItemKey:      candidate.ItemKey,
		Provider:     "tmdb",
		ExternalID:   strconv.Itoa(bestHit.ID),
		MatchStatus:  "matched",
		MatchScore:   clampTMDBMatchScore(bestScore),
		MatchedTitle: firstPresent(strings.TrimSpace(detail.Title), strings.TrimSpace(bestHit.Title)),
		MatchedYear:  extractYearFromDate(firstPresent(detail.ReleaseDate, bestHit.ReleaseDate)),
		Payload: compactPayload(map[string]any{
			"kind":              "movie",
			"matched_query":     matchedQuery,
			"title":             strings.TrimSpace(detail.Title),
			"original_title":    strings.TrimSpace(detail.OriginalTitle),
			"overview":          strings.TrimSpace(detail.Overview),
			"poster_url":        tmdb.BuildPosterURL(detail.PosterPath),
			"backdrop_url":      tmdb.BuildPosterURL(detail.BackdropPath),
			"release_year":      extractYearFromDate(detail.ReleaseDate),
			"rating":            detail.VoteAverage,
			"runtime_minutes":   detail.Runtime,
			"tagline":           strings.TrimSpace(detail.Tagline),
			"status":            strings.TrimSpace(detail.Status),
			"original_language": strings.TrimSpace(detail.OriginalLanguage),
			"genres":            tmdb.GenreNames(detail),
			"country_names":     movieCountryNames(detail),
			"trailer_url":       tmdb.PickTrailerURL(detail),
			"cast":              tmdb.CastNameObjects(detail, 6),
			"directors":         tmdb.PickDirectorNames(detail),
		}),
	}
	if err := s.writer.UpsertItemEnrichment(ctx, record); err != nil {
		return fmt.Errorf("upsert tmdb movie enrichment: %w", err)
	}
	return nil
}

func (s *TMDBEnrichmentBackfillService) enrichSeries(ctx context.Context, candidate TMDBEnrichmentCandidate, queries []string) error {
	bestScore := -1
	bestReason := tmdb.MatchReasonSearchEmpty
	var bestHit tmdb.SeriesSearchHit
	var matchedQuery string

	for _, query := range queries {
		results, err := s.client.SearchTV(ctx, query, candidate.ReleaseYear, 5)
		if err != nil {
			return fmt.Errorf("search tmdb tv for %s: %w", candidate.Slug, err)
		}
		result := tmdb.PickBestSeriesMatchResult(query, candidate.ReleaseYear, results)
		if result.BestScore > bestScore {
			bestScore = result.BestScore
			bestReason = result.Reason
			bestHit = result.Hit
			matchedQuery = query
		}
		if result.Matched {
			break
		}
	}

	if strings.TrimSpace(bestHit.Name) == "" || bestScore <= 0 {
		recordFailure := MediaItemEnrichmentRecord{
			ItemKey:      candidate.ItemKey,
			Provider:     "tmdb",
			MatchStatus:  string(bestReason),
			MatchScore:   clampTMDBMatchScore(bestScore),
			MatchedTitle: firstPresent(strings.TrimSpace(bestHit.Name), candidate.Title, candidate.Slug),
			Payload: map[string]any{
				"kind":    "series",
				"queries": queries,
			},
		}
		if err := s.writer.UpsertItemEnrichment(ctx, recordFailure); err != nil {
			return fmt.Errorf("record tmdb series mismatch: %w", err)
		}
		return fmt.Errorf("tmdb series match failed for %s: %s", candidate.Slug, bestReason)
	}

	detail, err := s.client.GetTVDetail(ctx, bestHit.ID)
	if err != nil {
		return fmt.Errorf("get tmdb tv detail for %s: %w", candidate.Slug, err)
	}

	record := MediaItemEnrichmentRecord{
		ItemKey:      candidate.ItemKey,
		Provider:     "tmdb",
		ExternalID:   strconv.Itoa(bestHit.ID),
		MatchStatus:  "matched",
		MatchScore:   clampTMDBMatchScore(bestScore),
		MatchedTitle: firstPresent(strings.TrimSpace(detail.Name), strings.TrimSpace(bestHit.Name)),
		MatchedYear:  extractYearFromDate(firstPresent(detail.FirstAirDate, bestHit.FirstAirDate)),
		Payload: compactPayload(map[string]any{
			"kind":              "series",
			"matched_query":     matchedQuery,
			"title":             strings.TrimSpace(detail.Name),
			"original_title":    strings.TrimSpace(detail.OriginalName),
			"overview":          strings.TrimSpace(detail.Overview),
			"poster_url":        tmdb.BuildPosterURL(detail.PosterPath),
			"backdrop_url":      tmdb.BuildPosterURL(detail.BackdropPath),
			"release_year":      extractYearFromDate(detail.FirstAirDate),
			"rating":            detail.VoteAverage,
			"runtime_minutes":   firstPositiveInt(detail.EpisodeRunTime),
			"tagline":           strings.TrimSpace(detail.Tagline),
			"status":            strings.TrimSpace(detail.Status),
			"original_language": strings.TrimSpace(detail.OriginalLanguage),
			"genres":            seriesGenreNames(detail),
			"country_codes":     compactStringSlice(detail.OriginCountry),
			"country_names":     seriesCountryNames(detail),
			"trailer_url":       pickSeriesTrailerURL(detail),
			"cast":              seriesCastNameObjects(detail, 6),
			"directors":         seriesDirectorNames(detail),
		}),
	}
	if err := s.writer.UpsertItemEnrichment(ctx, record); err != nil {
		return fmt.Errorf("upsert tmdb series enrichment: %w", err)
	}
	return nil
}

func buildTMDBSearchQueries(candidate TMDBEnrichmentCandidate) []string {
	detail := candidate.Detail
	queries := make([]string, 0, 8)
	seen := make(map[string]struct{})

	add := func(values ...string) {
		for _, value := range values {
			for _, query := range expandTMDBQueryVariants(candidate, value) {
				normalized := strings.ToLower(query)
				if _, ok := seen[normalized]; ok {
					continue
				}
				seen[normalized] = struct{}{}
				queries = append(queries, query)
			}
		}
	}

	searchHit := readAnyMap(detail["search_hit"])
	jikanMeta := readAnyMap(detail["jikan_meta_json"])
	jikanSearchHit := readAnyMap(jikanMeta["search_hit"])
	jikanAnimeFull := readAnyMap(jikanMeta["anime_full"])
	add(readAnyString(searchHit["title_english"]))
	add(readAnyString(jikanSearchHit["title_english"]))
	add(readAnyString(searchHit["title"]))
	add(readAnyString(jikanSearchHit["title"]))
	add(readAnyString(jikanAnimeFull["title_english"]))
	add(readAnyString(jikanAnimeFull["title"]))
	add(readAnyString(detail["source_title"]))
	add(strings.TrimSpace(candidate.Title))

	for _, entry := range readAnySlice(searchHit["title_synonyms"]) {
		add(readAnyString(entry))
	}
	for _, entry := range readAnySlice(jikanSearchHit["title_synonyms"]) {
		add(readAnyString(entry))
	}
	for _, entry := range readAnySlice(jikanAnimeFull["title_synonyms"]) {
		add(readAnyString(entry))
	}
	add(readAnyString(detail["alt_title"]))

	if candidate.Source == "drakorid" {
		add(readAnyString(detail["native_title"]))
	}

	return queries
}

func expandTMDBQueryVariants(candidate TMDBEnrichmentCandidate, raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '|'
	})
	if len(parts) == 0 {
		parts = []string{raw}
	}

	result := make([]string, 0, len(parts))
	for _, part := range parts {
		cleaned := strings.TrimSpace(part)
		if cleaned == "" {
			continue
		}
		cleaned = yearSuffixPattern.ReplaceAllString(cleaned, "")
		if candidate.Source == "drakorid" {
			cleaned = dramaPrefixPattern.ReplaceAllString(cleaned, "")
		}
		cleaned = strings.Join(strings.Fields(cleaned), " ")
		if cleaned == "" {
			continue
		}
		if candidate.Source == "drakorid" && !hasASCIILetters(cleaned) {
			continue
		}
		result = append(result, cleaned)
	}
	return result
}

func compactPayload(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				output[key] = strings.TrimSpace(typed)
			}
		case int:
			if typed > 0 {
				output[key] = typed
			}
		case float64:
			if typed > 0 {
				output[key] = typed
			}
		case []string:
			if compacted := compactStringSlice(typed); len(compacted) > 0 {
				output[key] = compacted
			}
		case []map[string]string:
			if len(typed) > 0 {
				output[key] = typed
			}
		case []any:
			if len(typed) > 0 {
				output[key] = typed
			}
		default:
			if value != nil {
				output[key] = value
			}
		}
	}
	return output
}

func compactStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func movieCountryNames(detail tmdb.MovieDetail) []string {
	result := make([]string, 0, len(detail.ProductionCountries))
	for _, country := range detail.ProductionCountries {
		if name := strings.TrimSpace(country.Name); name != "" {
			result = append(result, name)
		}
	}
	return compactStringSlice(result)
}

func seriesCountryNames(detail tmdb.SeriesDetail) []string {
	result := make([]string, 0, len(detail.ProductionCountries)+len(detail.OriginCountry))
	for _, country := range detail.ProductionCountries {
		if name := strings.TrimSpace(country.Name); name != "" {
			result = append(result, name)
		}
	}
	switch strings.TrimSpace(detail.OriginalLanguage) {
	case "ko":
		result = append(result, "South Korea")
	case "ja":
		result = append(result, "Japan")
	case "zh":
		result = append(result, "China")
	}
	return compactStringSlice(result)
}

func seriesGenreNames(detail tmdb.SeriesDetail) []string {
	result := make([]string, 0, len(detail.Genres))
	for _, genre := range detail.Genres {
		if name := strings.TrimSpace(genre.Name); name != "" {
			result = append(result, name)
		}
	}
	return compactStringSlice(result)
}

func seriesCastNameObjects(detail tmdb.SeriesDetail, limit int) []map[string]string {
	if limit <= 0 {
		limit = 8
	}
	result := make([]map[string]string, 0, limit)
	for _, cast := range detail.Credits.Cast {
		if len(result) >= limit {
			break
		}
		name := strings.TrimSpace(cast.Name)
		if name == "" {
			continue
		}
		entry := map[string]string{
			"name":   name,
			"source": "tmdb",
		}
		if role := strings.TrimSpace(cast.Character); role != "" {
			entry["role"] = role
		}
		result = append(result, entry)
	}
	return result
}

func seriesDirectorNames(detail tmdb.SeriesDetail) []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, crew := range detail.Credits.Crew {
		if !strings.EqualFold(crew.Job, "Director") {
			continue
		}
		name := strings.TrimSpace(crew.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func pickSeriesTrailerURL(detail tmdb.SeriesDetail) string {
	best := ""
	for _, video := range detail.Videos.Results {
		if !strings.EqualFold(video.Site, "YouTube") || strings.TrimSpace(video.Key) == "" {
			continue
		}
		url := "https://www.youtube.com/watch?v=" + strings.TrimSpace(video.Key)
		if strings.EqualFold(video.Type, "Trailer") && video.Official {
			return url
		}
		if strings.EqualFold(video.Type, "Trailer") && best == "" {
			best = url
		}
		if best == "" {
			best = url
		}
	}
	return best
}

func extractYearFromDate(value string) int {
	if len(value) >= 4 {
		year, err := strconv.Atoi(value[:4])
		if err == nil && year >= 1900 && year <= 2100 {
			return year
		}
	}
	return 0
}

func firstPositiveInt(values []int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func readAnyMap(value any) map[string]any {
	record, _ := value.(map[string]any)
	return record
}

func readAnySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		values := make([]any, 0, len(typed))
		for _, item := range typed {
			values = append(values, item)
		}
		return values
	default:
		return nil
	}
}

func readAnyString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func hasASCIILetters(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func firstPresent(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func max(left, right int) int {
	return slices.Max([]int{left, right})
}

func clampTMDBMatchScore(value int) int {
	return min(max(value, 0), 100)
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
