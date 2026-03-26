package manhwaindo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Fetcher interface {
	FetchPage(ctx context.Context, rawURL string) ([]byte, error)
}

type Client struct {
	baseURL   string
	client    *http.Client
	userAgent string
	cookie    string
}

func NewClient(baseURL, userAgent, cookie string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:    &http.Client{Timeout: timeout},
		userAgent: strings.TrimSpace(userAgent),
		cookie:    strings.TrimSpace(cookie),
	}
}

func (c *Client) FetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return body, nil
}
