package kusonime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
}

func NewClient(baseURL, userAgent string, timeout time.Duration) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://kusonime.com"
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "Mozilla/5.0"
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL:   baseURL,
		userAgent: userAgent,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) SearchAnime(ctx context.Context, query string) ([]SearchResult, error) {
	endpoint, err := url.Parse(c.baseURL + "/")
	if err != nil {
		return nil, fmt.Errorf("parse search endpoint: %w", err)
	}
	values := endpoint.Query()
	values.Set("s", strings.TrimSpace(query))
	values.Set("post_type", "post")
	endpoint.RawQuery = values.Encode()

	body, err := c.fetch(ctx, endpoint.String())
	if err != nil {
		return nil, err
	}
	return ParseSearchHTML(body)
}

func (c *Client) FetchAnimePage(ctx context.Context, rawURL string) (AnimePage, error) {
	body, err := c.fetch(ctx, rawURL)
	if err != nil {
		return AnimePage{}, err
	}
	page, err := ParseAnimeHTML(body)
	if err != nil {
		return AnimePage{}, err
	}
	if page.URL == "" {
		page.URL = strings.TrimSpace(rawURL)
	}
	return page, nil
}

func (c *Client) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request %s: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rawURL, err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch %s failed with status %d", rawURL, resp.StatusCode)
	}
	return body, nil
}
