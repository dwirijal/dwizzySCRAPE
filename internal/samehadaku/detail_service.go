package samehadaku

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/jikan"
)

type CatalogLookup interface {
	GetCatalogBySlug(ctx context.Context, slug string) (CatalogItem, error)
}

type AnimeDetailWriter interface {
	UpsertAnimeDetail(ctx context.Context, detail AnimeDetail) error
}

type PageFetcher interface {
	FetchPage(ctx context.Context, url string) ([]byte, error)
}

type JikanClient interface {
	SearchAnime(ctx context.Context, query string, limit int) ([]jikan.AnimeSearchHit, error)
	GetAnimeFull(ctx context.Context, malID int) (jikan.AnimeFull, error)
	GetAnimeCharacters(ctx context.Context, malID int) ([]jikan.AnimeCharacter, error)
}

type DetailSyncReport struct {
	Slug              string
	MALID             int
	SourceFetchStatus string
}

type DetailService struct {
	catalog CatalogLookup
	writer  AnimeDetailWriter
	jikan   JikanClient
	fetcher PageFetcher
	now     func() time.Time
}

func NewDetailService(catalog CatalogLookup, writer AnimeDetailWriter, jikanClient JikanClient, fetcher PageFetcher, fixedNow time.Time) *DetailService {
	nowFn := time.Now
	if !fixedNow.IsZero() {
		nowFn = func() time.Time { return fixedNow }
	}
	return &DetailService{
		catalog: catalog,
		writer:  writer,
		jikan:   jikanClient,
		fetcher: fetcher,
		now:     nowFn,
	}
}

func (s *DetailService) SyncAnimeDetail(ctx context.Context, slug string) (DetailSyncReport, error) {
	if s.catalog == nil {
		return DetailSyncReport{}, fmt.Errorf("catalog lookup is required")
	}
	if s.writer == nil {
		return DetailSyncReport{}, fmt.Errorf("anime detail writer is required")
	}
	if s.jikan == nil {
		return DetailSyncReport{}, fmt.Errorf("jikan client is required")
	}

	catalogItem, err := s.catalog.GetCatalogBySlug(ctx, slug)
	if err != nil {
		return DetailSyncReport{}, err
	}

	detail := AnimeDetail{
		Slug:                  catalogItem.Slug,
		CanonicalURL:          catalogItem.CanonicalURL,
		PrimarySourceURL:      catalogItem.CanonicalURL,
		PrimarySourceDomain:   catalogItem.SourceDomain,
		SecondarySourceURL:    BuildMirrorAnimeURL(catalogItem.Slug),
		SecondarySourceDomain: extractDomain(BuildMirrorAnimeURL(catalogItem.Slug)),
		EffectiveSourceURL:    catalogItem.CanonicalURL,
		EffectiveSourceDomain: catalogItem.SourceDomain,
		EffectiveSourceKind:   "primary",
		SourceTitle:           catalogItem.Title,
		SynopsisSource:        catalogItem.SynopsisExcerpt,
		AnimeType:             catalogItem.AnimeType,
		Status:                catalogItem.Status,
		StudioNames:           []string{},
		GenreNames:            append([]string(nil), catalogItem.Genres...),
		BatchLinksJSON:        mustMarshalJSON(map[string]string{}),
		CastJSON:              mustMarshalJSON([]any{}),
		JikanMetaJSON:         mustMarshalJSON(map[string]any{}),
		SourceFetchStatus:     "not_attempted",
		ScrapedAt:             s.now().UTC(),
	}

	primaryAttempt := SourceAttempt{
		Kind:   "primary",
		URL:    detail.PrimarySourceURL,
		Domain: detail.PrimarySourceDomain,
		Status: "not_attempted",
	}
	secondaryAttempt := SourceAttempt{
		Kind:   "secondary",
		URL:    detail.SecondarySourceURL,
		Domain: detail.SecondarySourceDomain,
		Status: "not_attempted",
	}

	if s.fetcher != nil && strings.TrimSpace(catalogItem.CanonicalURL) != "" {
		var primaryBody []byte
		primaryAttempt, primaryBody = fetchSourceAttempt(ctx, s.fetcher, "primary", detail.PrimarySourceURL)
		if primaryAttempt.Status == "fetched" {
			if len(primaryBody) == 0 {
				primaryAttempt.Status = "empty"
			} else {
				supplementWithPrimaryPage(&detail, primaryBody, primaryAttempt.URL)
			}
		}
		if primaryAttempt.Status != "fetched" {
			var secondaryBody []byte
			secondaryAttempt, secondaryBody = fetchSourceAttempt(ctx, s.fetcher, "secondary", detail.SecondarySourceURL)
			if secondaryAttempt.Status == "fetched" {
				supplementWithMirrorPage(&detail, secondaryBody, secondaryAttempt.URL)
			}
		}
		detail.SourceFetchStatus = buildOverallFetchStatus(primaryAttempt, secondaryAttempt)
		detail.SourceFetchError = buildFetchError(primaryAttempt, secondaryAttempt)
		detail.EffectiveSourceKind, detail.EffectiveSourceURL, detail.EffectiveSourceDomain = resolveEffectiveSource(primaryAttempt, secondaryAttempt)
	}
	detail.SourceMetaJSON = mustMarshalJSON(buildSourceMeta(catalogItem, detail, primaryAttempt, secondaryAttempt))

	match, full, characters, err := s.lookupJikan(ctx, catalogItem.Title)
	if err != nil {
		detail.JikanMetaJSON = mustMarshalJSON(map[string]any{
			"matched_by":   "jikan",
			"matched_at":   detail.ScrapedAt,
			"lookup_error": err.Error(),
		})
	} else if match.MALID != 0 {
		detail.MALID = match.MALID
		detail.MALURL = firstNonEmpty(full.URL, match.URL)
		detail.MALThumbnailURL = firstNonEmpty(imageFromFull(full), imageFromSearch(match))
		detail.SynopsisEnriched = strings.TrimSpace(full.Synopsis)
		detail.AnimeType = firstNonEmpty(full.Type, detail.AnimeType)
		detail.Status = firstNonEmpty(full.Status, detail.Status)
		detail.Season = full.Season
		detail.StudioNames = mergeUniqueStrings(detail.StudioNames, collectNames(full.Studios))
		detail.GenreNames = mergeUniqueStrings(detail.GenreNames, collectNames(full.Genres), collectNames(full.Themes), collectNames(full.Demographics))
		detail.CastJSON = mustMarshalJSON(mergeCastLists(detail.CastJSON, normalizeCharacters(characters)))
		detail.JikanMetaJSON = mustMarshalJSON(map[string]any{
			"search_hit": match,
			"anime_full": full,
			"characters": characters,
			"matched_by": "jikan",
			"matched_at": detail.ScrapedAt,
		})
	}

	if err := s.writer.UpsertAnimeDetail(ctx, detail); err != nil {
		return DetailSyncReport{}, err
	}
	return DetailSyncReport{
		Slug:              detail.Slug,
		MALID:             detail.MALID,
		SourceFetchStatus: detail.SourceFetchStatus,
	}, nil
}

