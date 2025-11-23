package test

import (
	"bytes"
	"context"
	"encoding/json"
	"excalidraw-complete/config"
	"excalidraw-complete/handlers/api/firebase"
	"excalidraw-complete/redis"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	chi "github.com/go-chi/chi/v5"
)

func makeCacheStoreForTest(t *testing.T) *redis.CacheStore {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	portInt, err := strconv.Atoi(s.Port())
	if err != nil {
		t.Fatalf("failed to convert port to int: %v", err)
	}
	cfg := config.Redis{
		Host:     s.Host(),
		Port:     portInt,
		Password: "",
		DB:       0,
	}
	cs := redis.NewCacheStore(ctx, cfg)

	return cs
}

func TestFirebaseBatchCommitAndGet(t *testing.T) {
	cfg := config.New()
	cacheStore := makeCacheStoreForTest(t)

	// setup handlers with cache store
	commit := firebase.HandleBatchCommit(cfg, cacheStore)
	get := firebase.HandleBatchGet(cfg, cacheStore)

	// commit an item
	payload := firebase.BatchCommitRequest{
		Writes: []firebase.WriteRequest{
			{Update: firebase.UpdateRequest{
				Name:   "projects/p/databases/(default)/documents/x/y",
				Fields: map[string]any{"a": map[string]any{"stringValue": "b"}},
			}},
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/projects/p/databases/(default)/documents:commit", bytes.NewReader(b))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("project_id", "p")
	rctx.URLParams.Add("database_id", "(default)")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	commit(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("commit unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}

	// now get the item
	getPayload := firebase.BatchGetRequest{Documents: []string{"projects/p/databases/(default)/documents/x/y"}}
	gb, _ := json.Marshal(getPayload)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/projects/p/databases/(default)/documents:batchGet", bytes.NewReader(gb))
	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("project_id", "p")
	rctx2.URLParams.Add("database_id", "(default)")
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))

	rr2 := httptest.NewRecorder()
	get(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("get unexpected status: %d body=%s", rr2.Code, rr2.Body.String())
	}
}
