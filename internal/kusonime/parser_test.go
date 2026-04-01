package kusonime

import "testing"

func TestParseSearchHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<div class="rseries">
  <div class="rapi">
    <div class="kover">
      <div class="detpost">
        <div class="thumb">
          <a href="https://kusonime.com/kusuriyanohitorigoto-s2-batch-subtitle-indonesia/">
            <div class="thumbz">
              <img src="https://kusonime.com/wp-content/uploads/2025/01/kusuriya-s2.jpg" />
            </div>
          </a>
        </div>
        <div class="content">
          <h2 class='episodeye'>
            <a href="https://kusonime.com/kusuriyanohitorigoto-s2-batch-subtitle-indonesia/" title="Kusuriya no Hitorigoto Season 2 BD Batch Subtitle Indonesia">
              Kusuriya no Hitorigoto Season 2 BD Batch Subtitle Indonesia
            </a>
          </h2>
          <p><i class="fa fa-tag"></i> Genre <a rel="tag">Drama</a>, <a rel="tag">Historical</a>, <a rel="tag">Mystery</a></p>
        </div>
      </div>
    </div>
    <div class="kover">
      <div class="detpost">
        <div class="thumb">
          <a href="https://kusonime.com/kusuriyanohitorigoto-batch-subtitle-indonesia/">
            <div class="thumbz">
              <img src="https://kusonime.com/wp-content/uploads/2023/10/kusuriya.jpg" />
            </div>
          </a>
        </div>
        <div class="content">
          <h2 class='episodeye'>
            <a href="https://kusonime.com/kusuriyanohitorigoto-batch-subtitle-indonesia/" title="Kusuriya no Hitorigoto BD Batch Subtitle Indonesia">
              Kusuriya no Hitorigoto BD Batch Subtitle Indonesia
            </a>
          </h2>
          <p><i class="fa fa-tag"></i> Genre <a rel="tag">Drama</a>, <a rel="tag">Historical</a>, <a rel="tag">Mystery</a></p>
        </div>
      </div>
    </div>
  </div>
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
		t.Fatalf("unexpected first title %q", results[0].Title)
	}
	if results[0].PosterURL == "" {
		t.Fatal("expected first poster url to be present")
	}
	if len(results[0].Genres) != 3 {
		t.Fatalf("len(results[0].Genres) = %d, want 3", len(results[0].Genres))
	}
}

func TestParseAnimeHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<html>
<head>
  <title>Kusuriya no Hitorigoto Batch Subtitle Indonesia | Kusonime</title>
  <meta property="article:published_time" content="2024-08-10T02:30:56+00:00" />
  <meta property="article:modified_time" content="2025-04-05T07:20:01+00:00" />
  <link rel="canonical" href="https://kusonime.com/kusuriyanohitorigoto-batch-subtitle-indonesia/" />
</head>
<body>
  <div class="post-thumb">
    <img src="https://kusonime.com/wp-content/uploads/2023/10/Kusuriya-no-Hitorigoto.jpg" />
    <h1 class="jdlz">Kusuriya no Hitorigoto BD Batch Subtitle Indonesia</h1>
  </div>
  <div class="lexot">
    <div class="info">
      <p><b>Japanese</b>: 薬屋のひとりごと</p>
      <p><b>Genre </b>: <a href="/genres/drama/" rel="tag">Drama</a>, <a href="/genres/historical/" rel="tag">Historical</a>, <a href="/genres/mystery/" rel="tag">Mystery</a></p>
      <p><b>Seasons </b>: <a href="/seasons/fall-2023/" rel="tag">Fall 2023</a></p>
      <p><b>Producers</b>: Dentsu, Square Enix, TOHO animation</p>
      <p><b>Type</b>: BD</p>
      <p><b>Status</b>: Completed</p>
      <p><b>Total Episode</b>: 24</p>
      <p><b>Score</b>: 8.89</p>
      <p><b>Duration</b>: 22 min. per ep.</p>
      <p><b>Released on</b>: Oct 22, 2023</p>
    </div>
    <div class="clear"></div>
    <p><strong>Kusuriya no Hitorigoto</strong> Ceritanya berlokasi di sebuah negara besar di tengah benua.</p>
    <p>Suatu hari, Maomao mengetahui fakta bahwa semua anak raja hanya memiliki umur pendek.</p>
    <p>Subtitle : Haruzorasubs</p>
    <p>Download Kusuriya no Hitorigoto BD Batch Sub Indo.</p>
    <div class="dlbodz">
      <div class="smokeddlrh">
        <div class="smokettlrh">Download Kusuriya no Hitorigoto Episode 01-12 Batch BD Subtitle Indonesia</div>
        <div class="smokeurlrh">
          <strong>360P</strong>
          <a href="https://drive.example/360">Google Drive</a> |
          <a href="https://mega.example/360">Mega.nz</a>
        </div>
        <div class="smokeurlrh">
          <strong>720P</strong>
          <a href="https://drive.example/720">Google Drive</a> |
          <a href="https://pixeldrain.example/720">Pixeldrain</a>
        </div>
      </div>
      <div class="smokeddlrh">
        <div class="smokettlrh">Download Kusuriya no Hitorigoto Episode 13-24 Batch BD Subtitle Indonesia</div>
        <div class="smokeurlrh">
          <strong>480P</strong>
          <a href="https://drive.example/480">Google Drive</a>
        </div>
      </div>
    </div>
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
	if page.BatchType != "BD" {
		t.Fatalf("unexpected batch type %q", page.BatchType)
	}
	if page.Season != "Fall 2023" {
		t.Fatalf("unexpected season %q", page.Season)
	}
	if page.TotalEpisodes != "24" {
		t.Fatalf("unexpected total episodes %q", page.TotalEpisodes)
	}
	if len(page.Genres) != 3 {
		t.Fatalf("len(page.Genres) = %d, want 3", len(page.Genres))
	}
	if len(page.Batches) != 2 {
		t.Fatalf("len(page.Batches) = %d, want 2", len(page.Batches))
	}
	if page.Batches[0].Downloads["360P"]["Mega.nz"] != "https://mega.example/360" {
		t.Fatalf("unexpected 360P Mega url %q", page.Batches[0].Downloads["360P"]["Mega.nz"])
	}
	if page.Synopsis == "" {
		t.Fatal("expected synopsis to be present")
	}
	if page.PublishedAt != "2024-08-10T02:30:56+00:00" {
		t.Fatalf("unexpected published time %q", page.PublishedAt)
	}
}
