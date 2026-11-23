// Package documentimports is the persistence the host import processors use
// while executing document processing jobs: reading the uploaded document,
// recording its outcome and writing the import.* tables. The Documents module
// owns the documents HTTP API and the job queue; these processors run behind
// its ProcessExecutor port.
package documentimports

import (
	"context"

	"github.com/google/uuid"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
)

// DocumentRepository reads documents and records processing outcomes.
type DocumentRepository interface {
	GetDocument(ctx context.Context, id uuid.UUID) (db.Document, error)
	UpdateDocumentStatus(ctx context.Context, arg db.UpdateDocumentStatusParams) (db.Document, error)
}

type documentRepository struct{ db db.Store }

// NewDocumentRepository reads and updates documents through the sqlc store.
func NewDocumentRepository(store db.Store) DocumentRepository {
	return &documentRepository{db: store}
}

func (r *documentRepository) GetDocument(ctx context.Context, id uuid.UUID) (db.Document, error) {
	doc, err := r.db.GetDocumentByID(ctx, id)
	if err != nil {
		return db.Document{}, err
	}
	return doc, nil
}

func (r *documentRepository) UpdateDocumentStatus(ctx context.Context, arg db.UpdateDocumentStatusParams) (db.Document, error) {
	doc, err := r.db.UpdateDocumentStatus(ctx, arg)
	if err != nil {
		return db.Document{}, err
	}
	return doc, nil
}
