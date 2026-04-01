package linkresolver

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Result struct {
	OriginalURL string   `json:"original_url"`
	FinalURL    string   `json:"final_url"`
	Hops        []string `json:"hops"`
	StatusCode  int      `json:"status_code"`
	FinalHost   string   `json:"final_host"`
	Error       string   `json:"error,omitempty"`
}

type Resolver struct {
	client    *http.Client
	userAgent string
}

func New(userAgent string, timeout time.Duration) *Resolver {
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "Mozilla/5.0"
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	return &Resolver{
		client: &http.Client{
			Timeout: timeout,
		},
		userAgent: userAgent,
	}
}

func (r *Resolver) Resolve(ctx context.Context, rawURL string) Result {
	rawURL = strings.TrimSpace(rawURL)
	result := Result{OriginalURL: rawURL}
	if rawURL == "" {
		result.Error = "empty_url"
		return result
	}

	var hops []string
	client := *r.client
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		hops = hops[:0]
		for _, previous := range via {
			hops = append(hops, previous.URL.String())
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("User-Agent", r.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		result.Hops = append([]string(nil), hops...)
		return result
	}
	defer resp.Body.Close()

	result.Hops = append([]string(nil), hops...)
	result.StatusCode = resp.StatusCode
	result.FinalURL = resp.Request.URL.String()
	result.FinalHost = normalizeHost(resp.Request.URL)
	return result
}

func normalizeHost(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	host = strings.TrimPrefix(host, "www.")
	return host
}
