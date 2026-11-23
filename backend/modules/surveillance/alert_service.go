package surveillance

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
	"log/slog"
)

const alertImportSource = "csv_import"

type alertsPayload struct {
	Created     string `json:"created"`
	Narrative   string `json:"narrative"`
	District    string `json:"district"`
	Disease     string `json:"disease"`
	EpiWeek     int32  `json:"weeks"`
	SubmittedBy string `json:"submitted_by"`
}

type parsedAlertsPayload struct {
	CreatedAt   time.Time
	Narrative   string
	District    string
	Disease     string
	EpiWeek     int32
	Year        int32
	SubmittedBy string
}

type AlertService struct {
	log                    *slog.Logger
	alertRepo              AlertRepository
	surveillanceImportRepo ImportRepository
	notifications          modulekit.Notifier
	cfg                    *modulekit.Platform
}

func NewAlertService(
	log *slog.Logger,
	alertRepo AlertRepository,
	surveillanceImportRepo ImportRepository,
	notifications modulekit.Notifier,
	cfg ...*modulekit.Platform,
) *AlertService {
	var appConfig *modulekit.Platform
	if len(cfg) > 0 {
		appConfig = cfg[0]
	}

	return &AlertService{
		log:                    log,
		alertRepo:              alertRepo,
		surveillanceImportRepo: surveillanceImportRepo,
		notifications:          notifications,
		cfg:                    appConfig,
	}
}

func (s *AlertService) GetAlertByID(ctx context.Context, id uuid.UUID) (db.Alert, error) {
	if err := requireUUID("alert id", id); err != nil {
		return db.Alert{}, err
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "getting alert by id", "alert_id", id)
	}

	item, err := s.alertRepo.GetByID(ctx, id)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to get alert by id", "alert_id", id, "error", err)
		}

		return db.Alert{}, err
	}

	return item, nil
}

func (s *AlertService) ListAlertsByDisease(
	ctx context.Context,
	diseaseID uuid.UUID,
) ([]db.ListAlertsByDiseaseRow, error) {
	if err := requireUUID("disease id", diseaseID); err != nil {
		return nil, err
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing alerts by disease", "disease_id", diseaseID)
	}

	items, err := s.alertRepo.ListByDisease(ctx, diseaseID)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list alerts by disease", "disease_id", diseaseID, "error", err)
		}

		return nil, err
	}

	return items, nil
}

func (s *AlertService) ListAlertsByDistrict(
	ctx context.Context,
	districtID uuid.UUID,
) ([]db.ListAlertsByDistrictRow, error) {
	if err := requireUUID("district id", districtID); err != nil {
		return nil, err
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing alerts by district", "district_id", districtID)
	}

	items, err := s.alertRepo.ListByDistrict(ctx, districtID)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list alerts by district", "district_id", districtID, "error", err)
		}

		return nil, err
	}

	return items, nil
}

func (s *AlertService) ListAlertsByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.ListAlertsByWeekRow, error) {
	if err := requireUUID("epi week id", epiWeekID); err != nil {
		return nil, err
	}

	if s.log != nil {
		s.log.DebugContext(ctx, "listing alerts by week", "epi_week_id", epiWeekID)
	}

	items, err := s.alertRepo.ListByWeek(ctx, epiWeekID)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list alerts by week", "epi_week_id", epiWeekID, "error", err)
		}

		return nil, err
	}

	return items, nil
}

func (s *AlertService) ListAlerts(
	ctx context.Context,
	params db.ListAlertsParams,
) ([]db.ListAlertsRow, error) {
	if s.log != nil {
		s.log.DebugContext(ctx, "listing alerts")
	}

	items, err := s.alertRepo.ListAlerts(ctx, params)
	if err != nil {
		if s.log != nil {
			s.log.ErrorContext(ctx, "failed to list alerts", "error", err)
		}

		return nil, err
	}

	return items, nil
}

func (s *AlertService) ListAlertsFromParams(
	ctx context.Context,
	params AlertListParams,
) ([]db.ListAlertsRow, error) {
	return s.ListAlerts(ctx, db.ListAlertsParams{
		EpiWeekID:  uuidNull(params.EpiWeekID),
		DiseaseID:  uuidNull(params.DiseaseID),
		DistrictID: uuidNull(params.DistrictID),
		RegionID:   uuidNull(params.RegionID),
	})
}

