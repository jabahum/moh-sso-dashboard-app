package report_scheduler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/mail"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/model"
	sharedservice "github.com/moh-sso-dashboard/internal/service"
	"github.com/moh-sso-dashboard/internal/storage"
)

type Service struct {
	repo        Repository
	healthBI    HealthBIClient
	email       EmailDelivery
	users       UserLookup
	fileStorage storage.Storage
	dwh         *sql.DB
	notifications sharedservice.NotificationsService
	audit         *sharedservice.AuditService
	publicBaseURL string
	deliveryLinkTTL time.Duration
}

func NewService(
	repo Repository,
	healthBI HealthBIClient,
	email EmailDelivery,
	users UserLookup,
	fileStorage storage.Storage,
	dwh *sql.DB,
) *Service {
	return &Service{repo: repo, healthBI: healthBI, email: email, users: users, fileStorage: fileStorage, dwh: dwh, deliveryLinkTTL: 24 * time.Hour}
}

func (s *Service) SetNotifications(notifications sharedservice.NotificationsService) { s.notifications = notifications }
func (s *Service) SetAudit(audit *sharedservice.AuditService) { s.audit = audit }
func (s *Service) SetPublicBaseURL(value string) { s.publicBaseURL = strings.TrimRight(strings.TrimSpace(value), "/") }
func (s *Service) SetDeliveryLinkTTL(value time.Duration) { if value > 0 { s.deliveryLinkTTL = value } }

func (s *Service) Module() ModuleResponse {
	healthBIEnabled := s != nil && s.healthBI != nil && s.healthBI.Enabled()
	return ModuleResponse{
		Name:              "Report Scheduler",
		Status:            "ready",
		SchedulingEnabled: s != nil && s.repo != nil,
		HealthBIEnabled:   healthBIEnabled,
	}
}

func (s *Service) ListReports(ctx context.Context) ([]HealthBIReport, error) {
	return s.healthBI.ListReports(ctx)
}

func (s *Service) GetReport(ctx context.Context, id string) (HealthBIReport, error) {
	return s.healthBI.GetReport(ctx, id)
}

func (s *Service) GetParameters(ctx context.Context, id string) ([]HealthBIParameter, error) {
	return s.healthBI.GetReportParameters(ctx, id)
}

func (s *Service) Generate(ctx context.Context, id string, request GenerateReportRequest) (HealthBIJob, error) {
	return s.healthBI.GenerateReport(ctx, id, request)
}

func (s *Service) GetJob(ctx context.Context, id string) (HealthBIJob, error) {
	return s.healthBI.GetJob(ctx, id)
}

func (s *Service) CreateSchedule(
	ctx context.Context,
	input CreateScheduleRequest,
	userID string,
	userHealth HealthContext,
) (Schedule, error) {
	normalized, err := s.validateSchedule(ctx, input, userHealth)
	if err != nil {
		return Schedule{}, err
	}
	item, err := s.repo.CreateSchedule(ctx, normalized, userID)
	if err == nil { s.auditEvent(ctx, userID, "report_scheduler.schedule_created", map[string]any{"schedule_id": item.ID, "report_id": item.HealthBIReportID, "health_context": item.HealthContext}) }
	return item, err
}

func (s *Service) ListSchedules(ctx context.Context, userID string, all bool, userHealth HealthContext, options ListOptions) ([]Schedule, error) {
	return s.repo.ListSchedules(ctx, userID, all, userHealth, options)
}

func (s *Service) GetSchedule(ctx context.Context, id, userID string, all bool, userHealth HealthContext) (Schedule, error) {
	item, err := s.repo.GetSchedule(ctx, id, userID, all)
	if err != nil { return Schedule{}, err }
	if all && !scheduleWithinHealthScope(item, userHealth) { return Schedule{}, ErrScheduleNotFound }
	return item, nil
}

func (s *Service) UpdateSchedule(
	ctx context.Context,
	id string,
	input UpdateScheduleRequest,
	userID string,
	all bool,
	userHealth HealthContext,
) (Schedule, error) {
	if _, err := s.GetSchedule(ctx, id, userID, all, userHealth); err != nil { return Schedule{}, err }
	normalized, err := s.validateSchedule(ctx, input, userHealth)
	if err != nil {
		return Schedule{}, err
	}
	item, err := s.repo.UpdateSchedule(ctx, id, normalized, userID, all)
	if err == nil { s.auditEvent(ctx, userID, "report_scheduler.schedule_updated", map[string]any{"schedule_id": item.ID, "report_id": item.HealthBIReportID, "health_context": item.HealthContext}) }
	return item, err
}

func (s *Service) DeleteSchedule(ctx context.Context, id, userID string, all bool, userHealth HealthContext) error {
	if _, err := s.GetSchedule(ctx, id, userID, all, userHealth); err != nil { return err }
	err := s.repo.DeleteSchedule(ctx, id, userID, all)
	if err == nil { s.auditEvent(ctx, userID, "report_scheduler.schedule_deleted", map[string]any{"schedule_id": id}) }
	return err
}

func (s *Service) PauseSchedule(ctx context.Context, id, userID string, all bool, userHealth HealthContext) (Schedule, error) {
	if _, err := s.GetSchedule(ctx, id, userID, all, userHealth); err != nil { return Schedule{}, err }
	item, err := s.repo.SetScheduleEnabled(ctx, id, userID, all, false)
	if err == nil { s.auditEvent(ctx, userID, "report_scheduler.schedule_paused", map[string]any{"schedule_id": id}) }
	return item, err
}

