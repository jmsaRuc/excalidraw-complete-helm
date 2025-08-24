package test

import (
	"bytes"
	"context"
	"excalidraw-complete/core"
	fs "excalidraw-complete/stores/filesystem"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesystemDocumentStore_CreateAndFindID(t *testing.T) {
	dir := t.TempDir()
	// ensure trailing slash behavior not required
	base := filepath.Clean(dir)

	store := fs.NewDocumentStore(base)

	data := []byte("some-bytes")
	id, err := store.Create(context.Background(), &core.Document{Data: *bytes.NewBuffer(data)})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := store.FindID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindID returned error: %v", err)
	}

	if got.Data.String() != string(data) {
		t.Fatalf("data mismatch: got %q want %q", got.Data.String(), string(data))
	}

	// file exists on disk
	if _, err := os.Stat(filepath.Join(base, id)); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}
