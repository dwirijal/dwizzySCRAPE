package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMovieV3StoreUpsertMoviesWithDB(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	store := NewMovieV3StoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "INSERT INTO public.movies") {
				t.Fatalf("unexpected query %q", query)
			}
			if !strings.Contains(query, "NULLIF(BTRIM(slug), '')") {
				t.Fatalf("expected slug backfill guard in query, got %q", query)
			}
			if len(args) != 1 {
				t.Fatalf("expected 1 query arg, got %d", len(args))
			}
			body, ok := args[0].([]byte)
			if !ok {
				t.Fatalf("expected JSON payload bytes, got %T", args[0])
			}
			var payload []map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			if len(payload) != 1 {
				t.Fatalf("expected 1 payload row, got %d", len(payload))
			}
			if got := payload[0]["slug"]; got != "movie-123" {
				t.Fatalf("expected fallback slug movie-123, got %#v", got)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int)) = 1
				return nil
			}}
		},
	})

	upserted, err := store.UpsertMovies(context.Background(), []MovieCoreRow{{
		TMDBID:         123,
		Title:          "Sample Movie",
		OriginalTitle:  "Sample Movie",
		PosterPath:     "/poster.jpg",
		BackdropPath:   "/backdrop.jpg",
		MetaSourceCode: "t",
		UpdatedAt:      now,
	}})
	if err != nil {
		t.Fatalf("UpsertMovies returned error: %v", err)
	}
	if upserted != 1 {
		t.Fatalf("expected 1 upserted row, got %d", upserted)
	}
}

func TestMovieV3StoreUpsertMoviesWithDBPreservesExplicitSlug(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	store := NewMovieV3StoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			body, ok := args[0].([]byte)
			if !ok {
				t.Fatalf("expected JSON payload bytes, got %T", args[0])
			}
			var payload []map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			if got := payload[0]["slug"]; got != "war-machine-2026-1265609" {
				t.Fatalf("unexpected canonical slug %#v", got)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int)) = 1
				return nil
			}}
		},
	})

	upserted, err := store.UpsertMovies(context.Background(), []MovieCoreRow{{
		TMDBID:         1265609,
		Slug:           "war-machine-2026-1265609",
		Title:          "War Machine",
		OriginalTitle:  "War Machine",
		PosterPath:     "/poster.jpg",
		BackdropPath:   "/backdrop.jpg",
		MetaSourceCode: "t",
		UpdatedAt:      now,
	}})
	if err != nil {
		t.Fatalf("UpsertMovies returned error: %v", err)
	}
	if upserted != 1 {
		t.Fatalf("expected 1 upserted row, got %d", upserted)
	}
}

func TestMovieV3StoreUpsertProviderRecordsWithDB(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	store := NewMovieV3StoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "INSERT INTO public.movie_provider_records") {
				t.Fatalf("unexpected query %q", query)
			}
			return stubRow{scanFn: func(dest ...any) error {
				payload, err := json.Marshal([]MovieProviderRecordRow{{
					ID:                77,
					TMDBID:            123,
					ProviderCode:      "k",
					ProviderMovieSlug: "sample-movie",
				}})
				if err != nil {
					return err
				}
				*(dest[0].(*[]byte)) = payload
				return nil
			}}
		},
	})

	rows, err := store.UpsertProviderRecords(context.Background(), []MovieProviderRecordRow{{
		TMDBID:            123,
		ProviderCode:      "k",
		ProviderMovieSlug: "sample-movie",
		UpdatedAt:         &now,
		LastSeenAt:        &now,
	}})
	if err != nil {
		t.Fatalf("UpsertProviderRecords returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != 77 {
		t.Fatalf("unexpected provider rows %#v", rows)
	}
}

func TestMovieV3StoreUpsertWatchOptionsWithDB(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	store := NewMovieV3StoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "INSERT INTO public.movie_watch_options") {
				t.Fatalf("unexpected query %q", query)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int)) = 1
				return nil
			}}
		},
	})

	upserted, err := store.UpsertWatchOptions(context.Background(), []MovieWatchOptionRow{{
		TMDBID:         123,
		ProviderRecord: 77,
		ProviderCode:   "k",
		HostCode:       "u",
		Label:          "Kanata Source",
		EmbedURL:       "https://example.com/embed/123",
		StatusCode:     "a",
		UpdatedAt:      now,
	}})
	if err != nil {
		t.Fatalf("UpsertWatchOptions returned error: %v", err)
	}
	if upserted != 1 {
		t.Fatalf("expected 1 upserted row, got %d", upserted)
	}
}

func TestMovieV3StoreUpsertDownloadOptionsWithDB(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	store := NewMovieV3StoreWithDB(&stubContentDB{
		queryRowFn: func(ctx context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "INSERT INTO public.movie_download_options") {
				t.Fatalf("unexpected query %q", query)
			}
			return stubRow{scanFn: func(dest ...any) error {
				*(dest[0].(*int)) = 1
				return nil
			}}
		},
	})

	upserted, err := store.UpsertDownloadOptions(context.Background(), []MovieDownloadOptionRow{{
		TMDBID:         123,
		ProviderRecord: 77,
		ProviderCode:   "k",
		HostCode:       "u",
		Label:          "Download 720p",
		DownloadURL:    "https://example.com/download/123",
		StatusCode:     "a",
		UpdatedAt:      now,
	}})
	if err != nil {
		t.Fatalf("UpsertDownloadOptions returned error: %v", err)
	}
	if upserted != 1 {
		t.Fatalf("expected 1 upserted row, got %d", upserted)
	}
}
