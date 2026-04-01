package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/nekopoi"
)

func TestNekopoiStoreUpsertFeedItems(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
	store := NewNekopoiStoreWithDB(db)

	_, err := store.UpsertFeedItems(context.Background(), []nekopoi.FeedItem{
		{
			SourceDomain:    "nekopoi.care",
			Title:           "Naruto Episode 1",
			NormalizedTitle: "Naruto Episode 1",
			EntryKind:       "episode",
			EpisodeNumber:   1,
			SeriesCandidate: true,
			CanonicalURL:    "https://nekopoi.care/naruto-episode-1/",
			Slug:            "naruto-episode-1",
			CoverURL:        "https://cdn.example/naruto.jpg",
			Categories:      []string{"2D Animation"},
			Genres:          []string{"Hentai"},
			ContentFormat:   "animation_2d",
			PublishedAt:     time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
			ScrapedAt:       time.Date(2026, time.March, 31, 1, 2, 3, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("UpsertFeedItems returned error: %v", err)
	}

	got := strings.Join(db.execQueries, "\n")
	if !strings.Contains(got, "select public.upsert_media_item") {
		t.Fatalf("expected media item upsert query, got:\n%s", got)
	}
	if !strings.Contains(got, "update public.media_items") {
		t.Fatalf("expected taxonomy update query, got:\n%s", got)
	}
	if len(db.execArgs) == 0 || db.execArgs[0][2] != "anime" {
		t.Fatalf("expected anime media type, got %#v", db.execArgs)
	}
}

func TestNekopoiStoreKeepsPreviewEpisodeAsMovie(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
	store := NewNekopoiStoreWithDB(db)

	_, err := store.UpsertFeedItems(context.Background(), []nekopoi.FeedItem{
		{
			SourceDomain:    "nekopoi.care",
			Title:           "[PV] Seifuku wa Kita mama de Episode 2",
			NormalizedTitle: "Seifuku wa Kita mama de Episode 2",
			TitleLabels:     []string{"PV"},
			EntryKind:       "preview",
			EpisodeNumber:   2,
			SeriesCandidate: false,
			CanonicalURL:    "https://nekopoi.care/seifuku-episode-2/",
			Slug:            "seifuku-episode-2",
			Categories:      []string{"2D Animation"},
			ContentFormat:   "animation_2d",
			ScrapedAt:       time.Date(2026, time.March, 31, 1, 2, 3, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("UpsertFeedItems returned error: %v", err)
	}
	if len(db.execArgs) == 0 || db.execArgs[0][2] != "movie" {
		t.Fatalf("expected preview episode to stay movie, got %#v", db.execArgs)
	}
}

func TestNekopoiStoreOmitsZeroTimestampsFromPayload(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
	store := NewNekopoiStoreWithDB(db)

	_, err := store.UpsertFeedItems(context.Background(), []nekopoi.FeedItem{
		{
			SourceDomain:    "nekopoi.care",
			Title:           "Example",
			NormalizedTitle: "Example",
			CanonicalURL:    "https://nekopoi.care/example/",
			Slug:            "example",
			ContentFormat:   "live_action",
		},
	})
	if err != nil {
		t.Fatalf("UpsertFeedItems returned error: %v", err)
	}
	payload, ok := db.execArgs[0][11].([]byte)
	if !ok {
		t.Fatalf("expected detail payload at arg 12, got %T", db.execArgs[0][11])
	}
	if strings.Contains(string(payload), "\"published_at\"") {
		t.Fatalf("expected zero published_at to be omitted, got %s", payload)
	}
	if strings.Contains(string(payload), "\"scraped_at\"") {
		t.Fatalf("expected zero scraped_at to be omitted, got %s", payload)
	}
}
