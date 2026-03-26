package kanata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSearchHandlesNumericYear(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "coach carter" {
			t.Fatalf("unexpected query: %s", got)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Fatalf("unexpected page: %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"poster":"/wp-content/uploads/2015/12/film-coach-carter-2005.jpg","quality":"HD","rating":7.3,"slug":"coach-carter-2005","title":"Coach Carter (2005)","type":"movie","url":"coach-carter-2005","year":2005}],"page":1}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	items, err := client.Search(context.Background(), "coach carter", 1)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Year != "2005" {
		t.Fatalf("expected year 2005, got %q", items[0].Year)
	}
}
