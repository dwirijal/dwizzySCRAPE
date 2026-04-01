package linkresolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolveFollowsRedirects(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer final.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/file", http.StatusFound)
	}))
	defer redirect.Close()

	resolver := New("test-agent", 5*time.Second)
	result := resolver.Resolve(context.Background(), redirect.URL+"/start")

	if result.Error != "" {
		t.Fatalf("unexpected error %q", result.Error)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", result.StatusCode)
	}
	if result.FinalURL != final.URL+"/file" {
		t.Fatalf("unexpected final url %q", result.FinalURL)
	}
	if len(result.Hops) == 0 || result.Hops[0] != redirect.URL+"/start" {
		t.Fatalf("unexpected hops %#v", result.Hops)
	}
}

func TestResolveDirectURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resolver := New("test-agent", 5*time.Second)
	result := resolver.Resolve(context.Background(), server.URL+"/file")

	if result.Error != "" {
		t.Fatalf("unexpected error %q", result.Error)
	}
	if result.FinalURL != server.URL+"/file" {
		t.Fatalf("unexpected final url %q", result.FinalURL)
	}
	if len(result.Hops) != 0 {
		t.Fatalf("expected no hops, got %#v", result.Hops)
	}
}
