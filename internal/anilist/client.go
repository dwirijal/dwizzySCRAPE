package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultBaseURL = "https://graphql.anilist.co"
const minimumMatchScore = 70

var ErrServiceUnavailable = errors.New("anilist service unavailable")

type MediaType string

const (
	MediaTypeAnime MediaType = "ANIME"
	MediaTypeManga MediaType = "MANGA"
)

type MatchReason string

const (
	MatchReasonMatched       MatchReason = "matched"
	MatchReasonSearchEmpty   MatchReason = "search_empty"
	MatchReasonScoreRejected MatchReason = "score_rejected"
)

type MediaTitle struct {
	Romaji  string `json:"romaji"`
	English string `json:"english"`
	Native  string `json:"native"`
}

type CoverImage struct {
	ExtraLarge string `json:"extraLarge"`
	Large      string `json:"large"`
	Medium     string `json:"medium"`
}

type SearchHit struct {
	ID              int        `json:"id"`
	IDMal           int        `json:"idMal"`
	Title           MediaTitle `json:"title"`
	Synonyms        []string   `json:"synonyms"`
	Description     string     `json:"description"`
	Format          string     `json:"format"`
	Status          string     `json:"status"`
	CountryOfOrigin string     `json:"countryOfOrigin"`
	SeasonYear      int        `json:"seasonYear"`
	AverageScore    int        `json:"averageScore"`
	IsAdult         bool       `json:"isAdult"`
	Genres          []string   `json:"genres"`
	CoverImage      CoverImage `json:"coverImage"`
	BannerImage     string     `json:"bannerImage"`
	SiteURL         string     `json:"siteUrl"`
}

type MatchResult struct {
	Hit            SearchHit
	Matched        bool
	Reason         MatchReason
	BestScore      int
	CandidateCount int
}

type UpstreamUnavailableError struct {
	StatusCode int
	Message    string
}

func (e UpstreamUnavailableError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "service unavailable"
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("anilist service unavailable (%d): %s", e.StatusCode, message)
	}
	return fmt.Sprintf("anilist service unavailable: %s", message)
}

func (e UpstreamUnavailableError) Unwrap() error {
	return ErrServiceUnavailable
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

func (c *Client) SearchMedia(ctx context.Context, query string, mediaType MediaType, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 5
	}
	body, err := json.Marshal(map[string]any{
		"query": `
query ($search: String!, $type: MediaType!, $perPage: Int!) {
  Page(page: 1, perPage: $perPage) {
    media(search: $search, type: $type, sort: SEARCH_MATCH) {
      id
      idMal
      title { romaji english native }
      synonyms
      description(asHtml: false)
      format
      status
      countryOfOrigin
      seasonYear
      averageScore
      isAdult
      genres
      coverImage { extraLarge large medium }
      bannerImage
      siteUrl
    }
  }
}`,
		"variables": map[string]any{
			"search":  strings.TrimSpace(query),
			"type":    mediaType,
			"perPage": limit,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal anilist request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build anilist request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform anilist request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anilist response: %w", err)
	}
	if resp.StatusCode >= 300 {
		message := parseGraphQLErrorMessage(raw)
		if resp.StatusCode == http.StatusForbidden && looksLikeTemporaryDisable(message) {
			return nil, UpstreamUnavailableError{StatusCode: resp.StatusCode, Message: message}
		}
		if message == "" {
			message = strings.TrimSpace(string(raw))
		}
		return nil, fmt.Errorf("anilist request failed with status %d: %s", resp.StatusCode, message)
	}

	var parsed struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Data struct {
			Page struct {
				Media []SearchHit `json:"media"`
			} `json:"Page"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode anilist response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		message := strings.TrimSpace(parsed.Errors[0].Message)
		if looksLikeTemporaryDisable(message) {
			return nil, UpstreamUnavailableError{Message: message}
		}
		return nil, fmt.Errorf("anilist graphql error: %s", message)
	}
	return parsed.Data.Page.Media, nil
}

func IsServiceUnavailable(err error) bool {
	return errors.Is(err, ErrServiceUnavailable)
}

func parseGraphQLErrorMessage(raw []byte) string {
	var parsed struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ""
	}
	if len(parsed.Errors) == 0 {
		return ""
	}
	return strings.TrimSpace(parsed.Errors[0].Message)
}

func looksLikeTemporaryDisable(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "temporarily disabled") ||
		strings.Contains(message, "service unavailable") ||
		strings.Contains(message, "stability issues")
}

func PickBestMediaMatchResult(query string, year int, results []SearchHit) MatchResult {
	out := MatchResult{
		Reason:         MatchReasonSearchEmpty,
		CandidateCount: len(results),
	}
	if len(results) == 0 {
		return out
	}
	target := normalizeTitle(query)
	bestScore := -1
	for _, hit := range results {
		score := scoreHit(target, year, hit)
		if score > bestScore {
			bestScore = score
			out.Hit = hit
		}
	}
	out.BestScore = bestScore
	if bestScore < minimumMatchScore {
		out.Reason = MatchReasonScoreRejected
		return out
	}
	out.Matched = true
	out.Reason = MatchReasonMatched
	return out
}

func scoreHit(target string, year int, hit SearchHit) int {
	candidates := []string{
		hit.Title.English,
		hit.Title.Romaji,
		hit.Title.Native,
	}
	candidates = append(candidates, hit.Synonyms...)

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
			score = 65
		case sharedTokens(target, value) >= 2:
			score = 45 + sharedTokens(target, value)
		}
		if year > 0 && hit.SeasonYear == year {
			score += 20
		}
		if year > 0 && hit.SeasonYear > 0 && abs(year-hit.SeasonYear) == 1 {
			score += 5
		}
		if score > best {
			best = score
		}
	}
	return best
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
