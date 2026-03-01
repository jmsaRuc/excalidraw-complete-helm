package stores

import (
	"testing"

	"excalidraw-complete/config"
)

func TestGetStore_DefaultIsMemory(t *testing.T) {
	cfg := config.New()
	cfg.StorageType = ""

	store := GetStore(cfg)
	if store == nil {
		t.Fatalf("expected a store instance")
	}
}

func TestGetStore_Filesystem(t *testing.T) {
	cfg := config.New()
	cfg.StorageType = "filesystem"
	cfg.Filesystem.LocalStoragePath = t.TempDir()

	store := GetStore(cfg)
	if store == nil {
		t.Fatalf("expected filesystem store instance")
	}
}
