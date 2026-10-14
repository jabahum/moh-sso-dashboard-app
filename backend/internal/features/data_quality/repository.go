package data_quality

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type Repository interface {
	CreateIssue(ctx context.Context, input createIssueInput) (issueResponse, error)
	ListIssues(ctx context.Context, filter listIssuesFilter) ([]issueResponse, error)
	GetIssuesByCodes(ctx context.Context, issueCodes []string) ([]issueResponse, error)
	ListIssueSummaryByProgram(ctx context.Context, limit int, offset int) ([]issueProgramSummaryResponse, error)
	UpdateIssue(ctx context.Context, input updateIssueInput) (issueResponse, error)
	ResolveIssue(ctx context.Context, input resolveIssueInput) (issueStageResponse, error)
	AssignIssues(ctx context.Context, input assignIssuesInput) ([]issueStageResponse, error)
	ListIssueResolutionTransactions(ctx context.Context, issueCode string, limit int, offset int) ([]issueStageResponse, error)
	CountIssues(ctx context.Context, filter listIssuesFilter) (int64, error)
}

type postgresRepository struct {
	dwhDB     *sql.DB
	primaryDB *sql.DB
}

func NewRepository(dwhDB *sql.DB, primaryDB *sql.DB) Repository {
	repo := &postgresRepository{
		dwhDB:     dwhDB,
		primaryDB: primaryDB,
	}
	repo.ensureSchema(context.Background())
	return repo
}

func (r *postgresRepository) ensureSchema(ctx context.Context) {
	if r.dwhDB == nil {
		return
	}
	_, _ = r.dwhDB.ExecContext(ctx, `
		ALTER TABLE hiv.issue ADD COLUMN IF NOT EXISTS assigned_to TEXT;
		ALTER TABLE hiv.issue_resolution ADD COLUMN IF NOT EXISTS assigned_to TEXT;
	`)
}

func (r *postgresRepository) CreateIssue(ctx context.Context, input createIssueInput) (issueResponse, error) {
	row := r.dwhDB.QueryRowContext(
		ctx,
		`WITH next_issue AS (
			SELECT nextval(pg_get_serial_sequence('hiv.issue', 'issue_id'))::bigint AS issue_id
		)
		INSERT INTO hiv.issue (
			issue_id,
			issue_code,
			dataset,
			data_element,
			org_unit,
			issue,
			issue_type,
			date_reported,
			reported_by,
			status,
			time_period
		)
		SELECT
			n.issue_id,
			'HMIS-' || LPAD(n.issue_id::text, 4, '0'),
			$1, $2, $3, $4, $5, CURRENT_DATE, $6, 'OPEN', $7
		FROM next_issue n
		RETURNING issue_id, issue_code, dataset, data_element, ''::text AS region, ''::text AS district, org_unit, issue, date_reported, reported_by, status, priority, severity, updated_date, updated_by, issue_type, time_period, assigned_to`,
		input.Dataset,
		input.DataElement,
		input.OrgUnit,
		input.Issue,
		input.IssueType,
		input.ReportedBy,
		input.TimePeriod,
	)
	return scanIssue(row)
}

