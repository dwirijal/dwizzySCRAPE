package kanzenin

import "testing"

func TestNormalizeCatalogURL(t *testing.T) {
	t.Parallel()

	if got := normalizeCatalogURL("https://kanzenin.info", "A", 1); got != "https://kanzenin.info/a-z-list/?show=A" {
		t.Fatalf("page 1 url = %q", got)
	}
	if got := normalizeCatalogURL("https://kanzenin.info", "A", 2); got != "https://kanzenin.info/a-z-list/page/2/?show=A" {
		t.Fatalf("page 2 url = %q", got)
	}
	if got := normalizeCatalogURL("https://kanzenin.info", ".", 3); got != "https://kanzenin.info/a-z-list/page/3/?show=." {
		t.Fatalf("page 3 symbol url = %q", got)
	}
}
