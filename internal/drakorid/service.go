package drakorid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type episodeAPIResponse struct {
	Status           int    `json:"status"`
	Type             string `json:"tipe"`
	SID              string `json:"sid"`
	StreamingPremium string `json:"streaming_premium"`
	Streaming        string `json:"streaming"`
	EpisodeName      string `json:"episode_name"`
	ImageURL         string `json:"img"`
	IsAnyCDN         bool   `json:"is_any_cdn"`
	IsAnyRTMP        bool   `json:"is_any_rtmp"`
	IsAnyAlternative bool   `json:"is_any_alternative"`
}

type Service struct {
	fetcher Fetcher
	baseURL string
	now     func() time.Time
}

func NewService(fetcher Fetcher, baseURL string, fixedNow time.Time) *Service {
	nowFn := time.Now
	if !fixedNow.IsZero() {
		nowFn = func() time.Time { return fixedNow }
	}
	return &Service{
		fetcher: fetcher,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		now:     nowFn,
	}
}

func (s *Service) FetchOngoingCatalog(ctx context.Context, page int) ([]CatalogItem, error) {
	target := s.baseURL + "/drama-ongoing/"
	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return nil, err
	}
	token, err := ParsePageToken(raw)
	if err != nil {
		return nil, err
	}
	payload, err := s.fetcher.PostForm(ctx, s.baseURL+"/ajax/drama/ongoing.php", url.Values{
		"token": {token},
		"page":  {strconv.Itoa(page)},
	})
	if err != nil {
		return nil, err
	}
	items, err := ParseOngoingHTML(payload, target, page)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].ScrapedAt = s.now().UTC()
	}
	return items, nil
}

func (s *Service) FetchMovieCatalog(ctx context.Context, page int) ([]CatalogItem, error) {
	target := fmt.Sprintf("%s/kategori/film-korea/%d", s.baseURL, page)
	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return nil, err
	}
	items, err := ParseMovieCatalogHTML(raw, target, page)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].ScrapedAt = s.now().UTC()
	}
	return items, nil
}

func (s *Service) FetchDetail(ctx context.Context, slug string) (Detail, error) {
	target := normalizeDetailURL(s.baseURL, slug)
	raw, err := s.fetcher.FetchPage(ctx, target)
	if err != nil {
		return Detail{}, err
	}
	detail, err := ParseDetailHTML(raw, target)
	if err != nil {
		return Detail{}, err
	}
	detail.ScrapedAt = s.now().UTC()
	return detail, nil
}