func buildIssuesWhereClause(filter listIssuesFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	if filter.Program != "" {
		if strings.EqualFold(filter.Program, "unspecified") {
			conditions = append(conditions, "LOWER(BTRIM(COALESCE(hiv.issue.program, ''))) = ''")
		} else {
			conditions = append(conditions, fmt.Sprintf("LOWER(BTRIM(COALESCE(hiv.issue.program, ''))) = LOWER(BTRIM($%d))", idx))
			args = append(args, filter.Program)
			idx++
		}
	}

	if filter.Dataset != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(BTRIM(COALESCE(hiv.issue.dataset, ''))) = LOWER(BTRIM($%d))", idx))
		args = append(args, filter.Dataset)
		idx++
	}

	if filter.DataElement != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(BTRIM(COALESCE(hiv.issue.data_element, ''))) = LOWER(BTRIM($%d))", idx))
		args = append(args, filter.DataElement)
		idx++
	}

	if filter.TimePeriod != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(BTRIM(COALESCE(hiv.issue.time_period, ''))) = LOWER(BTRIM($%d)) OR hiv.issue.time_period ILIKE '%%' || $%d || '%%')", idx, idx))
		args = append(args, filter.TimePeriod)
		idx++
	}

	if filter.Region != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(BTRIM(COALESCE(org.region, ''))) = LOWER(BTRIM($%d)) OR LOWER(BTRIM(COALESCE(org.district, ''))) = LOWER(BTRIM($%d)) OR LOWER(BTRIM(COALESCE(hiv.issue.org_unit, ''))) = LOWER(BTRIM($%d)))", idx, idx, idx))
		args = append(args, filter.Region)
		idx++
	}

	if filter.District != "" {
		conditions = append(conditions, fmt.Sprintf("(LOWER(BTRIM(COALESCE(org.district, ''))) = LOWER(BTRIM($%d)) OR LOWER(BTRIM(COALESCE(hiv.issue.org_unit, ''))) = LOWER(BTRIM($%d)))", idx, idx))
		args = append(args, filter.District)
		idx++
	}

	if filter.OrgUnit != "" && filter.OrgUnit != filter.Region {
		conditions = append(conditions, fmt.Sprintf("(LOWER(BTRIM(COALESCE(hiv.issue.org_unit, ''))) = LOWER(BTRIM($%d)) OR LOWER(BTRIM(COALESCE(org.region, ''))) = LOWER(BTRIM($%d)) OR LOWER(BTRIM(COALESCE(org.district, ''))) = LOWER(BTRIM($%d)))", idx, idx, idx))
		args = append(args, filter.OrgUnit)
		idx++
	}

	if filter.AssignedTo != "" {
		conditions = append(conditions, fmt.Sprintf("LOWER(BTRIM(COALESCE(hiv.issue.assigned_to, ''))) = LOWER(BTRIM($%d))", idx))
		args = append(args, filter.AssignedTo)
		idx++
	}

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf(`(
			hiv.issue.issue_code ILIKE '%%' || $%d || '%%'
			OR hiv.issue.dataset ILIKE '%%' || $%d || '%%'
			OR hiv.issue.data_element ILIKE '%%' || $%d || '%%'
			OR hiv.issue.issue ILIKE '%%' || $%d || '%%'
			OR hiv.issue.org_unit ILIKE '%%' || $%d || '%%'
			OR COALESCE(org.region, '') ILIKE '%%' || $%d || '%%'
			OR COALESCE(org.district, '') ILIKE '%%' || $%d || '%%'
			OR hiv.issue.status ILIKE '%%' || $%d || '%%'
		)`, idx, idx, idx, idx, idx, idx, idx, idx))
		args = append(args, filter.Search)
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	return whereClause, args
}

