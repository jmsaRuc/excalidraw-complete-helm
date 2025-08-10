package test

import (
	"testing"

	"excalidraw-complete/config"
	"excalidraw-complete/stores"
)

func TestGetStore_DefaultIsMemory(t *testing.T) {
	cfg := config.New()
	cfg.StorageType = ""

	store := stores.GetStore(cfg)
	if store == nil {
		t.Fatalf("expected a store instance")
	}
}

func TestGetStore_Filesystem(t *testing.T) {
	cfg := config.New()
	cfg.StorageType = "filesystem"
	cfg.Filesystem.LocalStoragePath = t.TempDir()

	store := stores.GetStore(cfg)
	if store == nil {
		t.Fatalf("expected filesystem store instance")
	}
}
