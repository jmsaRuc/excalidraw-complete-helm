package firebase

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type (
	BatchGetRequest struct {
		Documents []string `json:"documents"`
	}
	BatchGetEmptyResponse struct {
		Missing  string `json:"missing"`
		ReadTime string `json:"readTime"`
	}

	FoundInfoResponse struct {
		Name       string      `json:"name"`
		Fields     interface{} `json:"fields"`
		CreateTime string      `json:"createTime"`
		UpdateTime string      `json:"updateTime"`
	}
	BatchGetExistsResponse struct {
		Found    FoundInfoResponse `json:"found"`
		ReadTime string            `json:"readTime"`
	}

	UpdateRequest struct {
		Name   string      `json:"name"`
		Fields interface{} `json:"fields"`
	}
	WriteRequest struct {
		Update UpdateRequest `json:"update"`
	}
	BatchCommitRequest struct {
		Writes []WriteRequest `json:"writes"`
	}

	WriteResult struct {
		UpdateTime string `json:"updateTime"`
	}
	BatchCommitResponse struct {
		WriteResults []WriteResult `json:"writeResults"`
		CommitTime   string        `json:"commitTime"`
	}
)

var savedItems = make(map[string]interface{})

func (body *BatchGetRequest) Bind(r *http.Request) (err error) {
	return nil
}
func (body *BatchCommitRequest) Bind(r *http.Request) (err error) {
	return nil
}
func HandleBatchCommit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectId := chi.URLParam(r, "project_id")
		databaseId := chi.URLParam(r, "database_id")
		_ = projectId
		_ = databaseId

		data := &BatchCommitRequest{}
		// Seems like requests is text/plain but content is json ...
		if err := render.DecodeJSON(r.Body, data); err != nil {
			fmt.Println(err)
			render.Status(r, http.StatusBadRequest)
			return
		}

		savedItems[data.Writes[0].Update.Name] = data.Writes[0].Update.Fields
		render.Status(r, http.StatusOK)

		// the timestamps must be UTC, i.e. no zone offsets allowed
		timestamp := time.Now().UTC().Format(time.RFC3339)
		render.JSON(w, r, BatchCommitResponse{
			CommitTime: timestamp,
			WriteResults: []WriteResult{
				WriteResult{UpdateTime: timestamp},
			},
		})
	}
}

func HandleBatchGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		projectId := chi.URLParam(r, "project_id")
		databaseId := chi.URLParam(r, "database_id")
		fmt.Printf("Got %v and %v\n", projectId, databaseId)
		data := &BatchGetRequest{}

		// Seems like requests is text/plain but content is json ...
		if err := render.DecodeJSON(r.Body, data); err != nil {
			fmt.Println(err)
			render.Status(r, http.StatusBadRequest)
			return
		}
		key := data.Documents[0]
		fmt.Printf("Got key %v \n", key)

		fields, ok := savedItems[key]

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
