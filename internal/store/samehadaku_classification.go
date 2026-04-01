package store

import "strings"

func annotateSamehadakuDetail(detail map[string]any, slug, title string) map[string]any {
	if detail == nil {
		detail = map[string]any{}
	}

	slug = strings.ToLower(strings.TrimSpace(slug))
	title = strings.ToLower(strings.TrimSpace(title))

	if strings.Contains(slug, "live-action") || strings.Contains(title, "live action") {
		detail["adaptation_type"] = "live_action"
		detail["entry_format"] = "series"
	}

	if slug == "kaguya-sama-wa-kokurasetai-first-kiss-wa-owaranai" {
		detail["entry_format"] = "special_series"
	}

	return detail
}
