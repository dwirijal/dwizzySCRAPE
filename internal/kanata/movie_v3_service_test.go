package kanata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/store"
	"github.com/dwirijal/dwizzySCRAPE/internal/tmdb"
)

func TestBuildCanonicalMovieSlugIncludesTMDBID(t *testing.T) {
	got := buildCanonicalMovieSlug("War Machine", 2026, 1265609)
	if got != "war-machine-2026-1265609" {
		t.Fatalf("unexpected slug %q", got)
	}
}

func TestBuildCanonicalMovieSlugFallsBackToMovieID(t *testing.T) {
	got := buildCanonicalMovieSlug("", 0, 1265609)
	if got != "movie-1265609" {
		t.Fatalf("unexpected fallback slug %q", got)
	}
}

func TestMovieSearchQueriesAddsAliasFreeFallback(t *testing.T) {
	got := movieSearchQueries("The Shadow&#039;s Edge (Bu feng zhui ying aka)")
	want := []string{
		"The Shadow's Edge (Bu feng zhui ying aka)",
		"The Shadow's Edge",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("movieSearchQueries() = %#v, want %#v", got, want)
	}
}

func TestMovieSearchQueriesKeepsSimpleTitle(t *testing.T) {
	got := movieSearchQueries("War Machine (2026)")
	want := []string{"War Machine"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("movieSearchQueries() = %#v, want %#v", got, want)
	}
}

func TestSyncHomeItemRetriesTMDBSearchWithoutAlias(t *testing.T) {
	tmdbQueries := make([]string, 0, 2)
	tmdbServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/search/movie":
			tmdbQueries = append(tmdbQueries, r.URL.Query().Get("query"))
			if len(tmdbQueries) == 1 {
				_, _ = fmt.Fprint(w, `{"results":[]}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"results":[{"id":1419406,"title":"The Shadow's Edge","original_title":"The Shadow's Edge","release_date":"2025-08-16","poster_path":"/poster.jpg","vote_average":7.2}]}`)
		case r.URL.Path == "/movie/1419406":
			_, _ = fmt.Fprint(w, `{"id":1419406,"title":"The Shadow's Edge","original_title":"The Shadow's Edge","overview":"Macau Police brings the tracking expert police officer out of retirement.","poster_path":"/poster.jpg","backdrop_path":"/backdrop.jpg","release_date":"2025-08-16","runtime":142,"vote_average":7.2,"tagline":"","status":"Released","original_language":"zh","genres":[{"id":28,"name":"Action"}],"videos":{"results":[]},"credits":{"cast":[],"crew":[]}}`)
		default:
			t.Fatalf("unexpected tmdb path %s", r.URL.Path)
		}
	}))
	defer tmdbServer.Close()

	kanataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/detail/bu-feng-zhui-ying-2025":
			_, _ = fmt.Fprint(w, `{"data":{"poster":"/poster.jpg","related":[],"synopsis":"Provider synopsis","tags":["action"],"title":"The Shadow's Edge","url":"bu-feng-zhui-ying-2025"}}`)
		case r.URL.Path == "/stream":
			_, _ = fmt.Fprint(w, `{"stream_url":"https://ngopi.web.id/dl.php?url=bu-feng-zhui-ying-2025&type=movie","token":""}`)
		default:
			t.Fatalf("unexpected kanata path %s", r.URL.Path)
		}
	}))
	defer kanataServer.Close()

	storeSpy := &recordingMovieStore{}
	service := NewMovieV3Service(
		NewClient(kanataServer.URL, kanataServer.Client()),
		tmdb.NewClient(tmdbServer.URL, "", "dummy", tmdbServer.Client()),
		storeSpy,
		time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC),
	)

	err := service.syncHomeItem(context.Background(), HomeMovie{
		Slug:     "bu-feng-zhui-ying-2025",
		Title:    "The Shadow&#039;s Edge (Bu feng zhui ying aka)",
		Poster:   "/provider-poster.jpg",
		Quality:  "HD",
		Duration: "02:22",
		Year:     "2025",
	})
	if err != nil {
		t.Fatalf("syncHomeItem returned error: %v", err)
	}

	wantQueries := []string{
		"The Shadow's Edge (Bu feng zhui ying aka)",
		"The Shadow's Edge",
	}
	if !reflect.DeepEqual(tmdbQueries, wantQueries) {
		t.Fatalf("tmdb queries = %#v, want %#v", tmdbQueries, wantQueries)
	}
	if len(storeSpy.movies) != 1 || storeSpy.movies[0].TMDBID != 1419406 {
		t.Fatalf("unexpected movie rows %#v", storeSpy.movies)
	}
}

func TestMovieSyncFailureFromErrUsesCode(t *testing.T) {
	err := withMovieSyncCode("tmdb_search_error", fmt.Errorf("tmdb search: boom"))
	failure := movieSyncFailureFromErr(err)
	if failure.Code != "tmdb_search_error" {
		t.Fatalf("unexpected code %q", failure.Code)
	}
	if failure.Message == "" {
		t.Fatalf("expected non-empty message")
	}
}

func TestMovieSyncFailureFromErrUnknown(t *testing.T) {
	failure := movieSyncFailureFromErr(errors.New("boom"))
	if failure.Code != "unknown_error" {
		t.Fatalf("unexpected code %q", failure.Code)
	}
	if failure.Message != "boom" {
		t.Fatalf("unexpected message %q", failure.Message)
	}
}

func TestSyncItemsCapturesFailureCodeForEmptyTitle(t *testing.T) {
	service := NewMovieV3Service(
		&Client{},
		tmdb.NewClient("https://example.test", "dummy", "", http.DefaultClient),
		&recordingMovieStore{},
		time.Time{},
	)

	report, err := service.syncItems(context.Background(), []HomeMovie{
		{Slug: "broken-item", Title: ""},
	}, 0)
	if err != nil {
		t.Fatalf("syncItems returned error: %v", err)
	}
	if report.Failed != 1 {
		t.Fatalf("expected 1 failed item, got %d", report.Failed)
	}
	if got := report.FailureCodes["broken-item"]; got != "empty_title" {
		t.Fatalf("unexpected failure code %q", got)
	}
	if got := report.Failures["broken-item"]; got == "" {
		t.Fatalf("expected failure message")
	}
}

type recordingMovieStore struct {
	movies []store.MovieCoreRow
}

func (s *recordingMovieStore) UpsertMovies(_ context.Context, rows []store.MovieCoreRow) (int, error) {
	s.movies = append(s.movies, rows...)
	return len(rows), nil
}

func (s *recordingMovieStore) UpsertMovieMeta(_ context.Context, rows []store.MovieMetaRow) (int, error) {
	return len(rows), nil
}

func (s *recordingMovieStore) UpsertProviderRecords(_ context.Context, rows []store.MovieProviderRecordRow) ([]store.MovieProviderRecordRow, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	clone := rows[0]
	clone.ID = 77
	return []store.MovieProviderRecordRow{clone}, nil
}

func (s *recordingMovieStore) UpsertWatchOptions(_ context.Context, rows []store.MovieWatchOptionRow) (int, error) {
	return len(rows), nil
}

func (s *recordingMovieStore) UpsertDownloadOptions(_ context.Context, rows []store.MovieDownloadOptionRow) (int, error) {
	return len(rows), nil
}
