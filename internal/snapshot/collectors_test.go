package snapshot

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

type testBuildCollector struct {
	domain string
	build  func(ctx Context, writer *Writer, options BuildOptions) error
	patch  func(ctx Context, writer *Writer, slug string, options BuildOptions) error
}

func (c testBuildCollector) Domain() string {
	return c.domain
}

func (c testBuildCollector) Build(ctx Context, writer *Writer, options BuildOptions) error {
	if c.build != nil {
		return c.build(ctx, writer, options)
	}
	return nil
}

func (c testBuildCollector) Patch(ctx Context, writer *Writer, slug string, options BuildOptions) error {
	if c.patch != nil {
		return c.patch(ctx, writer, slug, options)
	}
	return nil
}

func TestBuildPackContinuesOnCollectorFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "snapshots")
	collectors := []Collector{
		testBuildCollector{
			domain: "movie",
			build: func(ctx Context, writer *Writer, options BuildOptions) error {
				_, err := writer.Write("movie", KindHome, "hot", map[string]any{"count": 1})
				return err
			},
		},
		testBuildCollector{
			domain: "anime",
			build: func(ctx Context, writer *Writer, options BuildOptions) error {
				return errors.New("provider blocked")
			},
		},
	}

	manifest, err := BuildPack(context.Background(), collectors, BuildOptions{
		OutputDir:   root,
		GeneratedAt: time.Date(2026, 3, 27, 2, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildPack() error = %v", err)
	}
	if len(manifest.Entries) != 1 {
		t.Fatalf("expected 1 manifest entry, got %d", len(manifest.Entries))
	}
	if manifest.Entries[0].Domain != "movie" || manifest.Entries[0].Kind != KindHome {
		t.Fatalf("unexpected manifest entry: %#v", manifest.Entries[0])
	}
}

func TestBuildPackFailsWhenAllCollectorsFail(t *testing.T) {
	root := filepath.Join(t.TempDir(), "snapshots")
	collectors := []Collector{
		testBuildCollector{
			domain: "movie",
			build: func(ctx Context, writer *Writer, options BuildOptions) error {
				return errors.New("provider blocked")
			},
		},
	}

	if _, err := BuildPack(context.Background(), collectors, BuildOptions{
		OutputDir: root,
	}); err == nil {
		t.Fatal("expected error when all collectors fail")
	}
}
