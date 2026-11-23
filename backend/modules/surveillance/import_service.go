package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type ImportService struct {
	importRepo ImportRepository
}

func NewImportService(importRepo ImportRepository) *ImportService {
	return &ImportService{
		importRepo: importRepo,
	}
}

func (s *ImportService) ListImportBatches(ctx context.Context) ([]db.SurveillanceImportBatch, error) {
	return s.importRepo.ListImportBatches(ctx)
}

func (s *ImportService) GetImportBatchByID(ctx context.Context, batchID uuid.UUID) (db.SurveillanceImportBatch, error) {
	return s.importRepo.GetImportBatchByID(ctx, batchID)
}

func (s *ImportService) CreateImportBatch(ctx context.Context, arg db.CreateImportBatchParams) (db.SurveillanceImportBatch, error) {
	return s.importRepo.CreateImportBatch(ctx, arg)
}

func (s *ImportService) CreateImportBatchFromInput(ctx context.Context, input CreateImportBatchInput) (db.SurveillanceImportBatch, error) {
	return s.CreateImportBatch(ctx, db.CreateImportBatchParams{
		SourceName:  input.SourceName,
		FileName:    sqlNullStringFromPtr(input.FileName),
		DatasetType: input.DatasetType,
		ImportedBy:  sqlNullStringFromPtr(input.ImportedBy),
		Status:      input.Status,
		Notes:       sqlNullStringFromPtr(input.Notes),
		DocumentID:  uuidNullFromPtr(input.DocumentID),
	})
}

func (s *ImportService) UpdateImportBatchStatus(ctx context.Context, arg db.UpdateImportBatchStatusParams) (db.SurveillanceImportBatch, error) {
	return s.importRepo.UpdateImportBatchStatus(ctx, arg)
}

func (s *ImportService) UpdateImportBatchStatusFromInput(ctx context.Context, input UpdateImportBatchStatusInput) (db.SurveillanceImportBatch, error) {
	return s.UpdateImportBatchStatus(ctx, db.UpdateImportBatchStatusParams{
		ID:     input.ID,
		Status: input.Status,
		Notes:  sqlNullStringFromPtr(input.Notes),
	})
}

func (s *ImportService) ListImportRawRowsByBatch(ctx context.Context, batchID uuid.UUID) ([]db.SurveillanceImportRawRow, error) {
	return s.importRepo.ListImportRawRowsByBatch(ctx, batchID)
}
