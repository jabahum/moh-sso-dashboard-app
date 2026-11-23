package dataquality

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type handlers struct {
	service        Service
	keyAdminClient UserDirectory
	dwhDB          *sql.DB
	dqaStore       *DQAStore
	caller         CallerName
}

func parseListLimit(c *modulekit.Context, defaultValue int, maxValue int) int {
	limit := defaultValue
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if limit > maxValue {
		return maxValue
	}

	return limit
}

func parseListOffset(c *modulekit.Context) int {
	offset := 0
	if raw := c.Query("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return offset
}

func issueDBErrorMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	return fallback
}

// callerName returns the caller's email, else username (from the host), else
// the verified subject.
func (h *handlers) callerName(c *modulekit.Context) string {
	if h.caller != nil {
		if name := strings.TrimSpace(h.caller(c.Request)); name != "" {
			return name
		}
	}
	return strings.TrimSpace(c.Subject())
}

func (h *handlers) CreateIssue(c *modulekit.Context) {
	var req createIssueRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
		return
	}

	issueText := strings.TrimSpace(req.Issue)
	if issueText == "" {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "issue is required")
		return
	}

	if req.IssueType != nil && strings.TrimSpace(*req.IssueType) == "" {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "issue_type cannot be empty")
		return
	}

	reportedBy := dqNullableString(req.ReportedBy)
	if !reportedBy.Valid {
		userEmail := h.callerName(c)
		if userEmail != "" {
			reportedBy = sql.NullString{
				String: userEmail,
				Valid:  true,
			}
		}
	}

	timePeriod := req.TimePeriod
	if timePeriod == nil || strings.TrimSpace(*timePeriod) == "" {
		timePeriod = req.TimePeriodAlt
	}

	issue, err := h.service.CreateIssue(c.Context(), createIssueInput{
		Program:     dqNullableString(req.Program),
		Dataset:     dqNullableString(req.Dataset),
		DataElement: dqNullableString(req.DataElement),
		OrgUnit:     dqNullableString(req.OrgUnit),
		Issue:       issueText,
		IssueType:   dqNullableString(req.IssueType),
		ReportedBy:  reportedBy,
		Priority:    dqNullableString(req.Priority),
		Severity:    dqNullableString(req.Severity),
		TimePeriod:  dqNullableString(timePeriod),
	}, healthContextScopeFromRequest(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "CREATE_ISSUE_FAILED", issueDBErrorMessage(err, "failed to create issue"))
		return
	}

	c.OK(http.StatusCreated, toIssueResponse(issue))
}

func (h *handlers) ListIssues(c *modulekit.Context) {
	limit := parseListLimit(c, 20, 10000)
	offset := parseListOffset(c)

	filter := listIssuesFilter{
		Scope:       healthContextScopeFromRequest(c),
		Limit:       limit,
		Offset:      offset,
		Program:     strings.TrimSpace(c.Query("program")),
		Dataset:     strings.TrimSpace(c.Query("dataset")),
		DataElement: strings.TrimSpace(c.Query("data_element")),
		TimePeriod:  strings.TrimSpace(c.Query("time_period")),
		OrgUnit:     strings.TrimSpace(c.Query("org_unit")),
		Region:      strings.TrimSpace(c.Query("region")),
		District:    strings.TrimSpace(c.Query("district")),
		AssignedTo:  strings.TrimSpace(c.Query("assigned_to")),
		Search:      strings.TrimSpace(c.Query("search")),
	}
	if filter.TimePeriod == "" {
		filter.TimePeriod = strings.TrimSpace(c.Query("period"))
	}
	if filter.OrgUnit == "" {
		filter.OrgUnit = strings.TrimSpace(c.Query("orgUnit"))
	}
	if filter.DataElement == "" {
		filter.DataElement = strings.TrimSpace(c.Query("dataElement"))
	}
	if filter.AssignedTo == "" {
		filter.AssignedTo = strings.TrimSpace(c.Query("assignedTo"))
	}

	issues, err := h.service.ListIssues(c.Context(), filter)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "LIST_ISSUES_FAILED", issueDBErrorMessage(err, "failed to list issues"))
		return
	}

	total, err := h.service.CountIssues(c.Context(), filter)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "LIST_ISSUES_FAILED", issueDBErrorMessage(err, "failed to count issues"))
		return
	}

	c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	c.Header("Access-Control-Expose-Headers", "X-Total-Count")

	c.OK(http.StatusOK, toIssueResponses(issues))
}

