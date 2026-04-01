package anilist

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPickBestMediaMatchResultSearchEmpty(t *testing.T) {
	result := PickBestMediaMatchResult("The Shadow's Edge", 2025, nil)
	if result.Matched {
		t.Fatalf("expected no match")
	}
	if result.Reason != MatchReasonSearchEmpty {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
}

func TestPickBestMediaMatchResultMatchedByEnglishTitleAndYear(t *testing.T) {
	result := PickBestMediaMatchResult("Weathering with You", 2019, []SearchHit{
		{
			ID:         106286,
			IDMal:      38826,
			SeasonYear: 2019,
			Title: MediaTitle{
				Romaji:  "Tenki no Ko",
				English: "Weathering with You",
				Native:  "天気の子",
			},
			Format:          "MOVIE",
			CountryOfOrigin: "JP",
		},
	})
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if result.Reason != MatchReasonMatched {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
	if result.Hit.ID != 106286 {
		t.Fatalf("unexpected hit id %d", result.Hit.ID)
	}
}

func TestPickBestMediaMatchResultRejectsWrongComic(t *testing.T) {
	result := PickBestMediaMatchResult("Blue Box", 0, []SearchHit{
		{
			ID:     1,
			Title:  MediaTitle{Romaji: "Completely Different"},
			Format: "MANGA",
		},
	})
	if result.Matched {
		t.Fatalf("expected no match")
	}
	if result.Reason != MatchReasonScoreRejected {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
}

func TestPickBestMediaMatchResultRejectsLowConfidencePartialMatch(t *testing.T) {
	t.Parallel()

	result := PickBestMediaMatchResult("Study Group", 0, []SearchHit{
		{
			ID:    1,
			Title: MediaTitle{English: "Group"},
		},
	})
	if result.Matched {
		t.Fatalf("expected no match, got %+v", result)
	}
	if result.Reason != MatchReasonScoreRejected {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
}

func TestSearchMediaMarksServiceUnavailableOn403GraphQLError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"message":"The AniList API has been temporarily disabled due to severe stability issues.","status":403}],"data":null}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, err := client.SearchMedia(context.Background(), "Study Group", MediaTypeManga, 3)
	if err == nil {
		t.Fatal("SearchMedia returned nil error")
	}
	if !IsServiceUnavailable(err) {
		t.Fatalf("expected service unavailable error, got %v", err)
	}
}

func TestSearchMediaReturnsErrorOnGraphQLErrorPayloadWith200(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"rate limit hit"}],"data":{"Page":{"media":[]}}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	_, err := client.SearchMedia(context.Background(), "Study Group", MediaTypeManga, 3)
	if err == nil {
		t.Fatal("SearchMedia returned nil error")
	}
}