func (s *Service) ResumeSchedule(ctx context.Context, id, userID string, all bool, userHealth HealthContext) (Schedule, error) {
	if _, err := s.GetSchedule(ctx, id, userID, all, userHealth); err != nil { return Schedule{}, err }
	item, err := s.repo.SetScheduleEnabled(ctx, id, userID, all, true)
	if err == nil { s.auditEvent(ctx, userID, "report_scheduler.schedule_resumed", map[string]any{"schedule_id": id, "next_run_at": item.NextRunAt}) }
	return item, err
}

func (s *Service) DuplicateSchedule(ctx context.Context, id, userID string, all bool, userHealth HealthContext) (Schedule, error) {
	source, err := s.GetSchedule(ctx, id, userID, all, userHealth)
	if err != nil { return Schedule{}, err }
	disabled := false
	input := CreateScheduleRequest{
		HealthBIReportID: source.HealthBIReportID,
		ReportName:       source.ReportName + " Copy",
		Description:      source.Description,
		Frequency:        source.Frequency,
		CronExpression:   source.CronExpression,
		Timezone:         source.Timezone,
		PeriodStrategy:   source.PeriodStrategy,
		Parameters:       source.Parameters,
		OutputFormat:     source.OutputFormat,
		OutputConfig:     source.OutputConfig,
		Timing:           source.Timing,
		HealthContext:    source.HealthContext,
		Recipients:       source.Recipients,
		Enabled:          &disabled,
	}
	item, err := s.repo.CreateSchedule(ctx, input, userID)
	if err == nil { s.auditEvent(ctx, userID, "report_scheduler.schedule_duplicated", map[string]any{"source_schedule_id": id, "new_schedule_id": item.ID}) }
	return item, err
}

func (s *Service) RunNow(ctx context.Context, id, userID string, all bool, userHealth HealthContext) (Execution, error) {
	schedule, err := s.GetSchedule(ctx, id, userID, all, userHealth)
	if err != nil { return Execution{}, err }
	reference := time.Now()
	period, err := ResolveReportingPeriod(schedule.PeriodStrategy, schedule.Timezone, reference)
	if err != nil { return Execution{}, err }
	execution, err := s.repo.CreateExecution(ctx, schedule, "manual", userID, reference.UTC(), period, buildExecutionParameters(schedule, period))
	if err != nil { return Execution{}, err }
	s.auditEvent(ctx, userID, "report_scheduler.execution_started", map[string]any{
		"execution_id": execution.ID,
		"schedule_id": schedule.ID,
		"trigger_type": "manual",
	})
	if err := s.submitExecution(ctx, schedule, execution); err != nil {
		return execution, err
	}
	return s.repo.GetExecution(ctx, execution.ID)
}

func (s *Service) ListExecutions(ctx context.Context, userID string, all bool, userHealth HealthContext, options ListOptions) ([]Execution, error) {
	return s.repo.ListExecutions(ctx, userID, all, userHealth, options)
}

func (s *Service) Overview(ctx context.Context, userID string, all bool, userHealth HealthContext) (SchedulerOverview, error) {
	return s.repo.SchedulerOverview(ctx, userID, all, userHealth)
}

func (s *Service) UpdateWorkerHeartbeat(ctx context.Context, workerID string, interval time.Duration, heartbeat time.Time, cycleStarted, cycleFinished *time.Time, cycleErr string) error {
	if s == nil || s.repo == nil { return nil }
	return s.repo.UpdateWorkerHeartbeat(ctx, workerID, interval, heartbeat, cycleStarted, cycleFinished, cycleErr)
}

func (s *Service) GetExecutionDetail(ctx context.Context, id, userID string, all bool, userHealth HealthContext) (ExecutionDetail, error) {
	detail, err := s.repo.GetExecutionDetail(ctx, id, userID, all)
	if err != nil { return ExecutionDetail{}, err }
	if all && !healthContextEmpty(userHealth) {
		if detail.Schedule == nil || !scheduleWithinHealthScope(*detail.Schedule, userHealth) { return ExecutionDetail{}, ErrScheduleNotFound }
	}
	for i := range detail.Artifacts {
		detail.Artifacts[i].DownloadURL = "/api/v1/report-scheduler/artifacts/" + detail.Artifacts[i].ID + "/download"
	}
	return detail, nil
}

func (s *Service) ArtifactDownloadURL(ctx context.Context, artifactID, userID string, all bool, userHealth HealthContext) (Artifact, string, error) {
	artifact, execution, err := s.repo.GetArtifactForUser(ctx, artifactID, userID, all)
	if err != nil { return Artifact{}, "", err }
	if all && !healthContextEmpty(userHealth) {
		if execution.ScheduleID == nil { return Artifact{}, "", ErrScheduleNotFound }
		schedule, scheduleErr := s.repo.GetSchedule(ctx, *execution.ScheduleID, "", true)
		if scheduleErr != nil || !scheduleWithinHealthScope(schedule, userHealth) { return Artifact{}, "", ErrScheduleNotFound }
	}
	var downloadURL string
	if strings.TrimSpace(artifact.ObjectKey) != "" {
		if s.fileStorage == nil { return Artifact{}, "", errors.New("file storage is not configured") }
		downloadURL, err = s.fileStorage.GetDownloadURL(ctx, artifact.ObjectKey, artifact.FileName)
	} else if strings.TrimSpace(artifact.ExternalURL) != "" {
		downloadURL = artifact.ExternalURL
	} else {
		err = errors.New("artifact has no downloadable source")
	}
	if err != nil { return Artifact{}, "", err }
	s.auditEvent(ctx, userID, "report_scheduler.artifact_download", map[string]any{"artifact_id": artifact.ID, "execution_id": execution.ID, "report_id": execution.ReportID})
	return artifact, downloadURL, nil
}

