package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/dwirijal/dwizzySCRAPE/internal/content"
	"github.com/dwirijal/dwizzySCRAPE/internal/kanata"
	"github.com/dwirijal/dwizzySCRAPE/internal/komiku"
	"github.com/dwirijal/dwizzySCRAPE/internal/manhwaindo"
	"github.com/dwirijal/dwizzySCRAPE/internal/samehadaku"
	"github.com/dwirijal/dwizzySCRAPE/internal/tmdb"
)

type MovieClient interface {
	GetHome(ctx context.Context) ([]kanata.HomeMovie, error)
	GetGenre(ctx context.Context, genre string, page int) ([]kanata.HomeMovie, error)
	Search(ctx context.Context, query string, page int) ([]kanata.HomeMovie, error)
	GetDetail(ctx context.Context, slug string) (kanata.DetailMovie, error)
	GetStream(ctx context.Context, slug string) (kanata.Stream, error)
}

type ReadingService interface {
	FetchCatalog(ctx context.Context, page int) ([]content.ManhwaSeries, error)
	FetchSeries(ctx context.Context, slug string) (content.ManhwaSeries, error)
	FetchChapter(ctx context.Context, slug string) (content.ManhwaChapter, error)
}

type MovieMetadataClient interface {
	Enabled() bool
	SearchMovies(ctx context.Context, query string, year, limit int) ([]tmdb.SearchHit, error)
}

type AnimeCollector struct {
	CatalogURL string
	Fetcher    samehadaku.Fetcher
}

func (c *AnimeCollector) Domain() string {
	return "anime"
}

func (c *AnimeCollector) Build(ctx Context, writer *Writer, options BuildOptions) error {
	items, err := c.writeDomainDocs(ctx, writer, options)
	if err != nil {
		return err
	}
	for _, item := range limitAnimeCatalog(items, options.HotLimit) {
		if err := c.writeTitleAndPlayback(ctx, writer, item.Slug, &item); err != nil {
			continue
		}
	}
	return nil
}

func (c *AnimeCollector) Patch(ctx Context, writer *Writer, slug string, _ BuildOptions) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return fmt.Errorf("anime patch slug is required")
	}
	if _, err := c.writeDomainDocs(ctx, writer, BuildOptions{}); err != nil {
		return err
	}
	return c.writeTitleAndPlayback(ctx, writer, slug, nil)
}

