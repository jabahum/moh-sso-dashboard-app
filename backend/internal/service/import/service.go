package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"

	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/moh-sso-dashboard/internal/model"
	documentRepo "github.com/moh-sso-dashboard/internal/repository/document_imports"
	processRepo "github.com/moh-sso-dashboard/internal/repository/processes"

	"github.com/moh-sso-dashboard/internal/storage"
	documenttemplates "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates"
)

type Service struct {
	documentRepo   documentRepo.DocumentRepository
	processRepo    processRepo.ProcessRepository
	fileRepository documentRepo.FileRepository

	documentTemplateService documenttemplates.Service

	registry *Registry
	storage  storage.Storage
	remoteDB *sql.DB
	logger   *logger.Logger
}

func NewService(documentRepo documentRepo.DocumentRepository,
	processRepo processRepo.ProcessRepository,
	fileRepository documentRepo.FileRepository,
	documentTemplateService documenttemplates.Service,
	surveillanceJobs SurveillanceJobs,
	storage storage.Storage, remote *sql.DB) *Service {
	reg := NewRegistry()

	s := &Service{
		documentRepo:            documentRepo,
		processRepo:             processRepo,
		fileRepository:          fileRepository,
		registry:                reg,
		storage:                 storage,
		remoteDB:                remote,
		documentTemplateService: documentTemplateService,
	}

	// Register processors
	// The surveillance module owns its import jobs; without it (core profile)
	// they are unsupported process types.
	if surveillanceJobs != nil {
		reg.Register(model.ProcessTypeSurveillanceBatchProcess, surveillanceProcessor(surveillanceJobs.ProcessImportBatch))
		reg.Register(model.ProcessTypeSurveillanceCSVImport, surveillanceProcessor(surveillanceJobs.ImportCSV))
	}

	reg.Register(model.ProcessTypeCSVImport, NewCSVProcessor(documentRepo, processRepo, fileRepository, storage, remote))

	reg.Register(model.ProcessTypeExcelImport, NewExcelProcessor(documentRepo, fileRepository, processRepo, documentTemplateService, storage, remote))

	reg.Register(model.ProcessTypeFHIRImport, NewFhirBundlerProcessor(documentRepo, storage))

	reg.Register(model.ProcessTypeUserBulkImport, NewUserBulkProcessor(documentRepo, storage))

	return s
}

func (s *Service) Execute(ctx context.Context, processID uuid.UUID) error {

	// Panic protection (important for background jobs)
	defer func() {
		if r := recover(); r != nil {
			_ = s.processRepo.Fail(ctx, processID, "internal panic")
		}
	}()

	// 1️⃣ Load process
	proc, err := s.processRepo.GetProcessByID(ctx, processID)
	if err != nil {
		return err
	}

	// 2️⃣ Resolve processor
	processor, ok := s.registry.Get(model.ProcessType(proc.ProcessType))
	if !ok {
		_ = s.processRepo.Fail(ctx, proc.ID, "unsupported process type")
		return fmt.Errorf("unsupported process type: %s", proc.ProcessType)
	}

	// 3️⃣ Execute processor
	if err := processor.Process(ctx, proc); err != nil {
		_ = s.processRepo.Fail(ctx, processID, err.Error())
		_, _ = s.documentRepo.UpdateDocumentStatus(ctx, db.UpdateDocumentStatusParams{
			ID:     proc.DocumentID,
			Status: db.DocumentStatusFAILED,
		})
		return err
	}

	// 4️⃣ Mark complete
	if err := s.processRepo.Complete(ctx, processID); err != nil {
		return err
	}

	// 5️⃣ Update document status to COMPLETED
	_, err = s.documentRepo.UpdateDocumentStatus(ctx, db.UpdateDocumentStatusParams{
		ID:     proc.DocumentID,
		Status: db.DocumentStatusCOMPLETED,
	})
	return err

}
