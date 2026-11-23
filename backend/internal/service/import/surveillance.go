package service

import (
	"context"

	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	surveillance "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance"
)

// SurveillanceJobs runs the surveillance module's import jobs.
type SurveillanceJobs interface {
	ImportCSV(ctx context.Context, job surveillance.ImportJob) error
	ProcessImportBatch(ctx context.Context, job surveillance.ImportJob) error
}

// surveillanceProcessor hands a queued process to the surveillance module.
type surveillanceProcessor func(context.Context, surveillance.ImportJob) error

func (run surveillanceProcessor) Process(ctx context.Context, p db.Process) error {
	return run(ctx, surveillance.ImportJob{ProcessID: p.ID, DocumentID: p.DocumentID, CreatedBy: p.CreatedBy})
}
