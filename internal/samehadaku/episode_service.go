package samehadaku

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	maxResolvedPlayerOptions       = 3
	playerOptionResolveTimeout     = 5 * time.Second
	playerOptionSkippedLimitStatus = "skipped_limit"
)

type EpisodeDetailWriter interface {
	UpsertEpisodeDetails(ctx context.Context, details []EpisodeDetail) (int, error)
}

type EpisodeSyncReport struct {
	AnimeSlug string
	Parsed    int
	Upserted  int
}

type EpisodeService struct {
	fetcher Fetcher
	writer  EpisodeDetailWriter
	now     func() time.Time
}

func NewEpisodeService(fetcher Fetcher, writer EpisodeDetailWriter, fixedNow time.Time) *EpisodeService {
	nowFn := time.Now
	if !fixedNow.IsZero() {
		nowFn = func() time.Time { return fixedNow }
	}
	return &EpisodeService{
		fetcher: fetcher,
		writer:  writer,
		now:     nowFn,
	}
}

func (s *EpisodeService) SyncAnimeEpisodes(ctx context.Context, animeSlug string) (EpisodeSyncReport, error) {
	if s.fetcher == nil {
		return EpisodeSyncReport{}, fmt.Errorf("fetcher is required")
	}
	if s.writer == nil {
		return EpisodeSyncReport{}, fmt.Errorf("episode detail writer is required")
	}
	animeSlug = strings.TrimSpace(animeSlug)
	if animeSlug == "" {
		return EpisodeSyncReport{}, fmt.Errorf("anime slug is required")
	}

	primaryAnimeURL := BuildPrimaryAnimeURL(animeSlug)
	secondaryAnimeURL := BuildMirrorAnimeURL(animeSlug)
	primaryAnimeAttempt, primaryAnimeRaw := fetchSourceAttempt(ctx, s.fetcher, "primary", primaryAnimeURL)
	secondaryAnimeAttempt := SourceAttempt{Kind: "secondary", URL: secondaryAnimeURL, Domain: extractDomain(secondaryAnimeURL), Status: "not_attempted"}

	var animePage MirrorAnimePage
	if primaryAnimeAttempt.Status == "fetched" {
		primaryPage, parseErr := ParsePrimaryAnimeHTML(primaryAnimeRaw, primaryAnimeURL)
		if parseErr == nil {
			animePage = MirrorAnimePage{
				CanonicalURL:  primaryPage.CanonicalURL,
				Title:         primaryPage.Title,
				PosterURL:     primaryPage.PosterURL,
				Synopsis:      primaryPage.Synopsis,
				Genres:        primaryPage.Genres,
				Studios:       primaryPage.Studios,
				Episodes:      primaryPage.Episodes,
				Metadata:      primaryPage.Metadata,
				LatestEpisode: primaryPage.LatestEpisode,
			}
		} else {
			primaryAnimeAttempt.Status = "parse_failed"
			primaryAnimeAttempt.Error = parseErr.Error()
		}
	}

	if len(animePage.Episodes) == 0 {
		var animeRaw []byte
		secondaryAnimeAttempt, animeRaw = fetchSourceAttempt(ctx, s.fetcher, "secondary", secondaryAnimeURL)
		if secondaryAnimeAttempt.Status != "fetched" {
			return EpisodeSyncReport{}, fmt.Errorf("fetch anime page: %s", strings.TrimSpace(buildFetchError(primaryAnimeAttempt, secondaryAnimeAttempt)))
		}
		parsedMirrorPage, err := ParseMirrorAnimeHTML(animeRaw, secondaryAnimeURL)
		if err != nil {
			return EpisodeSyncReport{}, fmt.Errorf("parse mirror anime page: %w", err)
		}
		animePage = parsedMirrorPage
	}
	if len(animePage.Episodes) == 0 {
		return EpisodeSyncReport{}, fmt.Errorf("no episodes found for %s", animeSlug)
	}

	details := make([]EpisodeDetail, 0, len(animePage.Episodes))
	for _, ref := range animePage.Episodes {
		primaryEpisodeURL := ref.CanonicalURL
		if extractDomain(primaryEpisodeURL) != extractDomain(primaryAnimeURL) || strings.TrimSpace(primaryEpisodeURL) == "" {
			primaryEpisodeURL = BuildPrimaryEpisodeURL(ref.EpisodeSlug)
		}
		secondaryEpisodeURL := ""
		if refDomain := extractDomain(ref.CanonicalURL); refDomain != "" && !strings.EqualFold(refDomain, extractDomain(primaryAnimeURL)) {
			secondaryEpisodeURL = ref.CanonicalURL
		}

		primaryAttempt, primaryRaw := fetchSourceAttempt(ctx, s.fetcher, "primary", primaryEpisodeURL)
		secondaryAttempt := SourceAttempt{Kind: "secondary", URL: secondaryEpisodeURL, Domain: extractDomain(secondaryEpisodeURL), Status: "not_attempted"}

		var (
			page                  MirrorEpisodePage
			primaryParsed         bool
			primaryStream         string
			streamPayload         any
			downloadPayload       any
			effectiveSourceKind   string
			effectiveSourceURL    string
			effectiveSourceDomain string
		)

		if primaryAttempt.Status == "fetched" {
			primaryPage, parseErr := ParsePrimaryEpisodeHTML(primaryRaw, primaryEpisodeURL)
			if parseErr == nil {
				primaryParsed = true
				resolvedOptions, resolvedMirrors, primaryStream := s.resolvePrimaryStreamOptions(ctx, primaryEpisodeURL, primaryPage.StreamOptions)
				page = MirrorEpisodePage{
					CanonicalURL:    primaryPage.CanonicalURL,
					Title:           primaryPage.Title,
					EpisodeSlug:     primaryPage.EpisodeSlug,
					EpisodeNumber:   primaryPage.EpisodeNumber,
					PosterURL:       primaryPage.PosterURL,
					AnimeTitle:      firstNonEmpty(primaryPage.AnimeTitle, animePage.Title),
					AnimeURL:        firstNonEmpty(primaryPage.AnimeURL, animePage.CanonicalURL),
					PublishedAt:     primaryPage.PublishedAt,
					PreviousEpisode: primaryPage.PreviousEpisode,
					NextEpisode:     primaryPage.NextEpisode,
					AllEpisodesURL:  primaryPage.AllEpisodesURL,
					SeriesGenres:    primaryPage.SeriesGenres,
					SeriesSynopsis:  primaryPage.SeriesSynopsis,
				}
				streamPayload = map[string]any{
					"primary":          primaryStream,
					"mirrors":          resolvedMirrors,
					"server_options":   primaryPage.StreamOptions,
					"resolved_options": resolvedOptions,
				}
				downloadPayload = primaryPage.DirectDownloads
				effectiveSourceKind = "primary"
				effectiveSourceURL = primaryAttempt.URL
				effectiveSourceDomain = primaryAttempt.Domain
			} else {
				primaryAttempt.Status = "parse_failed"
				primaryAttempt.Error = parseErr.Error()
			}
		}

		if !primaryParsed {
			var raw []byte
			secondaryAttempt, raw = fetchSourceAttempt(ctx, s.fetcher, "secondary", secondaryEpisodeURL)
			if secondaryAttempt.Status != "fetched" {
				return EpisodeSyncReport{}, fmt.Errorf("fetch episode page %s: %s", primaryEpisodeURL, strings.TrimSpace(buildFetchError(primaryAttempt, secondaryAttempt)))
			}
			parsedMirrorPage, err := ParseMirrorEpisodeHTML(raw, secondaryEpisodeURL)
			if err != nil {
				return EpisodeSyncReport{}, fmt.Errorf("parse mirror episode page %s: %w", secondaryEpisodeURL, err)
			}
			page = parsedMirrorPage
			primaryStream = strings.TrimSpace(page.PrimaryStream)
			if primaryStream == "" {
				for _, value := range page.StreamMirrors {
					if trimmed := strings.TrimSpace(value); trimmed != "" {
						primaryStream = trimmed
						break
					}
				}
			}
			streamPayload = map[string]any{
				"primary":          primaryStream,
				"mirrors":          page.StreamMirrors,
				"resolved_options": []PlayerOptionResolution{},
			}
			downloadPayload = page.DirectDownloads
			effectiveSourceKind = "secondary"
			effectiveSourceURL = secondaryAttempt.URL
			effectiveSourceDomain = secondaryAttempt.Domain
		}

		streamJSON, _ := json.Marshal(streamPayload)
		downloadJSON, _ := json.Marshal(downloadPayload)
		fetchStatus := buildOverallFetchStatus(primaryAttempt, secondaryAttempt)
		fetchError := buildFetchError(primaryAttempt, secondaryAttempt)
		sourceMetaJSON, _ := json.Marshal(map[string]any{
			"source":                  "samehadaku",
			"anime_title":             firstNonEmpty(page.AnimeTitle, animePage.Title),
			"anime_url":               firstNonEmpty(page.AnimeURL, animePage.CanonicalURL),
			"published_at":            page.PublishedAt,
			"previous_episode":        page.PreviousEpisode,
			"next_episode":            page.NextEpisode,
			"all_episodes_url":        page.AllEpisodesURL,
			"series_metadata":         page.SeriesMetadata,
			"series_genres":           page.SeriesGenres,
			"series_synopsis":         page.SeriesSynopsis,
			"mirror_poster_url":       firstNonEmpty(page.PosterURL, animePage.PosterURL),
			"mirror_trailer_url":      animePage.TrailerURL,
			"anime_primary_attempt":   primaryAnimeAttempt,
			"anime_secondary_attempt": secondaryAnimeAttempt,
			"primary_attempt":         primaryAttempt,
			"secondary_attempt":       secondaryAttempt,
			"effective_source_kind":   effectiveSourceKind,
			"effective_source_url":    effectiveSourceURL,
			"effective_source_host":   effectiveSourceDomain,
			"parser_source_kind":      effectiveSourceKind,
			"fetch_status":            fetchStatus,
			"fetch_error":             fetchError,
		})

		details = append(details, EpisodeDetail{
			AnimeSlug:             animeSlug,
			EpisodeSlug:           firstNonEmpty(page.EpisodeSlug, ref.EpisodeSlug),
			CanonicalURL:          primaryEpisodeURL,
			PrimarySourceURL:      primaryEpisodeURL,
			PrimarySourceDomain:   extractDomain(primaryEpisodeURL),
			SecondarySourceURL:    secondaryEpisodeURL,
			SecondarySourceDomain: extractDomain(secondaryEpisodeURL),
			EffectiveSourceURL:    effectiveSourceURL,
			EffectiveSourceDomain: effectiveSourceDomain,
			EffectiveSourceKind:   effectiveSourceKind,
			Title:                 firstNonEmpty(page.Title, ref.Title),
			EpisodeNumber:         firstNonZero(page.EpisodeNumber, ref.EpisodeNumber),
			ReleaseLabel:          firstNonEmpty(ref.ReleaseDate, page.PublishedAt),
			StreamLinksJSON:       streamJSON,
			DownloadLinksJSON:     downloadJSON,
			SourceMetaJSON:        sourceMetaJSON,
			FetchStatus:           fetchStatus,
			FetchError:            fetchError,
			ScrapedAt:             s.now().UTC(),
		})
	}

	upserted, err := s.writer.UpsertEpisodeDetails(ctx, details)
	if err != nil {
		return EpisodeSyncReport{}, err
	}
	return EpisodeSyncReport{
		AnimeSlug: animeSlug,
		Parsed:    len(details),
		Upserted:  upserted,
	}, nil
}

