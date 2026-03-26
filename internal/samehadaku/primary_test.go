package samehadaku

import "testing"

func TestParsePrimaryAnimeHTML(t *testing.T) {
	raw := []byte(`
<html>
  <head><link rel="canonical" href="https://v2.samehadaku.how/anime/demo-anime/" /></head>
  <body>
    <div class="infoanime widget_senction">
      <h2 class="entry-title">Nonton Anime Demo Anime</h2>
      <div class="thumb"><img src="https://v2.samehadaku.how/poster.jpg" /></div>
      <div class="infox">
        <div class="desc"><div class="entry-content">Synopsis demo anime.</div></div>
        <div class="genre-info"><a>Action</a><a>Fantasy</a></div>
      </div>
    </div>
    <a class="play-new-episode" href="https://v2.samehadaku.how/demo-anime-episode-12-end/"></a>
    <a class="downloadbatch" href="https://gofile.io/d/demo-anime-batch">Download Batch</a>
    <div class="whites lsteps widget_senction">
      <div class="lstepsiode listeps">
        <ul>
          <li><div class="epsright"><span class="eps"><a href="https://v2.samehadaku.how/demo-anime-episode-2/">2</a></span></div><div class="epsleft"><span class="lchx"><a href="https://v2.samehadaku.how/demo-anime-episode-2/">Demo Anime Episode 2</a></span><span class="date">2 Jan 2026</span></div></li>
          <li><div class="epsright"><span class="eps"><a href="https://v2.samehadaku.how/demo-anime-episode-1/">1</a></span></div><div class="epsleft"><span class="lchx"><a href="https://v2.samehadaku.how/demo-anime-episode-1/">Demo Anime Episode 1</a></span><span class="date">1 Jan 2026</span></div></li>
        </ul>
      </div>
    </div>
    <div class="anim-senct">
      <div class="right-senc">
        <div class="spe">
          <span><b>Status</b> Completed</span>
          <span><b>Studio</b> <a href="/studio/demo/">Demo Studio</a></span>
        </div>
      </div>
    </div>
  </body>
</html>`)

	page, err := ParsePrimaryAnimeHTML(raw, "https://v2.samehadaku.how/anime/demo-anime/")
	if err != nil {
		t.Fatalf("ParsePrimaryAnimeHTML returned error: %v", err)
	}
	if page.CanonicalURL != "https://v2.samehadaku.how/anime/demo-anime/" {
		t.Fatalf("unexpected canonical url %q", page.CanonicalURL)
	}
	if len(page.Episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(page.Episodes))
	}
	if page.Episodes[0].CanonicalURL != "https://v2.samehadaku.how/demo-anime-episode-2/" {
		t.Fatalf("unexpected first episode url %q", page.Episodes[0].CanonicalURL)
	}
	if page.LatestEpisode != "https://v2.samehadaku.how/demo-anime-episode-12-end/" {
		t.Fatalf("unexpected latest episode %q", page.LatestEpisode)
	}
	if len(page.Studios) != 1 || page.Studios[0] != "Demo Studio" {
		t.Fatalf("unexpected studios %#v", page.Studios)
	}
	if page.BatchLinks["Download Batch"] != "https://gofile.io/d/demo-anime-batch" {
		t.Fatalf("unexpected batch links %#v", page.BatchLinks)
	}
}

func TestParsePrimaryEpisodeHTML(t *testing.T) {
	raw := []byte(`
<html>
  <head>
    <link rel="canonical" href="https://v2.samehadaku.how/demo-anime-episode-1/" />
    <meta property="article:published_time" content="2026-01-01T00:00:00+00:00" />
    <meta property="og:image" content="https://v2.samehadaku.how/poster-episode.jpg" />
  </head>
  <body>
    <div class="player-area">
      <h1 class="entry-title">Demo Anime Episode 1 Sub Indo</h1>
      <span class="epx">Episode <span itemprop="episodeNumber">1</span></span>
    </div>
    <div class="east_player_option" data-post="123" data-nume="1" data-type="schtml"><span>Blogspot 360p</span></div>
    <div class="east_player_option" data-post="123" data-nume="2" data-type="schtml"><span>Mega 720p</span></div>
    <div class="naveps">
      <div class="nvs"><a href="https://v2.samehadaku.how/demo-anime-episode-0/"></a></div>
      <div class="nvs nvsc"><a href="https://v2.samehadaku.how/anime/demo-anime/">All Episode</a></div>
      <div class="nvs rght"><a href="https://v2.samehadaku.how/demo-anime-episode-2/"></a></div>
    </div>
    <div class="download-eps">
      <p><b>MKV</b></p>
      <ul>
        <li><strong>720p</strong><span><a href="https://gofile.io/d/demo">Gofile</a></span><span><a href="https://krakenfiles.com/demo">Krakenfiles</a></span></li>
      </ul>
    </div>
    <div class="episodeinf">
      <div class="thumb"><img src="https://v2.samehadaku.how/poster-series.jpg" /></div>
      <div class="desc"><div class="entry-content">Series synopsis.</div></div>
      <div class="genre-info"><a>Drama</a><a>Music</a></div>
    </div>
  </body>
</html>`)

	page, err := ParsePrimaryEpisodeHTML(raw, "https://v2.samehadaku.how/demo-anime-episode-1/")
	if err != nil {
		t.Fatalf("ParsePrimaryEpisodeHTML returned error: %v", err)
	}
	if page.CanonicalURL != "https://v2.samehadaku.how/demo-anime-episode-1/" {
		t.Fatalf("unexpected canonical url %q", page.CanonicalURL)
	}
	if page.EpisodeNumber != 1 {
		t.Fatalf("unexpected episode number %v", page.EpisodeNumber)
	}
	if page.AnimeURL != "https://v2.samehadaku.how/anime/demo-anime/" {
		t.Fatalf("unexpected anime url %q", page.AnimeURL)
	}
	if len(page.StreamOptions) != 2 {
		t.Fatalf("unexpected stream options %#v", page.StreamOptions)
	}
	if page.DirectDownloads["MKV"]["720p"]["Gofile"] != "https://gofile.io/d/demo" {
		t.Fatalf("unexpected download payload %#v", page.DirectDownloads)
	}
	if page.SeriesSynopsis != "Series synopsis." {
		t.Fatalf("unexpected synopsis %q", page.SeriesSynopsis)
	}
}