func (s *AlertService) ListAlertsFromParamsInHealthContext(
	ctx context.Context,
	params AlertListParams,
	scope HealthContextScope,
) ([]db.ListAlertsRow, error) {
	return s.alertRepo.ListAlertsInHealthContext(ctx, db.ListAlertsParams{
		EpiWeekID: uuidNull(params.EpiWeekID), DiseaseID: uuidNull(params.DiseaseID),
		DistrictID: uuidNull(params.DistrictID), RegionID: uuidNull(params.RegionID),
	}, scope)
}

func (s *AlertService) ProcessAlerts(ctx context.Context, batchID uuid.UUID) error {
	if err := requireUUID("batch id", batchID); err != nil {
		return err
	}

	if s.log != nil {
		s.log.InfoContext(ctx, "processing alert batch", "batch_id", batchID)
	}

	var successRows int32
	var failedRows int32

	err := s.alertRepo.WithTx(ctx, func(q db.Querier) error {
		rows, err := s.surveillanceImportRepo.ListImportRawRowsByBatch(ctx, batchID)
		if err != nil {
			return fmt.Errorf("list import raw rows by batch: %w", err)
		}

		for _, raw := range rows {
			if err := s.processAlertRow(ctx, q, raw); err != nil {
				failedRows++

				if s.log != nil {
					s.log.WarnContext(
						ctx,
						"failed to process alert row",
						"batch_id", batchID,
						"raw_row_id", raw.ID,
						"row_number", raw.RowNumber,
						"error", err,
					)
				}

				if markErr := s.surveillanceImportRepo.MarkRawRowFailed(ctx, raw.ID, "failed to process alert row"); markErr != nil {
					return fmt.Errorf("mark raw row failed: %w", markErr)
				}

				continue
			}

			successRows++

			if err := s.surveillanceImportRepo.MarkRawRowProcessed(ctx, raw.ID); err != nil {
				return fmt.Errorf("mark raw row processed: %w", err)
			}
		}

		if err := s.surveillanceImportRepo.CompleteImportBatch(ctx, batchID, successRows, failedRows); err != nil {
			return fmt.Errorf("complete batch: %w", err)
		}

		if failedRows == 0 {
			if err := s.surveillanceImportRepo.DeleteProcessedRawRows(ctx, batchID); err != nil {
				return fmt.Errorf("delete processed raw rows: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		notification := modulekit.Notification{
			Type:       "SURVEILLANCE_ALERT_IMPORT_FAILED",
			Title:      "Alert import failed",
			Severity:   "critical",
			Message:    "Surveillance alert batch processing failed",
			TargetRole: "admin",
			Metadata: mustJSON(map[string]any{
				"batch_id":     batchID.String(),
				"success_rows": successRows,
				"failed_rows":  failedRows,
				"error":        "surveillance alert batch processing failed",
			}),
		}

		s.attachAdminEmailDelivery(
			&notification,
			"surveillance-alert-import-failed",
			"Surveillance alert import failed",
			"Surveillance alert batch processing failed.",
			map[string]any{
				"Name":        s.systemAdminName(),
				"Platform":    s.platformName(),
				"BatchID":     batchID.String(),
				"SuccessRows": successRows,
				"FailedRows":  failedRows,
				"Error":       "surveillance alert batch processing failed",
				"ActionURL":   s.adminSurveillanceURL(),
				"Details": fmt.Sprintf(
					"Batch ID: %s\nSuccess Rows: %d\nFailed Rows: %d\nError: %s",
					batchID.String(),
					successRows,
					failedRows,
					"surveillance alert batch processing failed",
				),
			},
		)

		s.notify(ctx, notification)

		return err
	}

	if s.log != nil {
		s.log.InfoContext(
			ctx,
			"completed alert batch processing",
			"batch_id", batchID,
			"success_rows", successRows,
			"failed_rows", failedRows,
		)
	}

	severity := "info"
	title := "Alert import completed"
	message := "Surveillance alert batch processing completed"

	if failedRows > 0 {
		severity = "warning"
		title = "Alert import completed with errors"
		message = "Surveillance alert batch processing completed with failed rows"
	}

	notification := modulekit.Notification{
		Type:       "SURVEILLANCE_ALERT_IMPORT_COMPLETED",
		Title:      title,
		Severity:   severity,
		Message:    message,
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"batch_id":     batchID.String(),
			"success_rows": successRows,
			"failed_rows":  failedRows,
		}),
	}

	// Email only when import completed with failed rows.
	// Fully successful imports remain in-app only.
	if failedRows > 0 {
		s.attachAdminEmailDelivery(
			&notification,
			"surveillance-alert-import-completed",
			"Surveillance alert import completed with errors",
			"Surveillance alert batch processing completed with failed rows.",
			map[string]any{
				"Name":        s.systemAdminName(),
				"Platform":    s.platformName(),
				"BatchID":     batchID.String(),
				"SuccessRows": successRows,
				"FailedRows":  failedRows,
				"ActionURL":   s.adminSurveillanceURL(),
				"Details": fmt.Sprintf(
					"Batch ID: %s\nSuccess Rows: %d\nFailed Rows: %d",
					batchID.String(),
					successRows,
					failedRows,
				),
			},
		)
	}

	s.notify(ctx, notification)

	return nil
}

func (s *AlertService) processAlertRow(
	ctx context.Context,
	q db.Querier,
	raw db.SurveillanceImportRawRow,
) error {
	payload, err := parseAlertsPayload(raw.Payload)
	if err != nil {
		return err
	}

	district, err := q.GetDistrictByName(ctx, payload.District)
	if err != nil {
		return fmt.Errorf("district not found: %s", payload.District)
	}

	disease, err := q.GetDiseaseByName(ctx, payload.Disease)
	if err != nil {
		return fmt.Errorf("disease not found: %s", payload.Disease)
	}

	epiWeek, err := q.GetEpiWeekByYearWeek(ctx, db.GetEpiWeekByYearWeekParams{
		EpiYear: payload.Year,
		EpiWeek: payload.EpiWeek,
	})
	if err != nil {
		return fmt.Errorf("epi week not found: year=%d week=%d", payload.Year, payload.EpiWeek)
	}

	externalID := buildAlertExternalID(payload)

	params := db.CreateAlertParams{
		ExternalID: sql.NullString{
			String: externalID,
			Valid:  externalID != "",
		},
		DiseaseID: disease.ID,
		DistrictID: uuid.NullUUID{
			UUID:  district.ID,
			Valid: true,
		},
		EpiWeekID: uuid.NullUUID{
			UUID:  epiWeek.ID,
			Valid: true,
		},
		OccurredOn: sql.NullTime{
			Time:  payload.CreatedAt,
			Valid: true,
		},
		CreatedOn: sql.NullTime{
			Time:  payload.CreatedAt,
			Valid: true,
		},
		Narrative: payload.Narrative,
		SubmittedBy: sql.NullString{
			String: payload.SubmittedBy,
			Valid:  payload.SubmittedBy != "",
		},
		Status: db.AlertStatusNEW,
		SourceName: sql.NullString{
			String: alertImportSource,
			Valid:  true,
		},
	}

	if _, err := s.alertRepo.Create(ctx, params); err != nil {
		return fmt.Errorf(
			"create alert for district=%s disease=%s week=%d year=%d: %w",
			payload.District,
			payload.Disease,
			payload.EpiWeek,
			payload.Year,
			err,
		)
	}

	return nil
}

func parseAlertsPayload(rawPayload []byte) (parsedAlertsPayload, error) {
	var raw alertsPayload

	if err := json.Unmarshal(rawPayload, &raw); err != nil {
		return parsedAlertsPayload{}, fmt.Errorf("invalid payload json: %w", err)
	}

	raw.Created = strings.TrimSpace(raw.Created)
	raw.Narrative = strings.TrimSpace(raw.Narrative)
	raw.District = strings.TrimSpace(raw.District)
	raw.Disease = strings.TrimSpace(raw.Disease)
	raw.SubmittedBy = strings.TrimSpace(raw.SubmittedBy)

	if raw.Created == "" {
		return parsedAlertsPayload{}, fmt.Errorf("created is required")
	}

	createdAt, err := parseDateOnly(raw.Created)
	if err != nil {
		return parsedAlertsPayload{}, fmt.Errorf("invalid created date %q: %w", raw.Created, err)
	}

	if raw.District == "" {
		return parsedAlertsPayload{}, fmt.Errorf("district is required")
	}

	if raw.Disease == "" {
		return parsedAlertsPayload{}, fmt.Errorf("disease is required")
	}

	if raw.Narrative == "" {
		return parsedAlertsPayload{}, fmt.Errorf("narrative is required")
	}

	if raw.EpiWeek <= 0 {
		return parsedAlertsPayload{}, fmt.Errorf("weeks is required")
	}

	if raw.EpiWeek > 53 {
		return parsedAlertsPayload{}, fmt.Errorf("weeks must not be greater than 53")
	}

	return parsedAlertsPayload{
		CreatedAt:   createdAt,
		Narrative:   raw.Narrative,
		District:    raw.District,
		Disease:     raw.Disease,
		EpiWeek:     raw.EpiWeek,
		Year:        int32(createdAt.Year()),
		SubmittedBy: raw.SubmittedBy,
	}, nil
}

func parseDateOnly(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

func buildAlertExternalID(payload parsedAlertsPayload) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(payload.District)),
		strings.ToLower(strings.TrimSpace(payload.Disease)),
		fmt.Sprintf("%d", payload.Year),
		fmt.Sprintf("%d", payload.EpiWeek),
		strings.ToLower(strings.TrimSpace(payload.Narrative)),
	}

	for i := range parts {
		parts[i] = strings.ReplaceAll(parts[i], " ", "_")
	}

	return strings.Join(parts, "|")
}

