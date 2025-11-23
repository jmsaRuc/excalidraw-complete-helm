package firebase

import (
	"excalidraw-complete/config"
	"excalidraw-complete/redis"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type (
	// BatchGetRequest represents a request to get multiple documents.
	BatchGetRequest struct {
		Documents []string `json:"documents"`
	}

	// BatchGetEmptyResponse models a "missing" entry in a batch get response.
	BatchGetEmptyResponse struct {
		Missing  string `json:"missing"`
		ReadTime string `json:"readTime"`
	}

	// FoundInfoResponse models the information of a found document.
	FoundInfoResponse struct {
		Name       string      `json:"name"`
		Fields     interface{} `json:"fields"`
		CreateTime string      `json:"createTime"`
		UpdateTime string      `json:"updateTime"`
	}

	// BatchGetExistsResponse models a "found" entry in a batch get response.
	BatchGetExistsResponse struct {
		Found    FoundInfoResponse `json:"found"`
		ReadTime string            `json:"readTime"`
	}

	// UpdateRequest represents an update to a document.
	UpdateRequest struct {
		Name   string      `json:"name"`
		Fields interface{} `json:"fields"`
	}

	// WriteRequest represents a request to write a document.
	WriteRequest struct {
		Update UpdateRequest `json:"update"`
	}

	// BatchCommitRequest represents a request to commit multiple writes.
	BatchCommitRequest struct {
		Writes []WriteRequest `json:"writes"`
	}

	// WriteResult represents the result of a write operation.
	WriteResult struct {
		UpdateTime string `json:"updateTime"`
	}

	// BatchCommitResponse represents the response for a batch commit request.
	BatchCommitResponse struct {
		WriteResults []WriteResult `json:"writeResults"`
		CommitTime   string        `json:"commitTime"`
	}
)

// Bind is a no-op for BatchGetRequest.
func (body *BatchGetRequest) Bind(r *http.Request) (err error) {
	return nil
}

// Bind is a no-op for BatchCommitRequest.
func (body *BatchCommitRequest) Bind(r *http.Request) (err error) {
	return nil
}

//create a local in-memory map to store firebase documents if HA is not active

var savedItemsLocal = make(map[string]interface{})

// HandleBatchCommit handles batch commit requests.
func HandleBatchCommit(config *config.Config, cacheStore *redis.CacheStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "project_id")
		databaseID := chi.URLParam(r, "database_id")
		_ = projectID
		_ = databaseID

		data := &BatchCommitRequest{}
		// Seems like requests is text/plain but content is json ...
		if err := render.DecodeJSON(r.Body, data); err != nil {
			fmt.Println(err)
			render.Status(r, http.StatusBadRequest)
			return
		}

		// if HA is active, get existing saved items from redis, then update with new items
		if config.HAActive {
			savedItemsRedis := make(map[string]interface{})
			savedData, err := cacheStore.GetSavedData("firebase-documents")
			if err != nil || len(savedData) == 0 {
				fmt.Println("No saved items yet, continuing")
			} else {
				savedItemsRedis = savedData
			}

			savedItemsRedis[data.Writes[0].Update.Name] = data.Writes[0].Update.Fields
			savedItemsLocal = savedItemsRedis
		} else {
			savedItemsLocal[data.Writes[0].Update.Name] = data.Writes[0].Update.Fields
		}

		if config.HAActive {
			if err := cacheStore.SetSavedData("firebase-documents", savedItemsLocal); err != nil {
				fmt.Println(err)
				render.Status(r, http.StatusInternalServerError)
				return
			}
		}
		// the timestamps must be UTC, i.e. no zone offsets allowed
		timestamp := time.Now().UTC().Format(time.RFC3339)

		render.Status(r, http.StatusOK)
		render.JSON(w, r, BatchCommitResponse{
			CommitTime: timestamp,
			WriteResults: []WriteResult{
				WriteResult{UpdateTime: timestamp},
			},
		})
	}
}

// HandleBatchGet handles batch get requests.
func HandleBatchGet(config *config.Config, cacheStore *redis.CacheStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		projectID := chi.URLParam(r, "project_id")
		databaseID := chi.URLParam(r, "database_id")
		fmt.Printf("Got %v and %v\n", projectID, databaseID)
		data := &BatchGetRequest{}

		// Seems like requests is text/plain but content is json ...
		if err := render.DecodeJSON(r.Body, data); err != nil {
			fmt.Println(err)
			render.Status(r, http.StatusBadRequest)
			return
		}

		key := data.Documents[0]
		fmt.Printf("Got key %v \n", key)

		// if HA is active, get saved items from redis, else use local map
		if config.HAActive {
			savedItemsRedis := make(map[string]interface{})
			savedData, err := cacheStore.GetSavedData("firebase-documents")
			if err != nil || len(savedData) == 0 {
				fmt.Println("No saved items yet, continuing")
			} else {
				savedItemsRedis = savedData
			}
			savedItemsLocal = savedItemsRedis
		}

		fields, ok := savedItemsLocal[key]

		// the timestamps must be UTC, i.e. no zone offsets allowed
		timestamp := time.Now().UTC().Format(time.RFC3339)
		if !ok {
			fmt.Println("missing key")
			render.JSON(w, r, []BatchGetEmptyResponse{BatchGetEmptyResponse{
				Missing:  key,
				ReadTime: timestamp,
			}})
			render.Status(r, http.StatusOK)
			return
		}
		fmt.Println("existing key")
		render.Status(r, http.StatusOK)
		render.JSON(w, r, []BatchGetExistsResponse{BatchGetExistsResponse{
			Found: FoundInfoResponse{
				Name:       key,
				Fields:     fields,
				CreateTime: timestamp,
				UpdateTime: timestamp,
			},
			ReadTime: timestamp,
		}})
	}
}
