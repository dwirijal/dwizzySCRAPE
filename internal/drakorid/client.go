package drakorid

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Fetcher interface {
	FetchPage(ctx context.Context, rawURL string) ([]byte, error)
	PostForm(ctx context.Context, rawURL string, values url.Values) ([]byte, error)
	ResolveFinalURL(ctx context.Context, rawURL string) (string, error)
}

type Client struct {
	client    *http.Client
	userAgent string
	cookie    string
}

func NewClient(userAgent, cookie string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
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
	c.applyHeaders(req)
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

func (c *Client) PostForm(ctx context.Context, rawURL string, values url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
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

func (c *Client) ResolveFinalURL(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	c.applyHeaders(req)
	client := *c.client
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if location := strings.TrimSpace(resp.Header.Get("Location")); location != "" {
			return location, nil
		}
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return rawURL, nil
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
}
