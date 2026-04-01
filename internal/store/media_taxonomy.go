package store

import (
	"context"
	"regexp"
	"strings"
	"time"
)

type MediaTaxonomy struct {
	SurfaceType        string
	PresentationType   string
	OriginType         string
	ReleaseCountry     string
	IsNSFW             bool
	GenreNames         []string
	TaxonomyConfidence int16
	TaxonomySource     string
	ReleaseDay         string
	ReleaseWindow      string
	ReleaseTimezone    string
	Cadence            string
	NextReleaseAt      *time.Time
}

var releaseDayPattern = regexp.MustCompile(`(?i)\b(monday|tuesday|wednesday|thursday|friday|saturday|sunday|senin|selasa|rabu|kamis|jumat|jum'at|sabtu|minggu)\b`)
var releaseTimePattern = regexp.MustCompile(`\b(\d{1,2}):(\d{2})\b`)

func ClassifyMediaItem(source, mediaType string, detail map[string]any) MediaTaxonomy {
	source = strings.ToLower(strings.TrimSpace(source))
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))

	out := MediaTaxonomy{
		SurfaceType:        "unknown",
		PresentationType:   "unknown",
		OriginType:         "unknown",
		ReleaseCountry:     "",
		IsNSFW:             false,
		GenreNames:         collectTaxonomyGenreNames(detail),
		TaxonomyConfidence: 20,
		TaxonomySource:     "legacy_fallback",
	}

	switch mediaType {
	case "movie":
		out.SurfaceType = "movie"
		out.OriginType = "movie"
	case "anime", "drama", "donghua":
		out.SurfaceType = "series"
		out.OriginType = mediaType
	case "manga", "manhwa", "manhua":
		out.SurfaceType = "comic"
		out.PresentationType = "illustrated"
		out.OriginType = mediaType
	}

	switch source {
	case "samehadaku":
		if mediaType == "movie" {
			out.SurfaceType = "movie"
			out.PresentationType = "animation"
			out.OriginType = "anime"
			out.ReleaseCountry = "JP"
			out.TaxonomyConfidence = 90
			out.TaxonomySource = "provider_heuristic"
		} else {
			out.SurfaceType = "series"
			out.PresentationType = "animation"
			out.OriginType = "anime"
			out.ReleaseCountry = "JP"
			out.TaxonomyConfidence = 95
			out.TaxonomySource = "provider_heuristic"
		}
	case "anichin":
		out.SurfaceType = "series"
		out.PresentationType = "animation"
		out.OriginType = "donghua"
		out.ReleaseCountry = "CN"
		out.TaxonomyConfidence = 98
		out.TaxonomySource = "provider_heuristic"
	case "drakorid":
		out.PresentationType = "live_action"
		if mediaType == "movie" {
			out.SurfaceType = "movie"
			out.OriginType = "movie"
		} else {
			out.SurfaceType = "series"
			out.OriginType = classifyDrakoridOriginType(detail)
		}
		out.ReleaseCountry = firstNonEmptyCountryCode(detail, "KR")
		out.TaxonomyConfidence = 94
		out.TaxonomySource = "provider_heuristic"
	case "manhwaindo":
		out.SurfaceType = "comic"
		out.PresentationType = "illustrated"
		out.OriginType = "manhwa"
		out.ReleaseCountry = "KR"
		out.TaxonomyConfidence = 92
		out.TaxonomySource = "provider_heuristic"
	case "kanzenin", "mangasusuku":
		out.SurfaceType = "comic"
		out.PresentationType = "illustrated"
		out.OriginType = "manga"
		out.ReleaseCountry = "JP"
		out.TaxonomyConfidence = 92
		out.TaxonomySource = "provider_heuristic"
	case "nekopoi":
		out.ReleaseCountry = "JP"
		out.IsNSFW = true
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(readTaxonomyString(detail["content_format"]))), "animation") {
			out.PresentationType = "animation"
			out.OriginType = "anime"
			if mediaType == "anime" {
				out.SurfaceType = "series"
			} else {
				out.SurfaceType = "movie"
			}
		} else {
			out.SurfaceType = "movie"
			out.PresentationType = "live_action"
			out.OriginType = "movie"
		}
		out.TaxonomyConfidence = 88
		out.TaxonomySource = "provider_heuristic"
	case "hanime":
		out.ReleaseCountry = "JP"
		out.IsNSFW = true
		out.PresentationType = "animation"
		out.OriginType = "anime"
		if mediaType == "anime" {
			out.SurfaceType = "series"
		} else {
			out.SurfaceType = "movie"
		}
		out.TaxonomyConfidence = 94
		out.TaxonomySource = "provider_heuristic"
	}

	if out.PresentationType == "unknown" {
		switch mediaType {
		case "anime", "donghua":
			out.PresentationType = "animation"
		case "drama", "movie":
			out.PresentationType = "live_action"
		}
	}

	if out.ReleaseCountry == "" {
		out.ReleaseCountry = firstNonEmptyCountryCode(detail, releaseCountryByOrigin(out.OriginType))
	}

	if out.OriginType == "unknown" {
		out.OriginType = mediaType
	}

	out.ReleaseDay = normalizeReleaseDay(firstNonEmptyString(
		readTaxonomyString(detail["release_day"]),
		readTaxonomyString(detail["schedule_day"]),
		readTaxonomyString(detail["broadcast_day"]),
		readTaxonomyString(detail["day_of_week"]),
		readTaxonomyString(detail["weekday"]),
		readTaxonomyString(detail["runtime"]),
		readTaxonomyString(detail["aired"]),
	))
	out.ReleaseWindow = strings.TrimSpace(firstNonEmptyString(
		readTaxonomyString(detail["release_window"]),
		readTaxonomyString(detail["broadcast_time"]),
		readTaxonomyString(detail["release_time"]),
		extractScheduleTimeWindow(readTaxonomyString(detail["runtime"])),
	))
	out.ReleaseTimezone = inferReleaseTimezone(detail, out.ReleaseCountry)
	out.Cadence = inferCadence(source, mediaType, detail, out)
	out.NextReleaseAt = inferNextReleaseAt(out, time.Now().UTC())
	out.IsNSFW = detectNSFW(source, detail, out.GenreNames)
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeReleaseDay(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if match := releaseDayPattern.FindString(strings.ToLower(value)); match != "" {
		switch match {
		case "monday", "senin":
			return "monday"
		case "tuesday", "selasa":
			return "tuesday"
		case "wednesday", "rabu":
			return "wednesday"
		case "thursday", "kamis":
			return "thursday"
		case "friday", "jumat", "jum'at":
			return "friday"
		case "saturday", "sabtu":
			return "saturday"
		case "sunday", "minggu":
			return "sunday"
		}
	}

	return ""
}

