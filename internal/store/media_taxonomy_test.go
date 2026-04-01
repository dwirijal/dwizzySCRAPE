package store

import (
	"testing"
	"time"
)

func TestClassifyMediaItemSamehadakuAnime(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("samehadaku", "anime", map[string]any{
		"genres": []string{"Action", "Fantasy"},
	})

	if got.SurfaceType != "series" {
		t.Fatalf("surface_type = %q, want %q", got.SurfaceType, "series")
	}
	if got.PresentationType != "animation" {
		t.Fatalf("presentation_type = %q, want %q", got.PresentationType, "animation")
	}
	if got.OriginType != "anime" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "anime")
	}
	if got.ReleaseCountry != "JP" {
		t.Fatalf("release_country = %q, want %q", got.ReleaseCountry, "JP")
	}
	if got.IsNSFW {
		t.Fatal("expected samehadaku anime to be safe by default")
	}
	if got.TaxonomySource != "provider_heuristic" {
		t.Fatalf("taxonomy_source = %q, want %q", got.TaxonomySource, "provider_heuristic")
	}
	if got.TaxonomyConfidence <= 0 {
		t.Fatalf("taxonomy_confidence = %d, want positive", got.TaxonomyConfidence)
	}
	if len(got.GenreNames) != 2 {
		t.Fatalf("genre_names length = %d, want 2", len(got.GenreNames))
	}
	if got.ReleaseTimezone != "Asia/Tokyo" {
		t.Fatalf("release_timezone = %q, want %q", got.ReleaseTimezone, "Asia/Tokyo")
	}
}

func TestClassifyMediaItemAnichinAsDonghua(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("anichin", "anime", map[string]any{
		"genre_names": []string{"Action"},
	})

	if got.SurfaceType != "series" {
		t.Fatalf("surface_type = %q, want %q", got.SurfaceType, "series")
	}
	if got.PresentationType != "animation" {
		t.Fatalf("presentation_type = %q, want %q", got.PresentationType, "animation")
	}
	if got.OriginType != "donghua" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "donghua")
	}
	if got.ReleaseCountry != "CN" {
		t.Fatalf("release_country = %q, want %q", got.ReleaseCountry, "CN")
	}
}

func TestClassifyMediaItemDrakoridDrama(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("drakorid", "drama", map[string]any{
		"country": "South Korea",
	})

	if got.SurfaceType != "series" {
		t.Fatalf("surface_type = %q, want %q", got.SurfaceType, "series")
	}
	if got.PresentationType != "live_action" {
		t.Fatalf("presentation_type = %q, want %q", got.PresentationType, "live_action")
	}
	if got.OriginType != "drama" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "drama")
	}
	if got.ReleaseCountry != "KR" {
		t.Fatalf("release_country = %q, want %q", got.ReleaseCountry, "KR")
	}
}

func TestClassifyMediaItemDrakoridMovie(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("drakorid", "movie", map[string]any{
		"country": "South Korea",
	})

	if got.SurfaceType != "movie" {
		t.Fatalf("surface_type = %q, want %q", got.SurfaceType, "movie")
	}
	if got.PresentationType != "live_action" {
		t.Fatalf("presentation_type = %q, want %q", got.PresentationType, "live_action")
	}
	if got.OriginType != "movie" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "movie")
	}
	if got.ReleaseCountry != "KR" {
		t.Fatalf("release_country = %q, want %q", got.ReleaseCountry, "KR")
	}
}

func TestClassifyMediaItemDrakoridVariety(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("drakorid", "drama", map[string]any{
		"source_title": "Running Man Variety Show (2026)",
		"country":      "Korea",
	})

	if got.SurfaceType != "series" {
		t.Fatalf("surface_type = %q, want %q", got.SurfaceType, "series")
	}
	if got.PresentationType != "live_action" {
		t.Fatalf("presentation_type = %q, want %q", got.PresentationType, "live_action")
	}
	if got.OriginType != "variety" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "variety")
	}
	if got.ReleaseCountry != "KR" {
		t.Fatalf("release_country = %q, want %q", got.ReleaseCountry, "KR")
	}
}

func TestClassifyMediaItemDrakoridRealityDatingShowAsVariety(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("drakorid", "drama", map[string]any{
		"source_title": "Singles Inferno S5 (2026)",
		"country":      "South Korea",
	})

	if got.OriginType != "variety" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "variety")
	}
}

func TestClassifyMediaItemMarksNSFWFromTags(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("kanzenin", "manga", map[string]any{
		"tags": []string{"Romance", "NSFW"},
	})

	if !got.IsNSFW {
		t.Fatal("expected nsfw tag to set IsNSFW")
	}
}

