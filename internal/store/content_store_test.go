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
		case *string:
			*target = r.values[i].(string)
		}
	}
	return nil
}

type fakeContentDB struct {
	execQueries []string
	execArgs    [][]any
	queryRows   map[string]fakeRow
}

func (db *fakeContentDB) Exec(_ context.Context, query string, args ...any) error {
	db.execQueries = append(db.execQueries, normalizeSQL(query))
	db.execArgs = append(db.execArgs, args)
	return nil
}

func (db *fakeContentDB) QueryRow(_ context.Context, query string, args ...any) rowScanner {
	if db.queryRows != nil {
		key := normalizeSQL(query)
		if row, ok := db.queryRows[key]; ok {
			return row
		}
	}
	return fakeRow{values: []any{""}}
}

func TestContentStoreUpsertManhwaSeries(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
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
		"select public.upsert_media_item",
		"select public.upsert_media_unit",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected query %q in executed queries:\n%s", want, got)
		}
	}
}

func TestNormalizePublishedAtParsesMonthFirstEnglish(t *testing.T) {
	t.Parallel()

	got := normalizePublishedAt("March 29, 2026")
	if got != "2026-03-29T00:00:00Z" {
		t.Fatalf("normalizePublishedAt returned %q", got)
	}
}

func TestNormalizePublishedAtFromEmbeddedDate(t *testing.T) {
	t.Parallel()

	got := normalizePublishedAtFromEmbeddedDate(`{"img":"http://sk13.drakor.cc/images/0-2026-01-17-272111178071ec41b1f550b2756ef697.jpg"}`)

	if got != "2026-01-17T00:00:00Z" {
		t.Fatalf("normalizePublishedAtFromEmbeddedDate returned %q", got)
	}
}

func TestNormalizePublishedAtFromEmbeddedTimestamp(t *testing.T) {
	t.Parallel()

	got := normalizePublishedAtFromEmbeddedTimestamp(`{"published_at":"2026-01-17T13:45:22+00:00"}`)

	if got != "2026-01-17T13:45:22Z" {
		t.Fatalf("normalizePublishedAtFromEmbeddedTimestamp returned %q", got)
	}
}

func TestContentStoreUpsertManhwaChapterPages(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
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
			var row map[string]any
			if json.Unmarshal(payload, &row) == nil {
				pages, ok := row["pages"].([]any)
				if ok && len(pages) == 2 {
					foundPagesJSON = true
				}
			}
			var pages []content.PageAsset
			if json.Unmarshal(payload, &pages) == nil && len(pages) == 2 {
				t.Fatalf("expected chapter detail payload to be an object, got array: %s", string(payload))
			}
		}
	}
	if !foundPagesJSON {
		t.Fatal("expected serialized pages json in exec args")
	}
}

func TestContentStoreUpsertManhwaChapterUsesExistingSeriesItemKey(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{
		queryRows: map[string]fakeRow{
			normalizeSQL(`
SELECT item_key
FROM public.media_items
WHERE source = $1 AND slug = $2
ORDER BY updated_at DESC
LIMIT 1
`): {values: []any{"kanzenin:manga:example-title"}},
		},
	}
	store := NewContentStore(db)

	err := store.UpsertManhwaChapter(context.Background(), content.ManhwaChapter{
		Source:       "kanzenin",
		SeriesSlug:   "example-title",
		SeriesTitle:  "Example Title",
		Slug:         "example-title-chapter-1",
		Title:        "Example Title Chapter 1",
		Label:        "Chapter 1",
		Number:       "1",
		CanonicalURL: "https://kanzenin.info/example-title-chapter-1/",
	})
	if err != nil {
		t.Fatalf("UpsertManhwaChapter returned error: %v", err)
	}

	if len(db.execArgs) == 0 {
		t.Fatal("expected exec args")
	}
	if got := db.execArgs[0][1]; got != "kanzenin:manga:example-title" {
		t.Fatalf("item key = %#v, want %#v", got, "kanzenin:manga:example-title")
	}
}