func (h *handlers) GetIssueByCode(c *modulekit.Context) {
	issueCode := strings.TrimSpace(c.Param("issueCode"))
	if issueCode == "" {
		c.Fail(http.StatusBadRequest, "INVALID_ISSUE_CODE", "issue code is required")
		return
	}

	issue, err := h.service.GetIssueByCode(c.Context(), issueCode, healthContextScopeFromRequest(c))
	if err != nil {
		if isNotFound(err) {
			c.Fail(http.StatusNotFound, "ISSUE_NOT_FOUND", "issue not found")
			return
		}
		c.Fail(http.StatusInternalServerError, "GET_ISSUE_FAILED", issueDBErrorMessage(err, "failed to get issue"))
		return
	}

	c.OK(http.StatusOK, toIssueResponse(issue))
}

func (h *handlers) ListIssueSummaryByProgram(c *modulekit.Context) {
	limit := parseListLimit(c, 50, 500)
	offset := parseListOffset(c)

	summary, err := h.service.ListIssueSummaryByProgram(c.Context(), limit, offset, healthContextScopeFromRequest(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "LIST_ISSUE_SUMMARY_FAILED", issueDBErrorMessage(err, "failed to list issue summary"))
		return
	}

	c.OK(http.StatusOK, toIssueProgramSummaryResponses(summary))
}

func (h *handlers) UpdateIssue(c *modulekit.Context) {
	issueCode := strings.TrimSpace(c.Param("issueCode"))
	if issueCode == "" {
		c.Fail(http.StatusBadRequest, "INVALID_ISSUE_CODE", "issue code is required")
		return
	}

	var req updateIssueRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
		return
	}

	timePeriod := req.TimePeriod
	if timePeriod == nil {
		timePeriod = req.TimePeriodAlt
	}

	if req.Program == nil &&
		req.Dataset == nil &&
		req.DataElement == nil &&
		req.OrgUnit == nil &&
		req.Issue == nil &&
		req.IssueType == nil &&
		req.DateReported == nil &&
		req.ReportedBy == nil &&
		req.Status == nil &&
		req.Priority == nil &&
		req.Severity == nil &&
		req.UpdatedBy == nil &&
		timePeriod == nil {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "at least one field is required")
		return
	}

	var issueParam interface{}
	if req.Issue != nil {
		trimmedIssue := strings.TrimSpace(*req.Issue)
		if trimmedIssue == "" {
			c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "issue cannot be empty")
			return
		}
		issueParam = trimmedIssue
	}

	var dateReportedParam interface{}
	if req.DateReported != nil {
		parsedDate, err := parseDateOnly(*req.DateReported)
		if err != nil {
			c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "date_reported must be in YYYY-MM-DD format")
			return
		}
		dateReportedParam = parsedDate
	}

	var statusParam interface{}
	if req.Status != nil {
		trimmedStatus := strings.TrimSpace(*req.Status)
		if trimmedStatus == "" {
			c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "status cannot be empty")
			return
		}
		statusParam = trimmedStatus
	}

	var priorityParam interface{}
	if req.Priority != nil {
		trimmedPriority := strings.TrimSpace(*req.Priority)
		if trimmedPriority == "" {
			c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "priority cannot be empty")
			return
		}
		priorityParam = trimmedPriority
	}

	var severityParam interface{}
	if req.Severity != nil {
		trimmedSeverity := strings.TrimSpace(*req.Severity)
		if trimmedSeverity == "" {
			c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "severity cannot be empty")
			return
		}
		severityParam = trimmedSeverity
	}

	var issueTypeParam interface{}
	if req.IssueType != nil {
		trimmedIssueType := strings.TrimSpace(*req.IssueType)
		if trimmedIssueType == "" {
			c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "issue_type cannot be empty")
			return
		}
		issueTypeParam = trimmedIssueType
	}

	updatedByParam := optionalTrimmedParam(req.UpdatedBy, true)
	if updatedByParam == nil {
		userEmail := h.callerName(c)
		if userEmail != "" {
			updatedByParam = userEmail
		}
	}

	issue, err := h.service.UpdateIssue(c.Context(), updateIssueInput{
		IssueCode:    issueCode,
		Program:      optionalTrimmedParam(req.Program, false),
		Dataset:      optionalTrimmedParam(req.Dataset, false),
		DataElement:  optionalTrimmedParam(req.DataElement, false),
		OrgUnit:      optionalTrimmedParam(req.OrgUnit, false),
		Issue:        issueParam,
		DateReported: dateReportedParam,
		ReportedBy:   optionalTrimmedParam(req.ReportedBy, false),
		Status:       statusParam,
		Priority:     priorityParam,
		Severity:     severityParam,
		UpdatedBy:    updatedByParam,
		IssueType:    issueTypeParam,
		TimePeriod:   optionalTrimmedParam(timePeriod, false),
	}, healthContextScopeFromRequest(c))
	if err != nil {
		if isNotFound(err) {
			c.Fail(http.StatusNotFound, "ISSUE_NOT_FOUND", "issue not found")
			return
		}
		c.Fail(http.StatusInternalServerError, "UPDATE_ISSUE_FAILED", issueDBErrorMessage(err, "failed to update issue"))
		return
	}

	c.OK(http.StatusOK, toIssueResponse(issue))
}