func extractScheduleTimeWindow(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	match := regexp.MustCompile(`\b\d{1,2}:\d{2}(?:\s*-\s*\d{1,2}:\d{2})?\b`).FindString(value)
	return strings.TrimSpace(match)
}

func inferNextReleaseAt(taxonomy MediaTaxonomy, now time.Time) *time.Time {
	if taxonomy.SurfaceType != "series" || taxonomy.Cadence != "weekly" {
		return nil
	}
	releaseDay := normalizeReleaseDay(taxonomy.ReleaseDay)
	if releaseDay == "" || strings.TrimSpace(taxonomy.ReleaseTimezone) == "" {
		return nil
	}

	match := releaseTimePattern.FindStringSubmatch(strings.TrimSpace(taxonomy.ReleaseWindow))
	if len(match) < 3 {
		return nil
	}

	hour, err := time.Parse("15:04", match[1]+":"+match[2])
	if err != nil {
		return nil
	}

	location, err := time.LoadLocation(strings.TrimSpace(taxonomy.ReleaseTimezone))
	if err != nil {
		return nil
	}

	localNow := now.In(location)
	targetWeekday := releaseDayToWeekday(releaseDay)
	if targetWeekday < 0 {
		return nil
	}

	offsetDays := (int(targetWeekday) - int(localNow.Weekday()) + 7) % 7
	candidate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour.Hour(), hour.Minute(), 0, 0, location).AddDate(0, 0, offsetDays)
	if !candidate.After(localNow) {
		candidate = candidate.AddDate(0, 0, 7)
	}

	utc := candidate.UTC()
	return &utc
}

func releaseDayToWeekday(day string) time.Weekday {
	switch normalizeReleaseDay(day) {
	case "monday":
		return time.Monday
	case "tuesday":
		return time.Tuesday
	case "wednesday":
		return time.Wednesday
	case "thursday":
		return time.Thursday
	case "friday":
		return time.Friday
	case "saturday":
		return time.Saturday
	case "sunday":
		return time.Sunday
	default:
		return -1
	}
}

