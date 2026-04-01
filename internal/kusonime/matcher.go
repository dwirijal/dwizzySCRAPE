package kusonime

import (
	"math"
	"regexp"
	"strings"
)

var nonAlphaNumPattern = regexp.MustCompile(`[^a-z0-9]+`)

func MatchScore(left, right string) float64 {
	leftTokens := titleTokens(left)
	rightTokens := titleTokens(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}

	leftSet := make(map[string]struct{}, len(leftTokens))
	for _, token := range leftTokens {
		leftSet[token] = struct{}{}
	}
	rightSet := make(map[string]struct{}, len(rightTokens))
	for _, token := range rightTokens {
		rightSet[token] = struct{}{}
	}

	intersection := 0
	for token := range leftSet {
		if _, ok := rightSet[token]; ok {
			intersection++
		}
	}
	union := len(leftSet)
	for token := range rightSet {
		if _, ok := leftSet[token]; !ok {
			union++
		}
	}
	if union == 0 {
		return 0
	}

	jaccard := float64(intersection) / float64(union)
	lengthPenalty := math.Abs(float64(len(leftSet)-len(rightSet))) * 0.03
	score := jaccard - lengthPenalty
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func titleTokens(value string) []string {
	value = strings.ToLower(cleanAnimeTitle(value))
	value = nonAlphaNumPattern.ReplaceAllString(value, " ")

	ignored := map[string]struct{}{
		"subtitle":  {},
		"indonesia": {},
		"sub":       {},
		"indo":      {},
		"batch":     {},
		"bd":        {},
		"lengkap":   {},
	}

	rawTokens := strings.Fields(value)
	tokens := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		if _, ok := ignored[token]; ok {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}
