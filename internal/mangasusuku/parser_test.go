package mangasusuku

import "testing"

func TestParseCatalogHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<div class="listupd">
  <div class="bsx">
    <a href="https://mangasusuku.com/komik/a-dangerous-deal-and-the-girl-next-door/">
      <div class="tt">A Dangerous Deal and The Girl Next Door</div>
      <div class="epxs">Chapter 43</div>
    </a>
  </div>
  <div class="bsx">
    <a href="https://mangasusuku.com/komik/a-bachelor-in-the-country/">
      <span>Completed</span>
      <div class="tt">A Bachelor in the Country</div>
      <div class="epxs">Chapter 46 End</div>
    </a>
  </div>
</div>
`)

	items, err := ParseCatalogHTML(raw, "https://mangasusuku.com/az-list/?show=A")
	if err != nil {
		t.Fatalf("ParseCatalogHTML returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Slug != "a-dangerous-deal-and-the-girl-next-door" {
		t.Fatalf("unexpected first slug: %q", items[0].Slug)
	}
	if items[0].LatestChapter == nil || items[0].LatestChapter.Number != "43" {
		t.Fatalf("unexpected latest chapter: %#v", items[0].LatestChapter)
	}
	if items[1].Status != "Completed" {
		t.Fatalf("unexpected second status: %q", items[1].Status)
	}
}

func TestParseSeriesHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<h1 class="entry-title">Smoking Hypnosis</h1>
<div class="thumb"><img src="https://mangasusuku.com/cover.jpg" alt="Smoking Hypnosis" /></div>
<div class="entry-content"><p>Sinopsis pendek.</p></div>
<table class="infotable">
  <tr><td>Status</td><td>Ongoing</td></tr>
  <tr><td>Type</td><td>Manhwa</td></tr>
  <tr><td>Released</td><td>2023</td></tr>
  <tr><td>Author</td><td>Author Name</td></tr>
</table>
<div class="seriestugenre">
  <a>Adult</a><a>Mature</a><a>Seinen</a>
</div>
<div class="bxcl">
  <ul>
    <li>
      <a href="https://mangasusuku.com/smoking-hypnosis-season-2-chapter-18/">Chapter 18 - Season 2</a>
      <span>March 5, 2026</span>
    </li>
    <li>
      <a href="https://mangasusuku.com/smoking-hypnosis-season-2-chapter-17/">Chapter 17 - Season 2</a>
      <span>February 5, 2026</span>
    </li>
  </ul>
</div>
`)

	series, err := ParseSeriesHTML(raw, "https://mangasusuku.com/komik/smoking-hypnosis/")
	if err != nil {
		t.Fatalf("ParseSeriesHTML returned error: %v", err)
	}
	if series.Title != "Smoking Hypnosis" {
		t.Fatalf("unexpected title: %q", series.Title)
	}
	if series.MediaType != "manhwa" {
		t.Fatalf("unexpected media type: %q", series.MediaType)
	}
	if len(series.Genres) != 3 {
		t.Fatalf("expected 3 genres, got %d", len(series.Genres))
	}
	if len(series.Chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(series.Chapters))
	}
	if series.LatestChapter == nil || series.LatestChapter.Slug != "smoking-hypnosis-season-2-chapter-18" {
		t.Fatalf("unexpected latest chapter: %#v", series.LatestChapter)
	}
}

func TestParseChapterHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<h1 class="entry-title">Smoking Hypnosis Chapter 1</h1>
<div class="allc"><a href="https://mangasusuku.com/komik/smoking-hypnosis/">Smoking Hypnosis</a></div>
<script>
ts_reader.run({"prevUrl":"","nextUrl":"https:\/\/mangasusuku.com\/smoking-hypnosis-chapter-2\/","sources":[{"source":"Server 1","images":["http:\/\/wibulep.xyz\/uploads\/manga-images\/s\/smoking-hypnosis\/chapter-1\/1.jpg","http:\/\/wibulep.xyz\/uploads\/manga-images\/s\/smoking-hypnosis\/chapter-1\/2.jpg"]}]});
</script>
`)

	chapter, err := ParseChapterHTML(raw, "https://mangasusuku.com/smoking-hypnosis-chapter-1/")
	if err != nil {
		t.Fatalf("ParseChapterHTML returned error: %v", err)
	}
	if chapter.SeriesSlug != "smoking-hypnosis" {
		t.Fatalf("unexpected series slug: %q", chapter.SeriesSlug)
	}
	if chapter.NextURL != "https://mangasusuku.com/smoking-hypnosis-chapter-2/" {
		t.Fatalf("unexpected next url: %q", chapter.NextURL)
	}
	if len(chapter.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(chapter.Pages))
	}
}
