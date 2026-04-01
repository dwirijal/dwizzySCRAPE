package kanzenin

import "testing"

func TestParseCatalogHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<div class="listupd">
  <div class="bsx">
    <a href="https://kanzenin.info/manga/a-dangerous-deal-and-the-girl-next-door/">
      <img src="https://kanzenin.info/a.jpg" alt="A Dangerous Deal and The Girl Next Door" />
      <div class="tt">A Dangerous Deal and The Girl Next Door</div>
      <div class="epxs">Chapter 43</div>
    </a>
  </div>
  <div class="bsx">
    <a href="https://kanzenin.info/manga/a-delicate-relationship/">
      <img src="https://kanzenin.info/b.jpg" alt="A Delicate Relationship" />
      <div class="tt">A Delicate Relationship</div>
      <div class="epxs">Chapter 45 End</div>
      <span>Completed</span>
    </a>
  </div>
</div>
`)

	items, err := ParseCatalogHTML(raw, "https://kanzenin.info/a-z-list/?show=A")
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
<h1 class="entry-title">The Pleasure Shop</h1>
<div class="thumb"><img src="https://kanzenin.info/cover.jpg" alt="The Pleasure Shop" /></div>
<div class="entry-content entry-content-single"><p>Sudah lama bekerja tapi tetap saja miskin...</p></div>
<div class="lastend">
  <div class="inepcx">
    <a href="https://kanzenin.info/the-pleasure-shop-chapter-151/">
      <span class="epcur epcurlast">Chapter 151</span>
    </a>
  </div>
</div>
<table class="infotable">
  <tr><td>Status</td><td>Completed</td></tr>
  <tr><td>Type</td><td>Manhwa</td></tr>
  <tr><td>Released</td><td>2022</td></tr>
  <tr><td>Author</td><td>kimteok</td></tr>
</table>
<div class="seriestugenre">
  <a>Comedy</a><a>Femdom</a><a>Mature</a>
</div>
<div class="bxcl">
  <ul>
    <li>
      <div class="lchx"><a href="https://kanzenin.info/the-pleasure-shop-chapter-151/">Chapter 151</a></div>
      <div class="eph-date">November 26, 2025</div>
    </li>
    <li>
      <div class="lchx"><a href="https://kanzenin.info/the-pleasure-shop-chapter-150/">Chapter 150</a></div>
      <div class="eph-date">November 18, 2025</div>
    </li>
  </ul>
</div>
`)

	series, err := ParseSeriesHTML(raw, "https://kanzenin.info/manga/the-pleasure-shop/")
	if err != nil {
		t.Fatalf("ParseSeriesHTML returned error: %v", err)
	}
	if series.Title != "The Pleasure Shop" {
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
	if series.LatestChapter == nil || series.LatestChapter.Slug != "the-pleasure-shop-chapter-151" {
		t.Fatalf("unexpected latest chapter: %#v", series.LatestChapter)
	}
}

func TestParseChapterHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<h1 class="entry-title">The Pleasure Shop Chapter 150</h1>
<div class="allc">All chapters are in <a href="https://kanzenin.info/manga/the-pleasure-shop/">The Pleasure Shop</a></div>
<script>
ts_reader.run({"prevUrl":"https:\/\/kanzenin.info\/the-pleasure-shop-chapter-149\/","nextUrl":"https:\/\/kanzenin.info\/the-pleasure-shop-chapter-151\/","sources":[{"source":"Server 1","images":["https:\/\/cdn.uqni.net\/1.jpg","https:\/\/cdn.uqni.net\/2.jpg"]}]});
</script>
`)

	chapter, err := ParseChapterHTML(raw, "https://kanzenin.info/the-pleasure-shop-chapter-150/")
	if err != nil {
		t.Fatalf("ParseChapterHTML returned error: %v", err)
	}
	if chapter.SeriesSlug != "the-pleasure-shop" {
		t.Fatalf("unexpected series slug: %q", chapter.SeriesSlug)
	}
	if chapter.PrevURL != "https://kanzenin.info/the-pleasure-shop-chapter-149/" {
		t.Fatalf("unexpected prev url: %q", chapter.PrevURL)
	}
	if chapter.NextURL != "https://kanzenin.info/the-pleasure-shop-chapter-151/" {
		t.Fatalf("unexpected next url: %q", chapter.NextURL)
	}
	if len(chapter.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(chapter.Pages))
	}
	if chapter.Pages[0].URL != "https://cdn.uqni.net/1.jpg" {
		t.Fatalf("unexpected first page url: %q", chapter.Pages[0].URL)
	}
}