func TestContentStoreUpsertManhwaSeriesAddsNSFWGenreForKanzenin(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
	store := NewContentStore(db)

	err := store.UpsertManhwaSeries(context.Background(), content.ManhwaSeries{
		Source:       "kanzenin",
		MediaType:    "manga",
		Slug:         "example-title",
		Title:        "Example Title",
		CanonicalURL: "https://kanzenin.info/manga/example-title/",
		Genres:       []string{"Drama", "Romance"},
	})
	if err != nil {
		t.Fatalf("UpsertManhwaSeries returned error: %v", err)
	}

	if len(db.execArgs) == 0 {
		t.Fatal("expected exec args")
	}

	var detail map[string]any
	payload, ok := db.execArgs[0][11].([]byte)
	if !ok {
		t.Fatalf("expected detail payload at arg 12, got %T", db.execArgs[0][11])
	}
	if err := json.Unmarshal(payload, &detail); err != nil {
		t.Fatalf("unmarshal detail payload: %v", err)
	}

	rawGenres, ok := detail["genres"].([]any)
	if !ok {
		t.Fatalf("expected genres array in detail payload, got %#v", detail["genres"])
	}

	got := make([]string, 0, len(rawGenres))
	for _, value := range rawGenres {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("expected string genre, got %T", value)
		}
		got = append(got, text)
	}

	want := []string{"Drama", "Romance", "nsfw"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("genres = %#v, want %#v", got, want)
	}
}

func TestContentStoreUpsertManhwaSeriesDedupesNSFWGenreForKanzenin(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
	store := NewContentStore(db)

	err := store.UpsertManhwaSeries(context.Background(), content.ManhwaSeries{
		Source:       "kanzenin",
		MediaType:    "manga",
		Slug:         "example-title",
		Title:        "Example Title",
		CanonicalURL: "https://kanzenin.info/manga/example-title/",
		Genres:       []string{"Drama", "NSFW", "Romance"},
	})
	if err != nil {
		t.Fatalf("UpsertManhwaSeries returned error: %v", err)
	}

	if len(db.execArgs) == 0 {
		t.Fatal("expected exec args")
	}

	var detail map[string]any
	payload, ok := db.execArgs[0][11].([]byte)
	if !ok {
		t.Fatalf("expected detail payload at arg 12, got %T", db.execArgs[0][11])
	}
	if err := json.Unmarshal(payload, &detail); err != nil {
		t.Fatalf("unmarshal detail payload: %v", err)
	}

	rawGenres, ok := detail["genres"].([]any)
	if !ok {
		t.Fatalf("expected genres array in detail payload, got %#v", detail["genres"])
	}

	got := make([]string, 0, len(rawGenres))
	for _, value := range rawGenres {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("expected string genre, got %T", value)
		}
		got = append(got, text)
	}

	want := []string{"Drama", "NSFW", "Romance"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("genres = %#v, want %#v", got, want)
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
		{raw: "bacaman", want: "bacaman"},
		{raw: "mangasusuku", want: "mangasusuku"},
		{raw: "kanzenin", want: "kanzenin"},
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

func TestContentStoreUpsertManhwaSeriesAddsNSFWGenreForMangasusuku(t *testing.T) {
	t.Parallel()

	db := &fakeContentDB{}
	store := NewContentStore(db)

	err := store.UpsertManhwaSeries(context.Background(), content.ManhwaSeries{
		Source:       "mangasusuku",
		MediaType:    "manhwa",
		Slug:         "smoking-hypnosis",
		Title:        "Smoking Hypnosis",
		CanonicalURL: "https://mangasusuku.com/komik/smoking-hypnosis/",
		Genres:       []string{"Drama", "Mature"},
	})
	if err != nil {
		t.Fatalf("UpsertManhwaSeries returned error: %v", err)
	}

	var detail map[string]any
	payload, ok := db.execArgs[0][11].([]byte)
	if !ok {
		t.Fatalf("expected detail payload at arg 12, got %T", db.execArgs[0][11])
	}
	if err := json.Unmarshal(payload, &detail); err != nil {
		t.Fatalf("unmarshal detail payload: %v", err)
	}
	rawGenres, ok := detail["genres"].([]any)
	if !ok {
		t.Fatalf("expected genres array in detail payload, got %#v", detail["genres"])
	}
	got := make([]string, 0, len(rawGenres))
	for _, value := range rawGenres {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("expected string genre, got %T", value)
		}
		got = append(got, text)
	}
	want := []string{"Drama", "Mature", "nsfw"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("genres = %#v, want %#v", got, want)
	}
}

func normalizeSQL(query string) string {
	return strings.Join(strings.Fields(strings.ToLower(query)), " ")
}
