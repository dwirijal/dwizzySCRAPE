package tmdb

import "testing"

func TestPickBestMovieMatchResultSearchEmpty(t *testing.T) {
	result := PickBestMovieMatchResult("The Shadow's Edge", 2025, nil)
	if result.Matched {
		t.Fatalf("expected no match")
	}
	if result.Reason != MatchReasonSearchEmpty {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
	if result.CandidateCount != 0 {
		t.Fatalf("unexpected candidate count %d", result.CandidateCount)
	}
}

func TestPickBestMovieMatchResultScoreRejected(t *testing.T) {
	result := PickBestMovieMatchResult("The Shadow's Edge", 0, []SearchHit{
		{
			ID:            1,
			Title:         "Completely Different",
			OriginalTitle: "Nothing Similar",
			ReleaseDate:   "2025-01-01",
			PosterPath:    "/foo.jpg",
		},
	})
	if result.Matched {
		t.Fatalf("expected no match")
	}
	if result.Reason != MatchReasonScoreRejected {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
	if result.CandidateCount != 1 {
		t.Fatalf("unexpected candidate count %d", result.CandidateCount)
	}
	if result.BestScore != 0 {
		t.Fatalf("expected best score 0, got %d", result.BestScore)
	}
}

func TestPickBestMovieMatchResultMatched(t *testing.T) {
	hit := SearchHit{
		ID:            1419406,
		Title:         "The Shadow's Edge",
		OriginalTitle: "The Shadow's Edge",
		ReleaseDate:   "2025-08-16",
		PosterPath:    "/poster.jpg",
	}
	result := PickBestMovieMatchResult("The Shadow's Edge", 2025, []SearchHit{hit})
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if result.Reason != MatchReasonMatched {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
	if result.Hit.ID != hit.ID {
		t.Fatalf("unexpected hit id %d", result.Hit.ID)
	}
}

func TestPickBestSeriesMatchResultMatched(t *testing.T) {
	hit := SeriesSearchHit{
		ID:               246,
		Name:             "Bleach",
		OriginalName:     "BLEACH",
		FirstAirDate:     "2004-10-05",
		OriginalLanguage: "ja",
		OriginCountry:    []string{"JP"},
	}

	result := PickBestSeriesMatchResult("Bleach", 2004, []SeriesSearchHit{hit})
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if result.Reason != MatchReasonMatched {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
	if result.Hit.ID != hit.ID {
		t.Fatalf("unexpected hit id %d", result.Hit.ID)
	}
}

func TestPickBestSeriesMatchResultRejectsWrongShow(t *testing.T) {
	result := PickBestSeriesMatchResult("Bleach", 2004, []SeriesSearchHit{
		{
			ID:            1,
			Name:          "Breaking Bad",
			OriginalName:  "Breaking Bad",
			FirstAirDate:  "2008-01-20",
			OriginCountry: []string{"US"},
		},
	})

	if result.Matched {
		t.Fatalf("expected no match")
	}
	if result.Reason != MatchReasonScoreRejected {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
}
