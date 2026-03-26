package jikan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
)

type AnimeSearchHit struct {
	MALID         int      `json:"mal_id"`
	Title         string   `json:"title"`
	TitleEnglish  string   `json:"title_english"`
	TitleJapanese string   `json:"title_japanese"`
	TitleSynonyms []string `json:"title_synonyms"`
	URL           string   `json:"url"`
	Images        struct {
		WebP struct {
			ImageURL      string `json:"image_url"`
			LargeImageURL string `json:"large_image_url"`
		} `json:"webp"`
		JPG struct {
			ImageURL      string `json:"image_url"`
			LargeImageURL string `json:"large_image_url"`
		} `json:"jpg"`
	} `json:"images"`
}

type AnimeFull struct {
	MALID    int     `json:"mal_id"`
	URL      string  `json:"url"`
	Title    string  `json:"title"`
	Type     string  `json:"type"`
	Status   string  `json:"status"`
	Season   string  `json:"season"`
	Synopsis string  `json:"synopsis"`
	Score    float64 `json:"score"`
	Images   struct {
		WebP struct {
			ImageURL      string `json:"image_url"`
			LargeImageURL string `json:"large_image_url"`
		} `json:"webp"`
		JPG struct {
			ImageURL      string `json:"image_url"`
			LargeImageURL string `json:"large_image_url"`
		} `json:"jpg"`
	} `json:"images"`
	Trailer struct {
		EmbedURL string `json:"embed_url"`
		URL      string `json:"url"`
	} `json:"trailer"`
	Studios      []NamedEntity `json:"studios"`
	Genres       []NamedEntity `json:"genres"`
	Themes       []NamedEntity `json:"themes"`
	Demographics []NamedEntity `json:"demographics"`
}

type NamedEntity struct {
	Name string `json:"name"`
}

type AnimeCharacter struct {
	Role      string `json:"role"`
	Character struct {
		MALID  int    `json:"mal_id"`
		Name   string `json:"name"`
		Images struct {
			WebP struct {
				ImageURL string `json:"image_url"`
			} `json:"webp"`
		} `json:"images"`
	} `json:"character"`
	VoiceActors []struct {
		Language string `json:"language"`
		Person   struct {
			MALID int    `json:"mal_id"`
			Name  string `json:"name"`
		} `json:"person"`
	} `json:"voice_actors"`
}

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.jikan.moe/v4"
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL: baseURL,
		client:  httpClient,
	}
}

func (c *Client) SearchAnime(ctx context.Context, query string, limit int) ([]AnimeSearchHit, error) {
	if limit <= 0 {
		limit = 5
	}
	endpoint, err := url.Parse(c.baseURL + "/anime")
	if err != nil {
		return nil, fmt.Errorf("build search endpoint: %w", err)
	}
	params := endpoint.Query()
	params.Set("q", strings.TrimSpace(query))
	params.Set("limit", strconv.Itoa(limit))
	endpoint.RawQuery = params.Encode()

	var resp struct {
		Data []AnimeSearchHit `json:"data"`
	}
	if err := c.getJSON(ctx, endpoint.String(), &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetAnimeFull(ctx context.Context, malID int) (AnimeFull, error) {
	var resp struct {
		Data AnimeFull `json:"data"`
	}
	if err := c.getJSON(ctx, c.baseURL+"/anime/"+strconv.Itoa(malID)+"/full", &resp); err != nil {
		return AnimeFull{}, err
	}
	return resp.Data, nil
}

func (c *Client) GetAnimeCharacters(ctx context.Context, malID int) ([]AnimeCharacter, error) {
	var resp struct {
		Data []AnimeCharacter `json:"data"`
	}
	if err := c.getJSON(ctx, c.baseURL+"/anime/"+strconv.Itoa(malID)+"/characters", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("build jikan request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("perform jikan request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read jikan response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("jikan request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode jikan response: %w", err)
	}
	return nil
}

func PickBestMatch(query string, results []AnimeSearchHit) (AnimeSearchHit, bool) {
	if len(results) == 0 {
		return AnimeSearchHit{}, false
	}
	target := normalizeTitle(query)
	type scoredHit struct {
		hit   AnimeSearchHit
		score int
	}
	scored := make([]scoredHit, 0, len(results))
	for _, result := range results {
		score := scoreHit(target, result)
		scored = append(scored, scoredHit{hit: result, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	if scored[0].score <= 0 {
		return AnimeSearchHit{}, false
	}
	return scored[0].hit, true
}

func scoreHit(target string, result AnimeSearchHit) int {
	candidates := []string{
		result.Title,
		result.TitleEnglish,
		result.TitleJapanese,
		path.Base(strings.TrimSpace(result.URL)),
	}
	candidates = append(candidates, result.TitleSynonyms...)

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
		case sharedTokens(target, value) >= 3:
			score = 40 + sharedTokens(target, value)
		}
		if hasSeasonNumber(target, value) {
			score += 15
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
	)
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func sharedTokens(a, b string) int {
	left := strings.Fields(a)
	rightSet := make(map[string]struct{}, len(strings.Fields(b)))
	for _, token := range strings.Fields(b) {
		rightSet[token] = struct{}{}
	}
	count := 0
	for _, token := range left {
		if _, ok := rightSet[token]; ok {
			count++
		}
	}
	return count
}

func hasSeasonNumber(target, candidate string) bool {
	for _, season := range []string{"season 2", "season 3", "season 4", "2nd season", "3rd season", "4th season"} {
		if strings.Contains(target, season) && strings.Contains(candidate, season) {
			return true
		}
	}
	return false
}