func (c *AnimeCollector) writeDomainDocs(ctx Context, writer *Writer, options BuildOptions) ([]samehadaku.CatalogItem, error) {
	options = normalizeOptions(options)
	items, err := c.fetchCatalog(ctx, options.CatalogPage)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(c.Domain(), KindCatalog, fmt.Sprintf("page-%d", options.CatalogPage), items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *AnimeCollector) fetchCatalog(ctx context.Context, page int) ([]samehadaku.CatalogItem, error) {
	if c.Fetcher == nil {
		return nil, fmt.Errorf("anime fetcher is required")
	}
	if page <= 0 {
		page = 1
	}
	target := strings.TrimRight(strings.TrimSpace(c.CatalogURL), "/")
	if target == "" {
		target = strings.TrimRight(strings.TrimSpace(samehadaku.DefaultPrimaryBaseURL), "/") + "/daftar-anime-2"
	}
	if page > 1 {
		target = fmt.Sprintf("%s/page/%d/", target, page)
	} else {
		target += "/"
	}
	raw, err := c.Fetcher.FetchCatalogPage(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("fetch anime catalog page %d: %w", page, err)
	}
	items, err := samehadaku.ParseCatalogHTML(raw, target, writerTimeNow(ctx))
	if err != nil {
		return nil, fmt.Errorf("parse anime catalog page %d: %w", page, err)
	}
	for i := range items {
		items[i].PageNumber = page
	}
	return items, nil
}

func (c *AnimeCollector) writeTitleAndPlayback(ctx context.Context, writer *Writer, slug string, catalog *samehadaku.CatalogItem) error {
	titleSnapshot, latestEpisodeSlug, err := c.fetchAnimeTitle(ctx, slug, catalog)
	if err != nil {
		return err
	}
	if _, err := writer.Write(c.Domain(), KindTitle, slug, titleSnapshot); err != nil {
		return err
	}
	if latestEpisodeSlug == "" {
		return nil
	}
	playbackSnapshot, err := c.fetchAnimePlayback(ctx, slug, latestEpisodeSlug)
	if err != nil {
		return err
	}
	_, err = writer.Write(c.Domain(), KindPlayback, slug, playbackSnapshot)
	return err
}

func (c *AnimeCollector) fetchAnimeTitle(ctx context.Context, slug string, catalog *samehadaku.CatalogItem) (map[string]any, string, error) {
	if c.Fetcher == nil {
		return nil, "", fmt.Errorf("anime fetcher is required")
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, "", fmt.Errorf("anime slug is required")
	}

	var (
		primary *samehadaku.PrimaryAnimePage
		mirror  *samehadaku.MirrorAnimePage
	)
	primaryURL := samehadaku.BuildPrimaryAnimeURL(slug)
	if raw, err := c.Fetcher.FetchPage(ctx, primaryURL); err == nil {
		if page, parseErr := samehadaku.ParsePrimaryAnimeHTML(raw, primaryURL); parseErr == nil {
			primary = &page
		}
	}
	mirrorURL := samehadaku.BuildMirrorAnimeURL(slug)
	if raw, err := c.Fetcher.FetchPage(ctx, mirrorURL); err == nil {
		if page, parseErr := samehadaku.ParseMirrorAnimeHTML(raw, mirrorURL); parseErr == nil {
			mirror = &page
		}
	}
	if primary == nil && mirror == nil {
		return nil, "", fmt.Errorf("fetch anime title %s: no source parsed", slug)
	}

	latestEpisodeSlug := pickLatestEpisodeSlug(primary, mirror)
	payload := map[string]any{
		"slug":                slug,
		"catalog":             catalog,
		"primary":             primary,
		"mirror":              mirror,
		"latest_episode_slug": latestEpisodeSlug,
	}
	return payload, latestEpisodeSlug, nil
}

func (c *AnimeCollector) fetchAnimePlayback(ctx context.Context, animeSlug, episodeSlug string) (map[string]any, error) {
	if c.Fetcher == nil {
		return nil, fmt.Errorf("anime fetcher is required")
	}
	episodeSlug = strings.TrimSpace(episodeSlug)
	if episodeSlug == "" {
		return nil, fmt.Errorf("episode slug is required")
	}

	var (
		primary *samehadaku.PrimaryEpisodePage
		mirror  *samehadaku.MirrorEpisodePage
	)
	primaryURL := samehadaku.BuildPrimaryEpisodeURL(episodeSlug)
	if raw, err := c.Fetcher.FetchPage(ctx, primaryURL); err == nil {
		if page, parseErr := samehadaku.ParsePrimaryEpisodeHTML(raw, primaryURL); parseErr == nil {
			primary = &page
		}
	}
	secondaryURL := samehadaku.BuildSecondaryEpisodeURL(episodeSlug)
	if raw, err := c.Fetcher.FetchPage(ctx, secondaryURL); err == nil {
		if page, parseErr := samehadaku.ParseMirrorEpisodeHTML(raw, secondaryURL); parseErr == nil {
			mirror = &page
		}
	}
	if primary == nil && mirror == nil {
		return nil, fmt.Errorf("fetch anime playback %s: no source parsed", episodeSlug)
	}
	return map[string]any{
		"anime_slug":   animeSlug,
		"episode_slug": episodeSlug,
		"primary":      primary,
		"mirror":       mirror,
	}, nil
}

type MovieCollector struct {
	Client         MovieClient
	MetadataClient MovieMetadataClient
	posterCache    map[string]string
}

func (c *MovieCollector) Domain() string {
	return "movie"
}

func (c *MovieCollector) Build(ctx Context, writer *Writer, options BuildOptions) error {
	hot, err := c.writeDomainDocs(ctx, writer, options)
	if err != nil {
		return err
	}
	for _, item := range hot {
		if err := c.writeTitleAndPlayback(ctx, writer, item.Slug); err != nil {
			continue
		}
	}
	return nil
}

func (c *MovieCollector) writeDomainDocs(ctx Context, writer *Writer, options BuildOptions) ([]kanata.HomeMovie, error) {
	if c.Client == nil {
		return nil, fmt.Errorf("movie client is required")
	}
	home, err := c.Client.GetHome(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch movie home: %w", err)
	}
	home = c.enrichMovieCards(ctx, home)
	if _, err := writer.Write(c.Domain(), KindHome, "hot", home); err != nil {
		return nil, err
	}

	genres := normalizedGenres(options.MovieGenres)
	for _, genre := range genres {
		items, err := c.Client.GetGenre(ctx, genre, options.CatalogPage)
		if err != nil {
			continue
		}
		items = c.enrichMovieCards(ctx, items)
		if _, err := writer.Write(c.Domain(), KindCatalog, fmt.Sprintf("genre-%s-page-%d", genre, options.CatalogPage), items); err != nil {
			return nil, err
		}
	}

	hot := limitHomeMovies(home, options.HotLimit)
	for _, query := range deriveMovieQueries(hot, options.MovieSearchQueries) {
		items, err := c.Client.Search(ctx, query, 1)
		if err != nil {
			continue
		}
		items = c.enrichMovieCards(ctx, items)
		if _, err := writer.Write(c.Domain(), KindSearch, query, items); err != nil {
			return nil, err
		}
	}
	return hot, nil
}

func (c *MovieCollector) Patch(ctx Context, writer *Writer, slug string, _ BuildOptions) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return fmt.Errorf("movie patch slug is required")
	}
	if _, err := c.writeDomainDocs(ctx, writer, BuildOptions{}); err != nil {
		return err
	}
	return c.writeTitleAndPlayback(ctx, writer, slug)
}

