package otakudesu

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

func BuildReviews(ctx context.Context, fetcher Fetcher, anime []SamehadakuAnime, options ReviewOptions) ([]ReviewResult, error) {
	if fetcher == nil {
		return nil, fmt.Errorf("fetcher is required")
	}

	minMatchScore := options.MinMatchScore
	if minMatchScore <= 0 {
		minMatchScore = 0.55
	}
	maxEpisodes := options.MaxEpisodes

	results := make([]ReviewResult, 0, len(anime))
	for _, entry := range anime {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		query := strings.TrimSpace(entry.SourceTitle)
		if query == "" {
			query = strings.TrimSpace(entry.Title)
		}
		review := ReviewResult{
			DBAnimeSlug:   entry.AnimeSlug,
			DBTitle:       entry.Title,
			DBSourceTitle: entry.SourceTitle,
			Query:         query,
			MatchStatus:   "no_match",
			Notes:         "no search results",
		}

		searchResults, err := fetcher.SearchAnime(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("search otakudesu for %s: %w", entry.AnimeSlug, err)
		}
		best, score := chooseBestResult(entry, searchResults)
		review.MatchScore = score
		if best.URL == "" || score < minMatchScore {
			results = append(results, review)
			continue
		}

		review.MatchStatus = "matched"
		review.NeedsReview = score < 0.8
		review.MatchedTitle = best.Title
		review.MatchedURL = best.URL
		if review.NeedsReview {
			review.Notes = "low confidence title match"
		} else {
			review.Notes = ""
		}

		animePage, err := fetcher.FetchAnimePage(ctx, best.URL)
		if err != nil {
			return nil, fmt.Errorf("fetch anime page %s: %w", best.URL, err)
		}

		episodeMap := make(map[string]SamehadakuEpisode, len(entry.Episodes))
		for _, episode := range entry.Episodes {
			key := strings.TrimSpace(episode.Number)
			if key == "" {
				continue
			}
			episodeMap[key] = episode
		}

		review.Episodes = make([]EpisodeReview, 0, len(animePage.Episodes))
		episodeRefs := animePage.Episodes
		if maxEpisodes > 0 && len(episodeRefs) > maxEpisodes {
			episodeRefs = episodeRefs[:maxEpisodes]
		}
		for _, ref := range episodeRefs {
			dbEpisode, ok := episodeMap[strings.TrimSpace(ref.Number)]
			episodePage, err := fetcher.FetchEpisodePage(ctx, ref.URL)
			if err != nil {
				return nil, fmt.Errorf("fetch episode page %s: %w", ref.URL, err)
			}
			episodeSlug := ""
			streamPresent := false
			downloadPresent := false
			if ok {
				episodeSlug = dbEpisode.EpisodeSlug
				streamPresent = dbEpisode.StreamPresent
				downloadPresent = dbEpisode.DownloadPresent
			}
			review.Episodes = append(review.Episodes, EpisodeReview{
				DBEpisodeSlug:             episodeSlug,
				EpisodeNumber:             firstNonEmpty(episodePage.Number, ref.Number),
				OtakudesuEpisodeURL:       ref.URL,
				StreamURL:                 episodePage.StreamURL,
				StreamMirrors:             episodePage.StreamMirrors,
				DownloadLinks:             episodePage.DownloadLinks,
				DownloadURL:               firstDownloadURL(episodePage.DownloadURLs),
				SamehadakuStreamPresent:   streamPresent,
				SamehadakuDownloadPresent: downloadPresent,
			})
		}

		sort.Slice(review.Episodes, func(i, j int) bool {
			return compareEpisodeNumbers(review.Episodes[i].EpisodeNumber, review.Episodes[j].EpisodeNumber)
		})
		results = append(results, review)
	}

	return results, nil
}

func chooseBestResult(entry SamehadakuAnime, results []SearchResult) (SearchResult, float64) {
	best := SearchResult{}
	bestScore := 0.0
	for _, result := range results {
		score := MatchScore(firstNonEmpty(entry.SourceTitle, entry.Title), result.Title)
		if score > bestScore {
			best = result
			bestScore = score
		}
	}
	return best, bestScore
}

func firstDownloadURL(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func compareEpisodeNumbers(left, right string) bool {
	leftValue, leftErr := strconv.ParseFloat(strings.TrimSpace(left), 64)
	rightValue, rightErr := strconv.ParseFloat(strings.TrimSpace(right), 64)
	switch {
	case leftErr == nil && rightErr == nil:
		if math.Abs(leftValue-rightValue) < 0.000001 {
			return left < right
		}
		return leftValue < rightValue
	case leftErr == nil:
		return true
	case rightErr == nil:
		return false
	default:
		return left < right
	}
}
