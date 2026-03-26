package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
)

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *int64:
			*target = r.values[i].(int64)
		}
	}
	return nil
}

type fakeContentDB struct {
	execQueries []string
	execArgs    [][]any
	queryArgs   [][]any
	titleID     int64
	genreID     int64
}

func (db *fakeContentDB) Exec(_ context.Context, query string, args ...any) error {
	db.execQueries = append(db.execQueries, normalizeSQL(query))
	db.execArgs = append(db.execArgs, args)
	return nil
}

func (db *fakeContentDB) QueryRow(_ context.Context, query string, args ...any) rowScanner {
	query = normalizeSQL(query)
	db.queryArgs = append(db.queryArgs, args)
	switch {
	case strings.Contains(query, "insert into content_titles"):
		return fakeRow{values: []any{db.titleID}}
	case strings.Contains(query, "insert into content_genres"):
		return fakeRow{values: []any{db.genreID}}
	case strings.Contains(query, "from content_source_links"):
		return fakeRow{values: []any{db.titleID}}
	case strings.Contains(query, "insert into content_units"):
		return fakeRow{values: []any{db.titleID}}
	default:
		return fakeRow{values: []any{int64(0)}}
	}
}

func TestContentStoreUpsertManhwaSeries(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{titleID: 101, genreID: 7}
	store := NewContentStore(db)

	err := store.UpsertManhwaSeries(context.Background(), content.ManhwaSeries{
		Source:       "manhwaindo",
		MediaType:    "manhwa",
		Slug:         "solo-leveling",
		Title:        "Solo Leveling",
		AltTitle:     "나 혼자만 레벨업",
		CanonicalURL: "https://www.manhwaindo.my/series/solo-leveling/",
		CoverURL:     "https://img.example/solo.jpg",
		Status:       "Ongoing",
		Type:         "Manhwa",
		ReleasedYear: "2018",
		Author:       "Chugong",
		Synopsis:     "Synopsis",
		Genres:       []string{"Action", "Fantasy"},
		LatestChapter: &content.ManhwaChapterRef{
			Slug:         "solo-leveling-chapter-179-2",
			Label:        "Chapter 179.2",
			Number:       "179.2",
			CanonicalURL: "https://www.manhwaindo.my/solo-leveling-chapter-179-2/",
		},
		Chapters: []content.ManhwaChapterRef{
			{
				Slug:         "solo-leveling-chapter-179-2",
				Title:        "Solo Leveling Chapter 179.2",
				Label:        "Chapter 179.2",
				Number:       "179.2",
				CanonicalURL: "https://www.manhwaindo.my/solo-leveling-chapter-179-2/",
				PublishedAt:  "2025-11-30",
			},
		},
	})
	if err != nil {
		t.Fatalf("UpsertManhwaSeries returned error: %v", err)
	}

	got := strings.Join(db.execQueries, "\n")
	for _, want := range []string{
		"insert into content_source_links",
		"delete from content_title_genres",
		"insert into content_title_genres",
		"insert into content_units",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected query %q in executed queries:\n%s", want, got)
		}
	}
}

func TestContentStoreUpsertManhwaChapterPages(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{titleID: 101}
	store := NewContentStore(db)

	err := store.UpsertManhwaChapter(context.Background(), content.ManhwaChapter{
		Source:       "manhwaindo",
		SeriesSlug:   "solo-leveling",
		SeriesTitle:  "Solo Leveling",
		Slug:         "solo-leveling-chapter-100",
		Title:        "Solo Leveling Chapter 100",
		Label:        "Chapter 100",
		Number:       "100",
		CanonicalURL: "https://www.manhwaindo.my/solo-leveling-chapter-100/",
		PrevURL:      "https://www.manhwaindo.my/solo-leveling-chapter-99/",
		NextURL:      "https://www.manhwaindo.my/solo-leveling-chapter-101/",
		Pages: []content.PageAsset{
			{Position: 1, URL: "https://img.example/1.jpg"},
			{Position: 2, URL: "https://img.example/2.jpg"},
		},
	})
	if err != nil {
		t.Fatalf("UpsertManhwaChapter returned error: %v", err)
	}

	foundPagesJSON := false
	for _, args := range db.execArgs {
		for _, arg := range args {
			payload, ok := arg.([]byte)
			if !ok {
				continue
			}
			var pages []content.PageAsset
			if json.Unmarshal(payload, &pages) == nil && len(pages) == 2 {
				foundPagesJSON = true
			}
		}
	}
	for _, args := range db.queryArgs {
		for _, arg := range args {
			payload, ok := arg.([]byte)
			if !ok {
				continue
			}
			var pages []content.PageAsset
			if json.Unmarshal(payload, &pages) == nil && len(pages) == 2 {
				foundPagesJSON = true
			}
		}
	}
	if !foundPagesJSON {
		t.Fatal("expected serialized pages json in exec args")
	}
}

func TestChapterSequenceIndex(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		number   string
		fallback int
		want     float64
	}{
		{name: "integer", number: "100", fallback: 9, want: 100},
		{name: "decimal", number: "179.2", fallback: 9, want: 179.2},
		{name: "embedded label", number: "Chapter 12.5", fallback: 9, want: 12.5},
		{name: "fallback", number: "", fallback: 9, want: 9},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := chapterSequenceIndex(tc.number, tc.fallback); got != tc.want {
				t.Fatalf("chapterSequenceIndex(%q, %d) = %v, want %v", tc.number, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestNormalizePublishedAt(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "iso", raw: "2025-11-30", want: "2025-11-30T00:00:00Z"},
		{name: "slash date", raw: "25/03/2026", want: "2026-03-25T00:00:00Z"},
		{name: "indonesian month", raw: "23 Maret 2026", want: "2026-03-23T00:00:00Z"},
		{name: "indonesian december", raw: "8 Desember 2025", want: "2025-12-08T00:00:00Z"},
		{name: "unknown", raw: "3 jam lalu", want: ""},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizePublishedAt(tc.raw); got != tc.want {
				t.Fatalf("normalizePublishedAt(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNormalizeSourceKey(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		raw  string
		want string
	}{
		{raw: "manhwaindo", want: "manhwaindo"},
		{raw: "komiku", want: "komiku"},
		{raw: "", want: "manhwaindo"},
		{raw: "unknown", want: "manhwaindo"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			if got := normalizeSourceKey(tc.raw); got != tc.want {
				t.Fatalf("normalizeSourceKey(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func normalizeSQL(query string) string {
	return strings.Join(strings.Fields(strings.ToLower(query)), " ")
}
