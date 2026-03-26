package jikan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSearchAnime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anime" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "Ao no Orchestra Season 2" {
			t.Fatalf("unexpected query %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("unexpected limit %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"mal_id":56877,"title":"Ao no Orchestra Season 2","title_synonyms":["Blue Orchestra Season 2"],"url":"https://myanimelist.net/anime/56877/Ao_no_Orchestra_Season_2","images":{"webp":{"large_image_url":"https://cdn.myanimelist.net/images/anime/1078/151796.webp"}}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	results, err := client.SearchAnime(context.Background(), "Ao no Orchestra Season 2", 5)
	if err != nil {
		t.Fatalf("SearchAnime returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MALID != 56877 {
		t.Fatalf("unexpected mal id %d", results[0].MALID)
	}
}

func TestClientGetAnimeFull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anime/56877/full" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"mal_id":56877,"title":"Ao no Orchestra Season 2","status":"Finished Airing","studios":[{"name":"Nippon Animation"}],"genres":[{"name":"Drama"}],"themes":[{"name":"Music"}],"trailer":{"embed_url":"https://www.youtube-nocookie.com/embed/demo"},"synopsis":"Following the regular summer concert..."}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	full, err := client.GetAnimeFull(context.Background(), 56877)
	if err != nil {
		t.Fatalf("GetAnimeFull returned error: %v", err)
	}
	if full.MALID != 56877 {
		t.Fatalf("unexpected mal id %d", full.MALID)
	}
	if full.Status != "Finished Airing" {
		t.Fatalf("unexpected status %q", full.Status)
	}
}

func TestClientGetAnimeCharacters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anime/56877/characters" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"role":"Main","character":{"mal_id":1,"name":"Hajime Aono","images":{"webp":{"image_url":"https://cdn.myanimelist.net/images/characters/1.webp"}}},"voice_actors":[{"person":{"mal_id":2,"name":"Actor Name"},"language":"Japanese"}]}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	characters, err := client.GetAnimeCharacters(context.Background(), 56877)
	if err != nil {
		t.Fatalf("GetAnimeCharacters returned error: %v", err)
	}
	if len(characters) != 1 {
		t.Fatalf("expected 1 character, got %d", len(characters))
	}
	if characters[0].Character.Name != "Hajime Aono" {
		t.Fatalf("unexpected character name %q", characters[0].Character.Name)
	}
}

func TestPickBestMatch(t *testing.T) {
	results := []AnimeSearchHit{
		{MALID: 51614, Title: "Ao no Orchestra"},
		{MALID: 56877, Title: "Ao no Orchestra Season 2", TitleSynonyms: []string{"Blue Orchestra Season 2"}},
	}

	match, ok := PickBestMatch("Ao no Orchestra Season 2", results)
	if !ok {
		t.Fatal("expected match")
	}
	if match.MALID != 56877 {
		t.Fatalf("unexpected mal id %d", match.MALID)
	}
}
