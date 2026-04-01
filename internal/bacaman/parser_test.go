package bacaman

import "testing"

func TestParseCatalogHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<div class="listmanga">
  <a href="/manga/list-mode/">Daftar Isi</a>
  <a href="https://bacaman.id/manga/2-5-dimensional-seduction-bahasa-indonesia/">2.5 Dimensional Seduction Bahasa Indonesia</a>
  <a href="https://bacaman.id/manga/black-clover-bahasa-indonesia/">Black Clover Bahasa Indonesia</a>
  <a href="https://bacaman.id/manga/2-5-dimensional-seduction-bahasa-indonesia/">2.5 Dimensional Seduction Bahasa Indonesia</a>
</div>
`)

	items, err := ParseCatalogHTML(raw, "https://bacaman.id/manga/list-mode/")
	if err != nil {
		t.Fatalf("ParseCatalogHTML returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Slug != "2-5-dimensional-seduction-bahasa-indonesia" {
		t.Fatalf("unexpected first slug: %q", items[0].Slug)
	}
	if items[0].Title != "2.5 Dimensional Seduction Bahasa Indonesia" {
		t.Fatalf("unexpected first title: %q", items[0].Title)
	}
	if items[1].Slug != "black-clover-bahasa-indonesia" {
		t.Fatalf("unexpected second slug: %q", items[1].Slug)
	}
}

func TestParseSeriesHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<article>
  <div>
    <h1 class="entry-title">Tensei Shitara Slime Datta Ken Bahasa Indonesia</h1>
    <span class="alternative">Mengenai Reinkarnasi Menjadi Slime, TenSura</span>
  </div>
  <div class="thumb"><img src="https://bacaman.id/cover.jpg" alt="cover" /></div>
  <div class="tsinfo">
    <div class="imptdt">Status <i>Ongoing</i></div>
    <div class="imptdt">Type <a href="https://bacaman.id/?order=title&type=Manga">Manga</a></div>
    <div class="imptdt">Released <i>2015</i></div>
    <div class="imptdt">Author <i>Fuse</i></div>
    <div class="imptdt">Artist <i>KAWAKAMI Taiki</i></div>
    <div class="imptdt">Serialization <i>Shounen Sirius (Kodansha)</i></div>
  </div>
  <div class="mgen">
    <a href="https://bacaman.id/genres/action/">Action</a>
    <a href="https://bacaman.id/genres/fantasy/">Fantasy</a>
  </div>
  <div class="entry-content">
    <p>Ini hanyalah hari biasa bagi Satoru Mikami.</p>
  </div>
  <div class="bxcl">
    <ul>
      <li>
        <a href="https://bacaman.id/tensei-shitara-slime-datta-ken-chapter-137-bahasa-indonesia/">Chapter 137</a>
        <span>Januari 14, 2026</span>
      </li>
      <li>
        <a href="https://bacaman.id/tensei-shitara-slime-datta-ken-chapter-136-bahasa-indonesia/">Chapter 136</a>
        <span>November 29, 2025</span>
      </li>
    </ul>
  </div>
</article>
`)

	series, err := ParseSeriesHTML(raw, "https://bacaman.id/manga/tensei-shitara-slime-datta-ken-bahasa-indonesia/")
	if err != nil {
		t.Fatalf("ParseSeriesHTML returned error: %v", err)
	}
	if series.Title != "Tensei Shitara Slime Datta Ken Bahasa Indonesia" {
		t.Fatalf("unexpected title: %q", series.Title)
	}
	if series.AltTitle != "Mengenai Reinkarnasi Menjadi Slime, TenSura" {
		t.Fatalf("unexpected alt title: %q", series.AltTitle)
	}
	if series.MediaType != "manga" {
		t.Fatalf("unexpected media type: %q", series.MediaType)
	}
	if series.Author != "Fuse" {
		t.Fatalf("unexpected author: %q", series.Author)
	}
	if len(series.Genres) != 2 {
		t.Fatalf("expected 2 genres, got %d", len(series.Genres))
	}
	if len(series.Chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(series.Chapters))
	}
	if series.LatestChapter == nil || series.LatestChapter.Slug != "tensei-shitara-slime-datta-ken-chapter-137-bahasa-indonesia" {
		t.Fatalf("unexpected latest chapter: %#v", series.LatestChapter)
	}
}

func TestParseChapterHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<h1 class="entry-title">2.5 Dimensional Seduction Chapter 86 Bahasa Indonesia</h1>
<div class="allc">All chapters are in <a href="https://bacaman.id/manga/2-5-dimensional-seduction-bahasa-indonesia/">2.5 Dimensional Seduction Bahasa Indonesia</a></div>
<script>
ts_reader.run({"prevUrl":"https:\/\/bacaman.id\/2-5-dimensional-seduction-chapter-85-bahasa-indonesia\/","nextUrl":"https:\/\/bacaman.id\/2-5-dimensional-seduction-chapter-87-bahasa-indonesia\/","sources":[{"source":"BACAMAN","images":["https:\/\/bacaman00.sokuja.id\/2024\/manga\/2-5.dimensional\/86\/1.bacaman.id.jpg","https:\/\/bacaman00.sokuja.id\/2024\/manga\/2-5.dimensional\/86\/2.bacaman.id.jpg"]}]});
</script>
`)

	chapter, err := ParseChapterHTML(raw, "https://bacaman.id/2-5-dimensional-seduction-chapter-86-bahasa-indonesia/")
	if err != nil {
		t.Fatalf("ParseChapterHTML returned error: %v", err)
	}
	if chapter.SeriesSlug != "2-5-dimensional-seduction-bahasa-indonesia" {
		t.Fatalf("unexpected series slug: %q", chapter.SeriesSlug)
	}
	if chapter.PrevURL != "https://bacaman.id/2-5-dimensional-seduction-chapter-85-bahasa-indonesia/" {
		t.Fatalf("unexpected prev url: %q", chapter.PrevURL)
	}
	if chapter.NextURL != "https://bacaman.id/2-5-dimensional-seduction-chapter-87-bahasa-indonesia/" {
		t.Fatalf("unexpected next url: %q", chapter.NextURL)
	}
	if len(chapter.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(chapter.Pages))
	}
}