func (s *Service) PublicDeliveryDownloadURL(ctx context.Context, token string) (Artifact, string, error) {
	token = strings.TrimSpace(token)
	if token == "" { return Artifact{}, "", ErrScheduleNotFound }
	delivery, artifact, execution, err := s.repo.GetDeliveryByAccessToken(ctx, hashDeliveryToken(token))
	if err != nil { return Artifact{}, "", err }
	var downloadURL string
	if strings.TrimSpace(artifact.ObjectKey) != "" {
		if s.fileStorage == nil { return Artifact{}, "", errors.New("file storage is not configured") }
		downloadURL, err = s.fileStorage.GetDownloadURL(ctx, artifact.ObjectKey, artifact.FileName)
	} else if strings.TrimSpace(artifact.ExternalURL) != "" {
		downloadURL = artifact.ExternalURL
	} else {
		err = errors.New("artifact has no downloadable source")
	}
	if err != nil { return Artifact{}, "", err }
	_ = s.repo.MarkDeliveryAccessTokenUsed(ctx, delivery.ID)
	s.auditEvent(ctx, "", "report_scheduler.delivery_link_download", map[string]any{
		"delivery_id": delivery.ID,
		"artifact_id": artifact.ID,
		"execution_id": execution.ID,
		"report_id": execution.ReportID,
	})
	return artifact, downloadURL, nil
}

func (s *Service) RetryExecution(ctx context.Context, id, userID string, all bool, userHealth HealthContext) (Execution, error) {
	detail, err := s.GetExecutionDetail(ctx, id, userID, all, userHealth)
	if err != nil { return Execution{}, err }
	execution := detail.Execution
	if execution.Status != "failed" {
		return execution, errors.New("only failed report executions can be retried manually")
	}
	if execution.ScheduleID == nil || detail.Schedule == nil {
		return execution, errors.New("execution is not attached to a report schedule")
	}
	schedule := *detail.Schedule

	if len(detail.Artifacts) > 0 {
		reset, resetErr := s.repo.ResetFailedDeliveries(ctx, execution.ID)
		if resetErr != nil { return execution, resetErr }
		if reset == 0 {
			if err := s.repo.UpdateExecutionJob(ctx, execution.ID, execution.HealthBIJobID, "delivering", ""); err != nil {
				return execution, err
			}
			if _, err := s.DeliverArtifact(ctx, execution.ID, schedule.ReportName, detail.Artifacts[0], schedule.Recipients); err != nil {
				return execution, err
			}
		}
		s.auditEvent(ctx, userID, "report_scheduler.delivery_retry_manual", map[string]any{
			"execution_id": execution.ID,
			"schedule_id": schedule.ID,
			"rearmed_deliveries": reset,
		})
		return s.repo.GetExecution(ctx, execution.ID)
	}

	execution, err = s.repo.ResetExecutionForManualRetry(ctx, id, userID, all)
	if err != nil { return Execution{}, err }
	s.auditEvent(ctx, userID, "report_scheduler.execution_retry_manual", map[string]any{"execution_id": execution.ID, "schedule_id": schedule.ID})
	if err := s.submitExecution(ctx, schedule, execution); err != nil { return execution, err }
	return s.repo.GetExecution(ctx, execution.ID)
}

func (s *Service) validateSchedule(
	ctx context.Context,
	input CreateScheduleRequest,
	userHealth HealthContext,
) (CreateScheduleRequest, error) {
	input.HealthBIReportID = strings.TrimSpace(input.HealthBIReportID)
	input.ReportName = strings.TrimSpace(input.ReportName)
	input.OutputFormat = strings.ToLower(strings.TrimSpace(input.OutputFormat))
	input.Frequency = strings.ToLower(strings.TrimSpace(input.Frequency))
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.PeriodStrategy = strings.TrimSpace(input.PeriodStrategy)
	if input.OutputConfig.Format == "" {
		input.OutputConfig.Format = input.OutputFormat
	}
	input.OutputConfig.Format = strings.ToLower(strings.TrimSpace(input.OutputConfig.Format))
	if input.OutputConfig.Format != input.OutputFormat {
		return input, errors.New("outputConfig.format must match outputFormat")
	}
	if input.Timezone == "" {
		input.Timezone = "Africa/Kampala"
	}
	timing, err := normalizeTiming(input.Timing)
	if err != nil {
		return input, err
	}
	input.Timing = timing
	if _, err := CalculateNextRun(input.Frequency, input.Timezone, input.Timing, time.Now()); err != nil {
		return input, err
	}
	if _, err := ResolveReportingPeriod(input.PeriodStrategy, input.Timezone, time.Now()); err != nil {
		return input, err
	}

	report, err := s.healthBI.GetReport(ctx, input.HealthBIReportID)
	if err != nil {
		return input, fmt.Errorf("validate Health BI report: %w", err)
	}
	if input.ReportName == "" {
		input.ReportName = report.Name
	}
	if len(report.SupportedFormats) > 0 && !containsFold(report.SupportedFormats, input.OutputFormat) {
		return input, fmt.Errorf("output format %q is not supported by Health BI report %q", input.OutputFormat, report.Name)
	}

	parameters, err := s.healthBI.GetReportParameters(ctx, input.HealthBIReportID)
	if err != nil {
		return input, fmt.Errorf("load Health BI report parameters: %w", err)
	}
	for _, parameter := range parameters {
		if !parameter.Required {
			continue
		}
		value, ok := input.Parameters[parameter.Name]
		if !ok || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return input, fmt.Errorf("required report parameter %q is missing", parameter.Name)
		}
	}

	input.HealthContext, err = s.constrainHealthContext(ctx, input.HealthContext, userHealth)
	if err != nil {
		return input, err
	}
	if err := validateRecipients(input.Recipients); err != nil {
		return input, err
	}
	return input, nil
}

