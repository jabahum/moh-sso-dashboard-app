package service

import (
	"context"

	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	repository "github.com/moh-sso-dashboard/internal/repository/document_imports"
	"github.com/moh-sso-dashboard/internal/storage"
)

type FacilityProcessor struct {
	repository repository.DocumentRepository
	storage    storage.Storage
}

func NewFacilityProcessor(repository repository.DocumentRepository, storage storage.Storage) *FacilityProcessor {
	return &FacilityProcessor{repository: repository, storage: storage}
}

func (c *FacilityProcessor) Process(ctx context.Context, p db.Process, storage storage.Storage) error {
	_, err := c.repository.GetDocument(ctx, p.DocumentID)
	if err != nil {
		return err
	}

	return nil
}
