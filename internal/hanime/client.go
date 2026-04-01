package hanime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Fetcher interface {
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
		},
		userAgent: strings.TrimSpace(userAgent),
		cookie:    strings.TrimSpace(cookie),
	}
}

func (c *Client) FetchPage(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
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
			if err := sleepWithContext(ctx, time.Duration(attempt)*500*time.Millisecond); err != nil {
				return nil, err
			}
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read response body: %w", readErr)
			if err := sleepWithContext(ctx, time.Duration(attempt)*500*time.Millisecond); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
			if attempt < 3 && isRetryableStatus(resp.StatusCode) {
				if err := sleepWithContext(ctx, retryDelay(resp, attempt)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}
		return body, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("page request failed without response")
}

func isRetryableStatus(status int) bool {
	switch status {
	case 403, 408, 425, 429:
		return true
	default:
		return status >= 500
	}
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if raw := strings.TrimSpace(resp.Header.Get("Retry-After")); raw != "" {
			if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
			if when, err := http.ParseTime(raw); err == nil {
				if delay := time.Until(when); delay > 0 {
					return delay
				}
			}
		}
	}
	return time.Duration(attempt) * time.Second
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
