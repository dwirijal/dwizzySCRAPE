package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const defaultBaseURL = "https://api.themoviedb.org/3"
const defaultPosterBaseURL = "https://image.tmdb.org/t/p/w500"

var yearPattern = regexp.MustCompile(`\b(19|20)\d{2}\b`)

type Client struct {
	baseURL    string
	readToken  string
	apiKey     string
	httpClient *http.Client
}

type SearchHit struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"`
	PosterPath    string  `json:"poster_path"`
	VoteAverage   float64 `json:"vote_average"`
}

type SeriesSearchHit struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	OriginalName     string   `json:"original_name"`
	FirstAirDate     string   `json:"first_air_date"`
	PosterPath       string   `json:"poster_path"`
	VoteAverage      float64  `json:"vote_average"`
	OriginalLanguage string   `json:"original_language"`
	OriginCountry    []string `json:"origin_country"`
}

type MatchReason string

const (
	MatchReasonMatched       MatchReason = "matched"
	MatchReasonSearchEmpty   MatchReason = "search_empty"
	MatchReasonScoreRejected MatchReason = "score_rejected"
)

type MatchResult struct {
	Hit            SearchHit
	Matched        bool
	Reason         MatchReason
	BestScore      int
	CandidateCount int
}

type searchResponse struct {
	Results []SearchHit `json:"results"`
}

type seriesSearchResponse struct {
	Results []SeriesSearchHit `json:"results"`
}

