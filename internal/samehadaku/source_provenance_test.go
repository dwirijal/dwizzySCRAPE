package samehadaku

import "testing"

func TestBuildOverallFetchStatus(t *testing.T) {
	t.Helper()

	primary := SourceAttempt{Kind: "primary", URL: "https://v2.samehadaku.how/anime/demo/", Domain: "v2.samehadaku.how", Status: "challenge_blocked", Error: "blocked"}
	secondary := SourceAttempt{Kind: "secondary", URL: "https://samehadaku.li/anime/demo/", Domain: "samehadaku.li", Status: "fetched"}

	if got := buildOverallFetchStatus(primary, secondary); got != "primary_challenge_blocked_secondary_fetched" {
		t.Fatalf("unexpected status %q", got)
	}
	kind, targetURL, domain := resolveEffectiveSource(primary, secondary)
	if kind != "secondary" || targetURL != secondary.URL || domain != secondary.Domain {
		t.Fatalf("unexpected effective source: %q %q %q", kind, targetURL, domain)
	}
	if got := buildFetchError(primary, secondary); got != "primary: blocked" {
		t.Fatalf("unexpected fetch error %q", got)
	}
}

func TestBuildOverallFetchStatusRedirectedOffsite(t *testing.T) {
	t.Helper()

	primary := SourceAttempt{Kind: "primary", URL: "https://v2.samehadaku.how/demo-episode/", Domain: "v2.samehadaku.how", Status: "redirected_offsite", Error: "https://linktr.ee/samehadaku.care"}
	secondary := SourceAttempt{Kind: "secondary", URL: "https://samehadaku.li/demo-episode/", Domain: "samehadaku.li", Status: "fetched"}

	if got := buildOverallFetchStatus(primary, secondary); got != "primary_redirected_offsite_secondary_fetched" {
		t.Fatalf("unexpected status %q", got)
	}
	if got := buildFetchError(primary, secondary); got != "primary: https://linktr.ee/samehadaku.care" {
		t.Fatalf("unexpected fetch error %q", got)
	}
}
