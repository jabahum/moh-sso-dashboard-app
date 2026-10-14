package data_quality

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/email"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/moh-sso-dashboard/internal/model"
	sharedservice "github.com/moh-sso-dashboard/internal/service"
)

func isNilInterface(i interface{}) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

type Service interface {
	CreateIssue(ctx context.Context, input createIssueInput) (issueResponse, error)
	ListIssues(ctx context.Context, filter listIssuesFilter) ([]issueResponse, error)
	GetIssueByCode(ctx context.Context, issueCode string) (issueResponse, error)
	ListIssueSummaryByProgram(ctx context.Context, limit int, offset int) ([]issueProgramSummaryResponse, error)
	UpdateIssue(ctx context.Context, input updateIssueInput) (issueResponse, error)
	ResolveIssue(ctx context.Context, input resolveIssueInput) (issueStageResponse, error)
	AssignIssues(ctx context.Context, input assignIssuesInput) ([]issueStageResponse, error)
	ListIssueResolutionTransactions(ctx context.Context, issueCode string, limit int, offset int) ([]issueStageResponse, error)
	CountIssues(ctx context.Context, filter listIssuesFilter) (int64, error)
}

type service struct {
	repository     Repository
	emailService   sharedservice.EmailService
	keyAdminClient *keycloak.KeyAdminClient
	cfg            *config.Config
}

func NewService(
	repository Repository,
	emailService sharedservice.EmailService,
	keyAdminClient *keycloak.KeyAdminClient,
	cfg ...*config.Config,
) Service {
	var appCfg *config.Config
	if len(cfg) > 0 {
		appCfg = cfg[0]
	}
	return &service{
		repository:     repository,
		emailService:   emailService,
		keyAdminClient: keyAdminClient,
		cfg:            appCfg,
	}
}

func (s *service) CreateIssue(ctx context.Context, input createIssueInput) (issueResponse, error) {
	return s.repository.CreateIssue(ctx, input)
}

func (s *service) ListIssues(ctx context.Context, filter listIssuesFilter) ([]issueResponse, error) {
	return s.repository.ListIssues(ctx, filter)
}

func (s *service) GetIssueByCode(ctx context.Context, issueCode string) (issueResponse, error) {
	issues, err := s.repository.GetIssuesByCodes(ctx, []string{issueCode})
	if err != nil {
		return issueResponse{}, err
	}
	if len(issues) == 0 {
		return issueResponse{}, fmt.Errorf("issue not found: %s", issueCode)
	}
	return issues[0], nil
}

func (s *service) ListIssueSummaryByProgram(ctx context.Context, limit int, offset int) ([]issueProgramSummaryResponse, error) {
	return s.repository.ListIssueSummaryByProgram(ctx, limit, offset)
}

func (s *service) UpdateIssue(ctx context.Context, input updateIssueInput) (issueResponse, error) {
	return s.repository.UpdateIssue(ctx, input)
}

func (s *service) ResolveIssue(ctx context.Context, input resolveIssueInput) (issueStageResponse, error) {
	result, err := s.repository.ResolveIssue(ctx, input)
	if err != nil {
		return issueStageResponse{}, err
	}

	go s.sendApprovalEmailToGroupMembers(context.Background(), input, result)

	return result, nil
}

func dqInterfaceString(val interface{}) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return strings.TrimSpace(s)
	}
	if ptr, ok := val.(*string); ok && ptr != nil {
		return strings.TrimSpace(*ptr)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", val))
}

