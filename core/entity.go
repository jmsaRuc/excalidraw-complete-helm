package core

import (
	"bytes"
	"context"
)

type (
	// Document represents a document in the system.
	Document struct {
		Data bytes.Buffer
	}

	// DocumentStore defines the interface for document storage operations.
	DocumentStore interface {
		FindID(ctx context.Context, id string) (*Document, error)
		Create(ctx context.Context, document *Document) (string, error)
	}
)