func (s *Service) constrainHealthContext(ctx context.Context, requested, user HealthContext) (HealthContext, error) {
	requested.Level = strings.ToLower(strings.TrimSpace(requested.Level))
	requested.District = strings.TrimSpace(requested.District)
	requested.Facility = strings.TrimSpace(requested.Facility)
	user.District = strings.TrimSpace(user.District)
	user.Facility = strings.TrimSpace(user.Facility)

	if user.Facility != "" {
		if requested.Facility != "" && !strings.EqualFold(requested.Facility, user.Facility) {
			return requested, errors.New("requested facility is outside the authenticated user's health context")
		}
		if user.District != "" && requested.District != "" && !strings.EqualFold(requested.District, user.District) {
			return requested, errors.New("requested district is outside the authenticated user's health context")
		}
		requested.Facility = user.Facility
		requested.District = user.District
		requested.Level = "facility"
		return requested, nil
	}
	if user.District != "" {
		if requested.District != "" && !strings.EqualFold(requested.District, user.District) {
			return requested, errors.New("requested district is outside the authenticated user's health context")
		}
		if requested.Facility != "" {
			if s.dwh == nil {
				return requested, errors.New("facility scope validation requires the DWH connection")
			}
			var exists bool
			err := s.dwh.QueryRowContext(ctx, `SELECT EXISTS (
				SELECT 1 FROM dwh.dim_org_hierarchy
				WHERE is_current=true AND "level"='6'
				  AND (facility_uid=$1 OR facility_name=$1 OR org_unit_id=$1 OR org_unit_name=$1)
				  AND (district_uid=$2 OR district=$2)
			)`, requested.Facility, user.District).Scan(&exists)
			if err != nil {
				return requested, fmt.Errorf("validate facility health context: %w", err)
			}
			if !exists {
				return requested, errors.New("requested facility is outside the authenticated user's district")
			}
			requested.District = user.District
			requested.Level = "facility"
			return requested, nil
		}
		requested.District = user.District
		requested.Level = "district"
		return requested, nil
	}
	if requested.Facility != "" {
		if s.dwh == nil {
			return requested, errors.New("facility scope validation requires the DWH connection")
		}
		var district sql.NullString
		err := s.dwh.QueryRowContext(ctx, `SELECT district
			FROM dwh.dim_org_hierarchy
			WHERE is_current=true AND "level"='6'
			  AND (facility_uid=$1 OR facility_name=$1 OR org_unit_id=$1 OR org_unit_name=$1)
			LIMIT 1`, requested.Facility).Scan(&district)
		if errors.Is(err, sql.ErrNoRows) {
			return requested, errors.New("requested facility was not found in the health hierarchy")
		}
		if err != nil {
			return requested, fmt.Errorf("validate facility health context: %w", err)
		}
		if requested.District != "" && district.Valid && !strings.EqualFold(requested.District, district.String) {
			return requested, errors.New("requested facility does not belong to the requested district")
		}
		if requested.District == "" && district.Valid {
			requested.District = district.String
		}
		requested.Level = "facility"
		return requested, nil
	}
	if requested.District != "" {
		if s.dwh == nil {
			return requested, errors.New("district scope validation requires the DWH connection")
		}
		var exists bool
		err := s.dwh.QueryRowContext(ctx, `SELECT EXISTS (
			SELECT 1 FROM dwh.dim_org_hierarchy
			WHERE is_current=true AND (district_uid=$1 OR district=$1)
		)`, requested.District).Scan(&exists)
		if err != nil {
			return requested, fmt.Errorf("validate district health context: %w", err)
		}
		if !exists {
			return requested, errors.New("requested district was not found in the health hierarchy")
		}
		requested.Level = "district"
		return requested, nil
	}
	requested.Level = "national"
	return requested, nil
}

