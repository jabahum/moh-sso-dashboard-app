package documentimports

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type fileRepository struct{}

func NewFileRepository() FileRepository {
	return &fileRepository{}
}

func (r *fileRepository) CreateFile(
	ctx context.Context,
	tx *sql.Tx,
	documentID uuid.UUID,
	templateCode, fileName, filePath string,
	reportDate *time.Time,
) (int64, error) {
	var fileKey int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO import.custom_files
			(document_id, template_code, file_name, file_path, report_date, effective_start_date)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (document_id) DO UPDATE
		  SET template_code    = EXCLUDED.template_code,
		      file_name        = EXCLUDED.file_name,
		      file_path        = EXCLUDED.file_path,
		      report_date      = EXCLUDED.report_date,
		      is_current       = 'Y',
		      last_update_date = NOW()
		RETURNING file_key
	`, documentID, templateCode, fileName, filePath, reportDate).Scan(&fileKey)
	return fileKey, err
}

func (r *fileRepository) UpsertRowsBatch(
	ctx context.Context,
	tx *sql.Tx,
	rows []CustomImportRow,
) error {
	if len(rows) == 0 {
		return nil
	}

	const colsPerRow = 8 // file_key, document_id, template_code, sheet_code, row_number, report_date, file_data, row_hash

	args := make([]any, 0, len(rows)*colsPerRow)
	placeholderSets := make([]string, 0, len(rows))

	for i, row := range rows {
		dataJSON, err := json.Marshal(row.Data)
		if err != nil {
			return fmt.Errorf("marshal data row %d: %w", row.RowNumber, err)
		}

		base := i * colsPerRow
		set := make([]string, colsPerRow)
		for j := range set {
			set[j] = fmt.Sprintf("$%d", base+j+1)
		}
		placeholderSets = append(placeholderSets, "("+strings.Join(set, ",")+", NOW())")

		args = append(args,
			row.FileKey,
			row.DocumentID,
			row.TemplateCode,
			row.SheetCode,
			row.RowNumber,
			row.ReportDate,
			dataJSON,
			row.RowHash,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO import.custom_data_files
			(file_key, document_id, template_code, sheet_code, row_number, report_date, file_data, row_hash, effective_start_date)
		VALUES %s
		ON CONFLICT (row_hash) DO UPDATE SET
			file_key         = EXCLUDED.file_key,
			document_id      = EXCLUDED.document_id,
			template_code    = EXCLUDED.template_code,
			sheet_code       = EXCLUDED.sheet_code,
			row_number       = EXCLUDED.row_number,
			report_date      = EXCLUDED.report_date,
			file_data        = EXCLUDED.file_data,
			is_current       = 'Y',
			last_update_date = NOW()
	`, strings.Join(placeholderSets, ","))

	_, err := tx.ExecContext(ctx, query, args...)
	return err
}

func (r *fileRepository) SoftDeleteBySheet(
	ctx context.Context,
	tx *sql.Tx,
	documentID uuid.UUID,
	templateCode, sheetCode string,
	keepHashes []string,
) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE import.custom_data_files
		SET is_current = 'N', last_update_date = NOW()
		WHERE document_id  = $1
		  AND template_code = $2
		  AND sheet_code    = $3
		  AND is_current    = 'Y'
		  AND row_hash IS NOT NULL
		  AND NOT (row_hash = ANY($4))
	`, documentID, templateCode, sheetCode, pq.Array(keepHashes))
	return err
}

func (r *fileRepository) InvalidateByDocument(
	ctx context.Context,
	tx *sql.Tx,
	documentID uuid.UUID,
) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE import.custom_data_files
		SET is_current = 'N', last_update_date = NOW()
		WHERE document_id = $1 AND is_current = 'Y'
	`, documentID)
	return err
}

func (r *fileRepository) InvalidateFile(
	ctx context.Context,
	tx *sql.Tx,
	documentID uuid.UUID,
) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE import.custom_files
		SET is_current = 'N', last_update_date = NOW()
		WHERE document_id = $1 AND is_current = 'Y'
	`, documentID)
	return err
}