func TestClassifyMediaItemNekopoiAnimationSeries(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("nekopoi", "anime", map[string]any{
		"content_format": "animation_2d",
		"tags":           []string{"2D Animation"},
		"genres":         []string{"Hentai"},
	})

	if got.SurfaceType != "series" {
		t.Fatalf("surface_type = %q, want %q", got.SurfaceType, "series")
	}
	if got.PresentationType != "animation" {
		t.Fatalf("presentation_type = %q, want %q", got.PresentationType, "animation")
	}
	if got.OriginType != "anime" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "anime")
	}
	if got.ReleaseCountry != "JP" {
		t.Fatalf("release_country = %q, want %q", got.ReleaseCountry, "JP")
	}
	if !got.IsNSFW {
		t.Fatal("expected nekopoi items to be nsfw")
	}
}

func TestClassifyMediaItemNekopoiLiveActionMovie(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("nekopoi", "movie", map[string]any{
		"content_format": "live_action",
		"tags":           []string{"JAV"},
	})

	if got.SurfaceType != "movie" {
		t.Fatalf("surface_type = %q, want %q", got.SurfaceType, "movie")
	}
	if got.PresentationType != "live_action" {
		t.Fatalf("presentation_type = %q, want %q", got.PresentationType, "live_action")
	}
	if got.OriginType != "movie" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "movie")
	}
	if !got.IsNSFW {
		t.Fatal("expected nekopoi items to be nsfw")
	}
}

func TestClassifyMediaItemHanimeEpisodeSeries(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("hanime", "anime", map[string]any{
		"content_format": "animation_hentai",
		"tags":           []string{"hentai", "fantasy"},
	})

	if got.SurfaceType != "series" {
		t.Fatalf("surface_type = %q, want %q", got.SurfaceType, "series")
	}
	if got.PresentationType != "animation" {
		t.Fatalf("presentation_type = %q, want %q", got.PresentationType, "animation")
	}
	if got.OriginType != "anime" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "anime")
	}
	if got.ReleaseCountry != "JP" {
		t.Fatalf("release_country = %q, want %q", got.ReleaseCountry, "JP")
	}
	if !got.IsNSFW {
		t.Fatal("expected hanime items to be nsfw")
	}
}

func TestClassifyMediaItemHanimeStandaloneMovie(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("hanime", "movie", map[string]any{
		"content_format": "animation_hentai",
		"tags":           []string{"hentai"},
	})

	if got.SurfaceType != "movie" {
		t.Fatalf("surface_type = %q, want %q", got.SurfaceType, "movie")
	}
	if got.PresentationType != "animation" {
		t.Fatalf("presentation_type = %q, want %q", got.PresentationType, "animation")
	}
	if got.OriginType != "anime" {
		t.Fatalf("origin_type = %q, want %q", got.OriginType, "anime")
	}
	if !got.IsNSFW {
		t.Fatal("expected hanime items to be nsfw")
	}
}

func TestClassifyMediaItemExtractsScheduleFields(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("drakorid", "drama", map[string]any{
		"country":        "South Korea",
		"aired":          "Every Friday",
		"broadcast_time": "22:30",
		"status":         "Ongoing",
	})

	if got.ReleaseDay != "friday" {
		t.Fatalf("release_day = %q, want %q", got.ReleaseDay, "friday")
	}
	if got.ReleaseWindow != "22:30" {
		t.Fatalf("release_window = %q, want %q", got.ReleaseWindow, "22:30")
	}
	if got.ReleaseTimezone != "Asia/Seoul" {
		t.Fatalf("release_timezone = %q, want %q", got.ReleaseTimezone, "Asia/Seoul")
	}
	if got.Cadence != "weekly" {
		t.Fatalf("cadence = %q, want %q", got.Cadence, "weekly")
	}
}

func TestClassifyMediaItemExtractsDrakoridScheduleFromRuntime(t *testing.T) {
	t.Parallel()

	got := ClassifyMediaItem("drakorid", "drama", map[string]any{
		"country": "South Korea",
		"runtime": "Monday & Tuesday 20:50",
		"status":  "Ongoing",
	})

	if got.ReleaseDay != "monday" {
		t.Fatalf("release_day = %q, want %q", got.ReleaseDay, "monday")
	}
	if got.ReleaseWindow != "20:50" {
		t.Fatalf("release_window = %q, want %q", got.ReleaseWindow, "20:50")
	}
	if got.Cadence != "weekly" {
		t.Fatalf("cadence = %q, want %q", got.Cadence, "weekly")
	}
}

func TestInferNextReleaseAtForWeeklySeries(t *testing.T) {
	t.Parallel()

	got := inferNextReleaseAt(MediaTaxonomy{
		SurfaceType:     "series",
		ReleaseDay:      "saturday",
		ReleaseWindow:   "22:30",
		ReleaseTimezone: "Asia/Seoul",
		Cadence:         "weekly",
	}, time.Date(2026, 3, 30, 8, 0, 0, 0, time.UTC))

	if got == nil {
		t.Fatal("expected next_release_at")
	}

	want := time.Date(2026, 4, 4, 13, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("next_release_at = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}
