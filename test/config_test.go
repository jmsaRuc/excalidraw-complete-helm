package test

import (
	"os"
	"testing"

	"excalidraw-complete/config"
)

func TestConfigDefaults(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "")
	cfg := config.New()

	if cfg.Port != "3002" {
		t.Fatalf("unexpected default port: %s", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Fatalf("unexpected default host: %s", cfg.Host)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("unexpected default log level: %s", cfg.LogLevel)
	}
	if cfg.Filesystem.LocalStoragePath == "" {
		t.Fatalf("expected default local storage path to be set")
	}
	if cfg.Sqlite.DataSourceName == "" {
		t.Fatalf("expected default sqlite DSN to be set")
	}

	// ensure int default works when env is not set
	if cfg.Postgres.Port != 5432 {
		t.Fatalf("unexpected default pg port: %d", cfg.Postgres.Port)
	}

	// verify getEnv override
	t.Setenv("PORT", "9999")
	if config.New().Port != "9999" {
		t.Fatalf("expected overridden PORT from env")
	}

	// avoid unused import on os in case of build tags adjustments
	_ = os.Environ
}