func validateRecipients(recipients []ScheduleRecipient) error {
	for _, recipient := range recipients {
		recipientType := strings.ToLower(strings.TrimSpace(recipient.Type))
		channel := strings.ToLower(strings.TrimSpace(recipient.DeliveryChannel))
		value := strings.TrimSpace(recipient.Value)
		if value == "" {
			return errors.New("recipient value is required")
		}
		switch recipientType {
		case "user", "group", "email":
		default:
			return fmt.Errorf("unsupported recipient type %q", recipient.Type)
		}
		switch channel {
		case "email":
		case "portal":
			if recipientType != "user" {
				return errors.New("portal delivery currently supports user recipients only")
			}
		default:
			return fmt.Errorf("unsupported delivery channel %q", recipient.DeliveryChannel)
		}
		if recipientType == "email" {
			if _, err := mail.ParseAddress(value); err != nil {
				return fmt.Errorf("invalid email recipient %q", value)
			}
		}
		if recipientType == "user" {
			if _, err := uuid.Parse(value); err != nil {
				return fmt.Errorf("invalid user recipient %q", value)
			}
		}
	}
	return nil
}

func (s *Service) PreviewRecipients(ctx context.Context, recipients []ScheduleRecipient) (RecipientPreview, error) {
	if err := validateRecipients(recipients); err != nil {
		return RecipientPreview{}, err
	}
	emailAddresses, portalUsers, err := s.resolveRecipients(ctx, recipients)
	if err != nil {
		return RecipientPreview{}, err
	}
	preview := RecipientPreview{
		EmailRecipients:  make([]string, 0, len(emailAddresses)),
		PortalRecipients: portalUsers,
		EmailCount:       len(emailAddresses),
		PortalCount:      len(portalUsers),
	}
	for _, address := range emailAddresses {
		preview.EmailRecipients = append(preview.EmailRecipients, address.Email)
	}
	return preview, nil
}

func (s *Service) resolveRecipients(
	ctx context.Context,
	recipients []ScheduleRecipient,
) ([]model.Address, []string, error) {
	emailByAddress := map[string]model.Address{}
	portalUsers := map[string]struct{}{}
	groupIDs := []string{}
	groupPaths := []string{}

	for _, recipient := range recipients {
		recipientType := strings.ToLower(strings.TrimSpace(recipient.Type))
		channel := strings.ToLower(strings.TrimSpace(recipient.DeliveryChannel))
		value := strings.TrimSpace(recipient.Value)
		if channel == "portal" {
			portalUsers[value] = struct{}{}
			continue
		}
		switch recipientType {
		case "email":
			parsed, _ := mail.ParseAddress(value)
			emailByAddress[strings.ToLower(parsed.Address)] = model.Address{Name: parsed.Name, Email: parsed.Address}
		case "user":
			if s.users == nil {
				return nil, nil, errors.New("user repository is required to resolve report recipients")
			}
			userID, _ := uuid.Parse(value)
			user, err := s.users.GetUserByID(userID)
			if err != nil {
				return nil, nil, fmt.Errorf("resolve report recipient user %s: %w", value, err)
			}
			if user != nil && user.Enabled && strings.TrimSpace(user.Email) != "" {
				emailByAddress[strings.ToLower(user.Email)] = model.Address{Name: user.FullName, Email: user.Email}
			}
		case "group":
			if strings.HasPrefix(value, "/") {
				groupPaths = append(groupPaths, value)
			} else {
				groupIDs = append(groupIDs, value)
			}
		}
	}

	if len(groupIDs)+len(groupPaths) > 0 {
		if s.email == nil {
			return nil, nil, errors.New("email service is required to resolve group recipients")
		}
		resolved, err := s.email.ResolveGroupEmailRecipients(ctx, groupIDs, groupPaths)
		if err != nil {
			return nil, nil, err
		}
		for _, address := range resolved {
			if strings.TrimSpace(address.Email) != "" {
				emailByAddress[strings.ToLower(address.Email)] = address
			}
		}
	}

	emails := make([]model.Address, 0, len(emailByAddress))
	for _, address := range emailByAddress {
		emails = append(emails, address)
	}
	portal := make([]string, 0, len(portalUsers))
	for id := range portalUsers {
		portal = append(portal, id)
	}
	return emails, portal, nil
}

func (s *Service) DeliverArtifact(
	ctx context.Context,
	executionID string,
	reportName string,
	artifact Artifact,
	recipients []ScheduleRecipient,
) (Artifact, error) {
	_ = reportName
	if s.repo == nil {
		return Artifact{}, errors.New("report scheduler repository is not configured")
	}
	if strings.TrimSpace(artifact.ExternalURL) == "" && strings.TrimSpace(artifact.ObjectKey) == "" {
		return Artifact{}, errors.New("artifact requires objectKey or externalUrl")
	}
	stored, err := s.repo.CreateArtifact(ctx, artifact)
	if err != nil {
		return Artifact{}, err
	}
	emails, portalUsers, err := s.resolveRecipients(ctx, recipients)
	if err != nil {
		return Artifact{}, err
	}
	for _, address := range emails {
		if _, err := s.repo.CreatePendingDelivery(ctx, executionID, stored.ID, "email", address.Email, "email"); err != nil {
			return Artifact{}, err
		}
	}
	for _, userID := range portalUsers {
		if _, err := s.repo.CreatePendingDelivery(ctx, executionID, stored.ID, "user", userID, "portal"); err != nil {
			return Artifact{}, err
		}
	}
	return stored, nil
}

func (s *Service) ListPortalReports(ctx context.Context, userID string, limit int) ([]PortalReport, error) {
	items, err := s.repo.ListPortalReports(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Artifact.DownloadURL = "/api/v1/report-scheduler/artifacts/" + items[index].Artifact.ID + "/download"
	}
	return items, nil
}

