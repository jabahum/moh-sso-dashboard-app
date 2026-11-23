package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type ImportRepository interface {
	CreateImportBatch(ctx context.Context, arg db.CreateImportBatchParams) (db.SurveillanceImportBatch, error)
	GetImportBatchByID(ctx context.Context, id uuid.UUID) (db.SurveillanceImportBatch, error)
	GetLatestImportBatchByDocumentID(ctx context.Context, documentID uuid.UUID) (db.SurveillanceImportBatch, error)
	ListImportBatches(ctx context.Context) ([]db.SurveillanceImportBatch, error)
	UpdateImportBatchStatus(ctx context.Context, arg db.UpdateImportBatchStatusParams) (db.SurveillanceImportBatch, error)
	UpdateImportBatchProgress(ctx context.Context, batchID uuid.UUID, totalRows, successRows, failedRows int32, notes string) error

	CreateImportRawRow(ctx context.Context, arg db.CreateImportRawRowParams) (db.SurveillanceImportRawRow, error)
	ListImportRawRowsByBatch(ctx context.Context, batchID uuid.UUID) ([]db.SurveillanceImportRawRow, error)

	MarkRawRowProcessed(ctx context.Context, rowID uuid.UUID) error
	MarkRawRowFailed(ctx context.Context, rowID uuid.UUID, message string) error
	CompleteImportBatch(ctx context.Context, batchID uuid.UUID, successRows, failedRows int32) error
	FailImportBatch(ctx context.Context, batchID uuid.UUID, notes string) error
	DeleteProcessedRawRows(ctx context.Context, batchID uuid.UUID) error
}