func (h *handlers) ResolveIssue(c *modulekit.Context) {
	issueCode := strings.TrimSpace(c.Param("issueCode"))
	if issueCode == "" {
		c.Fail(http.StatusBadRequest, "INVALID_ISSUE_CODE", "issue code is required")
		return
	}

	var req createIssueResolutionRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
		return
	}

	var status string
	switch {
	case req.Status != nil && strings.TrimSpace(*req.Status) != "":
		status = strings.TrimSpace(*req.Status)
	case req.Stage != nil && strings.TrimSpace(*req.Stage) != "":
		// Backward compatibility for existing clients using "stage".
		status = strings.TrimSpace(*req.Stage)
	default:
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "status is required")
		return
	}

	var resolutionDateParam interface{}
	if req.ResolutionDate != nil {
		parsedDate, err := parseDateOnly(*req.ResolutionDate)
		if err != nil {
			c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "resolution_date must be in YYYY-MM-DD format")
			return
		}
		resolutionDateParam = parsedDate
	}

	var verificationDateParam interface{}
	if req.VerificationDate != nil {
		parsedDate, err := parseDateOnly(*req.VerificationDate)
		if err != nil {
			c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "verification_date must be in YYYY-MM-DD format")
			return
		}
		verificationDateParam = parsedDate
	}

	var dueDateParam interface{}
	if req.DueDate != nil {
		parsedDate, err := parseDateOnly(*req.DueDate)
		if err != nil {
			c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "due_date must be in YYYY-MM-DD format")
			return
		}
		dueDateParam = parsedDate
	}

	resolvedBy := optionalTrimmedParam(req.ResolvedBy, true)
	if resolvedBy == nil {
		resolvedBy = h.callerName(c)
	}

	stageRow, err := h.service.ResolveIssue(c.Context(), resolveIssueInput{
		IssueCode:          issueCode,
		Status:             status,
		ResolutionAction:   optionalTrimmedParam(req.ResolutionAction, true),
		ResolvedBy:         resolvedBy,
		ResolutionDate:     resolutionDateParam,
		VerificationStatus: optionalTrimmedParam(req.VerificationStatus, true),
		VerifiedBy:         optionalTrimmedParam(req.VerifiedBy, true),
		VerificationDate:   verificationDateParam,
		PreventiveAction:   optionalTrimmedParam(req.PreventiveAction, true),
		ProcessChange:      optionalTrimmedParam(req.ProcessChange, true),
		PreventiveOwner:    optionalTrimmedParam(req.PreventiveOwner, true),
		DueDate:            dueDateParam,
	}, healthContextScopeFromRequest(c))
	if err != nil {
		if isNotFound(err) {
			c.Fail(http.StatusNotFound, "ISSUE_NOT_FOUND", "issue not found")
			return
		}
		c.Fail(http.StatusInternalServerError, "CREATE_ISSUE_STAGE_FAILED", issueDBErrorMessage(err, "failed to create issue resolution"))
		return
	}

	c.OK(http.StatusCreated, toIssueStageResponse(stageRow))
}

