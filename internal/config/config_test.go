package config

import "testing"

func TestLoadPrefersDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://database-url")
	t.Setenv("POSTGRES_URL", "postgres://postgres-url")
	t.Setenv("NEON_DATABASE_URL", "postgres://neon-url")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DatabaseURL != "postgres://database-url" {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://database-url")
	}
}

func TestLoadFallsBackToLegacyDatabaseEnvVars(t *testing.T) {
	t.Setenv("POSTGRES_URL", "postgres://postgres-url")
	t.Setenv("NEON_DATABASE_URL", "postgres://neon-url")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DatabaseURL != "postgres://postgres-url" {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://postgres-url")
	}
}

func TestLoadSetsKanzeninDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.KanzeninBaseURL != DefaultKanzeninBaseURL {
		t.Fatalf("KanzeninBaseURL = %q, want %q", cfg.KanzeninBaseURL, DefaultKanzeninBaseURL)
	}
	if cfg.KanzeninUserAgent == "" {
		t.Fatal("expected KanzeninUserAgent default")
	}
}

func TestLoadSetsAnichinDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AnichinBaseURL != DefaultAnichinBaseURL {
		t.Fatalf("AnichinBaseURL = %q, want %q", cfg.AnichinBaseURL, DefaultAnichinBaseURL)
	}
	if cfg.AnichinUserAgent == "" {
		t.Fatal("expected AnichinUserAgent default")
	}
}

func TestLoadSetsBacamanDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.BacamanBaseURL != DefaultBacamanBaseURL {
		t.Fatalf("BacamanBaseURL = %q, want %q", cfg.BacamanBaseURL, DefaultBacamanBaseURL)
	}
	if cfg.BacamanUserAgent == "" {
		t.Fatal("expected BacamanUserAgent default")
	}
}

func TestLoadSetsDrakoridDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DrakoridBaseURL != DefaultDrakoridBaseURL {
		t.Fatalf("DrakoridBaseURL = %q, want %q", cfg.DrakoridBaseURL, DefaultDrakoridBaseURL)
	}
	if cfg.DrakoridUserAgent == "" {
		t.Fatal("expected DrakoridUserAgent default")
	}
}

func TestLoadSetsMangasusukuDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MangasusukuBaseURL != DefaultMangasusukuBaseURL {
		t.Fatalf("MangasusukuBaseURL = %q, want %q", cfg.MangasusukuBaseURL, DefaultMangasusukuBaseURL)
	}
	if cfg.MangasusukuUserAgent == "" {
		t.Fatal("expected MangasusukuUserAgent default")
	}
}

func TestLoadSetsAniListDefault(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AniListBaseURL != "https://graphql.anilist.co" {
		t.Fatalf("AniListBaseURL = %q, want %q", cfg.AniListBaseURL, "https://graphql.anilist.co")
	}
}
