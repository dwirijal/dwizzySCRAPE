package samehadaku

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestResolvePlayerOption(t *testing.T) {
	t.Helper()

	var (
		gotMethod  string
		gotPath    string
		gotBody    string
		gotReferer string
		gotOrigin  string
		gotCookie  string
		gotXHR     string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody = string(body)
		gotReferer = r.Header.Get("Referer")
		gotOrigin = r.Header.Get("Origin")
		gotCookie = r.Header.Get("Cookie")
		gotXHR = r.Header.Get("X-Requested-With")

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<div class="player"><iframe src="https://video.example/embed-1"></iframe></div>`))
	}))
	defer server.Close()

	client := &HTTPClient{
		client:    server.Client(),
		userAgent: "dwizzyscrape-test",
		cookie:    "cf_clearance=test-token",
	}
	refererURL := server.URL + "/demo-anime-episode-1/"
	option := PrimaryServerOption{
		Label:  "Blogspot 360p",
		PostID: "46236",
		Number: "1",
		Type:   "schtml",
	}

	resolution, err := client.ResolvePlayerOption(context.Background(), refererURL, option)
	if err != nil {
		t.Fatalf("ResolvePlayerOption returned error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST method, got %q", gotMethod)
	}
	if gotPath != "/wp-admin/admin-ajax.php" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
	form, err := url.ParseQuery(gotBody)
	if err != nil {
		t.Fatalf("parse form body: %v", err)
	}
	if form.Get("action") != "player_ajax" {
		t.Fatalf("unexpected action %q", form.Get("action"))
	}
	if form.Get("post") != "46236" || form.Get("nume") != "1" || form.Get("type") != "schtml" {
		t.Fatalf("unexpected form payload %#v", form)
	}
	if gotReferer != refererURL {
		t.Fatalf("unexpected referer %q", gotReferer)
	}
	if gotOrigin != server.URL {
		t.Fatalf("unexpected origin %q", gotOrigin)
	}
	if gotCookie != "cf_clearance=test-token" {
		t.Fatalf("unexpected cookie %q", gotCookie)
	}
	if gotXHR != "XMLHttpRequest" {
		t.Fatalf("unexpected X-Requested-With header %q", gotXHR)
	}
	if resolution.Status != "resolved" {
		t.Fatalf("unexpected resolution status %q", resolution.Status)
	}
	if resolution.EmbedURL != "https://video.example/embed-1" {
		t.Fatalf("unexpected embed url %q", resolution.EmbedURL)
	}
	if resolution.Label != option.Label {
		t.Fatalf("unexpected label %q", resolution.Label)
	}
}

func TestParsePlayerEmbedHTMLRequiresIframe(t *testing.T) {
	t.Helper()

	_, err := ParsePlayerEmbedHTML([]byte(`<div class="player-empty"></div>`))
	if err == nil {
		t.Fatal("expected error when iframe src is missing")
	}
	if !strings.Contains(err.Error(), "missing iframe src") {
		t.Fatalf("unexpected error %q", err)
	}
}
