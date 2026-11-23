package surveillance

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

// ImportJob identifies one host document-processing job (a row in the host's
// processes queue) that the module runs.
type ImportJob struct {
	ProcessID  uuid.UUID
	DocumentID uuid.UUID
	CreatedBy  uuid.UUID
}

// ImportDocument is the uploaded file a surveillance CSV import reads.
type ImportDocument struct {
	ObjectKey        string
	OriginalFilename string
}

// ImportHost is the host's document pipeline that surveillance CSV imports
// run inside: uploaded documents, their file storage and the process queue.
type ImportHost interface {
	// GetDocument loads an uploaded document.
	GetDocument(ctx context.Context, documentID uuid.UUID) (ImportDocument, error)
	// Download opens a stored object for reading.
	Download(ctx context.Context, objectKey string) (io.ReadCloser, error)
	// UpdateProgress reports a job's progress (0-100) and message.
	UpdateProgress(ctx context.Context, processID uuid.UUID, progress int32, message *string) error
	// QueueBatchProcess queues the surveillance batch-process job for a
	// document; the host runs it later through ProcessImportBatch.
	QueueBatchProcess(ctx context.Context, documentID, createdBy uuid.UUID) error
}

var errImportHostUnavailable = errors.New("surveillance import host is not configured")

type SurveillanceBatchProcessor struct {
	repo                   ImportRepository
	facilityMetricsService *FacilityWeeklyMetricsService
	alertsService          *AlertService
}

type SurveillanceBatchProcessPayload struct {
	BatchID uuid.UUID `json:"batch_id"`
}

func NewSurveillanceBatchProcessor(
	repo ImportRepository,
	facilityMetricsService *FacilityWeeklyMetricsService,
	alertsService *AlertService,
) *SurveillanceBatchProcessor {
	return &SurveillanceBatchProcessor{
		repo:                   repo,
		facilityMetricsService: facilityMetricsService,
		alertsService:          alertsService,
	}
}

func (s *SurveillanceBatchProcessor) Process(ctx context.Context, p ImportJob) error {
	batchID, err := s.resolveBatchID(ctx, p)
	if err != nil {
		return err
	}

	return s.processBatch(ctx, batchID)
}

func (s *SurveillanceBatchProcessor) resolveBatchID(ctx context.Context, p ImportJob) (uuid.UUID, error) {

	if p.DocumentID != uuid.Nil {
		batch, err := s.repo.GetLatestImportBatchByDocumentID(ctx, p.DocumentID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to resolve latest batch by document_id: %w", err)
		}
		return batch.ID, nil
	}

	return uuid.Nil, fmt.Errorf("batch_id is required")
}

func (s *SurveillanceBatchProcessor) processBatch(ctx context.Context, batchID uuid.UUID) error {
	batch, err := s.repo.GetImportBatchByID(ctx, batchID)
	if err != nil {
		return err
	}

	_, err = s.repo.UpdateImportBatchStatus(ctx, db.UpdateImportBatchStatusParams{
		ID:     batchID,
		Status: "PROCESSING",
		Notes: sql.NullString{
			String: fmt.Sprintf("Processing imported raw rows for %s", batch.DatasetType),
			Valid:  true,
		},
	})
	if err != nil {
		return err
	}

	var processErr error

	switch batch.DatasetType {
	case "facility_metrics":
		if s.facilityMetricsService == nil {
			processErr = fmt.Errorf("facility metrics service is not configured")
			break
		}
		processErr = s.facilityMetricsService.ProcessFacilityMetrics(ctx, batchID)

	case "alerts":
		if s.alertsService == nil {
			processErr = fmt.Errorf("alerts service is not configured")
			break
		}
		processErr = s.alertsService.ProcessAlerts(ctx, batchID)

	default:
		processErr = fmt.Errorf("unsupported dataset type: %s", batch.DatasetType)
	}

	if processErr != nil {
		_ = s.repo.FailImportBatch(ctx, batchID, processErr.Error())
		return processErr
	}

	successRows := batch.TotalRows - batch.FailedRows
	if successRows < 0 {
		successRows = 0
	}

	if err := s.repo.CompleteImportBatch(ctx, batchID, successRows, batch.FailedRows); err != nil {
		return err
	}

	return nil
}

type SurveillanceCSVProcessor struct {
	host             ImportHost
	importRepository ImportRepository
}

func NewSurveillanceCSVProcessor(host ImportHost, importRepository ImportRepository) *SurveillanceCSVProcessor {
	return &SurveillanceCSVProcessor{host: host, importRepository: importRepository}
}

