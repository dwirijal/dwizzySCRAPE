package samehadaku

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type PlayerOptionResolution struct {
	Label      string    `json:"label"`
	PostID     string    `json:"post_id"`
	Number     string    `json:"number"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	EmbedURL   string    `json:"embed_url,omitempty"`
	SourceKind string    `json:"source_kind"`
	ResolvedAt time.Time `json:"resolved_at"`
	Error      string    `json:"error,omitempty"`
}

type PlayerOptionResolver interface {
	ResolvePlayerOption(ctx context.Context, refererURL string, option PrimaryServerOption) (PlayerOptionResolution, error)
}

func (c *HTTPClient) ResolvePlayerOption(ctx context.Context, refererURL string, option PrimaryServerOption) (PlayerOptionResolution, error) {
	resolution := PlayerOptionResolution{
		Label:      strings.TrimSpace(option.Label),
		PostID:     strings.TrimSpace(option.PostID),
		Number:     strings.TrimSpace(option.Number),
		Type:       strings.TrimSpace(option.Type),
		Status:     "not_attempted",
		SourceKind: "primary",
		ResolvedAt: time.Now().UTC(),
	}

	if c == nil {
		return resolution, fmt.Errorf("http client is required")
	}
	if strings.TrimSpace(refererURL) == "" {
		return resolution, fmt.Errorf("referer url is required")
	}
	if resolution.PostID == "" || resolution.Number == "" || resolution.Type == "" {
		return resolution, fmt.Errorf("player option is incomplete")
	}

	adminURL, err := buildAdminAJAXURL(refererURL)
	if err != nil {
		return resolution, err
	}

	form := url.Values{}
	form.Set("action", "player_ajax")
	form.Set("post", resolution.PostID)
	form.Set("nume", resolution.Number)
	form.Set("type", resolution.Type)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, adminURL, strings.NewReader(form.Encode()))
	if err != nil {
		return resolution, fmt.Errorf("build player ajax request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", originFromURL(refererURL))
	req.Header.Set("Referer", refererURL)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return resolution, fmt.Errorf("perform player ajax request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resolution, fmt.Errorf("read player ajax response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return resolution, fmt.Errorf("player ajax unexpected status %d", resp.StatusCode)
	}
	if strings.Contains(string(body), "cf-turnstile-response") || strings.Contains(string(body), "Just a moment...") {
		return resolution, fmt.Errorf("cloudflare challenge detected on player ajax")
	}

	embedURL, err := ParsePlayerEmbedHTML(body)
	if err != nil {
		return resolution, err
	}

	resolution.Status = "resolved"
	resolution.EmbedURL = embedURL
	return resolution, nil
}

func ParsePlayerEmbedHTML(raw []byte) (string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("parse player ajax html: %w", err)
	}

	iframe := strings.TrimSpace(attrOrEmpty(doc.Find("iframe").First(), "src"))
	if iframe == "" {
		return "", fmt.Errorf("player ajax response missing iframe src")
	}
	return iframe, nil
}

func buildAdminAJAXURL(refererURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(refererURL))
	if err != nil {
		return "", fmt.Errorf("parse referer url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("referer url must include scheme and host")
	}
	parsed.Path = "/wp-admin/admin-ajax.php"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func originFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
