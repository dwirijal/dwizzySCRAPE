package hanime

import "testing"

func TestParseCatalogHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<html><body>
  <a class="no-touch" href="/videos/hentai/my-mother-1" alt="My Mother 1">
    <div class="hv-title">My Mother 1</div>
  </a>
  <a class="no-touch" href="/videos/hentai/my-mother-2" alt="My Mother 2">
    <div class="hv-title">My Mother 2</div>
  </a>
  <a class="no-touch" href="/videos/hentai/alone" alt="Standalone OVA"></a>
</body></html>`)

	items, err := ParseCatalogHTML(raw, "https://hanime.tv/browse/trending")
	if err != nil {
		t.Fatalf("ParseCatalogHTML returned error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Slug != "my-mother-1" {
		t.Fatalf("slug = %q", items[0].Slug)
	}
	if items[0].NormalizedTitle != "My Mother" {
		t.Fatalf("normalized_title = %q", items[0].NormalizedTitle)
	}
	if !items[0].SeriesCandidate || items[0].EpisodeNumber != 1 {
		t.Fatalf("expected numbered title to be episodic, got %+v", items[0])
	}
	if !items[1].SeriesCandidate || items[1].EpisodeNumber != 2 {
		t.Fatalf("expected numbered sibling title to be episodic, got %+v", items[1])
	}
	if items[2].SeriesCandidate {
		t.Fatalf("expected standalone title to stay movie-like, got %+v", items[2])
	}
}

func TestParseDetailHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<html><body>
  <h1 class="tv-title">L'amour fou de l'automate 1</h1>
  <img class="hvpi-cover" src="https://hanime-cdn.com/images/covers/example.webp">
  <div class="hvpist-description">
    Marie, a married researcher, studies a homunculus in a laboratory.
  </div>
  <div class="hvpi-summary">
    <a href="/browse/tags/ntr">ntr</a>
    <a href="/browse/tags/fantasy">fantasy</a>
  </div>
  <div class="hvpimbc-item">
    <div class="hvpimbc-header">Brand</div>
    <a class="hvpimbc-text" href="/browse/brands/nur">nur</a>
  </div>
  <div class="hvpimbc-item">
    <div class="hvpimbc-header">Alternate Titles</div>
    <div class="hvpimbc-text">Alt One / Alt Two</div>
  </div>
  <div class="hvpimbc-item">
    <div class="hvpimbc-header">Released</div>
    <div class="hvpimbc-text">Jan 2, 2026</div>
  </div>
  <div class="htv-video-page-action-bar">
    <a href="/downloads/l-amour-fou-de-l-automate">Download</a>
  </div>
  <script>var x = "application/x-mpegURL";</script>
</body></html>`)

	meta, err := ParseDetailHTML(raw, "https://hanime.tv/videos/hentai/l-amour-fou-de-l-automate")
	if err != nil {
		t.Fatalf("ParseDetailHTML returned error: %v", err)
	}
	if meta.Title != "L'amour fou de l'automate 1" {
		t.Fatalf("title = %q", meta.Title)
	}
	if meta.Brand != "nur" || meta.BrandSlug != "nur" {
		t.Fatalf("expected brand metadata, got %+v", meta)
	}
	if len(meta.Tags) != 2 {
		t.Fatalf("tags length = %d", len(meta.Tags))
	}
	if !meta.DownloadPresent {
		t.Fatalf("expected download link, got %+v", meta)
	}
	if !meta.ManifestPresent {
		t.Fatal("expected manifest presence from html markers")
	}
	if len(meta.AlternateTitles) != 2 {
		t.Fatalf("alternate titles = %#v", meta.AlternateTitles)
	}
	if meta.ReleasedAt.IsZero() {
		t.Fatal("expected released_at")
	}
}