func (s *Service) ProcessOneDue(ctx context.Context) (bool, error) {
	schedule, execution, found, err := s.repo.ClaimDueExecution(ctx, time.Now())
	if err != nil || !found { return found, err }
	s.auditEvent(ctx, schedule.CreatedBy, "report_scheduler.execution_started", map[string]any{
		"execution_id": execution.ID,
		"schedule_id": schedule.ID,
		"trigger_type": execution.TriggerType,
	})
	if err := s.submitExecution(ctx, schedule, execution); err != nil {
		return true, err
	}
	return true, nil
}

func (s *Service) ProcessOneRetry(ctx context.Context) (bool, error) {
	execution, found, err := s.repo.ClaimRetryExecution(ctx, time.Now())
	if err != nil || !found { return found, err }
	if execution.ScheduleID == nil {
		_ = s.repo.UpdateExecutionJob(ctx, execution.ID, execution.HealthBIJobID, "failed", "retry execution has no schedule")
		return true, nil
	}
	schedule, err := s.repo.GetSchedule(ctx, *execution.ScheduleID, "", true)
	if err != nil {
		_ = s.repo.UpdateExecutionJob(ctx, execution.ID, execution.HealthBIJobID, "failed", err.Error())
		return true, nil
	}
	s.auditEvent(ctx, schedule.CreatedBy, "report_scheduler.execution_retry_started", map[string]any{
		"execution_id": execution.ID,
		"schedule_id": schedule.ID,
		"attempt": execution.GenerationAttempts + 1,
	})
	return true, s.submitExecution(ctx, schedule, execution)
}

func (s *Service) PollGenerating(ctx context.Context, limit int) error {
	executions, err := s.repo.ClaimGeneratingExecutions(ctx, limit)
	if err != nil { return err }
	for _, execution := range executions {
		if execution.ScheduleID == nil || strings.TrimSpace(execution.HealthBIJobID) == "" {
			_ = s.repo.UpdateExecutionJob(ctx, execution.ID, execution.HealthBIJobID, "failed", "execution is missing schedule or Health BI job reference")
			continue
		}
		schedule, err := s.repo.GetSchedule(ctx, *execution.ScheduleID, "", true)
		if err != nil {
			_ = s.repo.UpdateExecutionJob(ctx, execution.ID, execution.HealthBIJobID, "failed", err.Error())
			continue
		}
		job, err := s.healthBI.GetJob(ctx, execution.HealthBIJobID)
		if err != nil {
			_ = s.repo.UpdateExecutionJob(ctx, execution.ID, execution.HealthBIJobID, "generating", "")
			continue
		}
		_ = s.applyJobResult(ctx, schedule, execution, job)
	}
	return nil
}

func (s *Service) ProcessOneDelivery(ctx context.Context) (bool, error) {
	delivery, found, err := s.repo.ClaimDueDelivery(ctx, time.Now())
	if err != nil || !found { return found, err }

	execution, err := s.repo.GetExecution(ctx, delivery.ExecutionID)
	if err != nil {
		_ = s.failOrRetryDelivery(ctx, delivery, err)
		return true, nil
	}
	if execution.ScheduleID == nil {
		_ = s.failOrRetryDelivery(ctx, delivery, errors.New("delivery execution has no schedule"))
		return true, nil
	}
	schedule, err := s.repo.GetSchedule(ctx, *execution.ScheduleID, "", true)
	if err != nil {
		_ = s.failOrRetryDelivery(ctx, delivery, err)
		return true, nil
	}
	artifact, err := s.repo.GetArtifact(ctx, delivery.ArtifactID)
	if err != nil {
		_ = s.failOrRetryDelivery(ctx, delivery, err)
		return true, nil
	}

	if delivery.DeliveryChannel == "email" {
		if s.email == nil {
			err = errors.New("email service is not configured")
		} else {
			if s.publicBaseURL == "" {
				err = errors.New("APP_BASE_URL is required for secure report delivery links")
			} else {
				token, tokenErr := newDeliveryToken()
				if tokenErr != nil {
					err = tokenErr
				} else {
					expiresAt := time.Now().Add(s.deliveryLinkTTL)
					if tokenErr = s.repo.SetDeliveryAccessToken(ctx, delivery.ID, hashDeliveryToken(token), expiresAt); tokenErr != nil {
						err = tokenErr
					} else {
						downloadURL := s.publicBaseURL + "/api/v1/report-scheduler/public/deliveries/" + token + "/download"
						err = s.email.Queue(ctx, model.Message{
							To:       []model.Address{{Email: delivery.RecipientValue}},
							Subject:  fmt.Sprintf("%s report is ready", schedule.ReportName),
							TextBody: fmt.Sprintf("Your scheduled report %q is ready. Download it here: %s", schedule.ReportName, downloadURL),
							HTMLBody: fmt.Sprintf("<p>Your scheduled report <strong>%s</strong> is ready.</p><p><a href=%q>Download report</a></p>", html.EscapeString(schedule.ReportName), downloadURL),
							Metadata: map[string]string{"report_execution_id": execution.ID, "report_artifact_id": artifact.ID},
						})
					}
				}
			}
		}
	}
	// Portal delivery is represented by the delivery record itself. Once it is
	// marked sent it becomes visible from /portal-reports.
	if err != nil {
		_ = s.failOrRetryDelivery(ctx, delivery, err)
		return true, nil
	}
	if err := s.repo.MarkDeliverySent(ctx, delivery.ID); err != nil { return true, err }
	return true, s.finalizeDeliveryExecution(ctx, schedule, execution)
}

