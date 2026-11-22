package data_quality

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	healthcontext "github.com/moh-sso-dashboard/internal/features/health_context"
)

var errIssueOutsideHealthContext = errors.New("issue is outside the selected health context")

func healthContextScopeFromRequest(c *gin.Context) healthContextScope {
	contextID, _ := c.Get("health_context_id")
	id, _ := contextID.(uuid.UUID)
	scopeMode, _ := c.Get("health_context_scope_mode")
	mode, _ := scopeMode.(healthcontext.ScopeMode)
	return healthContextScope{
		ContextID:          id,
		IncludeDescendants: mode == healthcontext.ScopeNodeAndDescendants,
	}
}

func (r *postgresRepository) contextForNewIssue(
	ctx context.Context,
	scope healthContextScope,
) (uuid.UUID, error) {
	if scope.ContextID != uuid.Nil {
		return scope.ContextID, nil
	}

	var nationalID uuid.UUID
	err := r.primaryDB.QueryRowContext(
		ctx,
		`SELECT id FROM health_context_nodes WHERE code = 'UG' AND enabled`,
	).Scan(&nationalID)
	return nationalID, err
}

func (r *postgresRepository) upsertIssueOwnership(
	ctx context.Context,
	issueCode string,
	contextID uuid.UUID,
) error {
	_, err := r.primaryDB.ExecContext(ctx, `
		INSERT INTO data_quality_issue_contexts (issue_code, health_context_id)
		VALUES ($1, $2)
		ON CONFLICT (issue_code) DO UPDATE
		SET health_context_id = EXCLUDED.health_context_id,
		    updated_at = NOW()
	`, issueCode, contextID)
	return err
}

func (r *postgresRepository) deleteIssueAfterOwnershipFailure(ctx context.Context, issueCode string) {
	_, _ = r.dwhDB.ExecContext(ctx, `DELETE FROM hiv.issue WHERE issue_code = $1`, issueCode)
}

// Existing DWH issues predate contextual authorization. Their ownership is
// backfilled to the national context without changing the external DWH schema.
func (r *postgresRepository) ensureIssueOwnershipBackfill(ctx context.Context) error {
	r.ownershipBackfillMu.Lock()
	defer r.ownershipBackfillMu.Unlock()

	if time.Since(r.ownershipBackfilledAt) < 5*time.Minute {
		return nil
	}
	if r.dwhDB == nil {
		return errors.New("DWH connection is not configured")
	}

	rows, err := r.dwhDB.QueryContext(ctx, `
		SELECT issue_code
		FROM hiv.issue
		WHERE issue_code IS NOT NULL AND BTRIM(issue_code) <> ''
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	codes := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return err
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(codes) == 0 {
		r.ownershipBackfilledAt = time.Now()
		return nil
	}

	_, err = r.primaryDB.ExecContext(ctx, `
		INSERT INTO data_quality_issue_contexts (issue_code, health_context_id)
		SELECT issue_code, national.id
		FROM unnest($1::text[]) AS issue_code
		CROSS JOIN health_context_nodes national
		WHERE national.code = 'UG'
		ON CONFLICT (issue_code) DO NOTHING
	`, pq.Array(codes))
	if err == nil {
		r.ownershipBackfilledAt = time.Now()
	}
	return err
}

func (r *postgresRepository) issueOwnershipsInScope(
	ctx context.Context,
	scope healthContextScope,
) (map[string]uuid.UUID, error) {
	if err := r.ensureIssueOwnershipBackfill(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT ownership.issue_code, ownership.health_context_id
		FROM data_quality_issue_contexts ownership`
	args := make([]any, 0, 2)
	if scope.ContextID != uuid.Nil {
		query += `
		JOIN health_context_closure closure
		  ON closure.descendant_id = ownership.health_context_id
		WHERE closure.ancestor_id = $1
		  AND ($2 OR closure.depth = 0)`
		args = append(args, scope.ContextID, scope.IncludeDescendants)
	}

	rows, err := r.primaryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ownerships := make(map[string]uuid.UUID)
	for rows.Next() {
		var code string
		var contextID uuid.UUID
		if err := rows.Scan(&code, &contextID); err != nil {
			return nil, err
		}
		ownerships[code] = contextID
	}
	return ownerships, rows.Err()
}

func ownershipCodes(ownerships map[string]uuid.UUID) []string {
	codes := make([]string, 0, len(ownerships))
	for code := range ownerships {
		codes = append(codes, code)
	}
	return codes
}

func (r *postgresRepository) requireIssueInScope(
	ctx context.Context,
	issueCode string,
	scope healthContextScope,
) error {
	if scope.ContextID == uuid.Nil {
		return nil
	}
	if err := r.ensureIssueOwnershipBackfill(ctx); err != nil {
		return err
	}

	var allowed bool
	err := r.primaryDB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM data_quality_issue_contexts ownership
			JOIN health_context_closure closure
			  ON closure.descendant_id = ownership.health_context_id
			WHERE ownership.issue_code = $1
			  AND closure.ancestor_id = $2
			  AND ($3 OR closure.depth = 0)
		)
	`, issueCode, scope.ContextID, scope.IncludeDescendants).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return errIssueOutsideHealthContext
	}
	return nil
}
