package drakorid

import (
	"strings"
	"testing"
)

func TestParsePageToken(t *testing.T) {
	t.Parallel()

	token, err := ParsePageToken([]byte(`<script>var token_now = "abc123";</script>`))
	if err != nil {
		t.Fatalf("ParsePageToken returned error: %v", err)
	}
	if token != "abc123" {
		t.Fatalf("token = %q, want %q", token, "abc123")
	}
}

func TestParseOngoingHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<div class="card">
  <a href="https://drakorid.co/nonton/bake-your-dream-2026/"><img src="https://example.com/poster.jpg"></a>
  <div class="top-right"><span class="badge badge-primary">Episode 9</span></div>
  <div class="card-body">
    <h5 data-original-title="Bake Your Dream (2026) Episode 9">Bake Your Dream</h5>
  </div>
</div>`)

	items, err := ParseOngoingHTML(raw, "https://drakorid.co/drama-ongoing/", 1)
	if err != nil {
		t.Fatalf("ParseOngoingHTML returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Slug != "bake-your-dream-2026" {
		t.Fatalf("unexpected slug %q", items[0].Slug)
	}
	if items[0].Title != "Bake Your Dream (2026)" {
		t.Fatalf("unexpected title %q", items[0].Title)
	}
	if items[0].Status != "ongoing" {
		t.Fatalf("unexpected status %q", items[0].Status)
	}
}

func TestParseDetailHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<html><head>
<title>Bake Your Dream (2026) Sub Indo - Drakor.id</title>
<meta property="og:image" content="https://example.com/poster.jpg" />
</head><body>
<p style="text-align: center;">Details<br />
Title: Bake Your Dream<br />
Type: TV Program<br />
Format: Reality Program<br />
Genres: Food<br />
Episodes: 10<br />
Aired: Feb 1, 2026 - ?<br />
Original Network: MBN<br />
Duration: 1 hr. 37 min.<br />
Language: Korean<br />
Country: South Korea</p>
<p style="text-align: center;">Sinopsis<br />
Kompetisi kue global.</p>
<a class="chip chip-outline">Variety Show</a>
<select id="formPilihEpisode">
  <option value="0">Klik</option>
  <option value="1">Episode 1</option>
  <option value="2">Episode 2</option>
</select>
<script>
var token = "tok";
var link = "bake-your-dream-2026";
var mId = 4815;
var mTipe = 2;
</script>
</body></html>`)

	detail, err := ParseDetailHTML(raw, "https://drakorid.co/nonton/bake-your-dream-2026/")
	if err != nil {
		t.Fatalf("ParseDetailHTML returned error: %v", err)
	}
	if detail.MediaType != "drama" {
		t.Fatalf("unexpected media type %q", detail.MediaType)
	}
	if detail.Title != "Bake Your Dream (2026)" {
		t.Fatalf("unexpected title %q", detail.Title)
	}
	if detail.Status != "ongoing" {
		t.Fatalf("unexpected status %q", detail.Status)
	}
	if len(detail.EpisodeRefs) != 2 {
		t.Fatalf("expected 2 episode refs, got %d", len(detail.EpisodeRefs))
	}
	if detail.SourceItemID != 4815 {
		t.Fatalf("unexpected source item id %d", detail.SourceItemID)
	}
}

func TestParseWatchHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`<iframe src="https://drakorid.co/player/bunny.php?v=aHR0cHM6Ly9obHMuZHJha29yLmNjL3Rlc3QubTN1OA==&c=aHR0cDovL2ltYWdlcw=="></iframe>`)
	streamURL, meta, err := ParseWatchHTML(raw)
	if err != nil {
		t.Fatalf("ParseWatchHTML returned error: %v", err)
	}
	if streamURL != "https://hls.drakor.cc/test.m3u8" {
		t.Fatalf("unexpected stream url %q", streamURL)
	}
	if !strings.Contains(meta["player"], "player/bunny.php") {
		t.Fatalf("unexpected player meta %#v", meta)
	}
}

func TestParseDownloadHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<a target="_blank" href="https://vod20.drakor.cc/download/abc/drakor.id.360p.demo.mp4">Download 360p (224 MB)</a>
<a target="_blank" href="https://vod20.drakor.cc/download/def/drakor.id.720p.demo.mp4">Download 720p (857 MB)</a>`)

	downloads, err := ParseDownloadHTML(raw)
	if err != nil {
		t.Fatalf("ParseDownloadHTML returned error: %v", err)
	}
	if downloads["360P"]["direct"] != "https://vod20.drakor.cc/download/abc/drakor.id.360p.demo.mp4" {
		t.Fatalf("unexpected 360P download %#v", downloads["360P"])
	}
	if downloads["720P"]["direct"] != "https://vod20.drakor.cc/download/def/drakor.id.720p.demo.mp4" {
		t.Fatalf("unexpected 720P download %#v", downloads["720P"])
	}
}
