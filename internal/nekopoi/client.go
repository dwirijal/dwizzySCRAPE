package nekopoi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Fetcher interface {
	FetchFeed(ctx context.Context, rawURL string) ([]byte, error)
	FetchPage(ctx context.Context, rawURL string) ([]byte, error)
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
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSHandshakeTimeout = timeout
	transport.ResponseHeaderTimeout = timeout
	transport.ExpectContinueTimeout = 2 * time.Second
	return &Client{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		userAgent: strings.TrimSpace(userAgent),
		cookie:    strings.TrimSpace(cookie),
	}
}

func (c *Client) FetchFeed(ctx context.Context, rawURL string) ([]byte, error) {
	return c.fetch(ctx, rawURL, "application/rss+xml,application/xml;q=0.9,text/xml;q=0.8,*/*;q=0.7")
}

func (c *Client) FetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	return c.fetch(ctx, rawURL, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
}

func (c *Client) fetch(ctx context.Context, rawURL, accept string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", accept)
		req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
		req.Header.Set("Cache-Control", "no-cache")
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
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return nil, lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read response body: %w", readErr)
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
			if isRetryableStatus(resp.StatusCode) && attempt < 5 {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			lowerBody := strings.ToLower(string(body))
			if !strings.Contains(lowerBody, "<rss") {
				lastErr = fmt.Errorf("unexpected redirect status %d without rss body", resp.StatusCode)
				if attempt < 5 {
					time.Sleep(time.Duration(attempt) * 2 * time.Second)
					continue
				}
				return nil, lastErr
			}
		}
		return body, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("feed request failed without response")
}

func isRetryableStatus(status int) bool {
	switch status {
	case 403, 408, 425, 429, 468:
		return true
	}
	return status >= 500
}