func (c *MovieCollector) writeTitleAndPlayback(ctx context.Context, writer *Writer, slug string) error {
	detail, err := c.Client.GetDetail(ctx, slug)
	if err != nil {
		return fmt.Errorf("fetch movie detail %s: %w", slug, err)
	}
	if _, err := writer.Write(c.Domain(), KindTitle, slug, detail); err != nil {
		return err
	}
	stream, err := c.Client.GetStream(ctx, slug)
	if err != nil {
		return fmt.Errorf("fetch movie playback %s: %w", slug, err)
	}
	_, err = writer.Write(c.Domain(), KindPlayback, slug, stream)
	return err
}

func (c *MovieCollector) enrichMovieCards(ctx context.Context, items []kanata.HomeMovie) []kanata.HomeMovie {
	if len(items) == 0 || c.MetadataClient == nil || !c.MetadataClient.Enabled() {
		return items
	}
	if c.posterCache == nil {
		c.posterCache = make(map[string]string)
	}
	out := make([]kanata.HomeMovie, len(items))
	copy(out, items)
	for i := range out {
		if poster := c.lookupMoviePoster(ctx, out[i]); poster != "" {
			out[i].Poster = poster
		}
	}
	return out
}

func (c *MovieCollector) lookupMoviePoster(ctx context.Context, item kanata.HomeMovie) string {
	if c.MetadataClient == nil || !c.MetadataClient.Enabled() {
		return ""
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return ""
	}
	cacheKey := strings.ToLower(title) + "#" + strings.TrimSpace(item.Year)
	if poster, ok := c.posterCache[cacheKey]; ok {
		return poster
	}
	year, _ := strconv.Atoi(strings.TrimSpace(item.Year))
	results, err := c.MetadataClient.SearchMovies(ctx, title, year, 5)
	if err != nil {
		c.posterCache[cacheKey] = ""
		return ""
	}
	match, ok := tmdb.PickBestMovieMatch(title, year, results)
	if !ok {
		c.posterCache[cacheKey] = ""
		return ""
	}
	poster := tmdb.BuildPosterURL(match.PosterPath)
	c.posterCache[cacheKey] = poster
	return poster
}

type ReadingCollector struct {
	domain  string
	service ReadingService
}

func NewReadingCollector(domain string, service ReadingService) *ReadingCollector {
	return &ReadingCollector{
		domain:  sanitizePathPart(domain),
		service: service,
	}
}