type MovieDetail struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	Overview         string  `json:"overview"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	ReleaseDate      string  `json:"release_date"`
	Runtime          int     `json:"runtime"`
	VoteAverage      float64 `json:"vote_average"`
	Tagline          string  `json:"tagline"`
	Status           string  `json:"status"`
	OriginalLanguage string  `json:"original_language"`
	Genres           []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCountries []struct {
		Name string `json:"name"`
	} `json:"production_countries"`
	Videos struct {
		Results []struct {
			Name     string `json:"name"`
			Key      string `json:"key"`
			Site     string `json:"site"`
			Type     string `json:"type"`
			Official bool   `json:"official"`
		} `json:"results"`
	} `json:"videos"`
	Credits struct {
		Cast []struct {
			Name      string `json:"name"`
			Character string `json:"character"`
		} `json:"cast"`
		Crew []struct {
			Name string `json:"name"`
			Job  string `json:"job"`
		} `json:"crew"`
	} `json:"credits"`
}

type SeriesDetail struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	OriginalName     string   `json:"original_name"`
	Overview         string   `json:"overview"`
	PosterPath       string   `json:"poster_path"`
	BackdropPath     string   `json:"backdrop_path"`
	FirstAirDate     string   `json:"first_air_date"`
	VoteAverage      float64  `json:"vote_average"`
	Tagline          string   `json:"tagline"`
	Status           string   `json:"status"`
	OriginalLanguage string   `json:"original_language"`
	OriginCountry    []string `json:"origin_country"`
	EpisodeRunTime   []int    `json:"episode_run_time"`
	Genres           []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCountries []struct {
		Name string `json:"name"`
	} `json:"production_countries"`
	Videos struct {
		Results []struct {
			Name     string `json:"name"`
			Key      string `json:"key"`
			Site     string `json:"site"`
			Type     string `json:"type"`
			Official bool   `json:"official"`
		} `json:"results"`
	} `json:"videos"`
	Credits struct {
		Cast []struct {
			Name      string `json:"name"`
			Character string `json:"character"`
		} `json:"cast"`
		Crew []struct {
			Name string `json:"name"`
			Job  string `json:"job"`
		} `json:"crew"`
	} `json:"credits"`
}

func NewClient(baseURL, readToken, apiKey string, httpClient *http.Client) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    baseURL,
		readToken:  strings.TrimSpace(readToken),
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: httpClient,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && (c.readToken != "" || c.apiKey != "")
}

func (c *Client) SearchMovies(ctx context.Context, query string, year, limit int) ([]SearchHit, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("tmdb credentials are not configured")
	}
	endpoint, err := url.Parse(c.baseURL + "/search/movie")
	if err != nil {
		return nil, fmt.Errorf("build tmdb search endpoint: %w", err)
	}
	params := endpoint.Query()
	params.Set("query", strings.TrimSpace(query))
	params.Set("include_adult", "false")
	if year > 0 {
		params.Set("year", strconv.Itoa(year))
	}
	if c.apiKey != "" && c.readToken == "" {
		params.Set("api_key", c.apiKey)
	}
	endpoint.RawQuery = params.Encode()

	var parsed searchResponse
	if err := c.getJSON(ctx, endpoint.String(), &parsed); err != nil {
		return nil, err
	}
	results := parsed.Results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (c *Client) GetMovieDetail(ctx context.Context, movieID int) (MovieDetail, error) {
	if !c.Enabled() {
		return MovieDetail{}, fmt.Errorf("tmdb credentials are not configured")
	}
	if movieID <= 0 {
		return MovieDetail{}, fmt.Errorf("tmdb movie id must be positive")
	}
	endpoint, err := url.Parse(fmt.Sprintf("%s/movie/%d", c.baseURL, movieID))
	if err != nil {
		return MovieDetail{}, fmt.Errorf("build tmdb detail endpoint: %w", err)
	}
	params := endpoint.Query()
	params.Set("append_to_response", "videos,credits")
	if c.apiKey != "" && c.readToken == "" {
		params.Set("api_key", c.apiKey)
	}
	endpoint.RawQuery = params.Encode()

	var detail MovieDetail
	if err := c.getJSON(ctx, endpoint.String(), &detail); err != nil {
		return MovieDetail{}, err
	}
	return detail, nil
}

func (c *Client) SearchTV(ctx context.Context, query string, year, limit int) ([]SeriesSearchHit, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("tmdb credentials are not configured")
	}
	endpoint, err := url.Parse(c.baseURL + "/search/tv")
	if err != nil {
		return nil, fmt.Errorf("build tmdb tv search endpoint: %w", err)
	}
	params := endpoint.Query()
	params.Set("query", strings.TrimSpace(query))
	params.Set("include_adult", "false")
	if year > 0 {
		params.Set("first_air_date_year", strconv.Itoa(year))
	}
	if c.apiKey != "" && c.readToken == "" {
		params.Set("api_key", c.apiKey)
	}
	endpoint.RawQuery = params.Encode()

	var parsed seriesSearchResponse
	if err := c.getJSON(ctx, endpoint.String(), &parsed); err != nil {
		return nil, err
	}
	results := parsed.Results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (c *Client) GetTVDetail(ctx context.Context, tvID int) (SeriesDetail, error) {
	if !c.Enabled() {
		return SeriesDetail{}, fmt.Errorf("tmdb credentials are not configured")
	}
	if tvID <= 0 {
		return SeriesDetail{}, fmt.Errorf("tmdb tv id must be positive")
	}
	endpoint, err := url.Parse(fmt.Sprintf("%s/tv/%d", c.baseURL, tvID))
	if err != nil {
		return SeriesDetail{}, fmt.Errorf("build tmdb tv detail endpoint: %w", err)
	}
	params := endpoint.Query()
	params.Set("append_to_response", "videos,credits")
	if c.apiKey != "" && c.readToken == "" {
		params.Set("api_key", c.apiKey)
	}
	endpoint.RawQuery = params.Encode()

	var detail SeriesDetail
	if err := c.getJSON(ctx, endpoint.String(), &detail); err != nil {
		return SeriesDetail{}, err
	}
	return detail, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("build tmdb request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.readToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.readToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform tmdb request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read tmdb response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("tmdb request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode tmdb response: %w", err)
	}
	return nil
}

func PickBestMovieMatch(query string, year int, results []SearchHit) (SearchHit, bool) {
	result := PickBestMovieMatchResult(query, year, results)
	return result.Hit, result.Matched
}

func PickBestMovieMatchResult(query string, year int, results []SearchHit) MatchResult {
	out := MatchResult{
		Reason:         MatchReasonSearchEmpty,
		CandidateCount: len(results),
	}
	if len(results) == 0 {
		return out
	}

	target := normalizeTitle(query)
	type scoredHit struct {
		hit   SearchHit
		score int
	}

	scored := make([]scoredHit, 0, len(results))
	for _, result := range results {
		score := scoreHit(target, year, result)
		scored = append(scored, scoredHit{hit: result, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	out.BestScore = scored[0].score
	if scored[0].score <= 0 {
		out.Reason = MatchReasonScoreRejected
		return out
	}
	out.Hit = scored[0].hit
	out.Matched = true
	out.Reason = MatchReasonMatched
	return out
}

type SeriesMatchResult struct {
	Hit            SeriesSearchHit
	Matched        bool
	Reason         MatchReason
	BestScore      int
	CandidateCount int
}

func PickBestSeriesMatchResult(query string, year int, results []SeriesSearchHit) SeriesMatchResult {
	out := SeriesMatchResult{
		Reason:         MatchReasonSearchEmpty,
		CandidateCount: len(results),
	}
	if len(results) == 0 {
		return out
	}

	target := normalizeTitle(query)
	type scoredHit struct {
		hit   SeriesSearchHit
		score int
	}

	scored := make([]scoredHit, 0, len(results))
	for _, result := range results {
		score := scoreSeriesHit(target, year, result)
		scored = append(scored, scoredHit{hit: result, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	out.BestScore = scored[0].score
	if scored[0].score <= 0 {
		out.Reason = MatchReasonScoreRejected
		return out
	}
	out.Hit = scored[0].hit
	out.Matched = true
	out.Reason = MatchReasonMatched
	return out
}

func scoreHit(target string, year int, result SearchHit) int {
	candidates := []string{
		result.Title,
		result.OriginalTitle,
		path.Base(strings.TrimSpace(result.PosterPath)),
	}

	resultYear := extractYear(result.ReleaseDate)
	best := 0
	for _, candidate := range candidates {
		value := normalizeTitle(candidate)
		if value == "" {
			continue
		}

		score := 0
		switch {
		case value == target:
			score = 100
		case strings.Contains(value, target) || strings.Contains(target, value):
			score = 70
		case sharedTokens(target, value) >= 2:
			score = 40 + sharedTokens(target, value)
		}
		if year > 0 && resultYear == year {
			score += 20
		}
		if year > 0 && resultYear > 0 && abs(year-resultYear) == 1 {
			score += 5
		}
		if score > best {
			best = score
		}
	}
	return best
}

func scoreSeriesHit(target string, year int, result SeriesSearchHit) int {
	candidates := []string{
		result.Name,
		result.OriginalName,
		path.Base(strings.TrimSpace(result.PosterPath)),
	}

	resultYear := extractYear(result.FirstAirDate)
	best := 0
	for _, candidate := range candidates {
		value := normalizeTitle(candidate)
		if value == "" {
			continue
		}

		score := 0
		switch {
		case value == target:
			score = 100
		case strings.Contains(value, target) || strings.Contains(target, value):
			score = 70
		case sharedTokens(target, value) >= 2:
			score = 40 + sharedTokens(target, value)
		}
		if year > 0 && resultYear == year {
			score += 20
		}
		if year > 0 && resultYear > 0 && abs(year-resultYear) == 1 {
			score += 5
		}
		if score > best {
			best = score
		}
	}
	return best
}

func BuildPosterURL(posterPath string) string {
	posterPath = strings.TrimSpace(posterPath)
	if posterPath == "" {
		return ""
	}
	if strings.HasPrefix(posterPath, "http://") || strings.HasPrefix(posterPath, "https://") {
		return posterPath
	}
	if !strings.HasPrefix(posterPath, "/") {
		posterPath = "/" + posterPath
	}
	return defaultPosterBaseURL + posterPath
}

func PickTrailerURL(detail MovieDetail) string {
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

func PickDirectorNames(detail MovieDetail) []string {
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

func CastNameObjects(detail MovieDetail, limit int) []map[string]string {
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

func GenreNames(detail MovieDetail) []string {
	result := make([]string, 0, len(detail.Genres))
	for _, genre := range detail.Genres {
		if name := strings.TrimSpace(genre.Name); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func normalizeTitle(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		":", " ",
		"!", " ",
		"?", " ",
		",", " ",
		".", " ",
		"'", "",
		"’", "",
		"(", " ",
		")", " ",
		"|", " ",
	)
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func sharedTokens(a, b string) int {
	left := strings.Fields(a)
	right := make(map[string]struct{}, len(strings.Fields(b)))
	for _, token := range strings.Fields(b) {
		right[token] = struct{}{}
	}
	count := 0
	for _, token := range left {
		if _, ok := right[token]; ok {
			count++
		}
	}
	return count
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func extractYear(value string) int {
	match := yearPattern.FindString(strings.TrimSpace(value))
	if match == "" {
		return 0
	}
	year, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return year
}
