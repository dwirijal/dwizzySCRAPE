package samehadaku

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Fetcher interface {
	FetchCatalogPage(ctx context.Context, url string) ([]byte, error)
	FetchPage(ctx context.Context, url string) ([]byte, error)
}

type HTTPClient struct {
	client    *http.Client
	userAgent string
	cookie    string
}

type ProbeResult struct {
	URL        string
	FinalURL   string
	StatusCode int
	Body       []byte
}

func NewHTTPClient(userAgent, cookie string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &HTTPClient{
		client:    &http.Client{Timeout: timeout},
		userAgent: strings.TrimSpace(userAgent),
		cookie:    strings.TrimSpace(cookie),
	}
}

func (c *HTTPClient) FetchCatalogPage(ctx context.Context, url string) ([]byte, error) {
	return c.FetchPage(ctx, url)
}

func (c *HTTPClient) FetchPage(ctx context.Context, url string) ([]byte, error) {
	result, err := c.FetchPageRaw(ctx, url)
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(result.Body), "cf-turnstile-response") || strings.Contains(string(result.Body), "Just a moment...") {
		return nil, fmt.Errorf("cloudflare challenge detected; provide a valid SAMEHADAKU_COOKIE from a solved browser session")
	}
	return result.Body, nil
}

func (c *HTTPClient) FetchPageRaw(ctx context.Context, url string) (ProbeResult, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return ProbeResult{}, fmt.Errorf("build request: %w", err)
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
			lastErr = fmt.Errorf("perform request: %w", err)
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
				continue
			}
			return ProbeResult{}, lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read response body: %w", readErr)
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
				continue
			}
			return ProbeResult{}, lastErr
		}
		if resp.StatusCode >= 400 {
			return ProbeResult{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
		}
		return ProbeResult{
			URL:        url,
			FinalURL:   resp.Request.URL.String(),
			StatusCode: resp.StatusCode,
			Body:       body,
		}, nil
	}
	if lastErr != nil {
		return ProbeResult{}, lastErr
	}
	return ProbeResult{}, fmt.Errorf("request failed without response")
}
