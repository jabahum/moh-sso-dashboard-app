package data_quality

type issueResponse struct {
	IssueID      int64   `json:"issue_id"`
	IssueCode    *string `json:"issue_code,omitempty"`
	Dataset      *string `json:"dataset,omitempty"`
	DataElement  *string `json:"data_element,omitempty"`
	Region       *string `json:"region,omitempty"`
	District     *string `json:"district,omitempty"`
	OrgUnit      *string `json:"org_unit,omitempty"`
	Issue        *string `json:"issue,omitempty"`
	IssueType    *string `json:"issue_type,omitempty"`
	DateReported string  `json:"date_reported"`
	ReportedBy   *string `json:"reported_by,omitempty"`
	Status       *string `json:"status,omitempty"`
	Priority     *string `json:"priority,omitempty"`
	Severity     *string `json:"severity,omitempty"`
	UpdatedDate  *string `json:"updated_date,omitempty"`
	UpdatedBy    *string `json:"updated_by,omitempty"`
	TimePeriod   *string `json:"time_Period,omitempty"`
	AssignedTo   *string `json:"assigned_to,omitempty"`
}

type createIssueRequest struct {
	Dataset     *string `json:"dataset"`
	DataElement *string `json:"data_element"`
	OrgUnit     *string `json:"org_unit"`
	Issue       string  `json:"issue" binding:"required"`
	IssueType   *string `json:"issue_type"`
	ReportedBy  *string `json:"reported_by"`
	TimePeriod  *string `json:"time_Period"`
}

type updateIssueRequest struct {
	Dataset      *string `json:"dataset"`
	DataElement  *string `json:"data_element"`
	OrgUnit      *string `json:"org_unit"`
	Issue        *string `json:"issue"`
	IssueType    *string `json:"issue_type"`
	DateReported *string `json:"date_reported"`
	ReportedBy   *string `json:"reported_by"`
	Status       *string `json:"status"`
	Priority     *string `json:"priority"`
	Severity     *string `json:"severity"`
	UpdatedBy    *string `json:"updated_by"`
	TimePeriod   *string `json:"time_Period"`
	AssignedTo   *string `json:"assigned_to"`
}

type createIssueResolutionRequest struct {
	Stage              *string `json:"stage"`
	Status             *string `json:"status"`
	ResolutionAction   *string `json:"resolution_action"`
	ResolvedBy         *string `json:"resolved_by"`
	ResolutionDate     *string `json:"resolution_date"`
	VerificationStatus *string `json:"verification_status"`
	VerifiedBy         *string `json:"verified_by"`
	VerificationDate   *string `json:"verification_date"`
	PreventiveAction   *string `json:"preventive_action"`
	ProcessChange      *string `json:"process_change"`
	PreventiveOwner    *string `json:"preventive_owner"`
	DueDate            *string `json:"due_date"`
	AssignedTo         *string `json:"assigned_to"`
}

type issueStageResponse struct {
	ID                 int64   `json:"id"`
	IssueCode          string  `json:"issue_code"`
	Status             *string `json:"status,omitempty"`
	Stage              *string `json:"stage,omitempty"`
	IsCurrent          bool    `json:"is_current"`
	ResolutionAction   *string `json:"resolution_action,omitempty"`
	ResolvedBy         *string `json:"resolved_by,omitempty"`
	ResolutionDate     *string `json:"resolution_date,omitempty"`
	VerificationStatus *string `json:"verification_status,omitempty"`
	VerifiedBy         *string `json:"verified_by,omitempty"`
	VerificationDate   *string `json:"verification_date,omitempty"`
	PreventiveAction   *string `json:"preventive_action,omitempty"`
	ProcessChange      *string `json:"process_change,omitempty"`
	PreventiveOwner    *string `json:"preventive_owner,omitempty"`
	DueDate            *string `json:"due_date,omitempty"`
	AssignedTo         *string `json:"assigned_to,omitempty"`
}

type assignIssuesRequest struct {
	IssueCodes []string `json:"issue_codes"`
	IssueCode  *string  `json:"issue_code"`
	AssignedTo string   `json:"assigned_to"`
	Comment    *string  `json:"comment"`
}

type assignIssuesInput struct {
	IssueCodes []string
	AssignedTo string
	AssignedBy string
	Comment    string
}

type keycloakGroupResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type keycloakGroupMemberResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type issueProgramSummaryResponse struct {
	Program       string `json:"program"`
	IssueCount    int64  `json:"issue_count"`
	OpenCount     int64  `json:"open_count"`
	ResolvedCount int64  `json:"resolved_count"`
}

type listIssuesFilter struct {
	Limit       int
	Offset      int
	Program     string
	Dataset     string
	DataElement string
	OrgUnit     string
	Region      string
	District    string
	TimePeriod  string
	AssignedTo  string
	Search      string
}

type createIssueInput struct {
	Dataset     interface{}
	DataElement interface{}
	OrgUnit     interface{}
	Issue       string
	IssueType   interface{}
	ReportedBy  interface{}
	TimePeriod  interface{}
}

type updateIssueInput struct {
	IssueCode    string
	Dataset      interface{}
	DataElement  interface{}
	OrgUnit      interface{}
	Issue        interface{}
	DateReported interface{}
	ReportedBy   interface{}
	Status       interface{}
	Priority     interface{}
	Severity     interface{}
	UpdatedBy    interface{}
	IssueType    interface{}
	TimePeriod   interface{}
}

type resolveIssueInput struct {
	IssueCode          string
	Status             string
	ResolutionAction   interface{}
	ResolvedBy         interface{}
	ResolutionDate     interface{}
	VerificationStatus interface{}
	VerifiedBy         interface{}
	VerificationDate   interface{}
	PreventiveAction   interface{}
	ProcessChange      interface{}
	PreventiveOwner    interface{}
	DueDate            interface{}
}