func (s *service) sendApprovalEmailToGroupMembers(ctx context.Context, input resolveIssueInput, result issueStageResponse) {
	defer func() {
		if r := recover(); r != nil {
			// Prevent panic from crashing background goroutine
		}
	}()

	if isNilInterface(s.keyAdminClient) {
		return
	}

	approverIdentifier := dqInterfaceString(input.ResolvedBy)
	if approverIdentifier == "" {
		return
	}

	// 1. Resolve Keycloak user ID for the approver
	userID := approverIdentifier
	if _, err := uuid.Parse(approverIdentifier); err != nil {
		users, err := s.keyAdminClient.FindUsers(ctx, approverIdentifier, true)
		if err != nil || len(users) == 0 {
			users, err = s.keyAdminClient.FindUsers(ctx, approverIdentifier, false)
		}
		if len(users) > 0 {
			userID = users[0].ID
		} else {
			return
		}
	}

	// 2. Retrieve all Keycloak groups where the approver belongs
	groups, err := s.keyAdminClient.GetUserGroups(ctx, userID)
	if err != nil || len(groups) == 0 {
		return
	}

	// 3. For each group, retrieve members and collect recipient emails
	recipientEmailsMap := make(map[string]struct{})
	for _, grp := range groups {
		members, err := s.keyAdminClient.ListGroupMembers(ctx, grp.ID)
		if err != nil {
			continue
		}
		for _, member := range members {
			emailAddr := strings.ToLower(strings.TrimSpace(member.Email))
			if emailAddr != "" {
				recipientEmailsMap[emailAddr] = struct{}{}
			}
		}
	}

	if len(recipientEmailsMap) == 0 {
		return
	}

	recipientAddresses := make([]model.Address, 0, len(recipientEmailsMap))
	for emailAddr := range recipientEmailsMap {
		recipientAddresses = append(recipientAddresses, model.Address{Email: emailAddr})
	}

	// 4. Fetch issue details for description and metadata
	issueCode := input.IssueCode
	fetchedIssues, _ := s.repository.GetIssuesByCodes(ctx, []string{issueCode})
	var issueDetail issueResponse
	if len(fetchedIssues) > 0 {
		issueDetail = fetchedIssues[0]
	}

	desc := "No description provided"
	if issueDetail.Issue != nil && strings.TrimSpace(*issueDetail.Issue) != "" {
		desc = strings.TrimSpace(*issueDetail.Issue)
	}

	datasetStr := ""
	if issueDetail.Dataset != nil {
		datasetStr = *issueDetail.Dataset
	}

	orgUnitStr := ""
	if issueDetail.OrgUnit != nil {
		orgUnitStr = *issueDetail.OrgUnit
	}

	statusStr := input.Status
	if statusStr == "" {
		statusStr = "RESOLVED"
	}

	resolutionAction := dqInterfaceString(input.ResolutionAction)

	// Determine portal link
	portalURL := "https://dashboards.health.go.ug/portal"
	if s.cfg != nil && strings.TrimSpace(s.cfg.FrontendBaseURL) != "" {
		portalURL = strings.TrimRight(strings.TrimSpace(s.cfg.FrontendBaseURL), "/")
	}
	if !strings.HasSuffix(portalURL, "/portal") {
		portalURL = portalURL + "/portal"
	}

	subject := fmt.Sprintf("[MOH Issue Tracker] Issue %s Resolved by %s", issueCode, approverIdentifier)

	var bodyBuilder strings.Builder
	var textBuilder strings.Builder
	bodyBuilder.WriteString(fmt.Sprintf("<p>Hello,</p><p>Issue <strong>%s</strong> has been <strong>resolved</strong> by team member <strong>%s</strong>:</p>", issueCode, approverIdentifier))

	bodyBuilder.WriteString("<div style='border: 1px solid #e0e0e0; border-left: 4px solid #198038; padding: 12px 16px; margin-bottom: 16px; background-color: #f8f9fa; border-radius: 4px;'>")
	bodyBuilder.WriteString(fmt.Sprintf("<div style='font-size: 15px; font-weight: bold; color: #198038;'>Issue Code: %s &nbsp; (Status: %s)</div>", issueCode, statusStr))
	bodyBuilder.WriteString(fmt.Sprintf("<div style='margin-top: 6px; font-size: 14px; color: #161616;'><strong>Description:</strong> %s</div>", desc))

	metaParts := make([]string, 0, 2)
	if datasetStr != "" {
		metaParts = append(metaParts, fmt.Sprintf("<strong>Dataset:</strong> %s", datasetStr))
	}
	if orgUnitStr != "" {
		metaParts = append(metaParts, fmt.Sprintf("<strong>Organization Unit:</strong> %s", orgUnitStr))
	}
	if len(metaParts) > 0 {
		bodyBuilder.WriteString(fmt.Sprintf("<div style='margin-top: 6px; font-size: 12px; color: #525252;'>%s</div>", strings.Join(metaParts, " &nbsp;|&nbsp; ")))
	}
	bodyBuilder.WriteString("</div>")

	if resolutionAction != "" {
		bodyBuilder.WriteString(fmt.Sprintf("<div style='background-color: #defbe6; border: 1px solid #a7f0ba; padding: 10px 14px; border-radius: 4px; margin-bottom: 16px;'><strong>Resolution Notes:</strong> %s</div>", resolutionAction))
	}

	issueLink := fmt.Sprintf("%s/apps/dwh/issue-tracker/issues?issueCode=%s", portalURL, issueCode)

	bodyBuilder.WriteString(fmt.Sprintf("<p style='margin-top: 20px;'><a href='%s' style='background-color: #198038; color: #ffffff; padding: 10px 20px; text-decoration: none; border-radius: 4px; font-weight: bold; display: inline-block;'>View Issue Details in MOH Portal &rarr;</a></p>", issueLink))
	bodyBuilder.WriteString(fmt.Sprintf("<p style='font-size: 13px; color: #6f6f6f; margin-top: 12px;'>Or visit directly: <a href='%s' style='color: #198038;'>%s</a></p>", issueLink, issueLink))
	bodyBuilder.WriteString("<hr style='border: none; border-top: 1px solid #e0e0e0; margin-top: 24px;'/>")
	bodyBuilder.WriteString("<p style='font-size: 12px; color: #777;'>This is an automated group notification from the MOH Issue Tracker.</p>")

	textBuilder.WriteString(fmt.Sprintf("Hello,\n\nIssue %s has been resolved by: %s:\n\n", issueCode, approverIdentifier))
	textBuilder.WriteString(fmt.Sprintf("• Issue Code: %s\n  Status: %s\n  Description: %s\n  Dataset: %s | Organization Unit: %s\n\n", issueCode, statusStr, desc, datasetStr, orgUnitStr))

	if resolutionAction != "" {
		textBuilder.WriteString(fmt.Sprintf("Resolution Notes: %s\n\n", resolutionAction))
	}

	textBuilder.WriteString(fmt.Sprintf("View Issue Details in MOH Integrated Portal:\n%s\n", issueLink))

	msg := model.Message{
		To:       recipientAddresses,
		Subject:  subject,
		HTMLBody: bodyBuilder.String(),
		TextBody: textBuilder.String(),
	}

	// Send notification using dedicated GM/Gmail SMTP config function
	if err := email.SendEmailViaGM(ctx, msg, s.cfg); err != nil {
		if !isNilInterface(s.emailService) {
			_ = s.emailService.Send(ctx, msg)
		}
	}
}