func (s *DetailService) lookupJikan(ctx context.Context, title string) (jikan.AnimeSearchHit, jikan.AnimeFull, []jikan.AnimeCharacter, error) {
	results, err := s.jikan.SearchAnime(ctx, title, 5)
	if err != nil {
		return jikan.AnimeSearchHit{}, jikan.AnimeFull{}, nil, fmt.Errorf("search jikan for %q: %w", title, err)
	}
	match, ok := jikan.PickBestMatch(title, results)
	if !ok {
		return jikan.AnimeSearchHit{}, jikan.AnimeFull{}, nil, nil
	}
	full, err := s.jikan.GetAnimeFull(ctx, match.MALID)
	if err != nil {
		return jikan.AnimeSearchHit{}, jikan.AnimeFull{}, nil, fmt.Errorf("get jikan full %d: %w", match.MALID, err)
	}
	characters, err := s.jikan.GetAnimeCharacters(ctx, match.MALID)
	if err != nil {
		return jikan.AnimeSearchHit{}, jikan.AnimeFull{}, nil, fmt.Errorf("get jikan characters %d: %w", match.MALID, err)
	}
	return match, full, characters, nil
}

func buildSourceMeta(item CatalogItem, detail AnimeDetail, primaryAttempt, secondaryAttempt SourceAttempt) map[string]any {
	return map[string]any{
		"source":                item.Source,
		"source_domain":         item.SourceDomain,
		"content_type":          item.ContentType,
		"catalog_page_number":   item.PageNumber,
		"catalog_poster_url":    item.PosterURL,
		"catalog_score":         item.Score,
		"catalog_views":         item.Views,
		"catalog_genres":        item.Genres,
		"primary_attempt":       primaryAttempt,
		"secondary_attempt":     secondaryAttempt,
		"effective_source_kind": detail.EffectiveSourceKind,
		"effective_source_url":  detail.EffectiveSourceURL,
		"effective_source_host": detail.EffectiveSourceDomain,
		"batch_links_count":     batchLinksCount(detail.BatchLinksJSON),
		"fetch_status":          detail.SourceFetchStatus,
		"fetch_error":           detail.SourceFetchError,
	}
}

func supplementWithPrimaryPage(detail *AnimeDetail, raw []byte, sourceURL string) {
	if detail == nil || len(raw) == 0 {
		return
	}
	page, err := ParsePrimaryAnimeHTML(raw, sourceURL)
	if err != nil {
		return
	}
	detail.SynopsisSource = firstNonEmpty(detail.SynopsisSource, page.Synopsis)
	detail.SourceTitle = firstNonEmpty(detail.SourceTitle, page.Title)
	detail.GenreNames = mergeUniqueStrings(detail.GenreNames, page.Genres)
	detail.StudioNames = mergeUniqueStrings(detail.StudioNames, page.Studios)
	detail.BatchLinksJSON = mergeBatchLinksJSON(detail.BatchLinksJSON, page.BatchLinks)
}

