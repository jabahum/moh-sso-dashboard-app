package report_scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	emailfeature "github.com/moh-sso-dashboard/internal/features/email"
	userfeature "github.com/moh-sso-dashboard/internal/features/users"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/storage"
)

type Service struct {
	repo        Repository
	healthBI    HealthBIClient
	email       *emailfeature.Service
	users       userfeature.UserRepository
	fileStorage storage.Storage
	dwh         *sql.DB
}

func NewService(
	repo Repository,
	healthBI HealthBIClient,
	email *emailfeature.Service,
	users userfeature.UserRepository,
	fileStorage storage.Storage,
	dwh *sql.DB,
) *Service {
	return &Service{repo: repo, healthBI: healthBI, email: email, users: users, fileStorage: fileStorage, dwh: dwh}
}

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
	return s.repo.CreateSchedule(ctx, normalized, userID)
}

func (s *Service) ListSchedules(ctx context.Context, userID string, all bool) ([]Schedule, error) {
	return s.repo.ListSchedules(ctx, userID, all)
}

func (s *Service) GetSchedule(ctx context.Context, id, userID string, all bool) (Schedule, error) {
	return s.repo.GetSchedule(ctx, id, userID, all)
}

func (s *Service) UpdateSchedule(
	ctx context.Context,
	id string,
	input UpdateScheduleRequest,
	userID string,
	all bool,
	userHealth HealthContext,
) (Schedule, error) {
	normalized, err := s.validateSchedule(ctx, input, userHealth)
	if err != nil {
		return Schedule{}, err
	}
	return s.repo.UpdateSchedule(ctx, id, normalized, userID, all)
}

func (s *Service) DeleteSchedule(ctx context.Context, id, userID string, all bool) error {
	return s.repo.DeleteSchedule(ctx, id, userID, all)
}

func (s *Service) ListExecutions(ctx context.Context, userID string, all bool) ([]Execution, error) {
	return s.repo.ListExecutions(ctx, userID, all)
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
	downloadURL := stored.ExternalURL
	if stored.ObjectKey != "" && s.fileStorage != nil {
		if generated, urlErr := s.fileStorage.GetDownloadURL(ctx, stored.ObjectKey, stored.FileName); urlErr == nil {
			downloadURL = generated
		}
	}
	if len(emails) > 0 {
		if s.email == nil {
			return Artifact{}, errors.New("email service is not configured")
		}
		message := model.Message{
			To:       emails,
			Subject:  fmt.Sprintf("%s report is ready", reportName),
			TextBody: fmt.Sprintf("Your scheduled report %q is ready. Download it here: %s", reportName, downloadURL),
			HTMLBody: fmt.Sprintf("<p>Your scheduled report <strong>%s</strong> is ready.</p><p><a href=%q>Download report</a></p>", reportName, downloadURL),
			Metadata: map[string]string{"report_execution_id": executionID, "report_artifact_id": stored.ID},
		}
		if err := s.email.Queue(ctx, message); err != nil {
			return Artifact{}, fmt.Errorf("queue report delivery email: %w", err)
		}
		for _, address := range emails {
			_ = s.repo.CreateDelivery(ctx, executionID, stored.ID, "email", address.Email, "email")
		}
	}
	for _, userID := range portalUsers {
		if err := s.repo.CreateDelivery(ctx, executionID, stored.ID, "user", userID, "portal"); err != nil {
			return Artifact{}, err
		}
	}
	return stored, nil
}

func (s *Service) ListPortalReports(ctx context.Context, userID string) ([]PortalReport, error) {
	items, err := s.repo.ListPortalReports(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.fileStorage == nil {
		return items, nil
	}
	for index := range items {
		if items[index].Artifact.ObjectKey == "" {
			continue
		}
		url, err := s.fileStorage.GetDownloadURL(ctx, items[index].Artifact.ObjectKey, items[index].Artifact.FileName)
		if err == nil {
			items[index].Artifact.ExternalURL = url
		}
	}
	return items, nil
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}
