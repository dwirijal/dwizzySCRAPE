package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type AniListPromotionInput struct {
	ItemKey               string
	Source                string
	MediaType             string
	SurfaceType           string
	PresentationType      string
	OriginType            string
	ReleaseCountry        string
	GenreNames            []string
	TaxonomyConfidence    int16
	CurrentTaxonomySource string
	CurrentDetail         map[string]any
	CurrentCoverURL       string
	AniListMatchScore     int
	AniListPayload        map[string]any
}

type AniListPromotionStore struct {
	db contentDB
}

func NewAniListPromotionStoreWithDB(db contentDB) *AniListPromotionStore {
	return &AniListPromotionStore{db: db}
}

func (s *AniListPromotionStore) ApplyPromotion(ctx context.Context, input AniListPromotionInput) error {
	if s.db == nil {
		return fmt.Errorf("content db is required")
	}

	taxonomy := buildAniListPromotedTaxonomy(input)
	releaseYear := readAniListPositiveInt(input.AniListPayload["season_year"])
	score := readAniListScore(input.AniListPayload["average_score"])
	coverURL := firstPresent(
		readTaxonomyString(input.AniListPayload["cover_image"]),
		strings.TrimSpace(input.CurrentCoverURL),
	)

	detailPatch, err := json.Marshal(buildAniListDetailPatch(input.AniListPayload))
	if err != nil {
		return fmt.Errorf("marshal anilist detail patch: %w", err)
	}

	return s.db.Exec(ctx, `
UPDATE public.media_items
SET
    surface_type = $2,
    presentation_type = $3,
    origin_type = $4,
    release_country = NULLIF($5, ''),
    is_nsfw = $6,
    genre_names = $7,
    taxonomy_confidence = GREATEST(taxonomy_confidence, $8),
    taxonomy_source = $9,
    release_year = COALESCE(release_year, NULLIF($10, 0)::smallint),
    score = CASE
        WHEN COALESCE(score, 0) <= 0 AND $11 > 0 THEN $11
        ELSE score
    END,
    cover_url = CASE
        WHEN btrim(COALESCE(cover_url, '')) = '' AND NULLIF($12, '') IS NOT NULL THEN $12
        ELSE cover_url
    END,
    detail = COALESCE(detail, '{}'::jsonb) || $13::jsonb
WHERE item_key = $1
`, input.ItemKey, taxonomy.SurfaceType, taxonomy.PresentationType, taxonomy.OriginType, taxonomy.ReleaseCountry, taxonomy.IsNSFW, taxonomy.GenreNames, taxonomy.TaxonomyConfidence, taxonomy.TaxonomySource, releaseYear, score, coverURL, detailPatch)
}

func buildAniListPromotedTaxonomy(input AniListPromotionInput) MediaTaxonomy {
	current := MediaTaxonomy{
		SurfaceType:        input.SurfaceType,
		PresentationType:   input.PresentationType,
		OriginType:         input.OriginType,
		ReleaseCountry:     input.ReleaseCountry,
		IsNSFW:             readAniListBool(input.AniListPayload["is_adult"]),
		GenreNames:         mergeStringLists(input.GenreNames, readTaxonomyStringSlice(input.AniListPayload["genres"])),
		TaxonomyConfidence: int16(max(int(input.TaxonomyConfidence), min(input.AniListMatchScore, 90))),
		TaxonomySource:     "anilist_promotion",
	}

	if isAniListStrongLock(input.Source, input.MediaType, input.OriginType) {
		current.TaxonomySource = firstPresent(strings.TrimSpace(input.CurrentTaxonomySource), "provider_heuristic")
		if current.ReleaseCountry == "" {
			current.ReleaseCountry = normalizeCountryCode(readTaxonomyString(input.AniListPayload["country_of_origin"]))
		}
		return current
	}

	mediaType := strings.ToUpper(strings.TrimSpace(readTaxonomyString(input.AniListPayload["type"])))
	country := normalizeCountryCode(readTaxonomyString(input.AniListPayload["country_of_origin"]))
	if country == "" {
		country = current.ReleaseCountry
	}

	if mediaType == "MANGA" || current.SurfaceType == "comic" {
		current.SurfaceType = "comic"
		current.PresentationType = "illustrated"
		current.OriginType = classifyComicOriginByCountry(country)
		current.ReleaseCountry = country
		return current
	}

	if mediaType == "ANIME" {
		if current.SurfaceType == "" || current.SurfaceType == "unknown" {
			current.SurfaceType = classifyVideoSurfaceByFormat(readTaxonomyString(input.AniListPayload["format"]))
		}
		if current.SurfaceType == "unknown" {
			current.SurfaceType = "series"
		}
		current.PresentationType = "animation"
		current.OriginType = classifyAnimationOriginByCountry(country)
		current.ReleaseCountry = country
	}

	return current
}

func isAniListStrongLock(source, mediaType, originType string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	originType = strings.ToLower(strings.TrimSpace(originType))

	switch {
	case source == "anichin":
		return true
	case source == "samehadaku" && mediaType == "anime":
		return true
	case source == "drakorid" && mediaType == "movie":
		return true
	case source == "drakorid" && originType == "variety":
		return true
	default:
		return false
	}
}

func classifyComicOriginByCountry(country string) string {
	switch country {
	case "KR":
		return "manhwa"
	case "CN":
		return "manhua"
	default:
		return "manga"
	}
}

func classifyAnimationOriginByCountry(country string) string {
	switch country {
	case "CN":
		return "donghua"
	default:
		return "anime"
	}
}

func classifyVideoSurfaceByFormat(format string) string {
	switch strings.ToUpper(strings.TrimSpace(format)) {
	case "MOVIE":
		return "movie"
	default:
		return "series"
	}
}

func readAniListPositiveInt(value any) int {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case float64:
		if typed > 0 {
			return int(typed)
		}
	}
	return 0
}

func readAniListScore(value any) float32 {
	score := readAniListPositiveInt(value)
	if score <= 0 {
		return 0
	}
	return float32(score) / 10
}

func readAniListBool(value any) bool {
	typed, _ := value.(bool)
	return typed
}

func buildAniListDetailPatch(payload map[string]any) map[string]any {
	patch := map[string]any{}
	for key, target := range map[string]string{
		"title_english": "anilist_title_english",
		"title_romaji":  "anilist_title_romaji",
		"title_native":  "anilist_title_native",
		"description":   "anilist_description",
		"banner_image":  "anilist_banner_url",
		"site_url":      "anilist_site_url",
		"format":        "anilist_format",
		"status":        "anilist_status",
	} {
		if value := readTaxonomyString(payload[key]); value != "" {
			patch[target] = value
		}
	}
	if synonyms := readTaxonomyStringSlice(payload["synonyms"]); len(synonyms) > 0 {
		patch["anilist_synonyms"] = synonyms
	}
	if readAniListBool(payload["is_adult"]) {
		patch["anilist_is_adult"] = true
	}
	return patch
}

func mergeStringLists(left, right []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(left)+len(right))
	for _, group := range [][]string{left, right} {
		for _, value := range group {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, trimmed)
		}
	}
	return out
}
