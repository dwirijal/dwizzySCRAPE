package store

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
)

func TestEpisodeDetailStoreUpsert(t *testing.T) {
	t.Helper()

	scrapedAt := time.Date(2026, 3, 22, 16, 30, 0, 0, time.UTC)
	details := []samehadaku.EpisodeDetail{
		{
			AnimeSlug:             "ao-no-orchestra-season-2",
			EpisodeSlug:           "ao-no-orchestra-season-2-episode-1-subtitle-indonesia",
			CanonicalURL:          "https://v2.samehadaku.how/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/",
			PrimarySourceURL:      "https://v2.samehadaku.how/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/",
			PrimarySourceDomain:   "v2.samehadaku.how",
			SecondarySourceURL:    "https://samehadaku.li/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/",
			SecondarySourceDomain: "samehadaku.li",
			EffectiveSourceURL:    "https://samehadaku.li/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/",
			EffectiveSourceDomain: "samehadaku.li",
			EffectiveSourceKind:   "secondary",
			Title:                 "Ao no Orchestra Season 2 Episode 1 Subtitle Indonesia",
			EpisodeNumber:         1,
			ReleaseLabel:          "October 5, 2025",
			StreamLinksJSON:       []byte(`{"primary":"https://video.example/embed/1","mirrors":{"Video":"https://video.example/embed/1"}}`),
			DownloadLinksJSON:     []byte(`{"Download":"https://gofile.io/d/demo"}`),
			SourceMetaJSON:        []byte(`{"source":"samehadaku","fetch_status":"primary_challenge_blocked_secondary_fetched"}`),
			FetchStatus:           "primary_challenge_blocked_secondary_fetched",
			FetchError:            "primary: cloudflare challenge detected",
			ScrapedAt:             scrapedAt,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/rest/v1/rpc/upsert_samehadaku_episode_v2" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload map[string][]map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		rows := payload["payload"]
		if len(rows) != 1 {
			t.Fatalf("expected single payload item, got %d", len(rows))
		}
		if got := rows[0]["episode_slug"]; got != "ao-no-orchestra-season-2-episode-1-subtitle-indonesia" {
			t.Fatalf("unexpected episode_slug %#v", got)
		}
		if got := rows[0]["canonical_url"]; got != "https://v2.samehadaku.how/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/" {
			t.Fatalf("unexpected canonical_url %#v", got)
		}
		if got := rows[0]["effective_source_kind"]; got != "secondary" {
			t.Fatalf("unexpected effective_source_kind %#v", got)
		}
		if got := rows[0]["fetch_status"]; got != "primary_challenge_blocked_secondary_fetched" {
			t.Fatalf("unexpected fetch_status %#v", got)
		}
		if !strings.Contains(string(body), "\"Download\":\"https://gofile.io/d/demo\"") {
			t.Fatalf("expected download url in payload, got %s", string(body))
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	store := NewEpisodeDetailStore(server.Client(), server.URL, "sb_secret_test")
	upserted, err := store.UpsertEpisodeDetails(context.Background(), details)
	if err != nil {
		t.Fatalf("UpsertEpisodeDetails returned error: %v", err)
	}
	if upserted != 1 {
		t.Fatalf("expected 1 upserted row, got %d", upserted)
	}
}

func TestEpisodeDetailStoreListAnimeSlugs(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/rest/v1/anime_stream_ready_v2_view" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("select"); got != "anime_slug" {
			t.Fatalf("unexpected select %q", got)
		}
		if got := r.URL.Query().Get("order"); got != "anime_slug.asc" {
			t.Fatalf("unexpected order %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "3" {
			t.Fatalf("unexpected limit %q", got)
		}
		if got := r.URL.Query().Get("offset"); got != "6" {
			t.Fatalf("unexpected offset %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"anime_slug":"ao-no-orchestra-season-2"},{"anime_slug":"ao-no-orchestra-season-2"},{"anime_slug":"yamada-kun-to-lv999-no-koi-wo-suru"}]`))
	}))
	defer server.Close()

	store := NewEpisodeDetailStore(server.Client(), server.URL, "sb_secret_test")
	slugs, err := store.ListAnimeSlugs(context.Background(), 6, 3)
	if err != nil {
		t.Fatalf("ListAnimeSlugs returned error: %v", err)
	}
	if len(slugs) != 2 {
		t.Fatalf("expected 2 unique slugs, got %d", len(slugs))
	}
	if slugs[0] != "ao-no-orchestra-season-2" {
		t.Fatalf("unexpected first slug %q", slugs[0])
	}
	if slugs[1] != "yamada-kun-to-lv999-no-koi-wo-suru" {
		t.Fatalf("unexpected second slug %q", slugs[1])
	}
}