func firstNonZero(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func (s *EpisodeService) resolvePrimaryStreamOptions(ctx context.Context, refererURL string, options []PrimaryServerOption) ([]PlayerOptionResolution, map[string]string, string) {
	resolvedOptions := make([]PlayerOptionResolution, 0, len(options))
	resolvedMirrors := make(map[string]string)
	primaryStream := ""

	resolver, ok := s.fetcher.(PlayerOptionResolver)
	if !ok {
		return resolvedOptions, resolvedMirrors, primaryStream
	}

	successful := 0
	for index, option := range options {
		if successful >= maxResolvedPlayerOptions {
			resolvedOptions = append(resolvedOptions, PlayerOptionResolution{
				Label:      strings.TrimSpace(option.Label),
				PostID:     strings.TrimSpace(option.PostID),
				Number:     strings.TrimSpace(option.Number),
				Type:       strings.TrimSpace(option.Type),
				Status:     playerOptionSkippedLimitStatus,
				SourceKind: "primary",
				ResolvedAt: s.now().UTC(),
			})
			continue
		}

		resolveCtx, cancel := context.WithTimeout(ctx, playerOptionResolveTimeout)
		resolution, err := resolver.ResolvePlayerOption(resolveCtx, refererURL, option)
		cancel()
		if err != nil {
			resolution = PlayerOptionResolution{
				Label:      strings.TrimSpace(option.Label),
				PostID:     strings.TrimSpace(option.PostID),
				Number:     strings.TrimSpace(option.Number),
				Type:       strings.TrimSpace(option.Type),
				Status:     classifyPlayerResolutionError(err),
				SourceKind: "primary",
				ResolvedAt: s.now().UTC(),
				Error:      err.Error(),
			}
			resolvedOptions = append(resolvedOptions, resolution)
			continue
		}

		resolvedOptions = append(resolvedOptions, resolution)
		embedURL := strings.TrimSpace(resolution.EmbedURL)
		if embedURL == "" {
			continue
		}
		label := firstNonEmpty(resolution.Label, strings.TrimSpace(option.Label), fmt.Sprintf("server_%d", index+1))
		resolvedMirrors[label] = embedURL
		if primaryStream == "" {
			primaryStream = embedURL
		}
		successful++
	}

	return resolvedOptions, resolvedMirrors, primaryStream
}

func classifyPlayerResolutionError(err error) string {
	if err == nil {
		return "resolved"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "resolve_timeout"
	}
	return "resolve_failed"
}