func (c *ReadingCollector) Domain() string {
	return c.domain
}

func (c *ReadingCollector) Build(ctx Context, writer *Writer, options BuildOptions) error {
	items, err := c.writeDomainDocs(ctx, writer, options)
	if err != nil {
		return err
	}
	for _, item := range limitSeries(items, options.HotLimit) {
		if err := c.writeTitleAndPlayback(ctx, writer, item.Slug); err != nil {
			continue
		}
	}
	return nil
}

func (c *ReadingCollector) writeDomainDocs(ctx Context, writer *Writer, options BuildOptions) ([]content.ManhwaSeries, error) {
	if c.service == nil {
		return nil, fmt.Errorf("%s service is required", c.domain)
	}
	items, err := c.service.FetchCatalog(ctx, options.CatalogPage)
	if err != nil {
		return nil, fmt.Errorf("fetch %s catalog: %w", c.domain, err)
	}
	if _, err := writer.Write(c.Domain(), KindCatalog, fmt.Sprintf("page-%d", options.CatalogPage), items); err != nil {
		return nil, err
	}
	return items, nil
}

func (c *ReadingCollector) Patch(ctx Context, writer *Writer, slug string, _ BuildOptions) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return fmt.Errorf("%s patch slug is required", c.domain)
	}
	if _, err := c.writeDomainDocs(ctx, writer, BuildOptions{}); err != nil {
		return err
	}
	return c.writeTitleAndPlayback(ctx, writer, slug)
}

func (c *ReadingCollector) writeTitleAndPlayback(ctx context.Context, writer *Writer, slug string) error {
	series, err := c.service.FetchSeries(ctx, slug)
	if err != nil {
		return fmt.Errorf("fetch %s title %s: %w", c.domain, slug, err)
	}
	if _, err := writer.Write(c.Domain(), KindTitle, slug, series); err != nil {
		return err
	}
	chapterSlug := latestChapterSlug(series)
	if chapterSlug == "" {
		return nil
	}
	chapter, err := c.service.FetchChapter(ctx, chapterSlug)
	if err != nil {
		return fmt.Errorf("fetch %s playback %s: %w", c.domain, chapterSlug, err)
	}
	_, err = writer.Write(c.Domain(), KindPlayback, slug, chapter)
	return err
}

func BuildPack(ctx Context, collectors []Collector, options BuildOptions) (Manifest, error) {
	options = normalizeOptions(options)
	writer := NewWriter(options.OutputDir, options.GeneratedAt)
	if err := osEnsureDir(options.OutputDir); err != nil {
		return Manifest{}, err
	}
	var buildWarnings []error
	for _, collector := range collectors {
		if collector == nil {
			continue
		}
		if err := collector.Build(ctx, writer, options); err != nil {
			wrapped := fmt.Errorf("%s build failed: %w", collector.Domain(), err)
			buildWarnings = append(buildWarnings, wrapped)
			fmt.Fprintf(os.Stderr, "snapshot warning: %v\n", wrapped)
			continue
		}
	}
	manifest, err := WriteManifest(options.OutputDir, options.GeneratedAt)
	if err != nil {
		return Manifest{}, err
	}
	if len(manifest.Entries) == 0 && len(buildWarnings) > 0 {
		return Manifest{}, errors.Join(buildWarnings...)
	}
	return manifest, nil
}

func PatchPack(ctx Context, collectors []Collector, domain, slug string, options BuildOptions) (Manifest, error) {
	options = normalizeOptions(options)
	writer := NewWriter(options.OutputDir, options.GeneratedAt)
	if err := osEnsureDir(options.OutputDir); err != nil {
		return Manifest{}, err
	}
	for _, collector := range collectors {
		if collector == nil || collector.Domain() != sanitizePathPart(domain) {
			continue
		}
		if err := collector.Patch(ctx, writer, slug, options); err != nil {
			return Manifest{}, err
		}
		return WriteManifest(options.OutputDir, options.GeneratedAt)
	}
	return Manifest{}, fmt.Errorf("unsupported snapshot domain %q", domain)
}

