package bacaman

import "testing"

func TestNormalizeCatalogURL(t *testing.T) {
	t.Parallel()

	if got := normalizeCatalogURL("https://bacaman.id"); got != "https://bacaman.id/manga/list-mode/" {
		t.Fatalf("normalizeCatalogURL() = %q", got)
	}
}

func TestNormalizeSeriesURL(t *testing.T) {
	t.Parallel()

	if got := normalizeSeriesURL("https://bacaman.id", "black-clover-bahasa-indonesia"); got != "https://bacaman.id/manga/black-clover-bahasa-indonesia/" {
		t.Fatalf("normalizeSeriesURL() = %q", got)
	}
}

func TestNormalizeChapterURL(t *testing.T) {
	t.Parallel()

	if got := normalizeChapterURL("https://bacaman.id", "black-clover-chapter-9-bahasa-indonesia"); got != "https://bacaman.id/black-clover-chapter-9-bahasa-indonesia/" {
		t.Fatalf("normalizeChapterURL() = %q", got)
	}
}
