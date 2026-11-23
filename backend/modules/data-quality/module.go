package dataquality

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"log/slog"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// ID is the module's registry identifier.
const ID = "data-quality"

// Permissions checked by the module's endpoints.
const (
	DataQualityReadPermission  = "data_quality:read"
	DataQualityWritePermission = "data_quality:write"

	IssueTrackerReadPermission    = "issue_tracker:read"
	IssueTrackerWritePermission   = "issue_tracker:write"
	IssueTrackerManagePermission  = "issue_tracker:manage"
	IssueTrackerAssignPermission  = "issue_tracker:assign"
	IssueTrackerClosePermission   = "issue_tracker:close"
	IssueTrackerCommentPermission = "issue_tracker:comment"
)

// DQA route prefixes. The rule engine is served under both; the second is the
// Data Validation application's alias.
var dqaPrefixes = []string{"/data-quality/dqa", "/data-validation/dqa"}

//go:embed openapi.json
var openAPIDocument []byte

// Dependencies are supplied by the host.
type Dependencies struct {
	// Primary is the platform database holding issue ownership
	// (data_quality_issue_contexts) and the health-context hierarchy. Required.
	Primary *sql.DB
	// Warehouse is the DWH connection: the hiv.issue tables and the source
	// tables a DQA scan reads. nil means the warehouse is unavailable; routes
	// that need it answer 503 or fail like before.
	Warehouse *sql.DB
	// DQA persists DQA results as the role that owns the dqa schema. When nil
	// the store uses Warehouse, and the scheduled-run worker does not start.
	DQA *sql.DB
	// Directory is the identity-provider admin client. Optional: without it
	// the group pickers are empty and no resolver lookups or emails happen.
	Directory UserDirectory
	// Mailer sends issue assignment/resolution emails. Optional.
	Mailer Mailer
	// CallerName names the caller in created_by/reported_by fields. Optional;
	// the verified subject is used when it returns "".
	CallerName CallerName
	// FrontendBaseURL is the portal base URL used in email links.
	FrontendBaseURL string
	// ScheduleInterval is how often the scheduled-run worker polls (default 1m).
	ScheduleInterval time.Duration
	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// Module is the mounted HTTP module plus its startup lifecycle.
type Module struct {
	*modulekit.HTTPModule
	repository *postgresRepository
	store      *DQAStore
	worker     *ScheduleWorker
	logger     *slog.Logger
}

// NewModule validates dependencies and builds the module. It performs no I/O;
// schema migrations and the worker start in Initialize.
func NewModule(deps Dependencies) (*Module, error) {
	if deps.Primary == nil {
		return nil, errors.New("data quality: Primary database is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	dqaDB := deps.DQA
	if dqaDB == nil {
		dqaDB = deps.Warehouse
	}
	repository := newPostgresRepository(deps.Warehouse, deps.Primary)
	store := NewDQAStore(dqaDB)
	h := &handlers{
		service:        NewService(repository, deps.Mailer, deps.Directory, deps.FrontendBaseURL),
		keyAdminClient: deps.Directory,
		dwhDB:          deps.Warehouse,
		dqaStore:       store,
		caller:         deps.CallerName,
	}
	router, err := modulekit.NewRouter(endpoints(h))
	if err != nil {
		return nil, err
	}
	httpModule, err := modulekit.NewHTTPModule(router, openAPIDocument)
	if err != nil {
		return nil, err
	}
	module := &Module{HTTPModule: httpModule, repository: repository, store: store, logger: logger}
	if deps.DQA != nil && deps.Warehouse != nil {
		module.worker = NewScheduleWorker(store, deps.Warehouse, deps.ScheduleInterval, logger)
	}
	return module, nil
}

// Initialize applies the best-effort warehouse schema migrations (the
// hiv.issue assignment columns and the dqa schema), then starts the
// scheduled-run worker when both the DQA and warehouse connections exist. The
// returned stop cancels the worker and waits for it.
func (m *Module) Initialize(ctx context.Context) (core.Stop, error) {
	if m == nil {
		return nil, errors.New("data quality module is nil")
	}
	m.repository.ensureSchema(ctx)
	m.store.migrate(ctx, m.logger)
	if m.worker == nil {
		return nil, nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := m.worker.Start(workerCtx); err != nil && !errors.Is(err, context.Canceled) {
			m.logger.Error("Background worker stopped", "worker", "DQA scheduled run", "error", err)
		}
	}()
	return func(shutdown context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-shutdown.Done():
			return shutdown.Err()
		}
	}, nil
}

func endpoints(h *handlers) []modulekit.Endpoint {
	var out []modulekit.Endpoint
	for _, prefix := range dqaPrefixes {
		out = append(out, dqaEndpoints(prefix, h)...)
	}
	issueAssign := []string{IssueTrackerAssignPermission, IssueTrackerManagePermission, IssueTrackerWritePermission}
	return append(out,
		modulekit.Endpoint{Method: "POST", Path: "/issues", Permission: IssueTrackerWritePermission, Handle: h.CreateIssue},
		modulekit.Endpoint{Method: "GET", Path: "/issues", Permission: IssueTrackerReadPermission, Handle: h.ListIssues},
		modulekit.Endpoint{Method: "GET", Path: "/issues/summary-by-program", Permission: IssueTrackerReadPermission, Handle: h.ListIssueSummaryByProgram},
		modulekit.Endpoint{Method: "PUT", Path: "/issues/{issueCode}", Permission: IssueTrackerWritePermission, Handle: h.UpdateIssue},
		modulekit.Endpoint{Method: "POST", Path: "/issues/{issueCode}/resolveIssue",
			AnyPermissions: []string{IssueTrackerCommentPermission, IssueTrackerManagePermission, IssueTrackerClosePermission}, Handle: h.ResolveIssue},
		modulekit.Endpoint{Method: "GET", Path: "/issues/{issueCode}/transactions", Permission: IssueTrackerReadPermission, Handle: h.ListIssueResolutionTransactions},
		modulekit.Endpoint{Method: "GET", Path: "/issues/keycloak-groups", Permission: IssueTrackerReadPermission, Handle: h.ListKeycloakGroups},
		modulekit.Endpoint{Method: "GET", Path: "/issues/keycloak-groups/{groupId}/members", Permission: IssueTrackerReadPermission, Handle: h.ListKeycloakGroupMembers},
		modulekit.Endpoint{Method: "GET", Path: "/issues/{issueCode}", Permission: IssueTrackerReadPermission, Handle: h.GetIssueByCode},
		modulekit.Endpoint{Method: "POST", Path: "/issues/assign", AnyPermissions: issueAssign, Handle: h.AssignIssues},
		modulekit.Endpoint{Method: "POST", Path: "/issues/{issueCode}/assign", AnyPermissions: issueAssign, Handle: h.AssignIssues},
	)
}

// dqaEndpoints wires the declarative rule engine (see the dqa subpackage):
// table registration, rule CRUD + compile preview, rule mapping, and
// triggering/scheduling/browsing scans and completeness runs.
func dqaEndpoints(prefix string, h *handlers) []modulekit.Endpoint {
	read := func(method, path string, handle func(*modulekit.Context)) modulekit.Endpoint {
		return modulekit.Endpoint{Method: method, Path: prefix + path, Permission: DataQualityReadPermission, Handle: handle}
	}
	write := func(method, path string, handle func(*modulekit.Context)) modulekit.Endpoint {
		return modulekit.Endpoint{Method: method, Path: prefix + path, Permission: DataQualityWritePermission, Handle: handle}
	}
	return []modulekit.Endpoint{
		read("GET", "/tables", h.ListDQATables),
		write("POST", "/tables", h.UpsertDQATable),
		write("DELETE", "/tables/{tableId}", h.DeleteDQATable),

		read("GET", "/columns", h.ListDQAPhysicalColumns),

		read("GET", "/rules", h.ListDQARules),
		write("POST", "/rules", h.UpsertDQARule),
		write("POST", "/rules/compile", h.CompileDQARule),
		read("POST", "/rules/mapping/preview", h.PreviewRuleMapping),
		write("POST", "/rules/mapping/apply", h.ApplyRuleMapping),
		write("DELETE", "/rules/{tableId}/{code}", h.DeleteDQARule),

		write("POST", "/run", h.RunDQATable),

		write("POST", "/schedules", h.ScheduleDQARun),
		read("GET", "/schedules", h.ListDQAScheduledRuns),
		write("DELETE", "/schedules/{scheduleId}", h.CancelDQAScheduledRun),
		read("GET", "/runs", h.ListDQARuns),
		read("GET", "/runs/{runId}/flags", h.ListDQAFlags),
		read("GET", "/runs/{runId}/flag-filters", h.ListDQAFlagFilters),

		write("POST", "/completeness/run", h.RunCompleteness),
		read("GET", "/completeness/runs", h.ListCompletenessRuns),
		read("GET", "/completeness/runs/{runId}", h.GetCompletenessRun),
		read("GET", "/completeness/runs/{runId}/districts", h.ListCompletenessDistricts),
		read("GET", "/completeness/runs/{runId}/vhts", h.ListCompletenessVHTs),
	}
}