func DefaultCollectors(movieClient MovieClient, movieMetadataClient MovieMetadataClient, animeFetcher samehadaku.Fetcher, catalogURL string, manhwaService *manhwaindo.Service, komikuService *komiku.Service, postgresURL string) []Collector {
	collectors := []Collector{
		&MovieCollector{Client: movieClient, MetadataClient: movieMetadataClient},
		&AnimeCollector{CatalogURL: catalogURL, Fetcher: animeFetcher},
		NewReadingCollector("manhwaindo", manhwaService),
		NewReadingCollector("komiku", komikuService),
	}
	if strings.TrimSpace(postgresURL) != "" {
		collectors = append(collectors, NewDonghuaCollector(postgresURL))
	}
	return collectors
}

func normalizeOptions(options BuildOptions) BuildOptions {
	if strings.TrimSpace(options.OutputDir) == "" {
		options.OutputDir = "snapshots"
	}
	if options.HotLimit <= 0 {
		options.HotLimit = 8
	}
	if options.CatalogPage <= 0 {
		options.CatalogPage = 1
	}
	if options.GeneratedAt.IsZero() {
		options.GeneratedAt = timeNowUTC()
	}
	if len(options.MovieGenres) == 0 {
		options.MovieGenres = []string{"action", "drama"}
	}
	return options
}

func limitAnimeCatalog(items []samehadaku.CatalogItem, limit int) []samehadaku.CatalogItem {
	if limit <= 0 || len(items) <= limit {
		return append([]samehadaku.CatalogItem(nil), items...)
	}
	return append([]samehadaku.CatalogItem(nil), items[:limit]...)
}

func limitSeries(items []content.ManhwaSeries, limit int) []content.ManhwaSeries {
	if limit <= 0 || len(items) <= limit {
		return append([]content.ManhwaSeries(nil), items...)
	}
	return append([]content.ManhwaSeries(nil), items[:limit]...)
}

func limitHomeMovies(items []kanata.HomeMovie, limit int) []kanata.HomeMovie {
	if limit <= 0 || len(items) <= limit {
		return dedupeHomeMovies(items)
	}
	return dedupeHomeMovies(items)[:limit]
}

func dedupeHomeMovies(items []kanata.HomeMovie) []kanata.HomeMovie {
	seen := make(map[string]struct{}, len(items))
	out := make([]kanata.HomeMovie, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.Slug)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func deriveMovieQueries(items []kanata.HomeMovie, configured []string) []string {
	if len(configured) > 0 {
		return dedupeStrings(configured)
	}
	queries := make([]string, 0, 3)
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		queries = append(queries, title)
		if len(queries) >= 3 {
			break
		}
	}
	return dedupeStrings(queries)
}

func normalizedGenres(genres []string) []string {
	if len(genres) == 0 {
		return []string{"action", "drama"}
	}
	return dedupeStrings(genres)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := sanitizePathPart(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func latestChapterSlug(series content.ManhwaSeries) string {
	if series.LatestChapter != nil && strings.TrimSpace(series.LatestChapter.Slug) != "" {
		return strings.TrimSpace(series.LatestChapter.Slug)
	}
	if len(series.Chapters) == 0 {
		return ""
	}
	return strings.TrimSpace(series.Chapters[0].Slug)
}

func pickLatestEpisodeSlug(primary *samehadaku.PrimaryAnimePage, mirror *samehadaku.MirrorAnimePage) string {
	candidates := []string{}
	if primary != nil {
		candidates = append(candidates, strings.TrimSpace(primary.LatestEpisode))
		if len(primary.Episodes) > 0 {
			candidates = append(candidates, strings.TrimSpace(primary.Episodes[0].EpisodeSlug))
		}
	}
	if mirror != nil {
		candidates = append(candidates, strings.TrimSpace(mirror.LatestEpisode))
		if len(mirror.Episodes) > 0 {
			candidates = append(candidates, strings.TrimSpace(mirror.Episodes[0].EpisodeSlug))
		}
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(candidate, "http://") || strings.HasPrefix(candidate, "https://") {
			parts := strings.Split(strings.Trim(candidate, "/"), "/")
			return parts[len(parts)-1]
		}
		return strings.Trim(candidate, "/")
	}
	return ""
}
