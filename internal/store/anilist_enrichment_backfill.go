package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/anilist"
)

type anilistEnrichmentClient interface {
	SearchMedia(ctx context.Context, query string, mediaType anilist.MediaType, limit int) ([]anilist.SearchHit, error)
}

type anilistCandidateReader interface {
	ListAniListEnrichmentCandidates(ctx context.Context, offset, limit int, options AniListEnrichmentCandidateOptions) ([]AniListEnrichmentCandidate, error)
}

type anilistPromotionWriter interface {
	ApplyPromotion(ctx context.Context, input AniListPromotionInput) error
}

type AniListEnrichmentBackfillOptions struct {
	Scope        AniListEnrichmentScope
	BatchSize    int
	Limit        int
	SkipExisting bool
	DelayBetween time.Duration
}

type AniListEnrichmentBackfillReport struct {
	Discovered int
	Attempted  int
	Skipped    int
	Succeeded  int
	Failed     int
}

type AniListEnrichmentBackfillService struct {
	reader    anilistCandidateReader
	writer    tmdbEnrichmentWriter
	promotion anilistPromotionWriter
	client    anilistEnrichmentClient
	sleep     func(time.Duration)
}

func NewAniListEnrichmentBackfillService(
	reader anilistCandidateReader,
	writer tmdbEnrichmentWriter,
	promotion anilistPromotionWriter,
	client anilistEnrichmentClient,
) *AniListEnrichmentBackfillService {
	return &AniListEnrichmentBackfillService{
		reader:    reader,
		writer:    writer,
		promotion: promotion,
		client:    client,
		sleep:     time.Sleep,
	}
}

func (s *AniListEnrichmentBackfillService) Backfill(ctx context.Context, options AniListEnrichmentBackfillOptions) (AniListEnrichmentBackfillReport, error) {
	if s.reader == nil || s.writer == nil || s.promotion == nil || s.client == nil {
		return AniListEnrichmentBackfillReport{}, fmt.Errorf("anilist backfill dependencies are required")
	}
	scope := options.Scope
	if scope == "" {
		scope = AniListEnrichmentScopeAll
	}
	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	report := AniListEnrichmentBackfillReport{}
	attemptedKeys := make(map[string]struct{})
	processed := 0

	for offset := 0; ; {
		batchOffset := offset
		if options.SkipExisting {
			batchOffset = 0
		}
		candidates, err := s.reader.ListAniListEnrichmentCandidates(ctx, batchOffset, batchSize, AniListEnrichmentCandidateOptions{
			Scope:        scope,
			SkipExisting: options.SkipExisting,
		})
		if err != nil {
			return report, err
		}
		if len(candidates) == 0 {
			break
		}

		fresh := make([]AniListEnrichmentCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			if _, ok := attemptedKeys[candidate.ItemKey]; ok {
				report.Skipped++
				continue
			}
			fresh = append(fresh, candidate)
		}
		if len(fresh) == 0 {
			break
		}

		for _, candidate := range fresh {
			if options.Limit > 0 && processed >= options.Limit {
				return report, nil
			}
			attemptedKeys[candidate.ItemKey] = struct{}{}
			processed++
			report.Discovered++
			report.Attempted++
			if err := s.enrichCandidate(ctx, candidate); err != nil {
				if anilist.IsServiceUnavailable(err) {
					return report, err
				}
				report.Failed++
				continue
			}
			report.Succeeded++
			if options.DelayBetween > 0 && s.sleep != nil {
				s.sleep(options.DelayBetween)
			}
		}
		if len(fresh) < batchSize {
			break
		}
		if !options.SkipExisting {
			offset += batchSize
		}
	}

	return report, nil
}

