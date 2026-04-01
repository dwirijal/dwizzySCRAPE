package anichin

import "testing"

func TestParseCatalogHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<div class="bs">
  <div class="bsx">
    <a href="https://anichin.cafe/seri/in-search-of-gods/">
      <img src="https://anichin.cafe/in-search.webp" alt="In Search of Gods" />
      <div class="tt">In Search of Gods</div>
      <div class="limit">
        <span class="typez Donghua">Donghua</span>
        <span class="status Ongoing">Ongoing</span>
      </div>
    </a>
  </div>
</div>
`)

	items, err := ParseCatalogHTML(raw, "https://anichin.cafe/ongoing/", "ongoing")
	if err != nil {
		t.Fatalf("ParseCatalogHTML returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Slug != "in-search-of-gods" {
		t.Fatalf("unexpected slug %q", items[0].Slug)
	}
	if items[0].Status != "Ongoing" {
		t.Fatalf("unexpected status %q", items[0].Status)
	}
	if items[0].AnimeType != "Donghua" {
		t.Fatalf("unexpected anime type %q", items[0].AnimeType)
	}
}

func TestParseSeriesHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<div class="thumb"><img src="https://anichin.cafe/poster.webp" alt="Stellar Transformation Season 5" /></div>
<div class="infox">
  <h1 class="entry-title">Stellar Transformation Season 5</h1>
  <span class="alter">Xingchen Bian 5th Season, Legend of Immortals 5th Season</span>
  <div class="info-content">
    <div class="spe">
      <span><b>Status:</b> Completed</span>
      <span><b>Network:</b> Tencent Penguin Pictures</span>
      <span><b>Studio:</b> Foch Films</span>
      <span><b>Released:</b> 2022</span>
      <span><b>Season:</b> Winter</span>
    </div>
    <div class="genxed"><a>Action</a><a>Adventure</a><a>Fantasy</a></div>
  </div>
</div>
<div class="bixbox synp">
  <div class="entry-content"><p>Musim kelima dari Stellar Transformation.</p></div>
</div>
<div class="eplister">
  <ul>
    <li>
      <a href="https://anichin.cafe/stellar-transformation-season-5-episode-28-tamat-subtitle-indonesia/">
        <div class="epl-num">28 END</div>
        <div class="epl-title">Stellar Transformation Season 5 Episode 28 Tamat Subtitle Indonesia</div>
        <div class="epl-date">March 11, 2023</div>
      </a>
    </li>
    <li>
      <a href="https://anichin.cafe/stellar-transformation-season-5-episode-27-subtitle-indonesia/">
        <div class="epl-num">27</div>
        <div class="epl-title">Stellar Transformation Season 5 Episode 27 Subtitle Indonesia</div>
        <div class="epl-date">March 4, 2023</div>
      </a>
    </li>
  </ul>
</div>
`)

	series, err := ParseSeriesHTML(raw, "https://anichin.cafe/seri/stellar-transformation-season-5/")
	if err != nil {
		t.Fatalf("ParseSeriesHTML returned error: %v", err)
	}
	if series.Title != "Stellar Transformation Season 5" {
		t.Fatalf("unexpected title %q", series.Title)
	}
	if series.AltTitle != "Xingchen Bian 5th Season, Legend of Immortals 5th Season" {
		t.Fatalf("unexpected alt title %q", series.AltTitle)
	}
	if series.Status != "Completed" {
		t.Fatalf("unexpected status %q", series.Status)
	}
	if len(series.GenreNames) != 3 {
		t.Fatalf("expected 3 genres, got %d", len(series.GenreNames))
	}
	if len(series.EpisodeRefs) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(series.EpisodeRefs))
	}
	if series.EpisodeRefs[0].Number != "28" {
		t.Fatalf("unexpected first episode number %q", series.EpisodeRefs[0].Number)
	}
}

func TestParseEpisodeHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<h1 class="entry-title">Tales of Herding Gods Episode 76 Subtitle Indonesia</h1>
<div id="embed_holder"><div class="player-embed" id="pembed"><iframe src="https://anichin.stream/?id=v75kz3s"></iframe></div></div>
<select class="mirror" name="mirror">
  <option value="">Select Video Server</option>
  <option value="PGlmcmFtZSBzcmM9Imh0dHBzOi8vb2sucnUvZW1iZWQvYWJjIj48L2lmcmFtZT4=" data-index="1">OK.ru</option>
  <option value="PGlmcmFtZSBzcmM9Imh0dHBzOi8vcnVtYmxlLmNvbS9lbWJlZC94eXoiPjwvaWZyYW1lPg==" data-index="2">Rumble</option>
</select>
<div class="bixbox">
  <div class="releases"><h3>Download Tales of Herding Gods Episode 76 Subtitle Indonesia</h3></div>
  <div class="mctnx">
    <div class="soraddlx soradlg">
      <div class="soraurlx">
        <strong>360p</strong>
        <a href="https://www.mirrored.to/multilinks/e67a870f74">Mirrored</a>
        <a href="https://1024terabox.com/s/18Jhqmp1OUNuhYM4B0zdrAg">Terabox</a>
      </div>
      <div class="soraurlx">
        <strong>4K</strong>
        <a href="https://www.mirrored.to/files/1D9739CC/file_links">Mirrored</a>
      </div>
    </div>
  </div>
</div>
`)

	detail, err := ParseEpisodeHTML(raw, "https://anichin.cafe/tales-of-herding-gods-episode-76-subtitle-indonesia/")
	if err != nil {
		t.Fatalf("ParseEpisodeHTML returned error: %v", err)
	}
	if detail.StreamURL != "https://anichin.stream/?id=v75kz3s" {
		t.Fatalf("unexpected stream url %q", detail.StreamURL)
	}
	if detail.EpisodeNumber != 76 {
		t.Fatalf("unexpected episode number %v", detail.EpisodeNumber)
	}
	if detail.StreamMirrors["OK.ru"] != "https://ok.ru/embed/abc" {
		t.Fatalf("unexpected OK mirror %#v", detail.StreamMirrors)
	}
	if detail.DownloadLinks["360p"]["Terabox"] != "https://1024terabox.com/s/18Jhqmp1OUNuhYM4B0zdrAg" {
		t.Fatalf("unexpected 360p downloads %#v", detail.DownloadLinks["360p"])
	}
}