func (s *service) AssignIssues(ctx context.Context, input assignIssuesInput) ([]issueStageResponse, error) {
	results, err := s.repository.AssignIssues(ctx, input)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.AssignedTo) != "" {
		go s.sendAssignmentEmail(context.Background(), input)
	}

	return results, nil
}

func (s *service) sendAssignmentEmail(ctx context.Context, input assignIssuesInput) {
	defer func() {
		if r := recover(); r != nil {
			// Prevent panic from crashing background goroutine
		}
	}()

	assignedToEmail := strings.TrimSpace(input.AssignedTo)
	if assignedToEmail == "" {
		return
	}

	issueCodes := input.IssueCodes
	if len(issueCodes) == 0 {
		return
	}

	// Fetch detailed issue records including description
	fetchedIssues, _ := s.repository.GetIssuesByCodes(ctx, issueCodes)
	issueMap := make(map[string]issueResponse, len(fetchedIssues))
	for _, issue := range fetchedIssues {
		if issue.IssueCode != nil {
			issueMap[*issue.IssueCode] = issue
		}
	}

	// Determine portal link
	portalURL := "https://dashboards.health.go.ug/portal"
	if s.cfg != nil && strings.TrimSpace(s.cfg.FrontendBaseURL) != "" {
		portalURL = strings.TrimRight(strings.TrimSpace(s.cfg.FrontendBaseURL), "/")
	}
	if !strings.HasSuffix(portalURL, "/portal") {
		portalURL = portalURL + "/portal"
	}

	subject := fmt.Sprintf("[MOH Issue Tracker] %d Issue(s) Assigned to You", len(issueCodes))
	if len(issueCodes) == 1 {
		subject = fmt.Sprintf("[MOH Issue Tracker] Issue %s Assigned to You", issueCodes[0])
	}

	var bodyBuilder strings.Builder
	var textBuilder strings.Builder

	bodyBuilder.WriteString("<p>Hello,</p><p>The following issue(s) have been assigned to you:</p>")
	bodyBuilder.WriteString("<div style='margin: 16px 0;'>")

	textBuilder.WriteString("Hello,\n\nThe following issue(s) have been assigned to you:\n\n")

	for _, code := range issueCodes {
		issueDetail := issueMap[code]
		desc := "No description provided"
		if issueDetail.Issue != nil && strings.TrimSpace(*issueDetail.Issue) != "" {
			desc = strings.TrimSpace(*issueDetail.Issue)
		}

		datasetStr := ""
		if issueDetail.Dataset != nil {
			datasetStr = *issueDetail.Dataset
		}

		orgUnitStr := ""
		if issueDetail.OrgUnit != nil {
			orgUnitStr = *issueDetail.OrgUnit
		}

		dataElementStr := ""
		if issueDetail.DataElement != nil {
			dataElementStr = *issueDetail.DataElement
		}

		// HTML Card for each issue
		bodyBuilder.WriteString("<div style='border: 1px solid #e0e0e0; border-left: 4px solid #0f62fe; padding: 12px 16px; margin-bottom: 12px; background-color: #f8f9fa; border-radius: 4px;'>")
		bodyBuilder.WriteString(fmt.Sprintf("<div style='font-size: 15px; font-weight: bold; color: #0f62fe;'>Issue Code: %s</div>", code))
		bodyBuilder.WriteString(fmt.Sprintf("<div style='margin-top: 6px; font-size: 14px; color: #161616;'><strong>Description:</strong> %s</div>", desc))

		metaParts := make([]string, 0, 3)
		if datasetStr != "" {
			metaParts = append(metaParts, fmt.Sprintf("<strong>Dataset:</strong> %s", datasetStr))
		}
		if orgUnitStr != "" {
			metaParts = append(metaParts, fmt.Sprintf("<strong>Organization Unit:</strong> %s", orgUnitStr))
		}
		if dataElementStr != "" {
			metaParts = append(metaParts, fmt.Sprintf("<strong>Data Element:</strong> %s", dataElementStr))
		}
		if len(metaParts) > 0 {
			bodyBuilder.WriteString(fmt.Sprintf("<div style='margin-top: 6px; font-size: 12px; color: #525252;'>%s</div>", strings.Join(metaParts, " &nbsp;|&nbsp; ")))
		}
		bodyBuilder.WriteString("</div>")

		// Text format for each issue
		textBuilder.WriteString(fmt.Sprintf("• Issue Code: %s\n  Description: %s\n", code, desc))
		if datasetStr != "" || orgUnitStr != "" {
			textBuilder.WriteString(fmt.Sprintf("  Dataset: %s | Data Element: %s | Organization Unit: %s\n", datasetStr, dataElementStr, orgUnitStr))
		}
		textBuilder.WriteString("\n")
	}
	bodyBuilder.WriteString("</div>")

	if strings.TrimSpace(input.Comment) != "" {
		bodyBuilder.WriteString(fmt.Sprintf("<div style='background-color: #edf5ff; border: 1px solid #c6e2ff; padding: 10px 14px; border-radius: 4px; margin-bottom: 16px;'><strong>Comment:</strong> %s</div>", strings.TrimSpace(input.Comment)))
		textBuilder.WriteString(fmt.Sprintf("Comment: %s\n\n", strings.TrimSpace(input.Comment)))
	}

	firstCode := issueCodes[0]
	issueLink := fmt.Sprintf("%s/apps/dwh/issue-tracker/issues?issueCode=%s", portalURL, firstCode)

	bodyBuilder.WriteString(fmt.Sprintf("<p style='margin-top: 20px;'><a href='%s' style='background-color: #0f62fe; color: #ffffff; padding: 10px 20px; text-decoration: none; border-radius: 4px; font-weight: bold; display: inline-block;'>View Assigned Issue(s) in MOH Portal &rarr;</a></p>", issueLink))
	bodyBuilder.WriteString(fmt.Sprintf("<p style='font-size: 13px; color: #6f6f6f; margin-top: 12px;'>Or visit directly: <a href='%s' style='color: #0f62fe;'>%s</a></p>", issueLink, issueLink))
	bodyBuilder.WriteString("<hr style='border: none; border-top: 1px solid #e0e0e0; margin-top: 24px;'/>")
	bodyBuilder.WriteString("<p style='font-size: 12px; color: #777;'>This is an automated notification from the MOH Integrated Health Portal Issue Tracker.</p>")

	textBuilder.WriteString(fmt.Sprintf("Please log in to the MOH Dashboard Portal to review and take action:\n%s\n", issueLink))

	msg := model.Message{
		To: []model.Address{
			{Email: assignedToEmail},
		},
		Subject:  subject,
		HTMLBody: bodyBuilder.String(),
		TextBody: textBuilder.String(),
	}

	// Dedicated email function using SMTP_HOST_GM configurations (Gmail SMTP)
	if err := email.SendEmailViaGM(ctx, msg, s.cfg); err != nil {
		if !isNilInterface(s.emailService) {
			_ = s.emailService.Send(ctx, msg)
		}
	}
}