func (c *SurveillanceCSVProcessor) Process(ctx context.Context, p ImportJob) error {
	if c.host == nil {
		return errImportHostUnavailable
	}
	var (
		batchID      uuid.UUID
		batchCreated bool
	)

	updateProcessProgress := func(progress int32, message string) {
		_ = c.host.UpdateProgress(ctx, p.ProcessID, progress, &message)
	}

	failBatch := func(err error) error {
		if batchCreated {
			_ = c.importRepository.FailImportBatch(ctx, batchID, err.Error())
		}

		msg := fmt.Sprintf("Import failed: %v", err)
		_ = c.host.UpdateProgress(ctx, p.ProcessID, 100, &msg)

		return err
	}

	updateProcessProgress(2, "Loading document")

	doc, err := c.host.GetDocument(ctx, p.DocumentID)
	if err != nil {
		return err
	}

	updateProcessProgress(5, "Inspecting CSV")

	headers, datasetType, totalRows, err := c.inspectCSV(ctx, doc.ObjectKey)
	if err != nil {
		return err
	}

	if totalRows == 0 {
		return fmt.Errorf("csv file has no data rows")
	}

	updateProcessProgress(12, fmt.Sprintf("Creating import batch for %d rows", totalRows))

	batch, err := c.importRepository.CreateImportBatch(ctx, db.CreateImportBatchParams{
		SourceName: "document_upload",
		FileName: sql.NullString{
			String: strings.TrimSpace(doc.OriginalFilename),
			Valid:  strings.TrimSpace(doc.OriginalFilename) != "",
		},
		DatasetType: datasetType,
		ImportedBy: sql.NullString{
			String: p.CreatedBy.String(),
			Valid:  p.CreatedBy != uuid.Nil,
		},
		Status: "PROCESSING",
		Notes: sql.NullString{
			String: fmt.Sprintf("Raw import in progress. Total rows: %d", totalRows),
			Valid:  true,
		},
		DocumentID: uuid.NullUUID{
			UUID:  p.DocumentID,
			Valid: p.DocumentID != uuid.Nil,
		},
	})
	if err != nil {
		return err
	}

	batchID = batch.ID
	batchCreated = true

	if err := c.importRepository.UpdateImportBatchProgress(ctx, batch.ID, int32(totalRows), 0, 0, fmt.Sprintf("Starting raw import of %d rows", totalRows)); err != nil {
		return failBatch(fmt.Errorf("failed to initialize batch progress: %w", err))
	}

	updateProcessProgress(15, fmt.Sprintf("Importing %d rows", totalRows))

	fileReader, err := c.host.Download(ctx, doc.ObjectKey)
	if err != nil {
		return failBatch(err)
	}
	defer fileReader.Close()

	reader := csv.NewReader(fileReader)
	reader.TrimLeadingSpace = true
	reader.ReuseRecord = false

	_, err = reader.Read()
	if err != nil {
		return failBatch(fmt.Errorf("failed to re-read csv headers: %w", err))
	}

	rowCount := 0
	rowNumber := 1
	lastProgress := int32(15)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return failBatch(fmt.Errorf("failed to read csv row %d: %w", rowNumber, err))
		}

		payloadMap := make(map[string]any, len(headers))
		for i, header := range headers {
			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			payloadMap[header] = value
		}

		payloadBytes, err := json.Marshal(payloadMap)
		if err != nil {
			return failBatch(fmt.Errorf("failed to marshal row %d payload: %w", rowNumber, err))
		}

		_, err = c.importRepository.CreateImportRawRow(ctx, db.CreateImportRawRowParams{
			BatchID:   batch.ID,
			RowNumber: int32(rowNumber),
			Payload:   payloadBytes,
		})
		if err != nil {
			return failBatch(fmt.Errorf("failed to insert raw row %d: %w", rowNumber, err))
		}

		rowCount++
		rowNumber++

		progress := calculateImportProgress(rowCount, totalRows)

		if progress > lastProgress || rowCount%25 == 0 || rowCount == totalRows {
			msg := fmt.Sprintf("Imported %d of %d rows", rowCount, totalRows)
			updateProcessProgress(progress, msg)

			if err := c.importRepository.UpdateImportBatchProgress(ctx, batch.ID, int32(totalRows), int32(rowCount), 0, msg); err != nil {
				return failBatch(fmt.Errorf("failed to update batch progress: %w", err))
			}

			lastProgress = progress
		}
	}

	_, err = c.importRepository.UpdateImportBatchStatus(ctx, db.UpdateImportBatchStatusParams{
		ID:     batch.ID,
		Status: "PENDING",
		Notes: sql.NullString{
			String: fmt.Sprintf("Uploaded %d raw rows. Awaiting batch processing.", rowCount),
			Valid:  true,
		},
	})
	if err != nil {
		return failBatch(fmt.Errorf("failed to mark batch pending: %w", err))
	}

	_, err = json.Marshal(SurveillanceBatchProcessPayload{
		BatchID: batch.ID,
	})
	if err != nil {
		return failBatch(fmt.Errorf("failed to marshal batch process payload: %w", err))
	}

	err = c.host.QueueBatchProcess(ctx, p.DocumentID, p.CreatedBy)
	if err != nil {
		return failBatch(fmt.Errorf("failed to queue batch processing: %w", err))
	}

	updateProcessProgress(95, fmt.Sprintf("Imported %d raw rows. Batch queued for processing", rowCount))
	updateProcessProgress(100, "Raw import completed")

	return nil
}