func (s *Service) FetchEpisodeDetail(ctx context.Context, item Detail, ref EpisodeRef) (EpisodeDetail, error) {
	number := firstNonBlank(ref.Number, "1")
	result := EpisodeDetail{
		MediaType:     item.MediaType,
		ItemSlug:      item.Slug,
		EpisodeSlug:   buildEpisodeSlug(item.Slug, number),
		CanonicalURL:  strings.TrimSpace(ref.CanonicalURL),
		Title:         firstNonBlank(ref.Title, episodeTitle(item.Title, number)),
		Label:         firstNonBlank(ref.Label, "Episode "+number),
		EpisodeNumber: parseNumber(number),
		ScrapedAt:     s.now().UTC(),
	}

	apiPayload, apiMeta := s.fetchEpisodeAPI(ctx, item, number)
	streamLinks := make(map[string]any)
	streamMeta := make(map[string]string)
	for _, server := range []string{"lite", "max", "fast"} {
		target := fmt.Sprintf("%s/watch-%s/%s/%s", s.baseURL, server, item.Slug, number)
		raw, err := s.fetcher.FetchPage(ctx, target)
		if err != nil {
			continue
		}
		streamURL, meta, err := ParseWatchHTML(raw)
		if err != nil {
			continue
		}
		streamLinks[server] = map[string]any{
			"stream_url": streamURL,
			"meta":       meta,
			"watch_url":  target,
		}
		if result.StreamURL == "" {
			result.StreamURL = streamURL
		}
		streamMeta[server] = target
	}

	downloadLinks := make(map[string]map[string]string)
	downloadMeta := make(map[string]string)
	for _, server := range []string{"lite", "fast"} {
		target := fmt.Sprintf("%s/download-%s/%s/%s", s.baseURL, server, item.Slug, number)
		raw, err := s.fetcher.FetchPage(ctx, target)
		if err != nil {
			continue
		}
		links, err := ParseDownloadHTML(raw)
		if err != nil {
			continue
		}
		for quality, hosts := range links {
			if _, ok := downloadLinks[quality]; !ok {
				downloadLinks[quality] = make(map[string]string)
			}
			for host, href := range hosts {
				downloadLinks[quality][server+"_"+host] = href
			}
		}
		downloadMeta[server] = target
	}

	if apiPayload.StreamingPremium != "" {
		resolved := apiPayload.StreamingPremium
		if finalURL, err := s.fetcher.ResolveFinalURL(ctx, apiPayload.StreamingPremium); err == nil && strings.TrimSpace(finalURL) != "" {
			resolved = strings.TrimSpace(finalURL)
		}
		if result.StreamURL == "" {
			result.StreamURL = resolved
		}
		if _, ok := streamLinks["api"]; !ok {
			streamLinks["api"] = map[string]any{
				"stream_url":       resolved,
				"premium_redirect": apiPayload.StreamingPremium,
			}
		}
		if len(downloadLinks) == 0 {
			downloadLinks["SOURCE"] = map[string]string{
				"premium_redirect": apiPayload.StreamingPremium,
				"direct":           resolved,
			}
		}
	}

	if len(streamLinks) == 0 && len(downloadLinks) == 0 {
		return EpisodeDetail{}, fmt.Errorf("episode %s missing stream and download links", result.EpisodeSlug)
	}

	streamLinksJSON, err := json.Marshal(streamLinks)
	if err != nil {
		return EpisodeDetail{}, fmt.Errorf("encode stream links json: %w", err)
	}
	downloadLinksJSON, err := json.Marshal(downloadLinks)
	if err != nil {
		return EpisodeDetail{}, fmt.Errorf("encode download links json: %w", err)
	}
	sourceMetaJSON, err := json.Marshal(map[string]any{
		"watch_pages":    streamMeta,
		"download_pages": downloadMeta,
		"media_type":     item.MediaType,
		"item_slug":      item.Slug,
		"episode_api":    apiMeta,
	})
	if err != nil {
		return EpisodeDetail{}, fmt.Errorf("encode source meta json: %w", err)
	}
	result.StreamLinksJSON = streamLinksJSON
	result.DownloadLinksJSON = downloadLinksJSON
	result.SourceMetaJSON = sourceMetaJSON
	return result, nil
}

func (s *Service) fetchEpisodeAPI(ctx context.Context, item Detail, number string) (episodeAPIResponse, map[string]any) {
	if strings.TrimSpace(item.DetailToken) == "" || item.SourceItemID <= 0 {
		return episodeAPIResponse{}, map[string]any{"available": false}
	}
	raw, err := s.fetcher.PostForm(ctx, s.baseURL+"/myapi/episode_detail.php", url.Values{
		"token":   {item.DetailToken},
		"id":      {strconv.Itoa(item.SourceItemID)},
		"episode": {strings.TrimSpace(number)},
	})
	if err != nil {
		return episodeAPIResponse{}, map[string]any{"available": true, "error": err.Error()}
	}
	var payload episodeAPIResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return episodeAPIResponse{}, map[string]any{"available": true, "error": err.Error(), "raw": string(raw)}
	}
	return payload, map[string]any{
		"available":          true,
		"status":             payload.Status,
		"sid":                payload.SID,
		"type":               payload.Type,
		"streaming_premium":  payload.StreamingPremium,
		"streaming":          payload.Streaming,
		"episode_name":       payload.EpisodeName,
		"img":                payload.ImageURL,
		"is_any_cdn":         payload.IsAnyCDN,
		"is_any_rtmp":        payload.IsAnyRTMP,
		"is_any_alternative": payload.IsAnyAlternative,
	}
}

func normalizeDetailURL(baseURL, slug string) string {
	slug = strings.TrimSpace(slug)
	if strings.HasPrefix(slug, "http://") || strings.HasPrefix(slug, "https://") {
		return slug
	}
	return strings.TrimRight(baseURL, "/") + "/nonton/" + strings.Trim(slug, "/") + "/"
}

func buildEpisodeSlug(itemSlug, number string) string {
	number = strings.TrimSpace(number)
	if number == "" {
		number = "1"
	}
	return itemSlug + "-episode-" + strings.ReplaceAll(number, ".", "-")
}