func inferReleaseTimezone(detail map[string]any, releaseCountry string) string {
	if explicit := strings.TrimSpace(firstNonEmptyString(
		readTaxonomyString(detail["release_timezone"]),
		readTaxonomyString(detail["timezone"]),
	)); explicit != "" {
		return explicit
	}

	switch strings.ToUpper(strings.TrimSpace(releaseCountry)) {
	case "JP":
		return "Asia/Tokyo"
	case "CN":
		return "Asia/Shanghai"
	case "KR":
		return "Asia/Seoul"
	case "US":
		return "America/New_York"
	default:
		return ""
	}
}

func inferCadence(source, mediaType string, detail map[string]any, taxonomy MediaTaxonomy) string {
	status := strings.ToLower(strings.TrimSpace(firstNonEmptyString(
		readTaxonomyString(detail["status"]),
		readTaxonomyString(detail["status_label"]),
	)))
	if status == "" {
		status = strings.ToLower(strings.TrimSpace(readTaxonomyString(detail["airing_status"])))
	}

	switch status {
	case "completed", "finished", "finished airing":
		return "completed"
	}

	if explicit := strings.ToLower(strings.TrimSpace(readTaxonomyString(detail["cadence"]))); explicit != "" {
		switch explicit {
		case "daily", "weekly", "biweekly", "monthly", "irregular", "completed", "unknown":
			return explicit
		}
	}

	if taxonomy.SurfaceType != "series" {
		return ""
	}
	if taxonomy.OriginType == "variety" {
		return ""
	}

	if status == "ongoing" || status == "airing" || status == "currently airing" {
		switch strings.ToLower(strings.TrimSpace(source)) {
		case "samehadaku", "anichin":
			return "weekly"
		case "drakorid":
			if mediaType == "drama" {
				return "weekly"
			}
		}
	}

	return ""
}

func classifyDrakoridOriginType(detail map[string]any) string {
	if isDrakoridVariety(detail) {
		return "variety"
	}
	return "drama"
}

func isDrakoridVariety(detail map[string]any) bool {
	for _, value := range drakoridTextCandidates(detail) {
		if isDrakoridVarietyText(value) {
			return true
		}
	}
	for _, value := range drakoridListCandidates(detail) {
		if isDrakoridVarietyText(value) {
			return true
		}
	}
	return false
}

func drakoridTextCandidates(detail map[string]any) []string {
	return []string{
		readTaxonomyString(detail["source_title"]),
		readTaxonomyString(detail["title"]),
		readTaxonomyString(detail["alt_title"]),
		readTaxonomyString(detail["native_title"]),
		readTaxonomyString(detail["format"]),
		readTaxonomyString(detail["episodes_text"]),
	}
}

func drakoridListCandidates(detail map[string]any) []string {
	values := make([]string, 0)
	for _, key := range []string{"categories", "category_names", "genres", "genre_names", "tags", "tag_names"} {
		values = append(values, readTaxonomyStringSlice(detail[key])...)
	}
	return values
}

func isDrakoridVarietyText(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}

	for _, keyword := range []string{
		"variety",
		"reality",
		"talk show",
		"dating show",
		"dating reality",
		"entertainment show",
		"competition show",
		"game show",
		"music show",
		"idol show",
		"travel show",
	} {
		if strings.Contains(value, keyword) {
			return true
		}
	}

	for _, titleKeyword := range []string{
		"running man",
		"how do you play",
		"1 night 2 days",
		"knowing brother",
		"my ugly duckling",
		"moms diary",
		"mom's diary",
		"the genius paik",
		"the return of superman",
		"superman returns",
		"whenever possible",
		"i live alone",
		"when our kids fall in love",
		"singles inferno",
	} {
		if strings.Contains(value, titleKeyword) {
			return true
		}
	}

	return false
}

func firstNonEmptyCountryCode(detail map[string]any, fallback string) string {
	candidates := []string{
		readTaxonomyString(detail["release_country"]),
		readTaxonomyString(detail["country"]),
		readTaxonomyString(detail["country_code"]),
		readTaxonomyString(detail["origin_country"]),
	}
	for _, candidate := range candidates {
		if code := normalizeCountryCode(candidate); code != "" {
			return code
		}
	}
	return normalizeCountryCode(fallback)
}

