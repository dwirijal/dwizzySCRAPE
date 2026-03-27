package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dwirijal/dwizzySCRAPE/internal/config"
	"github.com/dwirijal/dwizzySCRAPE/internal/jikan"
	"github.com/dwirijal/dwizzySCRAPE/internal/kanata"
	"github.com/dwirijal/dwizzySCRAPE/internal/komiku"
	"github.com/dwirijal/dwizzySCRAPE/internal/manhwaindo"
	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
	"github.com/dwirijal/dwizzySCRAPE/internal/snapshot"
	"github.com/dwirijal/dwizzySCRAPE/internal/store"
	"github.com/dwirijal/dwizzySCRAPE/internal/tmdb"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: dwizzyscrape <migrate|backfill|sync|detail-anime <slug>|backfill-anime-details|detail-episodes <slug>|backfill-episodes|manhwa-catalog [page]|manhwa-series <slug>|manhwa-chapter <slug>|sync-manhwa-catalog [page]|sync-manhwa-series <slug>|sync-manhwa-chapter <slug>|backfill-manhwa-series <startPage> [endPage]|backfill-manhwa-chapters <startPage> [endPage] [latestPerSeries]|komiku-catalog [page]|komiku-series <slug>|komiku-chapter <slug>|sync-komiku-catalog [page]|sync-komiku-series <slug>|sync-komiku-chapter <slug>|backfill-komiku-series <startPage> [endPage]|backfill-komiku-chapters <startPage> [endPage] [latestPerSeries]|refresh-anime-v2|refresh-media-v2|refresh-movie-v3|sync-movie-v3-kanata-home [limit]|sync-movie-v3-kanata-genre <genre> [page] [limit]|sync-movie-v3-kanata-search <query> [page] [limit]|snapshot-build [outputDir]|snapshot-patch <domain> <slug> [outputDir]|snapshot-webhook [build|patch] [domain] [slug] [outputDir]|probe-url <url>>")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	switch args[0] {
	case "migrate":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
		log.Printf("migration replay done")
		return nil
	case "backfill", "sync":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		fetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
		catalogStore := store.NewCatalogStoreWithDB(db)
		service := samehadaku.NewService(fetcher, catalogStore, time.Time{})

		report, err := service.SyncCatalog(ctx, cfg.CatalogURL)
		if err != nil {
			return err
		}
		log.Printf("samehadaku catalog sync done: parsed=%d upserted=%d", report.Parsed, report.Upserted)
		return nil
	case "detail-anime":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape detail-anime <slug>")
		}
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		fetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
		catalogStore := store.NewCatalogStoreWithDB(db)
		detailStore := store.NewAnimeDetailStoreWithDB(db)
		jikanClient := jikan.NewClient(cfg.JikanBaseURL, httpClient)
		service := samehadaku.NewDetailService(catalogStore, detailStore, jikanClient, fetcher, time.Time{})

		report, err := service.SyncAnimeDetail(ctx, args[1])
		if err != nil {
			return err
		}
		log.Printf("samehadaku anime detail sync done: slug=%s mal_id=%d source_fetch_status=%s", report.Slug, report.MALID, report.SourceFetchStatus)
		return nil
	case "backfill-anime-details":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		fetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
		catalogStore := store.NewCatalogStoreWithDB(db)
		detailStore := store.NewAnimeDetailStoreWithDB(db)
		jikanClient := jikan.NewClient(cfg.JikanBaseURL, httpClient)
		detailService := samehadaku.NewDetailService(catalogStore, detailStore, jikanClient, fetcher, time.Time{})
		backfillService := samehadaku.NewDetailBackfillService(catalogStore, detailStore, detailService, time.Time{})

		report, err := backfillService.Backfill(ctx, samehadaku.DetailBackfillOptions{
			BatchSize:    100,
			SkipExisting: true,
			DelayBetween: 1250 * time.Millisecond,
			Progress: func(progress samehadaku.DetailBackfillProgress) {
				switch progress.Action {
				case "skip":
					log.Printf(
						"samehadaku anime detail backfill skip: slug=%s page=%d skipped=%d discovered=%d",
						progress.Slug,
						progress.PageNumber,
						progress.Counts.Skipped,
						progress.Counts.Discovered,
					)
				case "success":
					log.Printf(
						"samehadaku anime detail backfill success: slug=%s page=%d attempted=%d succeeded=%d failed=%d",
						progress.Slug,
						progress.PageNumber,
						progress.Counts.Attempted,
						progress.Counts.Succeeded,
						progress.Counts.Failed,
					)
				case "fail":
					log.Printf(
						"samehadaku anime detail backfill fail: slug=%s page=%d attempted=%d succeeded=%d failed=%d error=%s",
						progress.Slug,
						progress.PageNumber,
						progress.Counts.Attempted,
						progress.Counts.Succeeded,
						progress.Counts.Failed,
						progress.Reason,
					)
				}
			},
		})
		if err != nil {
			return err
		}
		log.Printf(
			"samehadaku anime detail backfill done: discovered=%d attempted=%d skipped=%d succeeded=%d failed=%d",
			report.Discovered,
			report.Attempted,
			report.Skipped,
			report.Succeeded,
			report.Failed,
		)
		if len(report.Failures) > 0 {
			for slug, reason := range report.Failures {
				log.Printf("samehadaku anime detail backfill failed: slug=%s error=%s", slug, reason)
			}
		}
		return nil
	case "detail-episodes":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape detail-episodes <slug>")
		}
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		fetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
		episodeStore := store.NewEpisodeDetailStoreWithDB(db)
		service := samehadaku.NewEpisodeService(fetcher, episodeStore, time.Time{})

		report, err := service.SyncAnimeEpisodes(ctx, args[1])
		if err != nil {
			return err
		}
		log.Printf("samehadaku episode detail sync done: anime_slug=%s parsed=%d upserted=%d", report.AnimeSlug, report.Parsed, report.Upserted)
		return nil
	case "backfill-episodes":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		fetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
		catalogStore := store.NewCatalogStoreWithDB(db)
		episodeStore := store.NewEpisodeDetailStoreWithDB(db)
		episodeService := samehadaku.NewEpisodeService(fetcher, episodeStore, time.Time{})
		backfillService := samehadaku.NewEpisodeBackfillService(catalogStore, episodeStore, episodeService, time.Time{})

		report, err := backfillService.Backfill(ctx, samehadaku.EpisodeBackfillOptions{
			BatchSize:    100,
			SkipExisting: true,
			DelayBetween: 250 * time.Millisecond,
			Progress: func(progress samehadaku.EpisodeBackfillProgress) {
				switch progress.Action {
				case "skip":
					log.Printf(
						"samehadaku episode backfill skip: slug=%s page=%d skipped=%d discovered=%d",
						progress.Slug,
						progress.PageNumber,
						progress.Counts.Skipped,
						progress.Counts.Discovered,
					)
				case "success":
					log.Printf(
						"samehadaku episode backfill success: slug=%s page=%d attempted=%d succeeded=%d failed=%d",
						progress.Slug,
						progress.PageNumber,
						progress.Counts.Attempted,
						progress.Counts.Succeeded,
						progress.Counts.Failed,
					)
				case "fail":
					log.Printf(
						"samehadaku episode backfill fail: slug=%s page=%d attempted=%d succeeded=%d failed=%d error=%s",
						progress.Slug,
						progress.PageNumber,
						progress.Counts.Attempted,
						progress.Counts.Succeeded,
						progress.Counts.Failed,
						progress.Reason,
					)
				}
			},
		})
		if err != nil {
			return err
		}
		log.Printf(
			"samehadaku episode backfill done: discovered=%d attempted=%d skipped=%d succeeded=%d failed=%d",
			report.Discovered,
			report.Attempted,
			report.Skipped,
			report.Succeeded,
			report.Failed,
		)
		if len(report.Failures) > 0 {
			for slug, reason := range report.Failures {
				log.Printf("samehadaku episode backfill failed: slug=%s error=%s", slug, reason)
			}
		}
		return nil
	case "probe-url":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape probe-url <url>")
		}
		fetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
		result, err := fetcher.FetchPageRaw(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("status=%d url=%s\n", result.StatusCode, result.URL)
		fmt.Print(string(result.Body))
		return nil
	case "snapshot-build":
		outputDir := cfg.SnapshotOutputDir
		if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
			outputDir = strings.TrimSpace(args[1])
		}
		manifest, err := buildSnapshots(ctx, cfg, httpClient, outputDir)
		if err != nil {
			return err
		}
		log.Printf("snapshot build done: output=%s entries=%d", outputDir, len(manifest.Entries))
		return writeJSON(os.Stdout, manifest)
	case "snapshot-patch":
		if len(args) < 3 || strings.TrimSpace(args[1]) == "" || strings.TrimSpace(args[2]) == "" {
			return fmt.Errorf("usage: dwizzyscrape snapshot-patch <domain> <slug> [outputDir]")
		}
		outputDir := cfg.SnapshotOutputDir
		if len(args) >= 4 && strings.TrimSpace(args[3]) != "" {
			outputDir = strings.TrimSpace(args[3])
		}
		manifest, err := patchSnapshots(ctx, cfg, httpClient, strings.TrimSpace(args[1]), strings.TrimSpace(args[2]), outputDir)
		if err != nil {
			return err
		}
		log.Printf("snapshot patch done: domain=%s slug=%s output=%s entries=%d", args[1], args[2], outputDir, len(manifest.Entries))
		return writeJSON(os.Stdout, manifest)
	case "snapshot-webhook":
		action := firstNonEmptyArgOrEnv(args, 1, "SNAPSHOT_ACTION", "build")
		outputDir := firstNonEmptyArgOrEnv(args, 4, "SNAPSHOT_OUTPUT_DIR", cfg.SnapshotOutputDir)
		switch action {
		case "build", "full":
			manifest, err := buildSnapshots(ctx, cfg, httpClient, outputDir)
			if err != nil {
				return err
			}
			log.Printf("snapshot webhook build done: output=%s entries=%d", outputDir, len(manifest.Entries))
			return writeJSON(os.Stdout, manifest)
		case "patch":
			domain := firstNonEmptyArgOrEnv(args, 2, "SNAPSHOT_DOMAIN", "")
			slug := firstNonEmptyArgOrEnv(args, 3, "SNAPSHOT_SLUG", "")
			if strings.TrimSpace(domain) == "" || strings.TrimSpace(slug) == "" {
				return fmt.Errorf("usage: dwizzyscrape snapshot-webhook patch <domain> <slug> [outputDir]")
			}
			manifest, err := patchSnapshots(ctx, cfg, httpClient, domain, slug, outputDir)
			if err != nil {
				return err
			}
			log.Printf("snapshot webhook patch done: domain=%s slug=%s output=%s entries=%d", domain, slug, outputDir, len(manifest.Entries))
			return writeJSON(os.Stdout, manifest)
		default:
			return fmt.Errorf("unknown snapshot-webhook action %q", action)
		}
	case "manhwa-catalog":
		page := 1
		if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
			parsed, err := strconv.Atoi(strings.TrimSpace(args[1]))
			if err != nil {
				return fmt.Errorf("parse page: %w", err)
			}
			if parsed > 0 {
				page = parsed
			}
		}

		client := manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout)
		service := manhwaindo.NewService(client, cfg.ManhwaindoBaseURL)
		items, err := service.FetchCatalog(ctx, page)
		if err != nil {
			return err
		}
		return writeJSON(os.Stdout, items)
	case "manhwa-series":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape manhwa-series <slug>")
		}

		client := manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout)
		service := manhwaindo.NewService(client, cfg.ManhwaindoBaseURL)
		series, err := service.FetchSeries(ctx, args[1])
		if err != nil {
			return err
		}
		return writeJSON(os.Stdout, series)
	case "manhwa-chapter":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape manhwa-chapter <slug>")
		}

		client := manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout)
		service := manhwaindo.NewService(client, cfg.ManhwaindoBaseURL)
		chapter, err := service.FetchChapter(ctx, args[1])
		if err != nil {
			return err
		}
		return writeJSON(os.Stdout, chapter)
	case "sync-manhwa-catalog":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		page := 1
		if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
			parsed, err := strconv.Atoi(strings.TrimSpace(args[1]))
			if err != nil {
				return fmt.Errorf("parse page: %w", err)
			}
			if parsed > 0 {
				page = parsed
			}
		}

		client := manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout)
		service := manhwaindo.NewService(client, cfg.ManhwaindoBaseURL)
		items, err := service.FetchCatalog(ctx, page)
		if err != nil {
			return err
		}
		contentStore := store.NewContentStore(db)
		for _, item := range items {
			if err := contentStore.UpsertManhwaSeries(ctx, item); err != nil {
				return err
			}
		}
		log.Printf("manhwa catalog sync done: page=%d upserted=%d", page, len(items))
		return nil
	case "sync-manhwa-series":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape sync-manhwa-series <slug>")
		}
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client := manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout)
		service := manhwaindo.NewService(client, cfg.ManhwaindoBaseURL)
		series, err := service.FetchSeries(ctx, args[1])
		if err != nil {
			return err
		}
		contentStore := store.NewContentStore(db)
		if err := contentStore.UpsertManhwaSeries(ctx, series); err != nil {
			return err
		}
		log.Printf("manhwa series sync done: slug=%s chapters=%d", series.Slug, len(series.Chapters))
		return nil
	case "sync-manhwa-chapter":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape sync-manhwa-chapter <slug>")
		}
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client := manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout)
		service := manhwaindo.NewService(client, cfg.ManhwaindoBaseURL)
		chapter, err := service.FetchChapter(ctx, args[1])
		if err != nil {
			return err
		}
		contentStore := store.NewContentStore(db)
		if err := contentStore.UpsertManhwaChapter(ctx, chapter); err != nil {
			return err
		}
		log.Printf("manhwa chapter sync done: slug=%s pages=%d", chapter.Slug, len(chapter.Pages))
		return nil
	case "backfill-manhwa-series":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape backfill-manhwa-series <startPage> [endPage]")
		}
		startPage, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil {
			return fmt.Errorf("parse start page: %w", err)
		}
		endPage := startPage
		if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
			endPage, err = strconv.Atoi(strings.TrimSpace(args[2]))
			if err != nil {
				return fmt.Errorf("parse end page: %w", err)
			}
		}

		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client := manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout)
		service := manhwaindo.NewService(client, cfg.ManhwaindoBaseURL)
		contentStore := store.NewContentStore(db)

		report, err := backfillManhwaSeriesPages(ctx, service, contentStore, startPage, endPage)
		if err != nil {
			return err
		}
		log.Printf(
			"manhwa series backfill done: start_page=%d end_page=%d discovered=%d succeeded=%d failed=%d",
			report.StartPage,
			report.EndPage,
			report.Discovered,
			report.Succeeded,
			report.Failed,
		)
		if len(report.Failures) > 0 {
			for slug, reason := range report.Failures {
				log.Printf("manhwa series backfill failed: slug=%s error=%s", slug, reason)
			}
		}
		return nil
	case "backfill-manhwa-chapters":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape backfill-manhwa-chapters <startPage> [endPage] [latestPerSeries]")
		}
		startPage, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil {
			return fmt.Errorf("parse start page: %w", err)
		}
		endPage := startPage
		if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
			endPage, err = strconv.Atoi(strings.TrimSpace(args[2]))
			if err != nil {
				return fmt.Errorf("parse end page: %w", err)
			}
		}
		latestPerSeries := 3
		if len(args) >= 4 && strings.TrimSpace(args[3]) != "" {
			latestPerSeries, err = strconv.Atoi(strings.TrimSpace(args[3]))
			if err != nil {
				return fmt.Errorf("parse latest per series: %w", err)
			}
		}

		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client := manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout)
		service := manhwaindo.NewService(client, cfg.ManhwaindoBaseURL)
		contentStore := store.NewContentStore(db)

		report, err := backfillManhwaChapterPages(ctx, service, contentStore, contentStore, startPage, endPage, latestPerSeries)
		if err != nil {
			return err
		}
		log.Printf(
			"manhwa chapter backfill done: start_page=%d end_page=%d latest_per_series=%d series=%d attempted=%d succeeded=%d failed=%d",
			report.StartPage,
			report.EndPage,
			report.MaxChaptersPerSlug,
			report.DiscoveredSeries,
			report.AttemptedChapters,
			report.SucceededChapters,
			report.FailedChapters,
		)
		if len(report.Failures) > 0 {
			for slug, reason := range report.Failures {
				log.Printf("manhwa chapter backfill failed: slug=%s error=%s", slug, reason)
			}
		}
		return nil
	case "komiku-catalog":
		page := 1
		if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
			parsed, err := strconv.Atoi(strings.TrimSpace(args[1]))
			if err != nil {
				return fmt.Errorf("parse page: %w", err)
			}
			if parsed > 0 {
				page = parsed
			}
		}

		client := komiku.NewClient(cfg.KomikuBaseURL, cfg.KomikuUserAgent, cfg.KomikuCookie, cfg.HTTPTimeout)
		service := komiku.NewService(client, cfg.KomikuBaseURL)
		items, err := service.FetchCatalog(ctx, page)
		if err != nil {
			return err
		}
		return writeJSON(os.Stdout, items)
	case "komiku-series":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape komiku-series <slug>")
		}

		client := komiku.NewClient(cfg.KomikuBaseURL, cfg.KomikuUserAgent, cfg.KomikuCookie, cfg.HTTPTimeout)
		service := komiku.NewService(client, cfg.KomikuBaseURL)
		series, err := service.FetchSeries(ctx, args[1])
		if err != nil {
			return err
		}
		return writeJSON(os.Stdout, series)
	case "komiku-chapter":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape komiku-chapter <slug>")
		}

		client := komiku.NewClient(cfg.KomikuBaseURL, cfg.KomikuUserAgent, cfg.KomikuCookie, cfg.HTTPTimeout)
		service := komiku.NewService(client, cfg.KomikuBaseURL)
		chapter, err := service.FetchChapter(ctx, args[1])
		if err != nil {
			return err
		}
		return writeJSON(os.Stdout, chapter)
	case "sync-komiku-catalog":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		page := 1
		if len(args) >= 2 && strings.TrimSpace(args[1]) != "" {
			parsed, err := strconv.Atoi(strings.TrimSpace(args[1]))
			if err != nil {
				return fmt.Errorf("parse page: %w", err)
			}
			if parsed > 0 {
				page = parsed
			}
		}

		client := komiku.NewClient(cfg.KomikuBaseURL, cfg.KomikuUserAgent, cfg.KomikuCookie, cfg.HTTPTimeout)
		service := komiku.NewService(client, cfg.KomikuBaseURL)
		items, err := service.FetchCatalog(ctx, page)
		if err != nil {
			return err
		}
		contentStore := store.NewContentStore(db)
		for _, item := range items {
			if err := contentStore.UpsertManhwaSeries(ctx, item); err != nil {
				return err
			}
		}
		log.Printf("komiku catalog sync done: page=%d upserted=%d", page, len(items))
		return nil
	case "sync-komiku-series":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape sync-komiku-series <slug>")
		}
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client := komiku.NewClient(cfg.KomikuBaseURL, cfg.KomikuUserAgent, cfg.KomikuCookie, cfg.HTTPTimeout)
		service := komiku.NewService(client, cfg.KomikuBaseURL)
		series, err := service.FetchSeries(ctx, args[1])
		if err != nil {
			return err
		}
		contentStore := store.NewContentStore(db)
		if err := contentStore.UpsertManhwaSeries(ctx, series); err != nil {
			return err
		}
		log.Printf("komiku series sync done: slug=%s chapters=%d", series.Slug, len(series.Chapters))
		return nil
	case "sync-komiku-chapter":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape sync-komiku-chapter <slug>")
		}
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client := komiku.NewClient(cfg.KomikuBaseURL, cfg.KomikuUserAgent, cfg.KomikuCookie, cfg.HTTPTimeout)
		service := komiku.NewService(client, cfg.KomikuBaseURL)
		chapter, err := service.FetchChapter(ctx, args[1])
		if err != nil {
			return err
		}
		contentStore := store.NewContentStore(db)
		if err := contentStore.UpsertManhwaChapter(ctx, chapter); err != nil {
			return err
		}
		log.Printf("komiku chapter sync done: slug=%s pages=%d", chapter.Slug, len(chapter.Pages))
		return nil
	case "backfill-komiku-series":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape backfill-komiku-series <startPage> [endPage]")
		}
		startPage, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil {
			return fmt.Errorf("parse start page: %w", err)
		}
		endPage := startPage
		if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
			endPage, err = strconv.Atoi(strings.TrimSpace(args[2]))
			if err != nil {
				return fmt.Errorf("parse end page: %w", err)
			}
		}

		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client := komiku.NewClient(cfg.KomikuBaseURL, cfg.KomikuUserAgent, cfg.KomikuCookie, cfg.HTTPTimeout)
		service := komiku.NewService(client, cfg.KomikuBaseURL)
		contentStore := store.NewContentStore(db)

		report, err := backfillManhwaSeriesPages(ctx, service, contentStore, startPage, endPage)
		if err != nil {
			return err
		}
		log.Printf(
			"komiku series backfill done: start_page=%d end_page=%d discovered=%d succeeded=%d failed=%d",
			report.StartPage,
			report.EndPage,
			report.Discovered,
			report.Succeeded,
			report.Failed,
		)
		if len(report.Failures) > 0 {
			for slug, reason := range report.Failures {
				log.Printf("komiku series backfill failed: slug=%s error=%s", slug, reason)
			}
		}
		return nil
	case "backfill-komiku-chapters":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape backfill-komiku-chapters <startPage> [endPage] [latestPerSeries]")
		}
		startPage, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil {
			return fmt.Errorf("parse start page: %w", err)
		}
		endPage := startPage
		if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
			endPage, err = strconv.Atoi(strings.TrimSpace(args[2]))
			if err != nil {
				return fmt.Errorf("parse end page: %w", err)
			}
		}
		latestPerSeries := 3
		if len(args) >= 4 && strings.TrimSpace(args[3]) != "" {
			latestPerSeries, err = strconv.Atoi(strings.TrimSpace(args[3]))
			if err != nil {
				return fmt.Errorf("parse latest per series: %w", err)
			}
		}

		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		client := komiku.NewClient(cfg.KomikuBaseURL, cfg.KomikuUserAgent, cfg.KomikuCookie, cfg.HTTPTimeout)
		service := komiku.NewService(client, cfg.KomikuBaseURL)
		contentStore := store.NewContentStore(db)

		report, err := backfillManhwaChapterPages(ctx, service, contentStore, contentStore, startPage, endPage, latestPerSeries)
		if err != nil {
			return err
		}
		log.Printf(
			"komiku chapter backfill done: start_page=%d end_page=%d latest_per_series=%d series=%d attempted=%d succeeded=%d failed=%d",
			report.StartPage,
			report.EndPage,
			report.MaxChaptersPerSlug,
			report.DiscoveredSeries,
			report.AttemptedChapters,
			report.SucceededChapters,
			report.FailedChapters,
		)
		if len(report.Failures) > 0 {
			for slug, reason := range report.Failures {
				log.Printf("komiku chapter backfill failed: slug=%s error=%s", slug, reason)
			}
		}
		return nil
	case "refresh-anime-v2":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
		if err := db.Exec(ctx, "select public.refresh_anime_v2();"); err != nil {
			return fmt.Errorf("refresh anime v2: %w", err)
		}
		log.Printf("anime v2 refresh done")
		return nil
	case "refresh-media-v2":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
		if err := db.Exec(ctx, "select public.refresh_media_v2();"); err != nil {
			return fmt.Errorf("refresh media v2: %w", err)
		}
		log.Printf("media v2 refresh done")
		return nil
	case "refresh-movie-v3":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
		if err := db.Exec(ctx, "select public.refresh_movie_v3();"); err != nil {
			return fmt.Errorf("refresh movie v3: %w", err)
		}
		log.Printf("movie v3 refresh done")
		return nil
	case "sync-movie-v3-kanata-home":
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		limit := 24
		if len(args) >= 2 {
			parsed, err := strconv.Atoi(strings.TrimSpace(args[1]))
			if err != nil {
				return fmt.Errorf("parse limit: %w", err)
			}
			if parsed > 0 {
				limit = parsed
			}
		}

		kanataClient := kanata.NewClient(cfg.KanataMovieTubeBaseURL, httpClient)
		tmdbClient := tmdb.NewClient(cfg.TMDBBaseURL, cfg.TMDBReadToken, cfg.TMDBAPIKey, httpClient)
		movieStore := store.NewMovieV3StoreWithDB(db)
		service := kanata.NewMovieV3Service(kanataClient, tmdbClient, movieStore, time.Time{})

		report, err := service.SyncHome(ctx, limit)
		if err != nil {
			return err
		}
		log.Printf(
			"movie v3 kanata sync done: discovered=%d matched=%d upserted=%d failed=%d",
			report.Discovered,
			report.Matched,
			report.Upserted,
			report.Failed,
		)
		if len(report.Failures) > 0 {
			for slug, reason := range report.Failures {
				code := strings.TrimSpace(report.FailureCodes[slug])
				if code == "" {
					code = "unknown_error"
				}
				log.Printf("movie v3 kanata sync failed: slug=%s code=%s error=%s", slug, code, reason)
			}
		}
		return nil
	case "sync-movie-v3-kanata-genre":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape sync-movie-v3-kanata-genre <genre> [page] [limit]")
		}
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		genre := strings.TrimSpace(args[1])
		page := 1
		limit := 24
		if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
			parsed, err := strconv.Atoi(strings.TrimSpace(args[2]))
			if err != nil {
				return fmt.Errorf("parse page: %w", err)
			}
			if parsed > 0 {
				page = parsed
			}
		}
		if len(args) >= 4 && strings.TrimSpace(args[3]) != "" {
			parsed, err := strconv.Atoi(strings.TrimSpace(args[3]))
			if err != nil {
				return fmt.Errorf("parse limit: %w", err)
			}
			if parsed > 0 {
				limit = parsed
			}
		}

		kanataClient := kanata.NewClient(cfg.KanataMovieTubeBaseURL, httpClient)
		tmdbClient := tmdb.NewClient(cfg.TMDBBaseURL, cfg.TMDBReadToken, cfg.TMDBAPIKey, httpClient)
		movieStore := store.NewMovieV3StoreWithDB(db)
		service := kanata.NewMovieV3Service(kanataClient, tmdbClient, movieStore, time.Time{})

		report, err := service.SyncGenre(ctx, genre, page, limit)
		if err != nil {
			return err
		}
		log.Printf(
			"movie v3 kanata genre sync done: genre=%s page=%d discovered=%d matched=%d upserted=%d failed=%d",
			genre,
			page,
			report.Discovered,
			report.Matched,
			report.Upserted,
			report.Failed,
		)
		if len(report.Failures) > 0 {
			for slug, reason := range report.Failures {
				code := strings.TrimSpace(report.FailureCodes[slug])
				if code == "" {
					code = "unknown_error"
				}
				log.Printf("movie v3 kanata genre sync failed: slug=%s code=%s error=%s", slug, code, reason)
			}
		}
		return nil
	case "sync-movie-v3-kanata-search":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("usage: dwizzyscrape sync-movie-v3-kanata-search <query> [page] [limit]")
		}
		db, err := openContentDB(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		query := strings.TrimSpace(args[1])
		page := 1
		limit := 24
		if len(args) >= 3 && strings.TrimSpace(args[2]) != "" {
			parsed, err := strconv.Atoi(strings.TrimSpace(args[2]))
			if err != nil {
				return fmt.Errorf("parse page: %w", err)
			}
			if parsed > 0 {
				page = parsed
			}
		}
		if len(args) >= 4 && strings.TrimSpace(args[3]) != "" {
			parsed, err := strconv.Atoi(strings.TrimSpace(args[3]))
			if err != nil {
				return fmt.Errorf("parse limit: %w", err)
			}
			if parsed > 0 {
				limit = parsed
			}
		}

		kanataClient := kanata.NewClient(cfg.KanataMovieTubeBaseURL, httpClient)
		tmdbClient := tmdb.NewClient(cfg.TMDBBaseURL, cfg.TMDBReadToken, cfg.TMDBAPIKey, httpClient)
		movieStore := store.NewMovieV3StoreWithDB(db)
		service := kanata.NewMovieV3Service(kanataClient, tmdbClient, movieStore, time.Time{})

		report, err := service.SyncSearch(ctx, query, page, limit)
		if err != nil {
			return err
		}
		log.Printf(
			"movie v3 kanata search sync done: query=%q page=%d discovered=%d matched=%d upserted=%d failed=%d",
			query,
			page,
			report.Discovered,
			report.Matched,
			report.Upserted,
			report.Failed,
		)
		if len(report.Failures) > 0 {
			for slug, reason := range report.Failures {
				code := strings.TrimSpace(report.FailureCodes[slug])
				if code == "" {
					code = "unknown_error"
				}
				log.Printf("movie v3 kanata search sync failed: slug=%s code=%s error=%s", slug, code, reason)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func applyMigrations(ctx context.Context, db *store.PgxContentDB) error {
	paths, err := filepath.Glob("sql/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no sql migrations found")
	}
	for _, migrationPath := range paths {
		info, err := os.Stat(migrationPath)
		if err != nil {
			return fmt.Errorf("stat migration %s: %w", migrationPath, err)
		}
		if info.IsDir() || info.Mode()&fs.ModeType != 0 {
			continue
		}
		query, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", migrationPath, err)
		}
		if err := db.Exec(ctx, string(query)); err != nil {
			return fmt.Errorf("apply migration %s: %w", migrationPath, err)
		}
	}
	return nil
}

func writeJSON(out *os.File, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func buildSnapshots(ctx context.Context, cfg config.Config, httpClient *http.Client, outputDir string) (snapshot.Manifest, error) {
	collectors := newSnapshotCollectors(cfg, httpClient)
	return snapshot.BuildPack(ctx, collectors, snapshot.BuildOptions{
		OutputDir:          outputDir,
		HotLimit:           cfg.SnapshotHotLimit,
		CatalogPage:        cfg.SnapshotCatalogPage,
		MovieGenres:        cfg.SnapshotMovieGenres,
		MovieSearchQueries: cfg.SnapshotMovieQueries,
		GeneratedAt:        time.Now().UTC(),
	})
}

func patchSnapshots(ctx context.Context, cfg config.Config, httpClient *http.Client, domain, slug, outputDir string) (snapshot.Manifest, error) {
	collectors := newSnapshotCollectors(cfg, httpClient)
	return snapshot.PatchPack(ctx, collectors, domain, slug, snapshot.BuildOptions{
		OutputDir:          outputDir,
		HotLimit:           cfg.SnapshotHotLimit,
		CatalogPage:        cfg.SnapshotCatalogPage,
		MovieGenres:        cfg.SnapshotMovieGenres,
		MovieSearchQueries: cfg.SnapshotMovieQueries,
		GeneratedAt:        time.Now().UTC(),
	})
}

func newSnapshotCollectors(cfg config.Config, httpClient *http.Client) []snapshot.Collector {
	animeFetcher := samehadaku.NewHTTPClient(cfg.UserAgent, cfg.Cookie, cfg.HTTPTimeout)
	movieClient := kanata.NewClient(cfg.KanataMovieTubeBaseURL, httpClient)
	movieMetadataClient := tmdb.NewClient(cfg.TMDBBaseURL, cfg.TMDBReadToken, cfg.TMDBAPIKey, httpClient)
	manhwaClient := manhwaindo.NewClient(cfg.ManhwaindoBaseURL, cfg.ManhwaindoUserAgent, cfg.ManhwaindoCookie, cfg.HTTPTimeout)
	komikuClient := komiku.NewClient(cfg.KomikuBaseURL, cfg.KomikuUserAgent, cfg.KomikuCookie, cfg.HTTPTimeout)
	return snapshot.DefaultCollectors(
		movieClient,
		movieMetadataClient,
		animeFetcher,
		cfg.CatalogURL,
		manhwaindo.NewService(manhwaClient, cfg.ManhwaindoBaseURL),
		komiku.NewService(komikuClient, cfg.KomikuBaseURL),
		cfg.PostgresURL,
	)
}

func firstNonEmptyArgOrEnv(args []string, index int, envKey, fallback string) string {
	if len(args) > index {
		if value := strings.TrimSpace(args[index]); value != "" {
			return value
		}
	}
	if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func openContentDB(ctx context.Context, cfg config.Config) (*store.PgxContentDB, error) {
	if strings.TrimSpace(cfg.PostgresURL) == "" {
		return nil, fmt.Errorf("NEON_DATABASE_URL (or POSTGRES_URL / DATABASE_URL) is required for content sync")
	}
	return store.NewPgxContentDB(ctx, cfg.PostgresURL)
}
