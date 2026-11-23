package service

import (
	"context"

	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	repository "github.com/moh-sso-dashboard/internal/repository/document_imports"
	"github.com/moh-sso-dashboard/internal/storage"
)

type PdfArchiverProcessor struct {
	repository repository.DocumentRepository
	storage    storage.Storage
}

func NewPdfArchiverProcessor(repository repository.DocumentRepository, storage storage.Storage) *PdfArchiverProcessor {
	return &PdfArchiverProcessor{repository: repository, storage: storage}
}

func (c *PdfArchiverProcessor) Process(ctx context.Context, p db.Process, storage storage.Storage) error {
	_, err := c.repository.GetDocument(ctx, p.DocumentID)
	if err != nil {
		return err
	}

	return nil
}
