package main

import (
	"testing"

	"github.com/dwirijal/dwizzySCRAPE/internal/store"
)

func TestParseArgsDefaults(t *testing.T) {
	t.Parallel()

	opts, err := parseArgs(nil)
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if opts.scope != store.AniListEnrichmentScopeAll {
		t.Fatalf("scope = %q, want %q", opts.scope, store.AniListEnrichmentScopeAll)
	}
	if opts.limit != 0 {
		t.Fatalf("limit = %d, want 0", opts.limit)
	}
	if opts.batchSize != 25 {
		t.Fatalf("batchSize = %d, want 25", opts.batchSize)
	}
}

func TestParseArgsSupportsComicScope(t *testing.T) {
	t.Parallel()

	opts, err := parseArgs([]string{"comic", "15", "7"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if opts.scope != store.AniListEnrichmentScopeComic {
		t.Fatalf("scope = %q, want %q", opts.scope, store.AniListEnrichmentScopeComic)
	}
	if opts.limit != 15 {
		t.Fatalf("limit = %d, want 15", opts.limit)
	}
	if opts.batchSize != 7 {
		t.Fatalf("batchSize = %d, want 7", opts.batchSize)
	}
}

func TestParseArgsRejectsUnknownScope(t *testing.T) {
	t.Parallel()

	if _, err := parseArgs([]string{"weird"}); err == nil {
		t.Fatal("parseArgs returned nil error for unknown scope")
	}
}
