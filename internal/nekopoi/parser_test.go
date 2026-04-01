package nekopoi

import "testing"

func TestParseFeedXML(t *testing.T) {
	t.Parallel()

	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>NekoPoi</title>
    <item>
      <title>[L2D] Compilation Naruto Melahap Semua Gadis Konoha Part 2 – NARUTO</title>
      <link>https://nekopoi.care/compilation-naruto-part-2/</link>
      <pubDate>Mon, 30 Mar 2026 12:34:56 +0700</pubDate>
      <category>2D Animation</category>
      <category>Action</category>
      <description><![CDATA[<p>Animasi naruto terbaru.</p>]]></description>
      <content:encoded><![CDATA[
        <p><img src="https://cdn.nekopoi.care/poster.jpg" /></p>
        <p><strong>Original Title</strong>: Naruto Special</p>
        <p><strong>Parody</strong>: Naruto</p>
        <p><strong>Producer</strong>: Studio Neko</p>
        <p><strong>Duration</strong>: 23 min</p>
        <p><strong>Genre</strong>: Action, Hentai</p>
        <p><strong>Size</strong>: 480 MB</p>
      ]]></content:encoded>
    </item>
    <item>
      <title>[UNCENSORED] PMTC026</title>
      <link>https://nekopoi.care/pmtc026/</link>
      <pubDate>Tue, 31 Mar 2026 01:02:03 +0700</pubDate>
      <category>JAV</category>
      <content:encoded><![CDATA[
        <p><strong>Original Title</strong>: PMTC026</p>
        <p><strong>Actress</strong>: Example Actress</p>
        <p><strong>Nuclear Code</strong>: 123456</p>
      ]]></content:encoded>
    </item>
  </channel>
</rss>`)

	items, err := ParseFeedXML(raw, "https://nekopoi.care/feed/")
	if err != nil {
		t.Fatalf("ParseFeedXML returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Slug != "compilation-naruto-part-2" {
		t.Fatalf("unexpected slug %q", items[0].Slug)
	}
	if items[0].CoverURL != "https://cdn.nekopoi.care/poster.jpg" {
		t.Fatalf("unexpected cover url %q", items[0].CoverURL)
	}
	if items[0].ContentFormat != "animation_2d" {
		t.Fatalf("unexpected content format %q", items[0].ContentFormat)
	}
	if items[0].NormalizedTitle != "Compilation Naruto Melahap Semua Gadis Konoha Part 2 – NARUTO" {
		t.Fatalf("unexpected normalized title %q", items[0].NormalizedTitle)
	}
	if len(items[0].TitleLabels) != 1 || items[0].TitleLabels[0] != "L2D" {
		t.Fatalf("unexpected title labels %#v", items[0].TitleLabels)
	}
	if items[0].EntryKind != "compilation" {
		t.Fatalf("unexpected entry kind %q", items[0].EntryKind)
	}
	if items[0].PartNumber != 2 {
		t.Fatalf("unexpected part number %d", items[0].PartNumber)
	}
	if len(items[0].Genres) != 2 {
		t.Fatalf("expected 2 genres, got %#v", items[0].Genres)
	}
	if items[1].ContentFormat != "live_action" {
		t.Fatalf("unexpected content format %q", items[1].ContentFormat)
	}
	if items[1].NuclearCode != "123456" {
		t.Fatalf("unexpected nuclear code %q", items[1].NuclearCode)
	}
}

func TestParseFeedXMLDerivesPreviewEpisodeMetadata(t *testing.T) {
	t.Parallel()

	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <item>
      <title>[PV] Seifuku wa Kita mama de Episode 2</title>
      <link>https://nekopoi.care/seifuku-episode-2/</link>
      <pubDate>Tue, 31 Mar 2026 01:02:03 +0700</pubDate>
      <category>2D Animation</category>
    </item>
  </channel>
</rss>`)

	items, err := ParseFeedXML(raw, "https://nekopoi.care/feed/")
	if err != nil {
		t.Fatalf("ParseFeedXML returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].NormalizedTitle != "Seifuku wa Kita mama de Episode 2" {
		t.Fatalf("unexpected normalized title %q", items[0].NormalizedTitle)
	}
	if items[0].EntryKind != "preview" {
		t.Fatalf("unexpected entry kind %q", items[0].EntryKind)
	}
	if items[0].EpisodeNumber != 2 {
		t.Fatalf("unexpected episode number %d", items[0].EpisodeNumber)
	}
	if items[0].SeriesCandidate {
		t.Fatal("preview episode should not be treated as series candidate")
	}
}

func TestParseDetailHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<html>
  <head>
    <meta name="description" content="Example excerpt">
    <meta property="og:image" content="https://nekopoi.care/wp-content/uploads/example.jpg">
  </head>
  <body class="single postid-58415">
    <div id="nk-stream-1" class="nk-player-frame"><iframe src="https://stream.example/1"></iframe></div>
    <div id="nk-stream-2" class="nk-player-frame"><iframe src="https://cdn.stream.example/embed/2"></iframe></div>
    <div class="nk-download-row">
      <div class="nk-download-name">Example Title [1080p]</div>
      <div class="nk-download-links">
        <a href="https://krakenfiles.com/file">KrakenFiles</a>
        <a href="https://www.mp4upload.com/file">Mp4Upload</a>
      </div>
    </div>
    <div class="nk-download-row">
      <div class="nk-download-name">Example Title [720p]</div>
      <div class="nk-download-links">
        <a href="https://pixeldrain.com/file">Pixeldrain</a>
      </div>
    </div>
    <div class="nk-download-row"></div>
  </body>
</html>`)

	meta, err := ParseDetailHTML(raw)
	if err != nil {
		t.Fatalf("ParseDetailHTML returned error: %v", err)
	}
	if meta.CoverURL != "https://nekopoi.care/wp-content/uploads/example.jpg" {
		t.Fatalf("unexpected cover url %q", meta.CoverURL)
	}
	if meta.PostID != "58415" {
		t.Fatalf("unexpected post id %q", meta.PostID)
	}
	if meta.Excerpt != "Example excerpt" {
		t.Fatalf("unexpected excerpt %q", meta.Excerpt)
	}
	if meta.PlayerCount != 2 {
		t.Fatalf("unexpected player count %d", meta.PlayerCount)
	}
	if len(meta.PlayerHosts) != 2 {
		t.Fatalf("unexpected player hosts %#v", meta.PlayerHosts)
	}
	if meta.DownloadCount != 3 {
		t.Fatalf("unexpected download count %d", meta.DownloadCount)
	}
	if len(meta.DownloadLabels) != 2 {
		t.Fatalf("unexpected download labels %#v", meta.DownloadLabels)
	}
	if len(meta.DownloadHosts) != 3 {
		t.Fatalf("unexpected download hosts %#v", meta.DownloadHosts)
	}
}
