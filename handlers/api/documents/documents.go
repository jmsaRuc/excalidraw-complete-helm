package documents

import (
	"bytes"
	"excalidraw-complete/core"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type (
	// DocumentCreateResponse represents the response for document creation.
	DocumentCreateResponse struct {
		ID string `json:"id"`
	}
)

// HandleCreate handles the creation of a new document.
func HandleCreate(documentStore core.DocumentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := new(bytes.Buffer)
		_, err := io.Copy(data, r.Body)
		if err != nil {
			http.Error(w, "Failed to copy", http.StatusInternalServerError)
			return
		}
		id, err := documentStore.Create(r.Context(), &core.Document{Data: *data})
		if err != nil {
			http.Error(w, "Failed to save", http.StatusInternalServerError)
			return
		}

		render.JSON(w, r, DocumentCreateResponse{ID: id})
		render.Status(r, http.StatusOK)
	}
}

// HandleGet handles retrieving a document by its ID.
func HandleGet(documentStore core.DocumentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		document, err := documentStore.FindID(r.Context(), id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Write(document.Data.Bytes())
	}
}