func (s *service) ListIssueResolutionTransactions(ctx context.Context, issueCode string, limit int, offset int) ([]issueStageResponse, error) {
	transactions, err := s.repository.ListIssueResolutionTransactions(ctx, issueCode, limit, offset)
	if err != nil {
		return nil, err
	}

	userCache := make(map[string]string)

	for i := range transactions {
		if transactions[i].ResolvedBy != nil {
			val := strings.TrimSpace(*transactions[i].ResolvedBy)
			if val != "" {
				if _, parseErr := uuid.Parse(val); parseErr == nil {
					if cached, found := userCache[val]; found {
						transactions[i].ResolvedBy = &cached
					} else if !isNilInterface(s.keyAdminClient) {
						if kcUser, err := s.keyAdminClient.GetUser(val); err == nil && kcUser != nil {
							displayName := strings.TrimSpace(kcUser.Email)
							if displayName == "" {
								displayName = strings.TrimSpace(kcUser.Username)
							}
							if displayName != "" {
								userCache[val] = displayName
								transactions[i].ResolvedBy = &displayName
							}
						}
					}
				}
			}
		}
	}

	return transactions, nil
}

func (s *service) CountIssues(ctx context.Context, filter listIssuesFilter) (int64, error) {
	return s.repository.CountIssues(ctx, filter)
}