func (s *Service) submitExecution(ctx context.Context, schedule Schedule, execution Execution) error {
	if err := s.repo.MarkGenerationAttempt(ctx, execution.ID); err != nil { return err }
	execution.GenerationAttempts++
	job, err := s.healthBI.GenerateReport(ctx, schedule.HealthBIReportID, GenerateReportRequest{
		Parameters: execution.Parameters,
		Format:     schedule.OutputFormat,
	})
	if err != nil {
		return s.handleGenerationFailure(ctx, schedule, execution, "", err.Error())
	}
	return s.applyJobResult(ctx, schedule, execution, job)
}

func (s *Service) applyJobResult(ctx context.Context, schedule Schedule, execution Execution, job HealthBIJob) error {
	status := strings.ToLower(strings.TrimSpace(job.Status))
	if strings.TrimSpace(job.ID) == "" && status != "completed" && status != "complete" && status != "success" && status != "succeeded" && status != "ready" && status != "generated" {
		return s.handleGenerationFailure(ctx, schedule, execution, "", "Health BI generation response did not include a job ID")
	}
	switch status {
	case "completed", "complete", "success", "succeeded", "ready", "generated":
		if strings.TrimSpace(job.ArtifactURL) == "" {
			err := errors.New("Health BI job completed without an artifact URL")
			return s.handleGenerationFailure(ctx, schedule, execution, job.ID, err.Error())
		}
		if err := s.repo.UpdateExecutionJob(ctx, execution.ID, job.ID, "delivering", ""); err != nil { return err }
		fileName := reportArtifactFileName(schedule)
		_, err := s.DeliverArtifact(ctx, execution.ID, schedule.ReportName, Artifact{
			ExecutionID: execution.ID,
			FileName:    fileName,
			ExternalURL: job.ArtifactURL,
		}, schedule.Recipients)
		if err != nil {
			_ = s.repo.UpdateExecutionJob(ctx, execution.ID, job.ID, "failed", err.Error())
			s.auditEvent(ctx, schedule.CreatedBy, "report_scheduler.execution_failed", map[string]any{
				"execution_id": execution.ID,
				"schedule_id": schedule.ID,
				"stage": "delivery_setup",
				"error": err.Error(),
			})
			s.notifyExecutionFailure(ctx, schedule, execution, err.Error())
			return nil
		}
		status, changed, err := s.repo.FinalizeExecutionDeliveryState(ctx, execution.ID)
		if err != nil { return err }
		if changed && status == "completed" {
			s.auditEvent(ctx, schedule.CreatedBy, "report_scheduler.execution_completed", map[string]any{
				"execution_id": execution.ID,
				"schedule_id": schedule.ID,
				"generation_attempts": execution.GenerationAttempts,
			})
		}
		return nil
	case "failed", "error", "cancelled", "canceled":
		message := strings.TrimSpace(job.Error); if message == "" { message = "Health BI report generation failed" }
		return s.handleGenerationFailure(ctx, schedule, execution, job.ID, message)
	default:
		return s.repo.UpdateExecutionJob(ctx, execution.ID, job.ID, "generating", "")
	}
}

func (s *Service) handleGenerationFailure(ctx context.Context, schedule Schedule, execution Execution, jobID, message string) error {
	maxAttempts := execution.MaxGenerationAttempts
	if maxAttempts <= 0 { maxAttempts = 3 }
	if execution.GenerationAttempts < maxAttempts {
		next := time.Now().Add(reportRetryDelay(execution.GenerationAttempts))
		if err := s.repo.ScheduleExecutionRetry(ctx, execution.ID, message, next); err != nil { return err }
		s.auditEvent(ctx, schedule.CreatedBy, "report_scheduler.generation_retry_scheduled", map[string]any{
			"execution_id": execution.ID,
			"schedule_id": schedule.ID,
			"attempt": execution.GenerationAttempts,
			"max_attempts": maxAttempts,
			"next_retry_at": next.UTC(),
			"error": message,
		})
		return nil
	}
	if err := s.repo.UpdateExecutionJob(ctx, execution.ID, jobID, "failed", message); err != nil { return err }
	s.auditEvent(ctx, schedule.CreatedBy, "report_scheduler.execution_failed", map[string]any{
		"execution_id": execution.ID,
		"schedule_id": schedule.ID,
		"stage": "generation",
		"attempts": execution.GenerationAttempts,
		"error": message,
	})
	s.notifyExecutionFailure(ctx, schedule, execution, message)
	return nil
}

func (s *Service) failOrRetryDelivery(ctx context.Context, delivery Delivery, cause error) error {
	nextAttempts := delivery.Attempts + 1
	maxAttempts := delivery.MaxAttempts
	if maxAttempts <= 0 { maxAttempts = 3 }
	if nextAttempts >= maxAttempts {
		if err := s.repo.MarkDeliveryFailed(ctx, delivery.ID, cause.Error()); err != nil { return err }
		execution, execErr := s.repo.GetExecution(ctx, delivery.ExecutionID)
		if execErr != nil || execution.ScheduleID == nil { return execErr }
		schedule, scheduleErr := s.repo.GetSchedule(ctx, *execution.ScheduleID, "", true)
		if scheduleErr != nil { return scheduleErr }
		return s.finalizeDeliveryExecution(ctx, schedule, execution)
	}
	next := time.Now().Add(reportRetryDelay(nextAttempts))
	return s.repo.ScheduleDeliveryRetry(ctx, delivery.ID, cause.Error(), next)
}

