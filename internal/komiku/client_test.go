package komiku

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientFetchPageSetsHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "dwizzy-test-agent" {
			t.Fatalf("unexpected user agent: %q", got)
		}
		if got := r.Header.Get("Cookie"); got != "session=test" {
			t.Fatalf("unexpected cookie header: %q", got)
		}
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "dwizzy-test-agent", "session=test", time.Second)
	body, err := client.FetchPage(context.Background(), server.URL+"/daftar-komik/")
	if err != nil {
		t.Fatalf("FetchPage returned error: %v", err)
	}
	if string(body) != "<html>ok</html>" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}