func (s *AniListEnrichmentBackfillService) enrichCandidate(ctx context.Context, candidate AniListEnrichmentCandidate) error {
	queries := buildAniListQueries(candidate)
	if len(queries) == 0 {
		return s.writer.UpsertItemEnrichment(ctx, MediaItemEnrichmentRecord{
			ItemKey:      candidate.ItemKey,
			Provider:     "anilist",
			MatchStatus:  "missing_query",
			MatchedTitle: firstPresent(candidate.Title, candidate.Slug),
			Payload: map[string]any{
				"kind": candidate.SurfaceType,
			},
		})
	}

	mediaType := determineAniListMediaType(candidate)
	bestScore := -1
	bestReason := anilist.MatchReasonSearchEmpty
	var bestHit anilist.SearchHit
	for _, query := range queries {
		results, err := s.client.SearchMedia(ctx, query, mediaType, 5)
		if err != nil {
			return err
		}
		result := anilist.PickBestMediaMatchResult(query, candidate.ReleaseYear, results)
		if result.BestScore > bestScore {
			bestScore = result.BestScore
			bestReason = result.Reason
			bestHit = result.Hit
		}
		if result.Matched {
			break
		}
	}

	payload := compactPayload(map[string]any{
		"type":              string(mediaType),
		"format":            strings.TrimSpace(bestHit.Format),
		"status":            strings.TrimSpace(bestHit.Status),
		"country_of_origin": strings.TrimSpace(bestHit.CountryOfOrigin),
		"season_year":       bestHit.SeasonYear,
		"average_score":     bestHit.AverageScore,
		"is_adult":          bestHit.IsAdult,
		"genres":            compactStringSlice(bestHit.Genres),
		"title_english":     strings.TrimSpace(bestHit.Title.English),
		"title_romaji":      strings.TrimSpace(bestHit.Title.Romaji),
		"title_native":      strings.TrimSpace(bestHit.Title.Native),
		"synonyms":          compactStringSlice(bestHit.Synonyms),
		"description":       strings.TrimSpace(bestHit.Description),
		"cover_image":       firstPresent(bestHit.CoverImage.ExtraLarge, bestHit.CoverImage.Large, bestHit.CoverImage.Medium),
		"banner_image":      strings.TrimSpace(bestHit.BannerImage),
		"site_url":          strings.TrimSpace(bestHit.SiteURL),
	})

	record := MediaItemEnrichmentRecord{
		ItemKey:      candidate.ItemKey,
		Provider:     "anilist",
		ExternalID:   fmt.Sprintf("%d", bestHit.ID),
		MatchStatus:  string(bestReason),
		MatchScore:   clampTMDBMatchScore(bestScore),
		MatchedTitle: firstPresent(bestHit.Title.English, bestHit.Title.Romaji, bestHit.Title.Native, candidate.Title, candidate.Slug),
		MatchedYear:  bestHit.SeasonYear,
		Payload:      payload,
	}
	if bestScore <= 0 {
		return s.writer.UpsertItemEnrichment(ctx, record)
	}
	record.MatchStatus = "matched"
	if err := s.writer.UpsertItemEnrichment(ctx, record); err != nil {
		return err
	}
	return s.promotion.ApplyPromotion(ctx, AniListPromotionInput{
		ItemKey:               candidate.ItemKey,
		Source:                candidate.Source,
		MediaType:             candidate.MediaType,
		SurfaceType:           candidate.SurfaceType,
		PresentationType:      candidate.PresentationType,
		OriginType:            candidate.OriginType,
		ReleaseCountry:        candidate.ReleaseCountry,
		GenreNames:            collectTaxonomyGenreNames(candidate.Detail),
		TaxonomyConfidence:    candidate.TaxonomyConfidence,
		CurrentTaxonomySource: candidate.TaxonomySource,
		CurrentDetail:         candidate.Detail,
		CurrentCoverURL:       "",
		AniListMatchScore:     record.MatchScore,
		AniListPayload:        payload,
	})
}

func determineAniListMediaType(candidate AniListEnrichmentCandidate) anilist.MediaType {
	if candidate.SurfaceType == "comic" || candidate.MediaType == "manga" || candidate.MediaType == "manhwa" || candidate.MediaType == "manhua" {
		return anilist.MediaTypeManga
	}
	return anilist.MediaTypeAnime
}

func buildAniListQueries(candidate AniListEnrichmentCandidate) []string {
	queries := make([]string, 0, 8)
	seen := make(map[string]struct{})
	add := func(values ...string) {
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			queries = append(queries, trimmed)
		}
	}

	detail := candidate.Detail
	jikanMeta := readAnyMap(detail["jikan_meta_json"])
	jikanSearchHit := readAnyMap(jikanMeta["search_hit"])
	add(readAnyString(jikanSearchHit["title_english"]))
	add(readAnyString(jikanSearchHit["title"]))
	add(readAnyString(detail["source_title"]))
	add(readAnyString(detail["alt_title"]))
	add(readAnyString(detail["native_title"]))
	add(strings.TrimSpace(candidate.Title))
	for _, entry := range readAnySlice(jikanSearchHit["title_synonyms"]) {
		add(readAnyString(entry))
	}
	return queries
}