func (s *AlertService) notify(
	ctx context.Context,
	notification modulekit.Notification,
) {
	if s == nil || s.notifications == nil {
		return
	}

	if strings.TrimSpace(notification.TargetRole) == "" {
		notification.TargetRole = "admin"
	}

	if err := s.notifications.Notify(ctx, notification); err != nil {
		if s.log != nil {
			s.log.ErrorContext(
				ctx,
				"surveillance alert notification failed",
				"type", notification.Type,
				"error", err,
			)
		}
	}
}

func (s *AlertService) attachAdminEmailDelivery(
	notification *modulekit.Notification,
	templateName string,
	subject string,
	textBody string,
	templateData map[string]any,
) {
	if notification == nil {
		return
	}

	adminEmail := strings.TrimSpace(s.systemAdminEmail())
	if adminEmail == "" {
		return
	}

	if templateData == nil {
		templateData = map[string]any{}
	}

	if _, ok := templateData["Name"]; !ok {
		templateData["Name"] = s.systemAdminName()
	}

	if _, ok := templateData["Platform"]; !ok {
		templateData["Platform"] = s.platformName()
	}

	if _, ok := templateData["ActionURL"]; !ok {
		templateData["ActionURL"] = s.adminSurveillanceURL()
	}

	notification.Deliveries = []modulekit.NotificationDelivery{
		{
			Channel: modulekit.ChannelInApp,
			Recipient: map[string]any{
				"target_role": notification.TargetRole,
			},
			Payload: map[string]any{
				"title":    notification.Title,
				"message":  notification.Message,
				"type":     notification.Type,
				"severity": notification.Severity,
			},
			MaxAttempts: 1,
		},
		{
			Channel: modulekit.ChannelEmail,
			Recipient: map[string]any{
				"name":  s.systemAdminName(),
				"email": adminEmail,
			},
			TemplateName: templateName,
			TemplateData: templateData,
			Payload: map[string]any{
				"subject":   subject,
				"text_body": textBody,
			},
			MaxAttempts: 5,
		},
	}
}

func (s *AlertService) platformName() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.Name) != "" {
		return strings.TrimSpace(s.cfg.Name)
	}

	return "MOH Integrated Health Portal"
}

func (s *AlertService) systemAdminName() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.SystemAdminName) != "" {
		return strings.TrimSpace(s.cfg.SystemAdminName)
	}

	return "System Administrator"
}

func (s *AlertService) systemAdminEmail() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.SystemAdminEmail) != "" {
		return strings.TrimSpace(s.cfg.SystemAdminEmail)
	}

	return ""
}

func (s *AlertService) adminDashboardURL() string {
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.AdminDashboardURL) != "" {
		return strings.TrimSpace(s.cfg.AdminDashboardURL)
	}

	return "http://localhost:3000/admin/home"
}

func (s *AlertService) adminSurveillanceURL() string {
	base := strings.TrimRight(s.adminDashboardURL(), "/")

	if strings.HasSuffix(base, "/admin/home") {
		return strings.TrimSuffix(base, "/admin/home") + "/surveillance"
	}

	return base + "/surveillance"
}
