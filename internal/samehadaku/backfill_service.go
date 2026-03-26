package samehadaku

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type CatalogSlugBatchReader interface {
	ListCatalogSlugs(ctx context.Context, offset, limit int) ([]CatalogItem, error)
}

type EpisodeAnimeSlugReader interface {
	ListAnimeSlugs(ctx context.Context, offset, limit int) ([]string, error)
}

type EpisodeSyncer interface {
	SyncAnimeEpisodes(ctx context.Context, animeSlug string) (EpisodeSyncReport, error)
}

type EpisodeBackfillOptions struct {
	BatchSize    int
	Limit        int
	SkipExisting bool
	DelayBetween time.Duration
	Progress     func(EpisodeBackfillProgress)
}

type EpisodeBackfillReport struct {
	Discovered int
	Attempted  int
	Skipped    int
	Succeeded  int
	Failed     int
	Failures   map[string]string
}

type EpisodeBackfillProgress struct {
	Slug       string
	PageNumber int
	Action     string
	Reason     string
	Counts     EpisodeBackfillReport
}

type EpisodeBackfillService struct {
	catalog  CatalogSlugBatchReader
	existing EpisodeAnimeSlugReader
	syncer   EpisodeSyncer
	sleep    func(time.Duration)
}

func NewEpisodeBackfillService(catalog CatalogSlugBatchReader, existing EpisodeAnimeSlugReader, syncer EpisodeSyncer, fixedNow time.Time) *EpisodeBackfillService {
	return &EpisodeBackfillService{
		catalog:  catalog,
		existing: existing,
		syncer:   syncer,
		sleep:    time.Sleep,
	}
}

func (s *EpisodeBackfillService) Backfill(ctx context.Context, options EpisodeBackfillOptions) (EpisodeBackfillReport, error) {
	if s.catalog == nil {
		return EpisodeBackfillReport{}, fmt.Errorf("catalog batch reader is required")
	}
	if s.syncer == nil {
		return EpisodeBackfillReport{}, fmt.Errorf("episode syncer is required")
	}

	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	existing := make(map[string]struct{})
	if options.SkipExisting {
		if s.existing == nil {
			return EpisodeBackfillReport{}, fmt.Errorf("episode anime slug reader is required when skip existing is enabled")
		}
		for offset := 0; ; offset += batchSize {
			slugs, err := s.existing.ListAnimeSlugs(ctx, offset, batchSize)
			if err != nil {
				return EpisodeBackfillReport{}, fmt.Errorf("list existing anime slugs: %w", err)
			}
			if len(slugs) == 0 {
				break
			}
			for _, slug := range slugs {
				if trimmed := strings.TrimSpace(slug); trimmed != "" {
					existing[trimmed] = struct{}{}
				}
			}
			if len(slugs) < batchSize {
				break
			}
		}
	}

	report := EpisodeBackfillReport{
		Failures: make(map[string]string),
	}
	seen := make(map[string]struct{})
	processed := 0

	for offset := 0; ; offset += batchSize {
		items, err := s.catalog.ListCatalogSlugs(ctx, offset, batchSize)
		if err != nil {
			return report, fmt.Errorf("list catalog slugs: %w", err)
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			slug := strings.TrimSpace(item.Slug)
			if slug == "" {
				continue
			}
			if _, ok := seen[slug]; ok {
				continue
			}
			seen[slug] = struct{}{}
			report.Discovered++

			if options.Limit > 0 && processed >= options.Limit {
				return report, nil
			}
			processed++

			if _, ok := existing[slug]; ok {
				report.Skipped++
				if options.Progress != nil {
					options.Progress(EpisodeBackfillProgress{
						Slug:       slug,
						PageNumber: item.PageNumber,
						Action:     "skip",
						Reason:     "already_backfilled",
						Counts:     report,
					})
				}
				continue
			}

			report.Attempted++
			if _, err := s.syncer.SyncAnimeEpisodes(ctx, slug); err != nil {
				report.Failed++
				report.Failures[slug] = err.Error()
				if options.Progress != nil {
					options.Progress(EpisodeBackfillProgress{
						Slug:       slug,
						PageNumber: item.PageNumber,
						Action:     "fail",
						Reason:     err.Error(),
						Counts:     report,
					})
				}
				continue
			}
			report.Succeeded++
			existing[slug] = struct{}{}
			if options.Progress != nil {
				options.Progress(EpisodeBackfillProgress{
					Slug:       slug,
					PageNumber: item.PageNumber,
					Action:     "success",
					Counts:     report,
				})
			}

			if options.DelayBetween > 0 && s.sleep != nil {
				s.sleep(options.DelayBetween)
			}
		}

		if len(items) < batchSize {
			break
		}
	}

	return report, nil
}
