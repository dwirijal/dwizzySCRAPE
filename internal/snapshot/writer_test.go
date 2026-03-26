package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeCollector struct {
	domain string
}

func (c fakeCollector) Domain() string {
	return c.domain
}

func (c fakeCollector) Build(_ Context, writer *Writer, _ BuildOptions) error {
	_, err := writer.Write(c.domain, KindCatalog, "page-1", map[string]any{"ok": true})
	return err
}

func (c fakeCollector) Patch(_ Context, writer *Writer, slug string, _ BuildOptions) error {
	if _, err := writer.Write(c.domain, KindTitle, slug, map[string]any{"slug": slug}); err != nil {
		return err
	}
	_, err := writer.Write(c.domain, KindPlayback, slug, map[string]any{"slug": slug, "playback": true})
	return err
}

func TestWriteManifestCollectsEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writer := NewWriter(root, time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC))
	if _, err := writer.Write("movie", KindHome, "hot", map[string]any{"count": 3}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := writer.Write("movie", KindTitle, "war-machine", map[string]any{"slug": "war-machine"}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	manifest, err := WriteManifest(root, writer.generatedAt)
	if err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	if len(manifest.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(manifest.Entries))
	}
	if _, err := os.Stat(filepath.Join(root, "manifest.json")); err != nil {
		t.Fatalf("manifest.json missing: %v", err)
	}
}

func TestPatchPackWritesManifestForSingleDomain(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	manifest, err := PatchPack(context.Background(), []Collector{fakeCollector{domain: "movie"}}, "movie", "war-machine", BuildOptions{
		OutputDir:   root,
		GeneratedAt: time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PatchPack() error = %v", err)
	}
	if len(manifest.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(manifest.Entries))
	}
	for _, rel := range []string{
		"movie/title/war-machine.json",
		"movie/playback/war-machine.json",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected snapshot file %s: %v", rel, err)
		}
	}
}
