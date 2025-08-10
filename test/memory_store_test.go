package test

import (
	"bytes"
	"context"
	"excalidraw-complete/core"
	mem "excalidraw-complete/stores/memory"
	"testing"
)

func TestMemoryDocumentStore_CreateAndFindID(t *testing.T) {
	store := mem.NewDocumentStore()

	data := []byte(`{"hello":"world"}`)
	id, err := store.Create(context.Background(), &core.Document{Data: *bytes.NewBuffer(data)})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if id == "" {
		t.Fatalf("expected non-empty id")
	}

	got, err := store.FindID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindID returned error: %v", err)
	}
	if !bytes.Equal(got.Data.Bytes(), data) {
		t.Fatalf("data mismatch: got %q want %q", got.Data.Bytes(), data)
	}
}
