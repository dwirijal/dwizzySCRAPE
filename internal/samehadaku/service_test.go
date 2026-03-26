package samehadaku

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeFetcher struct {
	body      []byte
	err       error
	bodies    map[string][]byte
	errors    map[string]error
	requested []string
}

func (f *fakeFetcher) FetchCatalogPage(ctx context.Context, url string) ([]byte, error) {
	f.requested = append(f.requested, url)
	if f.errors != nil {
		if err, ok := f.errors[url]; ok {
			return nil, err
		}
	}
	if f.bodies != nil {
		if body, ok := f.bodies[url]; ok {
			return body, nil
		}
	}
	return f.body, f.err
}

func (f *fakeFetcher) FetchPage(ctx context.Context, url string) ([]byte, error) {
	return f.FetchCatalogPage(ctx, url)
}

type fakeCatalogStore struct {
	items []CatalogItem
	err   error
}

func (f *fakeCatalogStore) UpsertCatalog(ctx context.Context, items []CatalogItem) (int, error) {
	f.items = append(f.items, items...)
	return len(items), f.err
}

func TestServiceSyncCatalog(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "catalog_sample.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	store := &fakeCatalogStore{}
	fetcher := &fakeFetcher{body: raw}
	svc := NewService(fetcher, store, time.Date(2026, 3, 22, 16, 0, 0, 0, time.UTC))

	report, err := svc.SyncCatalog(context.Background(), "https://v2.samehadaku.how/daftar-anime-2/")
	if err != nil {
		t.Fatalf("SyncCatalog returned error: %v", err)
	}
	if report.Parsed != 2 {
		t.Fatalf("expected parsed count 2, got %d", report.Parsed)
	}
	if report.Upserted != 2 {
		t.Fatalf("expected upserted count 2, got %d", report.Upserted)
	}
	if len(store.items) != 2 {
		t.Fatalf("expected 2 stored items, got %d", len(store.items))
	}
	if len(fetcher.requested) != 2 {
		t.Fatalf("expected 2 fetched pages, got %d", len(fetcher.requested))
	}
	if fetcher.requested[1] != "https://v2.samehadaku.how/daftar-anime-2/page/2/" {
		t.Fatalf("unexpected second page url %q", fetcher.requested[1])
	}
}

func TestServiceSyncCatalogAllPages(t *testing.T) {
	page1, err := os.ReadFile(filepath.Join("testdata", "catalog_sample.html"))
	if err != nil {
		t.Fatalf("read page1 fixture: %v", err)
	}

	page2 := []byte(strings.TrimSpace(`
<!DOCTYPE html>
<html lang="id">
  <body>
    <main>
      <section class="anime-list">
        <article class="anime-card">
          <a href="https://v2.samehadaku.how/anime/absolute-duo/">
            <img src="https://cdn.samehadaku.example/posters/absolute-duo.webp" alt="Absolute Duo" />
            <h4>Absolute Duo</h4>
          </a>
          <div class="meta">
            <span class="score">6.44</span>
            <span class="type">TV</span>
            <span class="status">Completed</span>
            <span class="views">120000 Views</span>
          </div>
          <p class="excerpt">Aksi sekolah dengan sistem blaze.</p>
          <div class="genres">
            <a href="/genre/action">Action</a>
            <a href="/genre/romance">Romance</a>
          </div>
        </article>
      </section>
    </main>
  </body>
</html>`))

	emptyPage := []byte("<!DOCTYPE html><html><body><main></main></body></html>")
	fetcher := &fakeFetcher{
		bodies: map[string][]byte{
			"https://v2.samehadaku.how/daftar-anime-2/":        page1,
			"https://v2.samehadaku.how/daftar-anime-2/page/2/": page2,
			"https://v2.samehadaku.how/daftar-anime-2/page/3/": emptyPage,
		},
	}
	store := &fakeCatalogStore{}
	svc := NewService(fetcher, store, time.Date(2026, 3, 22, 16, 0, 0, 0, time.UTC))

	report, err := svc.SyncCatalog(context.Background(), "https://v2.samehadaku.how/daftar-anime-2/")
	if err != nil {
		t.Fatalf("SyncCatalog returned error: %v", err)
	}
	if report.Parsed != 3 {
		t.Fatalf("expected parsed count 3, got %d", report.Parsed)
	}
	if report.Upserted != 3 {
		t.Fatalf("expected upserted count 3, got %d", report.Upserted)
	}
	if len(store.items) != 3 {
		t.Fatalf("expected 3 stored items, got %d", len(store.items))
	}
	if got := store.items[2].Slug; got != "absolute-duo" {
		t.Fatalf("unexpected third slug %q", got)
	}
}

func TestServiceSyncCatalogStopsOnRepeatedPage(t *testing.T) {
	page1, err := os.ReadFile(filepath.Join("testdata", "catalog_sample.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	fetcher := &fakeFetcher{
		bodies: map[string][]byte{
			"https://v2.samehadaku.how/daftar-anime-2/":        page1,
			"https://v2.samehadaku.how/daftar-anime-2/page/2/": page1,
		},
	}
	store := &fakeCatalogStore{}
	svc := NewService(fetcher, store, time.Date(2026, 3, 22, 16, 0, 0, 0, time.UTC))

	report, err := svc.SyncCatalog(context.Background(), "https://v2.samehadaku.how/daftar-anime-2/")
	if err != nil {
		t.Fatalf("SyncCatalog returned error: %v", err)
	}
	if report.Parsed != 2 {
		t.Fatalf("expected parsed count 2, got %d", report.Parsed)
	}
	if len(fetcher.requested) != 2 {
		t.Fatalf("expected 2 fetched pages before stop, got %d", len(fetcher.requested))
	}
}
