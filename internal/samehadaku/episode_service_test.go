package samehadaku

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type fakeEpisodeWriter struct {
	details []EpisodeDetail
	err     error
}

func (f *fakeEpisodeWriter) UpsertEpisodeDetails(ctx context.Context, details []EpisodeDetail) (int, error) {
	f.details = append(f.details, details...)
	return len(details), f.err
}

type fakeResolverFetcher struct {
	fakeFetcher
	resolutions map[string]PlayerOptionResolution
}

func (f *fakeResolverFetcher) ResolvePlayerOption(ctx context.Context, refererURL string, option PrimaryServerOption) (PlayerOptionResolution, error) {
	key := option.Label
	if key == "" {
		key = option.PostID + ":" + option.Number + ":" + option.Type
	}
	if resolution, ok := f.resolutions[key]; ok {
		return resolution, nil
	}
	return PlayerOptionResolution{
		Label:      option.Label,
		PostID:     option.PostID,
		Number:     option.Number,
		Type:       option.Type,
		Status:     "resolved",
		EmbedURL:   "",
		SourceKind: "primary",
		ResolvedAt: time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC),
	}, nil
}

func TestEpisodeServiceSyncAnimeEpisodesResolvesPrimaryStreams(t *testing.T) {
	t.Helper()

	const animeURL = "https://v2.samehadaku.how/anime/demo-anime/"
	const episodeURL = "https://v2.samehadaku.how/demo-anime-episode-1/"
	fixedNow := time.Date(2026, 3, 23, 1, 2, 3, 0, time.UTC)

	animeHTML := []byte(`
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
    <div class="whites lsteps widget_senction">
      <div class="lstepsiode listeps">
        <ul>
          <li>
            <div class="epsright"><span class="eps"><a href="https://v2.samehadaku.how/demo-anime-episode-1/">1</a></span></div>
            <div class="epsleft"><span class="lchx"><a href="https://v2.samehadaku.how/demo-anime-episode-1/">Demo Anime Episode 1</a></span><span class="date">1 Jan 2026</span></div>
          </li>
        </ul>
      </div>
    </div>
  </body>
</html>`)

	episodeHTML := []byte(`
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
      <div class="nvs nvsc"><a href="https://v2.samehadaku.how/anime/demo-anime/">All Episode</a></div>
    </div>
    <div class="download-eps">
      <p><b>MKV</b></p>
      <ul>
        <li><strong>720p</strong><span><a href="https://gofile.io/d/demo">Gofile</a></span></li>
      </ul>
    </div>
    <div class="episodeinf">
      <div class="thumb"><img src="https://v2.samehadaku.how/poster-series.jpg" /></div>
      <div class="desc"><div class="entry-content">Series synopsis.</div></div>
      <div class="genre-info"><a>Drama</a><a>Music</a></div>
    </div>
  </body>
</html>`)

	fetcher := &fakeResolverFetcher{
		fakeFetcher: fakeFetcher{
			bodies: map[string][]byte{
				animeURL:   animeHTML,
				episodeURL: episodeHTML,
			},
		},
		resolutions: map[string]PlayerOptionResolution{
			"Blogspot 360p": {
				Label:      "Blogspot 360p",
				PostID:     "123",
				Number:     "1",
				Type:       "schtml",
				Status:     "resolved",
				EmbedURL:   "https://video.example/blogspot-360p",
				SourceKind: "primary",
				ResolvedAt: fixedNow,
			},
			"Mega 720p": {
				Label:      "Mega 720p",
				PostID:     "123",
				Number:     "2",
				Type:       "schtml",
				Status:     "resolved",
				EmbedURL:   "https://video.example/mega-720p",
				SourceKind: "primary",
				ResolvedAt: fixedNow,
			},
		},
	}
	writer := &fakeEpisodeWriter{}
	service := NewEpisodeService(fetcher, writer, fixedNow)

	report, err := service.SyncAnimeEpisodes(context.Background(), "demo-anime")
	if err != nil {
		t.Fatalf("SyncAnimeEpisodes returned error: %v", err)
	}
	if report.Parsed != 1 || report.Upserted != 1 {
		t.Fatalf("unexpected report %+v", report)
	}
	if len(writer.details) != 1 {
		t.Fatalf("expected 1 detail row, got %d", len(writer.details))
	}

	var stream struct {
		Primary         string                   `json:"primary"`
		Mirrors         map[string]string        `json:"mirrors"`
		ServerOptions   []PrimaryServerOption    `json:"server_options"`
		ResolvedOptions []PlayerOptionResolution `json:"resolved_options"`
	}
	if err := json.Unmarshal(writer.details[0].StreamLinksJSON, &stream); err != nil {
		t.Fatalf("unmarshal stream links: %v", err)
	}
	if stream.Primary != "https://video.example/blogspot-360p" {
		t.Fatalf("unexpected primary stream %q", stream.Primary)
	}
	if len(stream.Mirrors) != 2 {
		t.Fatalf("expected 2 mirrors, got %#v", stream.Mirrors)
	}
	if stream.Mirrors["Mega 720p"] != "https://video.example/mega-720p" {
		t.Fatalf("unexpected mirror payload %#v", stream.Mirrors)
	}
	if len(stream.ServerOptions) != 2 {
		t.Fatalf("expected 2 server options, got %#v", stream.ServerOptions)
	}
	if len(stream.ResolvedOptions) != 2 {
		t.Fatalf("expected 2 resolved options, got %#v", stream.ResolvedOptions)
	}
	if writer.details[0].FetchStatus != "primary_fetched" {
		t.Fatalf("unexpected fetch status %q", writer.details[0].FetchStatus)
	}
	if writer.details[0].CanonicalURL != episodeURL {
		t.Fatalf("unexpected canonical url %q", writer.details[0].CanonicalURL)
	}
}