func (c *SurveillanceCSVProcessor) inspectCSV(
	ctx context.Context,
	objectKey string,
) ([]string, string, int, error) {
	fileReader, err := c.host.Download(ctx, objectKey)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to open csv for inspection: %w", err)
	}
	defer fileReader.Close()

	reader := csv.NewReader(fileReader)
	reader.TrimLeadingSpace = true
	reader.ReuseRecord = false

	headers, err := reader.Read()
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to read csv headers: %w", err)
	}

	for i := range headers {
		headers[i] = normalizeHeader(headers[i])
	}

	if len(headers) == 0 {
		return nil, "", 0, fmt.Errorf("csv file has no headers")
	}

	datasetType := inferDatasetType(headers)
	if datasetType == "unknown" {
		return nil, "", 0, fmt.Errorf("unable to infer dataset type from csv headers")
	}

	totalRows := 0
	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", 0, fmt.Errorf("failed while counting csv rows: %w", err)
		}
		totalRows++
	}

	return headers, datasetType, totalRows, nil
}

func normalizeHeader(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func inferDatasetType(headers []string) string {
	headerSet := make(map[string]struct{}, len(headers))

	normalize := func(value string) string {
		value = strings.TrimSpace(strings.ToLower(value))
		value = strings.ReplaceAll(value, "-", "_")
		value = strings.ReplaceAll(value, " ", "_")
		return value
	}

	for _, h := range headers {
		headerSet[normalize(h)] = struct{}{}
	}

	hasAny := func(keys ...string) bool {
		for _, key := range keys {
			if _, ok := headerSet[normalize(key)]; ok {
				return true
			}
		}
		return false
	}

	hasRecordID := hasAny("record_id", "external_id", "id")
	hasFacility := hasAny("facility", "facility_name")
	hasDisease := hasAny("disease", "disease_name", "indicator", "indicator_name")
	hasValue := hasAny("value", "cases", "count", "metric_value")
	hasRegion := hasAny("region", "region_name")
	hasDistrict := hasAny("district", "district_name")
	hasSubCounty := hasAny("sub_county", "subcounty", "sub_county_name", "subcounty_name")
	hasWeek := hasAny("week", "weeks", "epi_week", "epiweek")
	hasYear := hasAny("year")
	hasMaroon := hasAny("maroon")
	hasRed := hasAny("red")
	hasYellow := hasAny("yellow")
	hasGreen := hasAny("green")
	hasNational := hasAny("national", "country", "uganda")

	switch {
	// facility metrics CSV like:
	// record_id,facility,region,district,sub_county,disease,value,year,weeks
	case hasFacility && hasDisease && hasValue && hasRegion && hasDistrict && hasSubCounty && hasWeek:
		return "facility_metrics"

	// looser facility metrics fallback
	case hasFacility && hasDisease && hasValue && hasWeek:
		return "facility_metrics"

	case hasWeek && hasYear && hasDistrict && hasMaroon && hasRed && hasYellow && hasGreen:
		return "district_status"

	case hasWeek && hasYear && hasRegion && hasMaroon && hasRed && hasYellow && hasGreen:
		return "region_status"

	case hasNational && hasDisease && hasWeek:
		return "national_status"

	case hasRecordID && hasFacility && hasDisease && hasValue:
		return "facility_metrics"

	default:
		return "unknown"
	}
}

func calculateImportProgress(processedRows, totalRows int) int32 {
	if totalRows <= 0 {
		return 20
	}

	progress := int32(15 + (float64(processedRows)/float64(totalRows))*70)
	if progress > 85 {
		return 85
	}
	if progress < 15 {
		return 15
	}
	return progress
}
