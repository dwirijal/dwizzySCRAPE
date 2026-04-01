package store

import (
	"context"
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/tmdb"
)

type tmdbEnrichmentClientStub struct {
	searchMoviesFn func(ctx context.Context, query string, year, limit int) ([]tmdb.SearchHit, error)
	getMovieFn     func(ctx context.Context, movieID int) (tmdb.MovieDetail, error)
	searchTVFn     func(ctx context.Context, query string, year, limit int) ([]tmdb.SeriesSearchHit, error)
	getTVFn        func(ctx context.Context, tvID int) (tmdb.SeriesDetail, error)
}

func (c tmdbEnrichmentClientStub) SearchMovies(ctx context.Context, query string, year, limit int) ([]tmdb.SearchHit, error) {
	return c.searchMoviesFn(ctx, query, year, limit)
}

func (c tmdbEnrichmentClientStub) GetMovieDetail(ctx context.Context, movieID int) (tmdb.MovieDetail, error) {
	return c.getMovieFn(ctx, movieID)
}

func (c tmdbEnrichmentClientStub) SearchTV(ctx context.Context, query string, year, limit int) ([]tmdb.SeriesSearchHit, error) {
	return c.searchTVFn(ctx, query, year, limit)
}

func (c tmdbEnrichmentClientStub) GetTVDetail(ctx context.Context, tvID int) (tmdb.SeriesDetail, error) {
	return c.getTVFn(ctx, tvID)
}

type tmdbCandidateReaderStub struct {
	listFn func(ctx context.Context, offset, limit int, options TMDBEnrichmentCandidateOptions) ([]TMDBEnrichmentCandidate, error)
}

func (r tmdbCandidateReaderStub) ListTMDBEnrichmentCandidates(
	ctx context.Context,
	offset, limit int,
	options TMDBEnrichmentCandidateOptions,
) ([]TMDBEnrichmentCandidate, error) {
	return r.listFn(ctx, offset, limit, options)
}

type tmdbEnrichmentWriterStub struct {
	records []MediaItemEnrichmentRecord
}

func (w *tmdbEnrichmentWriterStub) UpsertItemEnrichment(_ context.Context, record MediaItemEnrichmentRecord) error {
	w.records = append(w.records, record)
	return nil
}

func TestBuildTMDBSearchQueriesPrefersEnglishTitleAndCleansVariants(t *testing.T) {
	candidate := TMDBEnrichmentCandidate{
		Source:      "samehadaku",
		SurfaceType: "movie",
		Title:       "Tenki no Ko",
		Detail: map[string]any{
			"source_title": "Tenki no Ko",
			"search_hit": map[string]any{
				"title_english": "Weathering with You",
				"title":         "Tenki no Ko",
				"title_synonyms": []any{
					"Weather Child",
					"Weathering With You",
				},
			},
		},
	}

	queries := buildTMDBSearchQueries(candidate)
	if len(queries) < 2 {
		t.Fatalf("expected multiple queries, got %v", queries)
	}
	if queries[0] != "Weathering with You" {
		t.Fatalf("expected english title first, got %q", queries[0])
	}
}

func TestBuildTMDBSearchQueriesStripsDramaPrefixAndYear(t *testing.T) {
	candidate := TMDBEnrichmentCandidate{
		Source:      "drakorid",
		SurfaceType: "series",
		Title:       "Drama Korea Honour (2026)",
		Detail: map[string]any{
			"source_title": "Drama Korea Honour (2026)",
			"country":      "South Korea",
		},
	}

	queries := buildTMDBSearchQueries(candidate)
	if len(queries) == 0 {
		t.Fatal("expected at least one search query")
	}
	if queries[0] != "Honour" {
		t.Fatalf("expected cleaned title, got %q", queries[0])
	}
}

