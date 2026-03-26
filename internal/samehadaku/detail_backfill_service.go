package samehadaku

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DetailAnimeSlugReader interface {
	ListAnimeSlugs(ctx context.Context, offset, limit int) ([]string, error)
}

type DetailSyncer interface {
	SyncAnimeDetail(ctx context.Context, slug string) (DetailSyncReport, error)
}

type DetailBackfillOptions struct {
	BatchSize    int
	Limit        int
	SkipExisting bool
	DelayBetween time.Duration
	Progress     func(DetailBackfillProgress)
}

type DetailBackfillReport struct {
	Discovered int
	Attempted  int
	Skipped    int
	Succeeded  int
	Failed     int
	Failures   map[string]string
}

type DetailBackfillProgress struct {
	Slug       string
	PageNumber int
	Action     string
	Reason     string
	Counts     DetailBackfillReport
}

type DetailBackfillService struct {
	catalog  CatalogSlugBatchReader
	existing DetailAnimeSlugReader
	syncer   DetailSyncer
	sleep    func(time.Duration)
}

func NewDetailBackfillService(catalog CatalogSlugBatchReader, existing DetailAnimeSlugReader, syncer DetailSyncer, fixedNow time.Time) *DetailBackfillService {
	return &DetailBackfillService{
		catalog:  catalog,
		existing: existing,
		syncer:   syncer,
		sleep:    time.Sleep,
	}
}

func (s *DetailBackfillService) Backfill(ctx context.Context, options DetailBackfillOptions) (DetailBackfillReport, error) {
	if s.catalog == nil {
		return DetailBackfillReport{}, fmt.Errorf("catalog batch reader is required")
	}
	if s.syncer == nil {
		return DetailBackfillReport{}, fmt.Errorf("detail syncer is required")
	}

	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	existing := make(map[string]struct{})
	if options.SkipExisting {
		if s.existing == nil {
			return DetailBackfillReport{}, fmt.Errorf("detail anime slug reader is required when skip existing is enabled")
		}
		for offset := 0; ; offset += batchSize {
			slugs, err := s.existing.ListAnimeSlugs(ctx, offset, batchSize)
			if err != nil {
				return DetailBackfillReport{}, fmt.Errorf("list existing detail slugs: %w", err)
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

	report := DetailBackfillReport{
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
					options.Progress(DetailBackfillProgress{
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
			if _, err := s.syncer.SyncAnimeDetail(ctx, slug); err != nil {
				report.Failed++
				report.Failures[slug] = err.Error()
				if options.Progress != nil {
					options.Progress(DetailBackfillProgress{
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
				options.Progress(DetailBackfillProgress{
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
