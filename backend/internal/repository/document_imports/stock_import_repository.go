package documentimports

import (
	"context"
	"database/sql"

	"github.com/moh-sso-dashboard/internal/model"
)

type StockImportRepository interface {
	InsertNMSStockIssue(ctx context.Context, tx *sql.Tx, row model.NMSStockIssue) error
	InsertNMSStockIssuesBatch(ctx context.Context, tx *sql.Tx, rows []model.NMSStockIssue) error

	InsertNMSStockOnHand(ctx context.Context, tx *sql.Tx, row model.NMSStockOnHand) error
	InsertNMSStockOnHandBatch(ctx context.Context, tx *sql.Tx, rows []model.NMSStockOnHand) error

	InsertJMSStockIssue(ctx context.Context, tx *sql.Tx, row model.JMSStockIssue) error
	InsertJMSStockIssuesBatch(ctx context.Context, tx *sql.Tx, rows []model.JMSStockIssue) error

	InsertJMSStockOnHand(ctx context.Context, tx *sql.Tx, row model.JMSStockOnHand) error
	InsertJMSStockOnHandBatch(ctx context.Context, tx *sql.Tx, rows []model.JMSStockOnHand) error
}
