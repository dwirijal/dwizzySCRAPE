package kanata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const defaultMovieTubeBaseURL = "https://api.kanata.web.id/movietube"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type HomeMovie struct {
	Duration string  `json:"duration"`
	Genres   string  `json:"genres"`
	Poster   string  `json:"poster"`
	Quality  string  `json:"quality"`
	Rating   float64 `json:"rating"`
	Slug     string  `json:"slug"`
	Title    string  `json:"title"`
	Type     string  `json:"type"`
	URL      string  `json:"url"`
	Year     string  `json:"year"`
}

func (m *HomeMovie) UnmarshalJSON(data []byte) error {
	type wireHomeMovie struct {
		Duration string          `json:"duration"`
		Genres   string          `json:"genres"`
		Poster   string          `json:"poster"`
		Quality  string          `json:"quality"`
		Rating   float64         `json:"rating"`
		Slug     string          `json:"slug"`
		Title    string          `json:"title"`
		Type     string          `json:"type"`
		URL      string          `json:"url"`
		Year     json.RawMessage `json:"year"`
	}

	var wire wireHomeMovie
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	year, err := decodeJSONScalarToString(wire.Year)
	if err != nil {
		return fmt.Errorf("decode home movie year: %w", err)
	}

	*m = HomeMovie{
		Duration: wire.Duration,
		Genres:   wire.Genres,
		Poster:   wire.Poster,
		Quality:  wire.Quality,
		Rating:   wire.Rating,
		Slug:     wire.Slug,
		Title:    wire.Title,
		Type:     wire.Type,
		URL:      wire.URL,
		Year:     year,
	}
	return nil
}

type DetailMovie struct {
	Poster   string      `json:"poster"`
	Related  []HomeMovie `json:"related"`
	Synopsis string      `json:"synopsis"`
	Tags     []string    `json:"tags"`
	Title    string      `json:"title"`
	URL      string      `json:"url"`
}

type Stream struct {
	StreamURL string `json:"stream_url"`
	Token     string `json:"token"`
}

type homeResponse struct {
	Data []HomeMovie `json:"data"`
}

type listResponse struct {
	Data []HomeMovie `json:"data"`
	Page int         `json:"page"`
}

type detailResponse struct {
	Data DetailMovie `json:"data"`
}

type streamResponse struct {
	StreamURL string `json:"stream_url"`
	Token     string `json:"token"`
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultMovieTubeBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (c *Client) GetHome(ctx context.Context) ([]HomeMovie, error) {
	var parsed homeResponse
	if err := c.getJSON(ctx, c.baseURL+"/home", &parsed); err != nil {
		return nil, err
	}
	return parsed.Data, nil
}

func (c *Client) GetGenre(ctx context.Context, genre string, page int) ([]HomeMovie, error) {
	genre = strings.Trim(strings.ToLower(genre), "/")
	if genre == "" {
		return nil, fmt.Errorf("genre is required")
	}
	if page <= 0 {
		page = 1
	}

	endpoint, err := url.Parse(fmt.Sprintf("%s/genre/%s", c.baseURL, url.PathEscape(genre)))
	if err != nil {
		return nil, fmt.Errorf("build kanata genre endpoint: %w", err)
	}
	params := endpoint.Query()
	params.Set("page", fmt.Sprintf("%d", page))
	endpoint.RawQuery = params.Encode()

	var parsed listResponse
	if err := c.getJSON(ctx, endpoint.String(), &parsed); err != nil {
		return nil, err
	}
	return parsed.Data, nil
}

func (c *Client) Search(ctx context.Context, query string, page int) ([]HomeMovie, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if page <= 0 {
		page = 1
	}

	endpoint, err := url.Parse(c.baseURL + "/search")
	if err != nil {
		return nil, fmt.Errorf("build kanata search endpoint: %w", err)
	}
	params := endpoint.Query()
	params.Set("q", query)
	params.Set("page", fmt.Sprintf("%d", page))
	endpoint.RawQuery = params.Encode()

	var parsed listResponse
	if err := c.getJSON(ctx, endpoint.String(), &parsed); err != nil {
		return nil, err
	}
	return parsed.Data, nil
}

func (c *Client) GetDetail(ctx context.Context, slug string) (DetailMovie, error) {
	endpoint := fmt.Sprintf("%s/detail/%s?type=movie", c.baseURL, url.PathEscape(strings.TrimSpace(slug)))
	var parsed detailResponse
	if err := c.getJSON(ctx, endpoint, &parsed); err != nil {
		return DetailMovie{}, err
	}
	return parsed.Data, nil
}

func (c *Client) GetStream(ctx context.Context, slug string) (Stream, error) {
	endpoint, err := url.Parse(c.baseURL + "/stream")
	if err != nil {
		return Stream{}, fmt.Errorf("build kanata stream endpoint: %w", err)
	}
	params := endpoint.Query()
	params.Set("id", strings.TrimSpace(slug))
	params.Set("type", "movie")
	endpoint.RawQuery = params.Encode()

	var parsed streamResponse
	if err := c.getJSON(ctx, endpoint.String(), &parsed); err != nil {
		return Stream{}, err
	}
	return Stream{
		StreamURL: strings.TrimSpace(parsed.StreamURL),
		Token:     strings.TrimSpace(parsed.Token),
	}, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("build kanata request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform kanata request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read kanata response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("kanata request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode kanata response: %w", err)
	}
	return nil
}

func decodeJSONScalarToString(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text), nil
	}

	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return strings.TrimSpace(number.String()), nil
	}

	var floatValue float64
	if err := json.Unmarshal(raw, &floatValue); err == nil {
		return strings.TrimSpace(strconv.FormatFloat(floatValue, 'f', -1, 64)), nil
	}

	return "", fmt.Errorf("unsupported json scalar %s", trimmed)
}