func releaseCountryByOrigin(originType string) string {
	switch strings.TrimSpace(originType) {
	case "anime", "manga":
		return "JP"
	case "donghua", "manhua":
		return "CN"
	case "drama", "manhwa":
		return "KR"
	default:
		return ""
	}
}

func collectTaxonomyGenreNames(detail map[string]any) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, key := range []string{"genres", "genre_names", "tags", "tag_names"} {
		for _, value := range readTaxonomyStringSlice(detail[key]) {
			normalized := strings.TrimSpace(value)
			if normalized == "" {
				continue
			}
			lower := strings.ToLower(normalized)
			if _, ok := seen[lower]; ok {
				continue
			}
			seen[lower] = struct{}{}
			out = append(out, normalized)
		}
	}
	return out
}

func detectNSFW(source string, detail map[string]any, genres []string) bool {
	switch source {
	case "kanzenin", "mangasusuku", "nekopoi", "hanime":
		return true
	}

	for _, value := range genres {
		if isNSFWKeyword(value) {
			return true
		}
	}

	for _, value := range readTaxonomyStringSlice(detail["tags"]) {
		if isNSFWKeyword(value) {
			return true
		}
	}

	return false
}

func isNSFWKeyword(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "nsfw", "adult", "18+", "hentai", "smut", "ecchi":
		return true
	default:
		return false
	}
}

func normalizeCountryCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "JP", "JAPAN":
		return "JP"
	case "CN", "CHINA", "CHINESE":
		return "CN"
	case "KR", "KOREA", "SOUTH KOREA", "KOREAN":
		return "KR"
	case "US", "USA", "UNITED STATES":
		return "US"
	default:
		if len(value) == 2 {
			return value
		}
		return ""
	}
}

func readTaxonomyString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return ""
	}
}

func readTaxonomyStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := readTaxonomyString(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		parts := strings.Split(typed, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if text := strings.TrimSpace(part); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func upsertMediaTaxonomy(ctx context.Context, db contentDB, itemKey string, taxonomy MediaTaxonomy) error {
	if db == nil {
		return nil
	}
	itemKey = strings.TrimSpace(itemKey)
	if itemKey == "" {
		return nil
	}
	if taxonomy.TaxonomySource == "" {
		taxonomy.TaxonomySource = "legacy_fallback"
	}
	if taxonomy.SurfaceType == "" {
		taxonomy.SurfaceType = "unknown"
	}
	if taxonomy.PresentationType == "" {
		taxonomy.PresentationType = "unknown"
	}
	if taxonomy.OriginType == "" {
		taxonomy.OriginType = "unknown"
	}
	if taxonomy.GenreNames == nil {
		taxonomy.GenreNames = []string{}
	}
	taxonomy.ReleaseDay = normalizeReleaseDay(taxonomy.ReleaseDay)
	taxonomy.ReleaseWindow = strings.TrimSpace(taxonomy.ReleaseWindow)
	taxonomy.ReleaseTimezone = strings.TrimSpace(taxonomy.ReleaseTimezone)
	taxonomy.Cadence = strings.TrimSpace(strings.ToLower(taxonomy.Cadence))

	return db.Exec(ctx, `
UPDATE public.media_items
SET
    surface_type = $2,
    presentation_type = $3,
    origin_type = $4,
    release_country = NULLIF($5, ''),
    is_nsfw = $6,
    genre_names = $7,
    taxonomy_confidence = $8,
    taxonomy_source = $9,
    release_day = NULLIF($10, ''),
    release_window = NULLIF($11, ''),
    release_timezone = NULLIF($12, ''),
    cadence = NULLIF($13, ''),
    next_release_at = $14
WHERE item_key = $1
`, itemKey, taxonomy.SurfaceType, taxonomy.PresentationType, taxonomy.OriginType, taxonomy.ReleaseCountry, taxonomy.IsNSFW, taxonomy.GenreNames, taxonomy.TaxonomyConfidence, taxonomy.TaxonomySource, taxonomy.ReleaseDay, taxonomy.ReleaseWindow, taxonomy.ReleaseTimezone, taxonomy.Cadence, taxonomy.NextReleaseAt)
}
