package mangasusuku

import "testing"

func TestNormalizeCatalogURL(t *testing.T) {
	t.Parallel()

	if got := normalizeCatalogURL("https://mangasusuku.com", "A", 1); got != "https://mangasusuku.com/az-list/?show=A" {
		t.Fatalf("page 1 url = %q", got)
	}
	if got := normalizeCatalogURL("https://mangasusuku.com", "A", 2); got != "https://mangasusuku.com/az-list/page/2/?show=A" {
		t.Fatalf("page 2 url = %q", got)
	}
}