func (h *handlers) ListIssueResolutionTransactions(c *modulekit.Context) {
	issueCode := strings.TrimSpace(c.Param("issueCode"))
	if issueCode == "" {
		c.Fail(http.StatusBadRequest, "INVALID_ISSUE_CODE", "issue code is required")
		return
	}

	limit := parseListLimit(c, 20, 100)
	offset := parseListOffset(c)

	transactions, err := h.service.ListIssueResolutionTransactions(c.Context(), issueCode, limit, offset, healthContextScopeFromRequest(c))
	if err != nil {
		if isNotFound(err) {
			c.Fail(http.StatusNotFound, "ISSUE_NOT_FOUND", "issue not found")
			return
		}
		c.Fail(http.StatusInternalServerError, "LIST_ISSUE_TRANSACTIONS_FAILED", issueDBErrorMessage(err, "failed to list issue transactions"))
		return
	}

	c.OK(http.StatusOK, toIssueStageResponses(transactions))
}

func (h *handlers) ListKeycloakGroups(c *modulekit.Context) {
	if isNilInterface(h.keyAdminClient) {
		c.OK(http.StatusOK, []keycloakGroupResponse{})
		return
	}

	groups, err := h.keyAdminClient.ListGroups(c.Context())
	if err != nil {
		c.Fail(http.StatusInternalServerError, "KEYCLOAK_GROUPS_FAILED", "failed to fetch Keycloak groups")
		return
	}

	out := make([]keycloakGroupResponse, 0, len(groups))
	for _, g := range groups {
		out = append(out, keycloakGroupResponse{
			ID:   g.ID,
			Name: g.Name,
			Path: g.Path,
		})
	}

	c.OK(http.StatusOK, out)
}

func (h *handlers) ListKeycloakGroupMembers(c *modulekit.Context) {
	groupID := strings.TrimSpace(c.Param("groupId"))
	if groupID == "" {
		c.Fail(http.StatusBadRequest, "INVALID_GROUP_ID", "group ID is required")
		return
	}

	if isNilInterface(h.keyAdminClient) {
		c.OK(http.StatusOK, []keycloakGroupMemberResponse{})
		return
	}

	members, err := h.keyAdminClient.ListGroupMembers(c.Context(), groupID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "KEYCLOAK_MEMBERS_FAILED", "failed to fetch Keycloak group members")
		return
	}

	out := make([]keycloakGroupMemberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, keycloakGroupMemberResponse{
			ID:        m.ID,
			Username:  m.Username,
			Email:     m.Email,
			FirstName: m.FirstName,
			LastName:  m.LastName,
		})
	}

	c.OK(http.StatusOK, out)
}

func (h *handlers) AssignIssues(c *modulekit.Context) {
	var req assignIssuesRequest
	if err := bindJSON(c, &req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request payload")
		return
	}

	assignedTo := strings.TrimSpace(req.AssignedTo)
	if assignedTo == "" {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "assigned_to (user email) is required")
		return
	}

	issueCodes := make([]string, 0)
	for _, code := range req.IssueCodes {
		trimmed := strings.TrimSpace(code)
		if trimmed != "" {
			issueCodes = append(issueCodes, trimmed)
		}
	}
	if len(issueCodes) == 0 {
		paramCode := strings.TrimSpace(c.Param("issueCode"))
		if paramCode != "" {
			issueCodes = append(issueCodes, paramCode)
		} else if req.IssueCode != nil && strings.TrimSpace(*req.IssueCode) != "" {
			issueCodes = append(issueCodes, strings.TrimSpace(*req.IssueCode))
		}
	}

	if len(issueCodes) == 0 {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "at least one issue code is required")
		return
	}

	assignedBy := h.callerName(c)
	if assignedBy == "" {
		assignedBy = "System"
	}

	comment := ""
	if req.Comment != nil {
		comment = strings.TrimSpace(*req.Comment)
	}

	results, err := h.service.AssignIssues(c.Context(), assignIssuesInput{
		IssueCodes: issueCodes,
		AssignedTo: assignedTo,
		AssignedBy: assignedBy,
		Comment:    comment,
	}, healthContextScopeFromRequest(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "ASSIGN_ISSUES_FAILED", issueDBErrorMessage(err, "failed to assign issue(s)"))
		return
	}

	c.OK(http.StatusOK, toIssueStageResponses(results))
}
