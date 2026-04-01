package otakudesu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

func NewClient(baseURL, userAgent string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "Mozilla/5.0"
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		userAgent: userAgent,
	}
}

func (c *Client) SearchAnime(ctx context.Context, query string) ([]SearchResult, error) {
	endpoint, err := url.Parse(c.baseURL + "/")
	if err != nil {
		return nil, fmt.Errorf("build search url: %w", err)
	}
	values := endpoint.Query()
	values.Set("s", query)
	values.Set("post_type", "anime")
	endpoint.RawQuery = values.Encode()

	raw, err := c.fetch(ctx, endpoint.String())
	if err != nil {
		return nil, err
	}
	return ParseSearchHTML(raw)
}

func (c *Client) FetchAnimePage(ctx context.Context, rawURL string) (AnimePage, error) {
	raw, err := c.fetch(ctx, rawURL)
	if err != nil {
		return AnimePage{}, err
	}
	return ParseAnimeHTML(raw)
}

func (c *Client) FetchEpisodePage(ctx context.Context, rawURL string) (EpisodePage, error) {
	raw, err := c.fetch(ctx, rawURL)
	if err != nil {
		return EpisodePage{}, err
	}
	page, err := ParseEpisodeHTML(raw, rawURL)
	if err != nil {
		return EpisodePage{}, err
	}
	if len(page.MirrorRequests) == 0 {
		return page, nil
	}

	nonce, err := c.fetchMirrorNonce(ctx)
	if err != nil {
		return page, nil
	}
	if page.StreamMirrors == nil {
		page.StreamMirrors = make(map[string]string)
	}
	for _, request := range page.MirrorRequests {
		streamURL, err := c.resolveMirrorStream(ctx, nonce, request)
		if err != nil || strings.TrimSpace(streamURL) == "" {
			continue
		}
		page.StreamMirrors[request.Label] = streamURL
	}
	return page, nil
}

func (c *Client) fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return body, nil
}

func (c *Client) fetchMirrorNonce(ctx context.Context) (string, error) {
	values := url.Values{}
	values.Set("action", "aa1208d27f29ca340c92c66d1926f13f")

	body, err := c.postForm(ctx, c.baseURL+"/wp-admin/admin-ajax.php", values)
	if err != nil {
		return "", err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode otakudesu nonce response: %w", err)
	}
	return strings.TrimSpace(response.Data), nil
}

func (c *Client) resolveMirrorStream(ctx context.Context, nonce string, request EpisodeMirrorRequest) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(request.EncodedContent)
	if err != nil {
		return "", fmt.Errorf("decode mirror payload: %w", err)
	}

	values := url.Values{}
	if err := json.Unmarshal(payload, (*jsonMap)(&values)); err != nil {
		return "", fmt.Errorf("decode mirror payload json: %w", err)
	}
	values.Set("nonce", strings.TrimSpace(nonce))
	values.Set("action", "2a3505c93b0035d3f455df82bf976b84")

	body, err := c.postForm(ctx, c.baseURL+"/wp-admin/admin-ajax.php", values)
	if err != nil {
		return "", err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode otakudesu mirror response: %w", err)
	}

	decodedHTML, err := base64.StdEncoding.DecodeString(strings.TrimSpace(response.Data))
	if err != nil {
		return "", fmt.Errorf("decode otakudesu mirror html: %w", err)
	}
	return ParseEmbedHTML(decodedHTML), nil
}

func (c *Client) postForm(ctx context.Context, rawURL string, values url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build post request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", c.baseURL+"/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform post request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read post response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected post status %d", resp.StatusCode)
	}
	return body, nil
}

type jsonMap url.Values

func (m *jsonMap) UnmarshalJSON(data []byte) error {
	raw := map[string]any{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	values := url.Values{}
	for key, value := range raw {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		values.Set(trimmedKey, fmt.Sprint(value))
	}
	*m = jsonMap(values)
	return nil
}