func (r *postgresRepository) ListIssues(ctx context.Context, filter listIssuesFilter) ([]issueResponse, error) {
	whereClause, args := buildIssuesWhereClause(filter)

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2
	query := fmt.Sprintf(`SELECT issue_id, issue_code, dataset, data_element, COALESCE(org.region, ''), COALESCE(org.district, ''), org_unit, issue, date_reported, reported_by, status, priority, severity, updated_date, updated_by, issue_type, time_period, hiv.issue.assigned_to
		FROM hiv.issue
		LEFT JOIN (
			SELECT DISTINCT ON (org_unit_name) org_unit_name, region, district
			FROM hiv.organisation_unit
			ORDER BY org_unit_name, organisation_unit ASC
		) org ON org.org_unit_name = hiv.issue.org_unit
		%s
		ORDER BY date_reported DESC, issue_id DESC
		LIMIT $%d OFFSET $%d`, whereClause, limitIdx, offsetIdx)

	execArgs := append(args, limit, offset)
	rows, err := r.dwhDB.QueryContext(ctx, query, execArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	issues := make([]issueResponse, 0)
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (r *postgresRepository) GetIssuesByCodes(ctx context.Context, issueCodes []string) ([]issueResponse, error) {
	if len(issueCodes) == 0 {
		return nil, nil
	}
	rows, err := r.dwhDB.QueryContext(
		ctx,
		`SELECT issue_id, issue_code, dataset, data_element, COALESCE(org.region, ''), COALESCE(org.district, ''), org_unit, issue, date_reported, reported_by, status, priority, severity, updated_date, updated_by, issue_type, time_period, hiv.issue.assigned_to
		FROM hiv.issue
		LEFT JOIN (
			SELECT DISTINCT ON (org_unit_name) org_unit_name, region, district
			FROM hiv.organisation_unit
			ORDER BY org_unit_name, organisation_unit ASC
		) org ON org.org_unit_name = hiv.issue.org_unit
		WHERE issue_code = ANY($1)`,
		pq.Array(issueCodes),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	issues := make([]issueResponse, 0, len(issueCodes))
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (r *postgresRepository) ListIssueSummaryByProgram(ctx context.Context, limit int, offset int) ([]issueProgramSummaryResponse, error) {
	rows, err := r.dwhDB.QueryContext(
		ctx,
		`SELECT
			COALESCE(NULLIF(BTRIM(program), ''), 'Unspecified') AS program,
			COUNT(*)::bigint AS issue_count,
			COUNT(CASE WHEN UPPER(BTRIM(status)) NOT IN ('RESOLVED', 'CLOSED') THEN 1 END)::bigint AS open_count,
			COUNT(CASE WHEN UPPER(BTRIM(status)) IN ('RESOLVED', 'CLOSED') THEN 1 END)::bigint AS resolved_count
		FROM hiv.issue
		GROUP BY 1
		ORDER BY issue_count DESC, program ASC
		LIMIT $1 OFFSET $2`,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := make([]issueProgramSummaryResponse, 0)
	for rows.Next() {
		var row issueProgramSummaryResponse
		if err := rows.Scan(&row.Program, &row.IssueCount, &row.OpenCount, &row.ResolvedCount); err != nil {
			return nil, err
		}
		summary = append(summary, row)
	}

	return summary, rows.Err()
}

func (r *postgresRepository) UpdateIssue(ctx context.Context, input updateIssueInput) (issueResponse, error) {
	row := r.dwhDB.QueryRowContext(
		ctx,
		`UPDATE hiv.issue
		SET
			dataset = COALESCE($2, dataset),
			data_element = COALESCE($3, data_element),
			org_unit = COALESCE($4, org_unit),
			issue = COALESCE($5, issue),
			date_reported = COALESCE($6, date_reported),
			reported_by = COALESCE($7, reported_by),
			status = COALESCE($8, status),
			priority = COALESCE($9, priority),
			severity = COALESCE($10, severity),
			updated_date = CURRENT_DATE,
			updated_by = COALESCE($11, updated_by),
			issue_type = COALESCE($12, issue_type),
			time_period = COALESCE($13, time_period)
		WHERE issue_code = $1
		RETURNING issue_id, issue_code, dataset, data_element, ''::text AS region, ''::text AS district, org_unit, issue, date_reported, reported_by, status, priority, severity, updated_date, updated_by, issue_type, time_period, assigned_to`,
		input.IssueCode,
		input.Dataset,
		input.DataElement,
		input.OrgUnit,
		input.Issue,
		input.DateReported,
		input.ReportedBy,
		input.Status,
		input.Priority,
		input.Severity,
		input.UpdatedBy,
		input.IssueType,
		input.TimePeriod,
	)
	return scanIssue(row)
}

func (r *postgresRepository) ResolveIssue(ctx context.Context, input resolveIssueInput) (issueStageResponse, error) {
	tx, err := r.dwhDB.BeginTx(ctx, nil)
	if err != nil {
		return issueStageResponse{}, err
	}
	defer tx.Rollback()

	var existingCode string
	err = tx.QueryRowContext(ctx, `SELECT issue_code FROM hiv.issue WHERE issue_code = $1`, input.IssueCode).Scan(&existingCode)
	if err != nil {
		return issueStageResponse{}, err
	}

	if _, err = tx.ExecContext(
		ctx,
		`UPDATE hiv.issue_resolution
		SET is_current = FALSE
		WHERE issue_code = $1 AND is_current = TRUE`,
		input.IssueCode,
	); err != nil {
		return issueStageResponse{}, err
	}

	rowScanner := tx.QueryRowContext(
		ctx,
		`INSERT INTO hiv.issue_resolution (
			issue_code,
			status,
			is_current,
			resolution_action,
			resolved_by,
			resolution_date,
			verification_status,
			verified_by,
			verification_date,
			preventive_action,
			process_change,
			preventive_owner,
			due_date,
			assigned_to
		) VALUES (
			$1, $2, TRUE, $3, $4, COALESCE($5, CURRENT_DATE), $6, $7, $8, $9, $10, $11, $12, NULL
		)
		RETURNING id, issue_code, status, is_current, resolution_action, resolved_by, resolution_date,
			verification_status, verified_by, verification_date, preventive_action, process_change, preventive_owner, due_date, assigned_to`,
		input.IssueCode,
		input.Status,
		input.ResolutionAction,
		input.ResolvedBy,
		input.ResolutionDate,
		input.VerificationStatus,
		input.VerifiedBy,
		input.VerificationDate,
		input.PreventiveAction,
		input.ProcessChange,
		input.PreventiveOwner,
		input.DueDate,
	)
	stageRow, err := scanIssueStage(rowScanner)
	if err != nil && isUniqueViolation(err) {
		stageRow, err = scanIssueStage(tx.QueryRowContext(
			ctx,
			`UPDATE hiv.issue_resolution
				SET
					status = $2,
					is_current = TRUE,
					resolution_action = $3,
					resolved_by = $4,
					resolution_date = COALESCE($5, CURRENT_DATE),
					verification_status = $6,
					verified_by = $7,
					verification_date = $8,
					preventive_action = $9,
					process_change = $10,
					preventive_owner = $11,
					due_date = $12
				WHERE issue_code = $1
				RETURNING id, issue_code, status, is_current, resolution_action, resolved_by, resolution_date,
					verification_status, verified_by, verification_date, preventive_action, process_change, preventive_owner, due_date, assigned_to`,
			input.IssueCode,
			input.Status,
			input.ResolutionAction,
			input.ResolvedBy,
			input.ResolutionDate,
			input.VerificationStatus,
			input.VerifiedBy,
			input.VerificationDate,
			input.PreventiveAction,
			input.ProcessChange,
			input.PreventiveOwner,
			input.DueDate,
		))
	}
	if err != nil {
		return issueStageResponse{}, err
	}

	if _, err = tx.ExecContext(
		ctx,
		`UPDATE hiv.issue
		SET
			status = $2,
			updated_date = CURRENT_DATE,
			updated_by = COALESCE($3, updated_by)
		WHERE issue_code = $1`,
		input.IssueCode,
		input.Status,
		input.ResolvedBy,
	); err != nil {
		return issueStageResponse{}, err
	}

	if err = tx.Commit(); err != nil {
		return issueStageResponse{}, err
	}

	return stageRow, nil
}

func (r *postgresRepository) AssignIssues(ctx context.Context, input assignIssuesInput) ([]issueStageResponse, error) {
	if len(input.IssueCodes) == 0 {
		return nil, errors.New("no issue codes provided")
	}

	tx, err := r.dwhDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	results := make([]issueStageResponse, 0, len(input.IssueCodes))

	for _, issueCode := range input.IssueCodes {
		var currentStatus string
		err := tx.QueryRowContext(ctx, `SELECT status FROM hiv.issue WHERE issue_code = $1`, issueCode).Scan(&currentStatus)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}

		if _, err = tx.ExecContext(
			ctx,
			`UPDATE hiv.issue_resolution
			SET is_current = FALSE
			WHERE issue_code = $1 AND is_current = TRUE`,
			issueCode,
		); err != nil {
			return nil, err
		}

		action := "Assigned to " + input.AssignedTo
		if input.Comment != "" {
			action += ": " + input.Comment
		}

		rowScanner := tx.QueryRowContext(
			ctx,
			`INSERT INTO hiv.issue_resolution (
				issue_code,
				status,
				is_current,
				resolution_action,
				resolved_by,
				resolution_date,
				assigned_to
			) VALUES (
				$1, $2, TRUE, $3, $4, CURRENT_DATE, $5
			)
			RETURNING id, issue_code, status, is_current, resolution_action, resolved_by, resolution_date,
				verification_status, verified_by, verification_date, preventive_action, process_change, preventive_owner, due_date, assigned_to`,
			issueCode,
			currentStatus,
			action,
			input.AssignedBy,
			input.AssignedTo,
		)
		stageRow, err := scanIssueStage(rowScanner)
		if err != nil {
			return nil, err
		}

		if _, err = tx.ExecContext(
			ctx,
			`UPDATE hiv.issue
			SET
				assigned_to = $2,
				updated_date = CURRENT_DATE,
				updated_by = COALESCE($3, updated_by)
			WHERE issue_code = $1`,
			issueCode,
			input.AssignedTo,
			input.AssignedBy,
		); err != nil {
			return nil, err
		}

		results = append(results, stageRow)
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return results, nil
}

func (r *postgresRepository) ListIssueResolutionTransactions(ctx context.Context, issueCode string, limit int, offset int) ([]issueStageResponse, error) {
	var existingCode string
	err := r.dwhDB.QueryRowContext(
		ctx,
		`SELECT issue_code FROM hiv.issue WHERE issue_code = $1`,
		issueCode,
	).Scan(&existingCode)
	if err != nil {
		return nil, err
	}

	rows, err := r.dwhDB.QueryContext(
		ctx,
		`SELECT
			id,
			issue_code,
			status,
			is_current,
			resolution_action,
			resolved_by,
			resolution_date,
			verification_status,
			verified_by,
			verification_date,
			preventive_action,
			process_change,
			preventive_owner,
			due_date,
			assigned_to
		FROM hiv.issue_resolution
		WHERE issue_code = $1
		ORDER BY id DESC
		LIMIT $2 OFFSET $3`,
		issueCode,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	transactions := make([]issueStageResponse, 0)
	for rows.Next() {
		transaction, err := scanIssueStage(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	if len(transactions) == 0 && existingCode == "" {
		return nil, sql.ErrNoRows
	}

	return transactions, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}

	return string(pqErr.Code) == "23505"
}

func (r *postgresRepository) CountIssues(ctx context.Context, filter listIssuesFilter) (int64, error) {
	whereClause, args := buildIssuesWhereClause(filter)
	query := fmt.Sprintf(`SELECT COUNT(*)::bigint
		FROM hiv.issue
		LEFT JOIN (
			SELECT DISTINCT ON (org_unit_name) org_unit_name, region, district
			FROM hiv.organisation_unit
			ORDER BY org_unit_name, organisation_unit ASC
		) org ON org.org_unit_name = hiv.issue.org_unit
		%s`, whereClause)

	var count int64
	err := r.dwhDB.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}
