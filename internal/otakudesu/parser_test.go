package otakudesu

import "testing"

func TestParseSearchHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<div class="page">
  <ul class="chivsrc">
    <li style='list-style:none;'>
      <h2><a href="https://otakudesu.blog/anime/kusuriya-hitorigoto-s2-sub-indo/">Kusuriya no Hitorigoto Season 2 (Episode 1 - 24) Subtitle Indonesia</a></h2>
      <div class="set"><b>Status</b> : Completed</div>
    </li>
    <li style='list-style:none;'>
      <h2><a href="https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/">Kusuriya no Hitorigoto (Episode 1 - 24) Subtitle Indonesia</a></h2>
      <div class="set"><b>Status</b> : Completed</div>
    </li>
  </ul>
</div>
`)

	results, err := ParseSearchHTML(raw)
	if err != nil {
		t.Fatalf("ParseSearchHTML returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Title != "Kusuriya no Hitorigoto Season 2" {
		t.Fatalf("unexpected title %q", results[0].Title)
	}
	if results[0].URL != "https://otakudesu.blog/anime/kusuriya-hitorigoto-s2-sub-indo/" {
		t.Fatalf("unexpected url %q", results[0].URL)
	}
}

func TestParseAnimeHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<html>
  <head>
    <link rel="canonical" href="https://otakudesu.blog/anime/kusuriya-hitorigoto-sub-indo/" />
    <title>Kusuriya no Hitorigoto (Episode 1 - 24) Subtitle Indonesia | Otaku Desu</title>
  </head>
  <body>
    <div class="episodelist">
      <ul>
        <li><span><a href="https://otakudesu.blog/episode/knh-episode-2-sub-indo/">Kusuriya no Hitorigoto Episode 2 Subtitle Indonesia</a></span></li>
        <li><span><a href="https://otakudesu.blog/episode/knh-episode-1-sub-indo/">Kusuriya no Hitorigoto Episode 1 Subtitle Indonesia</a></span></li>
      </ul>
    </div>
  </body>
</html>
`)

	page, err := ParseAnimeHTML(raw)
	if err != nil {
		t.Fatalf("ParseAnimeHTML returned error: %v", err)
	}
	if page.Title != "Kusuriya no Hitorigoto" {
		t.Fatalf("unexpected title %q", page.Title)
	}
	if len(page.Episodes) != 2 {
		t.Fatalf("len(page.Episodes) = %d, want 2", len(page.Episodes))
	}
	if page.Episodes[0].Number != "2" {
		t.Fatalf("unexpected episode number %q", page.Episodes[0].Number)
	}
	if page.Episodes[1].URL != "https://otakudesu.blog/episode/knh-episode-1-sub-indo/" {
		t.Fatalf("unexpected episode url %q", page.Episodes[1].URL)
	}
}

func TestParseEpisodeHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<html>
  <head>
    <title>Kusuriya no Hitorigoto Episode 1 Subtitle Indonesia | Otaku Desu</title>
  </head>
  <body>
    <div class="responsive-embed-stream">
      <iframe src="https://desustream.info/dstream/desudesu3/v2/index.php?id=demo"></iframe>
    </div>
    <div class="mirrorstream">
      <a href="#mirror" data-content="eyJwb3N0IjoiMTYwNDYzIiwibnVtYmVyIjoiMiJ9">Mirror 2</a>
    </div>
    <div class="download">
      <ul>
        <li><strong>360p</strong>
          <a href="https://link.desustream.com/?id=one">OtakuFiles</a>
          <a href="https://link.desustream.com/?id=two">Mega</a>
        </li>
        <li><strong>480p</strong>
          <a href="https://link.desustream.com/?id=three">OtakuFiles</a>
        </li>
      </ul>
    </div>
  </body>
</html>
`)

	page, err := ParseEpisodeHTML(raw, "https://otakudesu.blog/episode/knh-episode-1-sub-indo/")
	if err != nil {
		t.Fatalf("ParseEpisodeHTML returned error: %v", err)
	}
	if page.Title != "Kusuriya no Hitorigoto" {
		t.Fatalf("unexpected title %q", page.Title)
	}
	if page.Number != "1" {
		t.Fatalf("unexpected number %q", page.Number)
	}
	if page.StreamURL != "https://desustream.info/dstream/desudesu3/v2/index.php?id=demo" {
		t.Fatalf("unexpected stream url %q", page.StreamURL)
	}
	if len(page.MirrorRequests) != 1 {
		t.Fatalf("len(page.MirrorRequests) = %d, want 1", len(page.MirrorRequests))
	}
	if page.MirrorRequests[0].Label != "Mirror 2" {
		t.Fatalf("unexpected mirror label %q", page.MirrorRequests[0].Label)
	}
	if page.DownloadLinks["360p"]["OtakuFiles"] != "https://link.desustream.com/?id=one" {
		t.Fatalf("unexpected 360p OtakuFiles link %#v", page.DownloadLinks)
	}
	if page.DownloadLinks["480p"]["OtakuFiles"] != "https://link.desustream.com/?id=three" {
		t.Fatalf("unexpected 480p OtakuFiles link %#v", page.DownloadLinks)
	}
	if len(page.DownloadURLs) != 3 {
		t.Fatalf("len(page.DownloadURLs) = %d, want 3", len(page.DownloadURLs))
	}
	if page.DownloadURLs[0] != "https://link.desustream.com/?id=one" {
		t.Fatalf("unexpected first download url %q", page.DownloadURLs[0])
	}
}

func TestParseEmbedHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`<div id="pembed"><iframe src="https://mirror.otakudesu.example/embed/two"></iframe></div>`)
	if got := ParseEmbedHTML(raw); got != "https://mirror.otakudesu.example/embed/two" {
		t.Fatalf("ParseEmbedHTML = %q, want expected iframe src", got)
	}
}
