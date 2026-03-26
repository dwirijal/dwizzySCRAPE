package samehadaku

import (
	"context"
	"strings"
)

const DefaultPrimaryBaseURL = "https://v2.samehadaku.how"

type SourceAttempt struct {
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Domain string `json:"domain"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func BuildPrimaryAnimeURL(slug string) string {
	return strings.TrimRight(DefaultPrimaryBaseURL, "/") + "/anime/" + strings.Trim(strings.TrimSpace(slug), "/") + "/"
}

func BuildPrimaryEpisodeURL(slug string) string {
	return strings.TrimRight(DefaultPrimaryBaseURL, "/") + "/" + strings.Trim(strings.TrimSpace(slug), "/") + "/"
}

func BuildSecondaryEpisodeURL(slug string) string {
	return strings.TrimRight(DefaultMirrorBaseURL, "/") + "/" + strings.Trim(strings.TrimSpace(slug), "/") + "/"
}

func fetchSourceAttempt(ctx context.Context, fetcher PageFetcher, kind, targetURL string) (SourceAttempt, []byte) {
	attempt := SourceAttempt{
		Kind:   strings.TrimSpace(kind),
		URL:    strings.TrimSpace(targetURL),
		Domain: extractDomain(targetURL),
		Status: "not_attempted",
	}
	if fetcher == nil || attempt.URL == "" {
		return attempt, nil
	}

	rawFetcher, ok := fetcher.(interface {
		FetchPageRaw(ctx context.Context, url string) (ProbeResult, error)
	})
	if !ok {
		body, err := fetcher.FetchPage(ctx, attempt.URL)
		if err != nil {
			attempt.Status = classifyFetchError(err)
			attempt.Error = strings.TrimSpace(err.Error())
			return attempt, nil
		}
		if len(body) == 0 {
			attempt.Status = "empty"
			return attempt, body
		}
		attempt.Status = "fetched"
		return attempt, body
	}

	result, err := rawFetcher.FetchPageRaw(ctx, attempt.URL)
	if err != nil {
		attempt.Status = classifyFetchError(err)
		attempt.Error = strings.TrimSpace(err.Error())
		return attempt, nil
	}
	finalDomain := extractDomain(result.FinalURL)
	if finalDomain != "" && attempt.Domain != "" && !strings.EqualFold(finalDomain, attempt.Domain) {
		attempt.Status = "redirected_offsite"
		attempt.Error = strings.TrimSpace(result.FinalURL)
		return attempt, nil
	}
	if len(result.Body) == 0 {
		attempt.Status = "empty"
		return attempt, result.Body
	}

	attempt.Status = "fetched"
	return attempt, result.Body
}

func classifyFetchError(err error) string {
	if err == nil {
		return "fetched"
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "cloudflare challenge"):
		return "challenge_blocked"
	case message == "":
		return "fetch_failed"
	default:
		return "fetch_failed"
	}
}

func resolveEffectiveSource(primary, secondary SourceAttempt) (kind, targetURL, domain string) {
	if primary.Status == "fetched" {
		return "primary", primary.URL, primary.Domain
	}
	if secondary.Status == "fetched" {
		return "secondary", secondary.URL, secondary.Domain
	}
	if primary.URL != "" {
		return "primary", primary.URL, primary.Domain
	}
	if secondary.URL != "" {
		return "secondary", secondary.URL, secondary.Domain
	}
	return "unknown", "", ""
}

func buildOverallFetchStatus(primary, secondary SourceAttempt) string {
	if primary.Status == "fetched" && secondary.Status == "not_attempted" {
		return "primary_fetched"
	}
	if primary.Status == "fetched" && secondary.Status == "fetched" {
		return "primary_fetched_secondary_fetched"
	}
	if primary.Status == "challenge_blocked" && secondary.Status == "fetched" {
		return "primary_challenge_blocked_secondary_fetched"
	}
	if primary.Status == "redirected_offsite" && secondary.Status == "fetched" {
		return "primary_redirected_offsite_secondary_fetched"
	}
	if primary.Status == "fetch_failed" && secondary.Status == "fetched" {
		return "primary_fetch_failed_secondary_fetched"
	}
	if primary.Status == "challenge_blocked" && secondary.Status == "fetch_failed" {
		return "primary_challenge_blocked_secondary_failed"
	}
	if primary.Status == "challenge_blocked" && secondary.Status == "not_attempted" {
		return "primary_challenge_blocked"
	}
	if primary.Status == "redirected_offsite" && secondary.Status == "not_attempted" {
		return "primary_redirected_offsite"
	}
	if primary.Status == "fetch_failed" && secondary.Status == "not_attempted" {
		return "primary_fetch_failed"
	}
	if primary.Status == "empty" && secondary.Status == "fetched" {
		return "primary_empty_secondary_fetched"
	}
	if primary.Status == "empty" && secondary.Status == "not_attempted" {
		return "primary_empty"
	}
	if secondary.Status == "fetched" {
		return "secondary_fetched"
	}
	if primary.Status != "" && primary.Status != "not_attempted" {
		return "primary_" + primary.Status
	}
	return "pending"
}

func buildFetchError(primary, secondary SourceAttempt) string {
	errors := make([]string, 0, 2)
	if primary.Error != "" {
		errors = append(errors, "primary: "+primary.Error)
	}
	if secondary.Error != "" {
		errors = append(errors, "secondary: "+secondary.Error)
	}
	return strings.Join(errors, " | ")
}
