package store

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
)

func TestAnimeDetailStoreUpsert(t *testing.T) {
	t.Helper()

	scrapedAt := time.Date(2026, 3, 22, 16, 0, 0, 0, time.UTC)
	detail := samehadaku.AnimeDetail{
		Slug:                  "ao-no-orchestra-season-2",
		CanonicalURL:          "https://v2.samehadaku.how/anime/ao-no-orchestra-season-2/",
		PrimarySourceURL:      "https://v2.samehadaku.how/anime/ao-no-orchestra-season-2/",
		PrimarySourceDomain:   "v2.samehadaku.how",
		SecondarySourceURL:    "https://samehadaku.li/anime/ao-no-orchestra-season-2/",
		SecondarySourceDomain: "samehadaku.li",
		EffectiveSourceURL:    "https://samehadaku.li/anime/ao-no-orchestra-season-2/",
		EffectiveSourceDomain: "samehadaku.li",
		EffectiveSourceKind:   "secondary",
		SourceTitle:           "Ao no Orchestra Season 2",
		MALID:                 56877,
		MALURL:                "https://myanimelist.net/anime/56877/Ao_no_Orchestra_Season_2",
		MALThumbnailURL:       "https://cdn.myanimelist.net/images/anime/1078/151796.webp",
		SynopsisSource:        "",
		SynopsisEnriched:      "Following the regular summer concert...",
		AnimeType:             "TV",
		Status:                "Finished Airing",
		Season:                "fall",
		StudioNames:           []string{"Nippon Animation"},
		GenreNames:            []string{"Drama", "Music", "School"},
		BatchLinksJSON:        []byte(`{"Batch":"https://gofile.io/d/demo-batch"}`),
		CastJSON:              []byte(`[{"character_mal_id":1,"name":"Hajime Aono"}]`),
		SourceMetaJSON:        []byte(`{"samehadaku_fetch_status":"primary_challenge_blocked_secondary_fetched"}`),
		JikanMetaJSON:         []byte(`{"score":7.5}`),
		SourceFetchStatus:     "primary_challenge_blocked_secondary_fetched",
		SourceFetchError:      "primary: cloudflare challenge detected",
		ScrapedAt:             scrapedAt,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/rest/v1/rpc/upsert_samehadaku_anime_detail_v2" {
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
		if got := rows[0]["slug"]; got != "ao-no-orchestra-season-2" {
			t.Fatalf("unexpected slug %#v", got)
		}
		if got := rows[0]["source_fetch_status"]; got != "primary_challenge_blocked_secondary_fetched" {
			t.Fatalf("unexpected source_fetch_status %#v", got)
		}
		if got := rows[0]["primary_source_domain"]; got != "v2.samehadaku.how" {
			t.Fatalf("unexpected primary_source_domain %#v", got)
		}
		if got := rows[0]["effective_source_kind"]; got != "secondary" {
			t.Fatalf("unexpected effective_source_kind %#v", got)
		}
		if got := rows[0]["mal_id"]; got != float64(56877) {
			t.Fatalf("unexpected mal_id %#v", got)
		}
		if got := rows[0]["batch_links_json"]; got == nil {
			t.Fatalf("expected batch_links_json to be present")
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	store := NewAnimeDetailStore(server.Client(), server.URL, "sb_secret_test")
	if err := store.UpsertAnimeDetail(context.Background(), detail); err != nil {
		t.Fatalf("UpsertAnimeDetail returned error: %v", err)
	}
}

func TestAnimeDetailStoreListAnimeSlugs(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/rest/v1/anime_detail_ready_v2_view" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("select"); got != "slug" {
			t.Fatalf("unexpected select %q", got)
		}
		if got := r.URL.Query().Get("order"); got != "slug.asc" {
			t.Fatalf("unexpected order %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "3" {
			t.Fatalf("unexpected limit %q", got)
		}
		if got := r.URL.Query().Get("offset"); got != "6" {
			t.Fatalf("unexpected offset %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"slug":"ao-no-orchestra-season-2"},{"slug":"ao-no-orchestra-season-2"},{"slug":"yamada-kun-to-lv999-no-koi-wo-suru"}]`))
	}))
	defer server.Close()

	store := NewAnimeDetailStore(server.Client(), server.URL, "sb_secret_test")
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