func (s *Service) finalizeDeliveryExecution(ctx context.Context, schedule Schedule, execution Execution) error {
	status, changed, err := s.repo.FinalizeExecutionDeliveryState(ctx, execution.ID)
	if err != nil || !changed { return err }
	if status == "failed" {
		message := "one or more report deliveries failed permanently"
		s.auditEvent(ctx, schedule.CreatedBy, "report_scheduler.execution_failed", map[string]any{
			"execution_id": execution.ID,
			"schedule_id": schedule.ID,
			"stage": "delivery",
			"error": message,
		})
		s.notifyExecutionFailure(ctx, schedule, execution, message)
		return nil
	}
	s.auditEvent(ctx, schedule.CreatedBy, "report_scheduler.execution_completed", map[string]any{
		"execution_id": execution.ID,
		"schedule_id": schedule.ID,
		"generation_attempts": execution.GenerationAttempts,
	})
	return nil
}

func reportRetryDelay(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return 30 * time.Second
	case attempts == 2:
		return 2 * time.Minute
	default:
		return 5 * time.Minute
	}
}

func newDeliveryToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil { return "", fmt.Errorf("generate delivery token: %w", err) }
	return hex.EncodeToString(value), nil
}

func hashDeliveryToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func healthContextEmpty(value HealthContext) bool {
	return strings.TrimSpace(value.District)=="" && strings.TrimSpace(value.Facility)==""
}

func scheduleWithinHealthScope(schedule Schedule, scope HealthContext) bool {
	if healthContextEmpty(scope) { return true }
	if facility:=strings.TrimSpace(scope.Facility); facility!="" { return strings.EqualFold(strings.TrimSpace(schedule.HealthContext.Facility),facility) }
	if district:=strings.TrimSpace(scope.District); district!="" { return strings.EqualFold(strings.TrimSpace(schedule.HealthContext.District),district) }
	return true
}

func (s *Service) notifyExecutionFailure(ctx context.Context, schedule Schedule, execution Execution, message string) {
	claimed, err := s.repo.ClaimFailureNotification(ctx, execution.ID)
	if err != nil || !claimed { return }

	metadata, _ := json.Marshal(map[string]any{
		"execution_id": execution.ID,
		"schedule_id": schedule.ID,
		"report_id": schedule.HealthBIReportID,
		"owner_id": schedule.CreatedBy,
		"error": message,
	})
	if s.notifications != nil {
		_, _ = s.notifications.Notify(ctx, model.Notification{
			Type:       "REPORT_SCHEDULER_FAILURE",
			Title:      "Scheduled report failed",
			Message:    fmt.Sprintf("%s: %s", schedule.ReportName, message),
			Severity:   "critical",
			TargetRole: "admin",
			Metadata:   metadata,
		})
	}

	ownerID, parseErr := uuid.Parse(strings.TrimSpace(schedule.CreatedBy))
	if parseErr != nil || s.users == nil || s.email == nil { return }
	owner, userErr := s.users.GetUserByID(ownerID)
	if userErr != nil || owner == nil || strings.TrimSpace(owner.Email) == "" { return }
	_ = s.email.Queue(ctx, model.Message{
		To:       []model.Address{{Name: owner.FullName, Email: owner.Email}},
		Subject:  fmt.Sprintf("Scheduled report failed: %s", schedule.ReportName),
		TextBody: fmt.Sprintf("The scheduled report %q failed after retries. Error: %s", schedule.ReportName, message),
		HTMLBody: fmt.Sprintf("<p>The scheduled report <strong>%s</strong> failed after retries.</p><p>%s</p>", html.EscapeString(schedule.ReportName), html.EscapeString(message)),
		Metadata: map[string]string{"report_execution_id": execution.ID, "report_schedule_id": schedule.ID},
	})
}

func (s *Service) auditEvent(ctx context.Context, userID, action string, metadata map[string]any) {
	if s == nil || s.audit == nil { return }
	var actor uuid.NullUUID
	if parsed, err := uuid.Parse(strings.TrimSpace(userID)); err == nil {
		actor = uuid.NullUUID{UUID: parsed, Valid: true}
	}
	_ = s.audit.Log(ctx, actor, action, metadata)
}

func reportArtifactFileName(schedule Schedule) string {
	prefix := strings.TrimSpace(schedule.OutputConfig.FileNamePrefix)
	if prefix == "" { prefix = schedule.ReportName }
	prefix = strings.TrimSpace(strings.Map(func(r rune) rune {
		switch r { case '/', '\\', ':', '*', '?', '"', '<', '>', '|': return '-'; default: return r }
	}, prefix))
	if prefix == "" { prefix = "report" }
	ext := strings.TrimPrefix(filepath.Ext(prefix), ".")
	if ext != "" { prefix = strings.TrimSuffix(prefix, filepath.Ext(prefix)) }
	return prefix + "." + strings.ToLower(schedule.OutputFormat)
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}
