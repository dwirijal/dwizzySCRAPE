package samehadaku

import "testing"

func TestParseMirrorAnimeHTML(t *testing.T) {
	raw := []byte(`
<html>
  <head><link rel="canonical" href="https://samehadaku.li/anime/ao-no-orchestra-season-2/" /></head>
  <body>
    <div class="animefull">
      <div class="thumb"><img src="https://samehadaku.li/poster.jpg" /></div>
      <a class="trailerbutton" href="https://www.youtube.com/watch?v=demo">Trailer</a>
      <div class="infox">
        <h1 class="entry-title">Ao no Orchestra Season 2</h1>
      </div>
      <div class="spe">
        <span><b>Status:</b> Ongoing</span>
        <span><b>Studio:</b> <a href="/studio/nippon-animation/">Nippon Animation</a></span>
      </div>
      <div class="genxed"><a>Drama</a><a>Music</a></div>
    </div>
    <div class="bixbox synp"><div class="entry-content"><p>Second season of Ao no Orchestra.</p></div></div>
    <a class="batchdl" href="https://mega.nz/file/demo_batch#key">Download Batch</a>
    <div class="cvlist">
      <div class="cvitem">
        <div class="cvsubitem cvchar">
          <img src="https://samehadaku.li/char.jpg" />
          <span class="charname">Aono, Hajime</span>
          <span class="charrole">Main</span>
        </div>
        <div class="cvsubitem cvactor">
          <img src="https://samehadaku.li/actor.jpg" />
          <span class="charname">Chiba, Shouya</span>
          <span class="charrole">Japanese</span>
        </div>
      </div>
    </div>
    <div class="eplister">
      <ul>
        <li><a href="https://samehadaku.li/ao-no-orchestra-season-2-episode-2-subtitle-indonesia/"><div class="epl-num">2</div><div class="epl-title">Ao no Orchestra Season 2 Episode 2 Subtitle Indonesia</div><div class="epl-date">October 11, 2025</div></a></li>
        <li><a href="https://samehadaku.li/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/"><div class="epl-num">1</div><div class="epl-title">Ao no Orchestra Season 2 Episode 1 Subtitle Indonesia</div><div class="epl-date">October 5, 2025</div></a></li>
      </ul>
    </div>
    <span class="epcur epcurfirst">Episode 1</span>
    <span class="epcur epcurlast">Episode 2</span>
  </body>
</html>`)

	page, err := ParseMirrorAnimeHTML(raw, "https://samehadaku.li/anime/ao-no-orchestra-season-2/")
	if err != nil {
		t.Fatalf("ParseMirrorAnimeHTML returned error: %v", err)
	}
	if page.Title != "Ao no Orchestra Season 2" {
		t.Fatalf("unexpected title %q", page.Title)
	}
	if page.TrailerURL != "https://www.youtube.com/watch?v=demo" {
		t.Fatalf("unexpected trailer %q", page.TrailerURL)
	}
	if len(page.Episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(page.Episodes))
	}
	if page.Episodes[0].EpisodeNumber != 2 {
		t.Fatalf("unexpected episode number %v", page.Episodes[0].EpisodeNumber)
	}
	if len(page.Cast) != 1 || page.Cast[0].ActorName != "Chiba, Shouya" {
		t.Fatalf("unexpected cast payload %#v", page.Cast)
	}
	if page.BatchLinks["Download Batch"] != "https://mega.nz/file/demo_batch#key" {
		t.Fatalf("unexpected batch links %#v", page.BatchLinks)
	}
}

func TestParseMirrorEpisodeHTML(t *testing.T) {
	raw := []byte(`
<html>
  <head>
    <link rel="canonical" href="https://samehadaku.li/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/" />
    <meta property="article:published_time" content="2025-10-05T12:52:28+00:00" />
  </head>
  <body>
    <article>
      <div class="item meta">
        <div class="tb"><img src="https://samehadaku.li/poster.jpg" /></div>
        <div class="lm">
          <h1 class="entry-title">Ao no Orchestra Season 2 Episode 1 Subtitle Indonesia</h1>
          <meta itemprop="episodeNumber" content="1" />
          <span class="year">series <a href="https://samehadaku.li/anime/ao-no-orchestra-season-2/">Ao no Orchestra Season 2</a></span>
        </div>
      </div>
      <div class="player-embed"><iframe src="https://video.example/embed/1"></iframe></div>
      <select class="mirror">
        <option value="">Select</option>
        <option value="PGlmcmFtZSBzcmM9Imh0dHBzOi8vdmlkZW8uZXhhbXBsZS9lbWJlZC8xIj48L2lmcmFtZT4=">Video</option>
      </select>
      <a href="https://gofile.io/d/demo" aria-label="Download">Download</a>
      <div class="naveps">
        <a rel="next" href="https://samehadaku.li/ao-no-orchestra-season-2-episode-2-subtitle-indonesia/">Next</a>
        <div class="nvsc"><a href="https://samehadaku.li/anime/ao-no-orchestra-season-2/">All Episodes</a></div>
      </div>
      <div class="bixbox infx">Download episode summary.</div>
      <div class="single-info">
        <div class="infox">
          <div class="infolimit"><h2>Ao no Orchestra Season 2</h2></div>
          <div class="spe">
            <span><b>Status:</b> Ongoing</span>
            <span><b>Studio:</b> <a href="/studio/nippon-animation/">Nippon Animation</a></span>
          </div>
          <div class="genxed"><a>Drama</a><a>Music</a></div>
          <div class="desc">Second season of Ao no Orchestra.</div>
        </div>
      </div>
    </article>
  </body>
</html>`)

	page, err := ParseMirrorEpisodeHTML(raw, "https://samehadaku.li/ao-no-orchestra-season-2-episode-1-subtitle-indonesia/")
	if err != nil {
		t.Fatalf("ParseMirrorEpisodeHTML returned error: %v", err)
	}
	if page.EpisodeNumber != 1 {
		t.Fatalf("unexpected episode number %v", page.EpisodeNumber)
	}
	if page.StreamMirrors["Video"] != "https://video.example/embed/1" {
		t.Fatalf("unexpected mirrors %#v", page.StreamMirrors)
	}
	if page.DirectDownloads["Download"] != "https://gofile.io/d/demo" {
		t.Fatalf("unexpected downloads %#v", page.DirectDownloads)
	}
	if page.AllEpisodesURL != "https://samehadaku.li/anime/ao-no-orchestra-season-2/" {
		t.Fatalf("unexpected all episodes url %q", page.AllEpisodesURL)
	}
}
