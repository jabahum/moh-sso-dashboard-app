package surveillance

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresImportRepository struct {
	db db.Store
}

func NewImportRepository(db db.Store) ImportRepository {
	return &postgresImportRepository{
		db: db,
	}
}

func (r *postgresImportRepository) CreateImportBatch(
	ctx context.Context,
	arg db.CreateImportBatchParams,
) (db.SurveillanceImportBatch, error) {
	return r.db.CreateImportBatch(ctx, arg)
}

func (r *postgresImportRepository) GetImportBatchByID(
	ctx context.Context,
	id uuid.UUID,
) (db.SurveillanceImportBatch, error) {
	return r.db.GetImportBatchByID(ctx, id)
}

func (r *postgresImportRepository) GetLatestImportBatchByDocumentID(
	ctx context.Context,
	documentID uuid.UUID,
) (db.SurveillanceImportBatch, error) {

	return r.db.GetLatestImportBatchByDocumentID(ctx, uuid.NullUUID{
		UUID:  documentID,
		Valid: documentID != uuid.Nil,
	})
}

func (r *postgresImportRepository) ListImportBatches(
	ctx context.Context,
) ([]db.SurveillanceImportBatch, error) {
	return r.db.ListImportBatches(ctx)
}

func (r *postgresImportRepository) UpdateImportBatchStatus(
	ctx context.Context,
	arg db.UpdateImportBatchStatusParams,
) (db.SurveillanceImportBatch, error) {
	return r.db.UpdateImportBatchStatus(ctx, arg)
}

func (r *postgresImportRepository) UpdateImportBatchProgress(
	ctx context.Context,
	batchID uuid.UUID,
	totalRows, successRows, failedRows int32,
	notes string,
) error {
	return r.db.UpdateImportBatchProgress(ctx, db.UpdateImportBatchProgressParams{
		ID:          batchID,
		TotalRows:   totalRows,
		SuccessRows: successRows,
		FailedRows:  failedRows,
		Notes: sql.NullString{
			String: notes,
			Valid:  strings.TrimSpace(notes) != "",
		},
	})
}

func (r *postgresImportRepository) CreateImportRawRow(
	ctx context.Context,
	arg db.CreateImportRawRowParams,
) (db.SurveillanceImportRawRow, error) {
	return r.db.CreateImportRawRow(ctx, arg)
}

func (r *postgresImportRepository) ListImportRawRowsByBatch(
	ctx context.Context,
	batchID uuid.UUID,
) ([]db.SurveillanceImportRawRow, error) {
	return r.db.ListImportRawRowsByBatch(ctx, batchID)
}

func (r *postgresImportRepository) MarkRawRowProcessed(
	ctx context.Context,
	rowID uuid.UUID,
) error {
	return r.db.MarkSurveillanceImportRawRowProcessed(ctx, rowID)
}

func (r *postgresImportRepository) MarkRawRowFailed(
	ctx context.Context,
	rowID uuid.UUID,
	message string,
) error {
	return r.db.MarkSurveillanceImportRawRowFailed(ctx, db.MarkSurveillanceImportRawRowFailedParams{
		ID: rowID,
		ErrorMessage: sql.NullString{
			String: message,
			Valid:  strings.TrimSpace(message) != "",
		},
	})
}

func (r *postgresImportRepository) CompleteImportBatch(
	ctx context.Context,
	batchID uuid.UUID,
	successRows, failedRows int32,
) error {
	return r.db.CompleteSurveillanceImportBatch(ctx, db.CompleteSurveillanceImportBatchParams{
		ID:          batchID,
		SuccessRows: successRows,
		FailedRows:  failedRows,
		Notes: sql.NullString{
			String: "Batch processed successfully",
			Valid:  true,
		},
	})
}

func (r *postgresImportRepository) FailImportBatch(
	ctx context.Context,
	batchID uuid.UUID,
	notes string,
) error {
	return r.db.FailSurveillanceImportBatch(ctx, db.FailSurveillanceImportBatchParams{
		ID: batchID,
		Notes: sql.NullString{
			String: notes,
			Valid:  strings.TrimSpace(notes) != "",
		},
	})
}

func (r *postgresImportRepository) DeleteProcessedRawRows(
	ctx context.Context,
	batchID uuid.UUID,
) error {
	return r.db.DeleteProcessedSurveillanceImportRawRows(ctx, batchID)
}
