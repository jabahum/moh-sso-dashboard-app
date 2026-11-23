package service

import (
	"context"

	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	repository "github.com/moh-sso-dashboard/internal/repository/document_imports"
	"github.com/moh-sso-dashboard/internal/storage"
)

type FhirBundlerProcessor struct {
	repository repository.DocumentRepository
	storage    storage.Storage
}

func NewFhirBundlerProcessor(repository repository.DocumentRepository, storage storage.Storage) *FhirBundlerProcessor {
	return &FhirBundlerProcessor{repository: repository, storage: storage}
}

func (c *FhirBundlerProcessor) Process(ctx context.Context, p db.Process) error {
	_, err := c.repository.GetDocument(ctx, p.DocumentID)
	if err != nil {
		return err
	}

	return nil
}
