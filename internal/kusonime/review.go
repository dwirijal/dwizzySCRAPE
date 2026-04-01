package kusonime

import (
	"context"
	"fmt"
	"strings"
)

func BuildReviews(ctx context.Context, fetcher Fetcher, anime []SamehadakuAnime, options ReviewOptions) ([]ReviewResult, error) {
	if fetcher == nil {
		return nil, fmt.Errorf("fetcher is required")
	}

	minMatchScore := options.MinMatchScore
	if minMatchScore <= 0 {
		minMatchScore = 0.60
	}

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
			return nil, fmt.Errorf("search kusonime for %s: %w", entry.AnimeSlug, err)
		}
		best, score := chooseBestResult(entry, searchResults)
		review.MatchScore = score
		if best.URL == "" || score < minMatchScore {
			results = append(results, review)
			continue
		}

		page, err := fetcher.FetchAnimePage(ctx, best.URL)
		if err != nil {
			return nil, fmt.Errorf("fetch anime page %s: %w", best.URL, err)
		}

		review.MatchStatus = "matched"
		review.NeedsReview = score < 0.8
		review.MatchedTitle = best.Title
		review.MatchedURL = best.URL
		review.Page = page
		if review.NeedsReview {
			review.Notes = "low confidence title match"
		} else {
			review.Notes = ""
		}
		results = append(results, review)
	}

	return results, nil
}

func chooseBestResult(entry SamehadakuAnime, results []SearchResult) (SearchResult, float64) {
	best := SearchResult{}
	bestScore := 0.0
	query := firstNonEmpty(entry.SourceTitle, entry.Title)
	for _, result := range results {
		score := MatchScore(query, result.Title)
		if score > bestScore {
			best = result
			bestScore = score
		}
	}
	return best, bestScore
}