func TestBuildTMDBSearchQueriesUsesNestedJikanEnglishTitle(t *testing.T) {
	candidate := TMDBEnrichmentCandidate{
		Source:      "samehadaku",
		SurfaceType: "movie",
		Title:       "Boku no Hero Academia the Movie 1: Futari no Hero",
		Detail: map[string]any{
			"jikan_meta_json": map[string]any{
				"search_hit": map[string]any{
					"title_english": "My Hero Academia: Two Heroes",
					"title":         "Boku no Hero Academia the Movie 1: Futari no Hero",
					"title_synonyms": []any{
						"Futari no Hero",
					},
				},
			},
		},
	}

	queries := buildTMDBSearchQueries(candidate)
	if len(queries) < 2 {
		t.Fatalf("expected multiple queries, got %v", queries)
	}
	if queries[0] != "My Hero Academia: Two Heroes" {
		t.Fatalf("expected nested jikan english title first, got %q", queries[0])
	}
}

func TestTMDBEnrichmentBackfillServiceBackfillMovieCandidateStoresCompactPayload(t *testing.T) {
	readerCalls := 0
	reader := tmdbCandidateReaderStub{
		listFn: func(_ context.Context, offset, limit int, options TMDBEnrichmentCandidateOptions) ([]TMDBEnrichmentCandidate, error) {
			readerCalls++
			if readerCalls > 1 {
				return nil, nil
			}
			if options.Scope != TMDBEnrichmentScopeMovie {
				t.Fatalf("unexpected scope %q", options.Scope)
			}
			return []TMDBEnrichmentCandidate{
				{
					ItemKey:     "samehadaku:movie:tenki-no-ko",
					Source:      "samehadaku",
					MediaType:   "movie",
					SurfaceType: "movie",
					Slug:        "tenki-no-ko",
					Title:       "Tenki no Ko",
					ReleaseYear: 2019,
					Detail: map[string]any{
						"source_title": "Tenki no Ko",
						"search_hit": map[string]any{
							"title_english": "Weathering with You",
						},
					},
				},
			}, nil
		},
	}
	writer := &tmdbEnrichmentWriterStub{}
	var seenQueries []string
	client := tmdbEnrichmentClientStub{
		searchMoviesFn: func(_ context.Context, query string, year, limit int) ([]tmdb.SearchHit, error) {
			seenQueries = append(seenQueries, query)
			if query != "Weathering with You" {
				return nil, nil
			}
			return []tmdb.SearchHit{{
				ID:            568160,
				Title:         "Weathering with You",
				OriginalTitle: "Tenki no Ko",
				ReleaseDate:   "2019-07-19",
				PosterPath:    "/k4.jpg",
			}}, nil
		},
		getMovieFn: func(_ context.Context, movieID int) (tmdb.MovieDetail, error) {
			if movieID != 568160 {
				t.Fatalf("unexpected movie id %d", movieID)
			}
			return tmdb.MovieDetail{
				ID:            568160,
				Title:         "Weathering with You",
				OriginalTitle: "Tenki no Ko",
				Overview:      "A runaway teen and a weather maiden meet in Tokyo.",
				PosterPath:    "/poster.jpg",
				BackdropPath:  "/backdrop.jpg",
				ReleaseDate:   "2019-07-19",
				Runtime:       112,
				VoteAverage:   8.0,
				Tagline:       "A story about the secret of the world.",
				Status:        "Released",
				Genres: []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				}{
					{Name: "Animation"},
					{Name: "Romance"},
				},
				ProductionCountries: []struct {
					Name string `json:"name"`
				}{
					{Name: "Japan"},
				},
				Videos: struct {
					Results []struct {
						Name     string `json:"name"`
						Key      string `json:"key"`
						Site     string `json:"site"`
						Type     string `json:"type"`
						Official bool   `json:"official"`
					} `json:"results"`
				}{
					Results: []struct {
						Name     string `json:"name"`
						Key      string `json:"key"`
						Site     string `json:"site"`
						Type     string `json:"type"`
						Official bool   `json:"official"`
					}{
						{Name: "Trailer", Key: "abc123", Site: "YouTube", Type: "Trailer", Official: true},
					},
				},
				Credits: struct {
					Cast []struct {
						Name      string `json:"name"`
						Character string `json:"character"`
					} `json:"cast"`
					Crew []struct {
						Name string `json:"name"`
						Job  string `json:"job"`
					} `json:"crew"`
				}{
					Cast: []struct {
						Name      string `json:"name"`
						Character string `json:"character"`
					}{
						{Name: "Kotaro Daigo", Character: "Hodaka"},
					},
					Crew: []struct {
						Name string `json:"name"`
						Job  string `json:"job"`
					}{
						{Name: "Makoto Shinkai", Job: "Director"},
					},
				},
			}, nil
		},
		searchTVFn: func(context.Context, string, int, int) ([]tmdb.SeriesSearchHit, error) {
			t.Fatal("unexpected tv search")
			return nil, nil
		},
		getTVFn: func(context.Context, int) (tmdb.SeriesDetail, error) {
			t.Fatal("unexpected tv detail fetch")
			return tmdb.SeriesDetail{}, nil
		},
	}

	service := NewTMDBEnrichmentBackfillService(reader, writer, client)
	report, err := service.Backfill(context.Background(), TMDBEnrichmentBackfillOptions{
		Scope:     TMDBEnrichmentScopeMovie,
		BatchSize: 1,
	})
	if err != nil {
		t.Fatalf("Backfill returned error: %v", err)
	}

	if report.Succeeded != 1 || report.Attempted != 1 || report.Failed != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(seenQueries) == 0 || seenQueries[0] != "Weathering with You" {
		t.Fatalf("expected english title search, got %v", seenQueries)
	}
	if len(writer.records) != 1 {
		t.Fatalf("expected 1 enrichment record, got %d", len(writer.records))
	}

	record := writer.records[0]
	if record.Provider != "tmdb" {
		t.Fatalf("unexpected provider %q", record.Provider)
	}
	if record.ExternalID != "568160" {
		t.Fatalf("unexpected external id %q", record.ExternalID)
	}
	if record.MatchStatus != "matched" {
		t.Fatalf("unexpected match status %q", record.MatchStatus)
	}
	if record.MatchScore != 100 {
		t.Fatalf("expected clamped match score 100, got %d", record.MatchScore)
	}
	if kind := record.Payload["kind"]; kind != "movie" {
		t.Fatalf("unexpected payload kind %#v", kind)
	}
	if releaseYear := record.Payload["release_year"]; releaseYear != 2019 {
		t.Fatalf("unexpected payload release year %#v", releaseYear)
	}
	if trailerURL := record.Payload["trailer_url"]; trailerURL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("unexpected trailer url %#v", trailerURL)
	}
	cast, ok := record.Payload["cast"].([]map[string]string)
	if !ok || len(cast) != 1 {
		t.Fatalf("unexpected cast payload %#v", record.Payload["cast"])
	}
	directors, ok := record.Payload["directors"].([]string)
	if !ok || len(directors) != 1 || directors[0] != "Makoto Shinkai" {
		t.Fatalf("unexpected directors payload %#v", record.Payload["directors"])
	}
}

