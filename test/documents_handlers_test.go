package test

import (
	"bytes"
	"context"
	"encoding/json"
	"excalidraw-complete/handlers/api/documents"
	mem "excalidraw-complete/stores/memory"
	"net/http"
	"net/http/httptest"
	"testing"

	chi "github.com/go-chi/chi/v5"
)

func TestHandleCreateAndGet(t *testing.T) {
	store := mem.NewDocumentStore()

	// Setup router with handlers
	create := documents.HandleCreate(store)
	get := documents.HandleGet(store)

	body := []byte(`{"a":1}`)

	// Test create
	req := httptest.NewRequest(http.MethodPost, "/api/documents", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	create(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}

	// parse response
	id := struct {
		ID string `json:"id"`
	}{}
	if err := json.Unmarshal(rr.Body.Bytes(), &id); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if id.ID == "" {
		t.Fatalf("expected id in response")
	}

	// Test get
	req2 := httptest.NewRequest(http.MethodGet, "/api/documents/"+id.ID, nil)
	// inject URL param used by chi
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.ID)
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx))

	rr2 := httptest.NewRecorder()
	get(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr2.Code, rr2.Body.String())
	}
	if !bytes.Equal(rr2.Body.Bytes(), body) {
		t.Fatalf("body mismatch: got %q want %q", rr2.Body.String(), string(body))
	}
}