func TestEpisodeServiceSyncAnimeEpisodesLimitsResolvedPrimaryOptions(t *testing.T) {
	t.Helper()

	const animeURL = "https://v2.samehadaku.how/anime/demo-anime/"
	const episodeURL = "https://v2.samehadaku.how/demo-anime-episode-1/"
	fixedNow := time.Date(2026, 3, 23, 1, 2, 3, 0, time.UTC)

	animeHTML := []byte(`
<html>
  <body>
    <div class="infoanime widget_senction">
      <h2 class="entry-title">Nonton Anime Demo Anime</h2>
    </div>
    <div class="whites lsteps widget_senction">
      <div class="lstepsiode listeps">
        <ul>
          <li>
            <div class="epsright"><span class="eps"><a href="https://v2.samehadaku.how/demo-anime-episode-1/">1</a></span></div>
            <div class="epsleft"><span class="lchx"><a href="https://v2.samehadaku.how/demo-anime-episode-1/">Demo Anime Episode 1</a></span></div>
          </li>
        </ul>
      </div>
    </div>
  </body>
</html>`)

	episodeHTML := []byte(`
<html>
  <body>
    <div class="player-area">
      <h1 class="entry-title">Demo Anime Episode 1 Sub Indo</h1>
      <span class="epx">Episode <span itemprop="episodeNumber">1</span></span>
    </div>
    <div class="east_player_option" data-post="123" data-nume="1" data-type="schtml"><span>Server 1</span></div>
    <div class="east_player_option" data-post="123" data-nume="2" data-type="schtml"><span>Server 2</span></div>
    <div class="east_player_option" data-post="123" data-nume="3" data-type="schtml"><span>Server 3</span></div>
    <div class="east_player_option" data-post="123" data-nume="4" data-type="schtml"><span>Server 4</span></div>
    <div class="naveps">
      <div class="nvs nvsc"><a href="https://v2.samehadaku.how/anime/demo-anime/">All Episode</a></div>
    </div>
    <div class="download-eps">
      <p><b>MKV</b></p>
      <ul>
        <li><strong>720p</strong><span><a href="https://gofile.io/d/demo">Gofile</a></span></li>
      </ul>
    </div>
  </body>
</html>`)

	fetcher := &fakeResolverFetcher{
		fakeFetcher: fakeFetcher{
			bodies: map[string][]byte{
				animeURL:   animeHTML,
				episodeURL: episodeHTML,
			},
		},
		resolutions: map[string]PlayerOptionResolution{
			"Server 1": {Label: "Server 1", PostID: "123", Number: "1", Type: "schtml", Status: "resolved", EmbedURL: "https://video.example/1", SourceKind: "primary", ResolvedAt: fixedNow},
			"Server 2": {Label: "Server 2", PostID: "123", Number: "2", Type: "schtml", Status: "resolved", EmbedURL: "https://video.example/2", SourceKind: "primary", ResolvedAt: fixedNow},
			"Server 3": {Label: "Server 3", PostID: "123", Number: "3", Type: "schtml", Status: "resolved", EmbedURL: "https://video.example/3", SourceKind: "primary", ResolvedAt: fixedNow},
			"Server 4": {Label: "Server 4", PostID: "123", Number: "4", Type: "schtml", Status: "resolved", EmbedURL: "https://video.example/4", SourceKind: "primary", ResolvedAt: fixedNow},
		},
	}
	writer := &fakeEpisodeWriter{}
	service := NewEpisodeService(fetcher, writer, fixedNow)

	if _, err := service.SyncAnimeEpisodes(context.Background(), "demo-anime"); err != nil {
		t.Fatalf("SyncAnimeEpisodes returned error: %v", err)
	}

	var stream struct {
		Mirrors         map[string]string        `json:"mirrors"`
		ResolvedOptions []PlayerOptionResolution `json:"resolved_options"`
	}
	if err := json.Unmarshal(writer.details[0].StreamLinksJSON, &stream); err != nil {
		t.Fatalf("unmarshal stream links: %v", err)
	}
	if len(stream.Mirrors) != 3 {
		t.Fatalf("expected 3 stored mirrors, got %#v", stream.Mirrors)
	}
	if stream.Mirrors["Server 4"] != "" {
		t.Fatalf("expected server 4 to be skipped, got %#v", stream.Mirrors)
	}
	if len(stream.ResolvedOptions) != 4 {
		t.Fatalf("expected all 4 options to be tracked, got %#v", stream.ResolvedOptions)
	}
	if stream.ResolvedOptions[3].Status != "skipped_limit" {
		t.Fatalf("expected fourth option status skipped_limit, got %#v", stream.ResolvedOptions[3])
	}
}