func TestTMDBEnrichmentBackfillServiceUsesNestedJikanAliasForMovieSearch(t *testing.T) {
	reader := tmdbCandidateReaderStub{
		listFn: func(_ context.Context, offset, limit int, options TMDBEnrichmentCandidateOptions) ([]TMDBEnrichmentCandidate, error) {
			if offset > 0 {
				return nil, nil
			}
			return []TMDBEnrichmentCandidate{
				{
					ItemKey:     "samehadaku:movie:boku-no-hero",
					Source:      "samehadaku",
					MediaType:   "movie",
					SurfaceType: "movie",
					Slug:        "movie-boku-no-hero-academia-the-movie-1-futari-no-hero",
					Title:       "Boku no Hero Academia the Movie 1: Futari no Hero",
					ReleaseYear: 2018,
					Detail: map[string]any{
						"jikan_meta_json": map[string]any{
							"search_hit": map[string]any{
								"title_english": "My Hero Academia: Two Heroes",
								"title":         "Boku no Hero Academia the Movie 1: Futari no Hero",
							},
						},
					},
				},
			}, nil
		},
	}
	writer := &tmdbEnrichmentWriterStub{}
	var seenQueries []string
	client := tmdbEnrichmentClientStub{
		searchMoviesFn: func(_ context.Context, query string, year, limit int) ([]tmdb.SearchHit, error) {
			seenQueries = append(seenQueries, query)
			if query != "My Hero Academia: Two Heroes" {
				return nil, nil
			}
			return []tmdb.SearchHit{{
				ID:            505262,
				Title:         "My Hero Academia: Two Heroes",
				OriginalTitle: "僕のヒーローアカデミア THE MOVIE ～2人の英雄～",
				ReleaseDate:   "2018-08-03",
			}}, nil
		},
		getMovieFn: func(_ context.Context, movieID int) (tmdb.MovieDetail, error) {
			if movieID != 505262 {
				t.Fatalf("unexpected movie id %d", movieID)
			}
			return tmdb.MovieDetail{
				ID:            505262,
				Title:         "My Hero Academia: Two Heroes",
				OriginalTitle: "僕のヒーローアカデミア THE MOVIE ～2人の英雄～",
				Overview:      "Test overview.",
				ReleaseDate:   "2018-08-03",
				Runtime:       96,
				VoteAverage:   7.9,
			}, nil
		},
		searchTVFn: func(context.Context, string, int, int) ([]tmdb.SeriesSearchHit, error) {
			t.Fatal("unexpected tv search")
			return nil, nil
		},
		getTVFn: func(context.Context, int) (tmdb.SeriesDetail, error) {
			t.Fatal("unexpected tv detail fetch")
			return tmdb.SeriesDetail{}, nil
		},
	}

	service := NewTMDBEnrichmentBackfillService(reader, writer, client)
	report, err := service.Backfill(context.Background(), TMDBEnrichmentBackfillOptions{
		Scope:     TMDBEnrichmentScopeMovie,
		BatchSize: 1,
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("Backfill returned error: %v", err)
	}

	if report.Succeeded != 1 || report.Failed != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(seenQueries) == 0 || seenQueries[0] != "My Hero Academia: Two Heroes" {
		t.Fatalf("expected english alias query first, got %v", seenQueries)
	}
	if len(writer.records) != 1 || writer.records[0].MatchStatus != "matched" {
		t.Fatalf("expected matched enrichment record, got %+v", writer.records)
	}
}

func TestTMDBEnrichmentBackfillServiceDoesNotRetrySameFailedCandidateInSingleRun(t *testing.T) {
	readerCalls := 0
	reader := tmdbCandidateReaderStub{
		listFn: func(_ context.Context, offset, limit int, options TMDBEnrichmentCandidateOptions) ([]TMDBEnrichmentCandidate, error) {
			readerCalls++
			switch readerCalls {
			case 1, 2, 3:
				return []TMDBEnrichmentCandidate{{
					ItemKey:     "drakorid:drama:singles-inferno-s5-2026",
					Source:      "drakorid",
					MediaType:   "drama",
					SurfaceType: "series",
					Slug:        "singles-inferno-s5-2026",
					Title:       "Singles Inferno S5 (2026)",
					ReleaseYear: 2026,
				}}, nil
			default:
				t.Fatalf("reader should not be called more than 3 times, got %d", readerCalls)
				return nil, nil
			}
		},
	}
	writer := &tmdbEnrichmentWriterStub{}
	seriesSearchCalls := 0
	client := tmdbEnrichmentClientStub{
		searchMoviesFn: func(context.Context, string, int, int) ([]tmdb.SearchHit, error) {
			t.Fatal("unexpected movie search")
			return nil, nil
		},
		getMovieFn: func(context.Context, int) (tmdb.MovieDetail, error) {
			t.Fatal("unexpected movie detail")
			return tmdb.MovieDetail{}, nil
		},
		searchTVFn: func(_ context.Context, query string, year, limit int) ([]tmdb.SeriesSearchHit, error) {
			seriesSearchCalls++
			return nil, nil
		},
		getTVFn: func(context.Context, int) (tmdb.SeriesDetail, error) {
			t.Fatal("unexpected tv detail")
			return tmdb.SeriesDetail{}, nil
		},
	}

	service := NewTMDBEnrichmentBackfillService(reader, writer, client)
	report, err := service.Backfill(context.Background(), TMDBEnrichmentBackfillOptions{
		Scope:        TMDBEnrichmentScopeSeries,
		BatchSize:    1,
		SkipExisting: true,
	})
	if err != nil {
		t.Fatalf("Backfill returned error: %v", err)
	}

	if report.Attempted != 1 {
		t.Fatalf("expected one attempted candidate, got %+v", report)
	}
	if report.Failed != 1 {
		t.Fatalf("expected one failed candidate, got %+v", report)
	}
	if report.Skipped == 0 {
		t.Fatalf("expected duplicate candidates to be skipped, got %+v", report)
	}
	if seriesSearchCalls != 1 {
		t.Fatalf("expected one tv search call, got %d", seriesSearchCalls)
	}
}

func TestTMDBEnrichmentBackfillServiceRecordsFailureWithFallbackMatchedTitle(t *testing.T) {
	readerCalls := 0
	reader := tmdbCandidateReaderStub{
		listFn: func(_ context.Context, offset, limit int, options TMDBEnrichmentCandidateOptions) ([]TMDBEnrichmentCandidate, error) {
			readerCalls++
			if readerCalls > 1 {
				return nil, nil
			}
			return []TMDBEnrichmentCandidate{
				{
					ItemKey:     "drakorid:drama:running-man-variety-show-2026",
					Source:      "drakorid",
					MediaType:   "drama",
					SurfaceType: "series",
					Slug:        "running-man-variety-show-2026",
					Title:       "Running Man Variety Show (2026)",
					ReleaseYear: 2026,
					Detail: map[string]any{
						"source_title": "Running Man Variety Show (2026)",
					},
				},
			}, nil
		},
	}
	writer := &tmdbEnrichmentWriterStub{}
	client := tmdbEnrichmentClientStub{
		searchMoviesFn: func(context.Context, string, int, int) ([]tmdb.SearchHit, error) {
			t.Fatal("unexpected movie search")
			return nil, nil
		},
		getMovieFn: func(context.Context, int) (tmdb.MovieDetail, error) {
			t.Fatal("unexpected movie detail")
			return tmdb.MovieDetail{}, nil
		},
		searchTVFn: func(_ context.Context, query string, year, limit int) ([]tmdb.SeriesSearchHit, error) {
			return nil, nil
		},
		getTVFn: func(context.Context, int) (tmdb.SeriesDetail, error) {
			t.Fatal("unexpected series detail")
			return tmdb.SeriesDetail{}, nil
		},
	}

	service := NewTMDBEnrichmentBackfillService(reader, writer, client)
	report, err := service.Backfill(context.Background(), TMDBEnrichmentBackfillOptions{
		Scope:        TMDBEnrichmentScopeSeries,
		BatchSize:    1,
		SkipExisting: true,
	})
	if err != nil {
		t.Fatalf("Backfill returned error: %v", err)
	}
	if report.Failed != 1 || report.Succeeded != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(writer.records) != 1 {
		t.Fatalf("expected 1 recorded failure, got %d", len(writer.records))
	}
	if writer.records[0].MatchedTitle != "Running Man Variety Show (2026)" {
		t.Fatalf("unexpected fallback matched title %q", writer.records[0].MatchedTitle)
	}
	if writer.records[0].MatchStatus != string(tmdb.MatchReasonSearchEmpty) {
		t.Fatalf("unexpected match status %q", writer.records[0].MatchStatus)
	}
}