func supplementWithMirrorPage(detail *AnimeDetail, raw []byte, sourceURL string) {
	if detail == nil || len(raw) == 0 {
		return
	}
	page, err := ParseMirrorAnimeHTML(raw, sourceURL)
	if err != nil {
		return
	}
	detail.SynopsisSource = firstNonEmpty(detail.SynopsisSource, page.Synopsis)
	detail.SourceTitle = firstNonEmpty(detail.SourceTitle, page.Title)
	detail.GenreNames = mergeUniqueStrings(detail.GenreNames, page.Genres)
	detail.StudioNames = mergeUniqueStrings(detail.StudioNames, page.Studios)
	detail.BatchLinksJSON = mergeBatchLinksJSON(detail.BatchLinksJSON, page.BatchLinks)
	if len(bytesTrimSpace(detail.CastJSON)) == 0 || string(bytesTrimSpace(detail.CastJSON)) == "[]" {
		detail.CastJSON = mustMarshalJSON(normalizeMirrorCast(page.Cast))
	}
}

func normalizeMirrorCast(entries []MirrorCastEntry) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.CharacterName) == "" {
			continue
		}
		voiceActors := make([]map[string]any, 0, 1)
		if strings.TrimSpace(entry.ActorName) != "" {
			voiceActors = append(voiceActors, map[string]any{
				"name":      entry.ActorName,
				"language":  entry.ActorRole,
				"image_url": entry.ActorImage,
			})
		}
		out = append(out, map[string]any{
			"role":                entry.CharacterRole,
			"character_name":      entry.CharacterName,
			"character_image_url": entry.CharacterImage,
			"voice_actors":        voiceActors,
		})
	}
	return out
}

func mergeCastLists(existing []byte, enriched []map[string]any) []map[string]any {
	trimmed := bytesTrimSpace(existing)
	if len(trimmed) == 0 || string(trimmed) == "[]" {
		return enriched
	}
	var current []map[string]any
	if err := json.Unmarshal(trimmed, &current); err != nil {
		return enriched
	}
	if len(enriched) == 0 {
		return current
	}
	return enriched
}

func bytesTrimSpace(raw []byte) []byte {
	return []byte(strings.TrimSpace(string(raw)))
}

func mergeBatchLinksJSON(existing []byte, incoming map[string]string) []byte {
	if len(incoming) == 0 {
		if len(bytesTrimSpace(existing)) == 0 {
			return mustMarshalJSON(map[string]string{})
		}
		return existing
	}
	out := make(map[string]string)
	trimmed := bytesTrimSpace(existing)
	if len(trimmed) > 0 && string(trimmed) != "{}" && string(trimmed) != "null" {
		var current map[string]string
		if err := json.Unmarshal(trimmed, &current); err == nil {
			for key, value := range current {
				if strings.TrimSpace(value) != "" {
					out[key] = value
				}
			}
		}
	}
	for key, value := range incoming {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}
	return mustMarshalJSON(out)
}

func batchLinksCount(raw []byte) int {
	trimmed := bytesTrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "{}" || string(trimmed) == "null" {
		return 0
	}
	var links map[string]string
	if err := json.Unmarshal(trimmed, &links); err != nil {
		return 0
	}
	return len(links)
}

func normalizeCharacters(characters []jikan.AnimeCharacter) []map[string]any {
	out := make([]map[string]any, 0, len(characters))
	for _, character := range characters {
		voiceActors := make([]map[string]any, 0, len(character.VoiceActors))
		for _, actor := range character.VoiceActors {
			voiceActors = append(voiceActors, map[string]any{
				"mal_id":   actor.Person.MALID,
				"name":     actor.Person.Name,
				"language": actor.Language,
			})
		}
		out = append(out, map[string]any{
			"role":                character.Role,
			"character_mal_id":    character.Character.MALID,
			"character_name":      character.Character.Name,
			"character_image_url": character.Character.Images.WebP.ImageURL,
			"voice_actors":        voiceActors,
		})
	}
	return out
}

func mustMarshalJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func collectNames(items []jikan.NamedEntity) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(item.Name); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func mergeUniqueStrings(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, group := range groups {
		for _, item := range group {
			key := strings.TrimSpace(item)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, key)
		}
	}
	return out
}

func imageFromSearch(hit jikan.AnimeSearchHit) string {
	return firstNonEmpty(hit.Images.WebP.LargeImageURL, hit.Images.WebP.ImageURL, hit.Images.JPG.LargeImageURL, hit.Images.JPG.ImageURL)
}

func imageFromFull(full jikan.AnimeFull) string {
	return firstNonEmpty(full.Images.WebP.LargeImageURL, full.Images.WebP.ImageURL, full.Images.JPG.LargeImageURL, full.Images.JPG.ImageURL)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
