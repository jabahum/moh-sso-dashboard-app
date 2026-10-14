package data_quality

// DQA v2 store: the DWH-side persistence for the ported rule engine
// (internal/features/data_quality/dqa). The DWH has no golang-migrate
// runner (only the primary app DB does — see internal/bootstrap/infrastructure.go),
// so this schema is self-migrating on first use, the same pattern
// postgresRepository.ensureSchema already uses for hiv.issue.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/moh-sso-dashboard/internal/features/data_quality/dqa"
)

// dqaPreambleDDL is best-effort. On the MoH DWH the dqa schema is pre-provisioned
// and owned by a role with no database-level CREATE, and Postgres checks that
// privilege before the IF NOT EXISTS short-circuit — so this must be run
// separately from the table DDL and its failure ignored.
const dqaPreambleDDL = `CREATE SCHEMA IF NOT EXISTS dqa;`

const dqaSchemaDDL = `
CREATE TABLE IF NOT EXISTS dqa.dqa_tables (
    table_id      TEXT PRIMARY KEY,
    physical_table TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE dqa.dqa_tables ADD COLUMN IF NOT EXISTS period_column  TEXT;
ALTER TABLE dqa.dqa_tables ADD COLUMN IF NOT EXISTS dim_columns    TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE dqa.dqa_tables ADD COLUMN IF NOT EXISTS filter_columns TEXT[] NOT NULL DEFAULT '{}';

CREATE TABLE IF NOT EXISTS dqa.dqa_rules (
    id           BIGSERIAL PRIMARY KEY,
    identity     UUID        NOT NULL DEFAULT gen_random_uuid(),
    code         TEXT        NOT NULL,
    table_id     TEXT        NOT NULL,
    category     TEXT        NOT NULL DEFAULT 'custom',
    name         TEXT        NOT NULL DEFAULT '',
    rule_type    TEXT        NOT NULL
                 CHECK (rule_type IN ('metric','row_expression','row_join',
                                      'aggregate_compare','reference','raw_sql')),
    enabled      BOOLEAN     NOT NULL DEFAULT TRUE,
    row_filter   TEXT,
    zones        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    definition   JSONB       NOT NULL DEFAULT '{}'::jsonb,
    compiled_sql TEXT,
    created_by   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (table_id, code)
);
CREATE INDEX IF NOT EXISTS idx_dqa_rules_table    ON dqa.dqa_rules (table_id) WHERE enabled;
CREATE INDEX IF NOT EXISTS idx_dqa_rules_defn_gin ON dqa.dqa_rules USING GIN (definition);

CREATE TABLE IF NOT EXISTS dqa.dqa_runs (
    id           BIGSERIAL PRIMARY KEY,
    table_id     TEXT NOT NULL,
    run_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    rules_evaluated INTEGER NOT NULL DEFAULT 0,
    total_flags  INTEGER NOT NULL DEFAULT 0,
    errors       INTEGER NOT NULL DEFAULT 0,
    warnings     INTEGER NOT NULL DEFAULT 0,
    period_start TEXT,
    period_end   TEXT,
    triggered_by TEXT
);
CREATE INDEX IF NOT EXISTS idx_dqa_runs_table_id ON dqa.dqa_runs (table_id, run_at DESC);

CREATE TABLE IF NOT EXISTS dqa.dqa_flags (
    id          BIGSERIAL PRIMARY KEY,
    run_id      BIGINT NOT NULL REFERENCES dqa.dqa_runs(id) ON DELETE CASCADE,
    table_id    TEXT NOT NULL,
    rule_code   TEXT NOT NULL,
    severity    TEXT NOT NULL CHECK (severity IN ('warn','fail')),
    category    TEXT NOT NULL DEFAULT 'custom',
    detail      TEXT NOT NULL DEFAULT '',
    entity_id   TEXT,
    period_date TEXT,
    district    TEXT,
    entity_name TEXT,
    row_id      TEXT,
    year        INTEGER,
    month       INTEGER,
    region      TEXT,
    subcounty   TEXT,
    village     TEXT,
    facility    TEXT,
    vht         TEXT,
    dims        JSONB NOT NULL DEFAULT '{}'::jsonb,
    comment     TEXT
);
CREATE INDEX IF NOT EXISTS idx_dqa_flags_period   ON dqa.dqa_flags (year, month);
CREATE INDEX IF NOT EXISTS idx_dqa_flags_district ON dqa.dqa_flags (district);
CREATE INDEX IF NOT EXISTS idx_dqa_flags_facility ON dqa.dqa_flags (facility);
CREATE INDEX IF NOT EXISTS idx_dqa_flags_vht      ON dqa.dqa_flags (vht);
CREATE INDEX IF NOT EXISTS idx_dqa_flags_run_id   ON dqa.dqa_flags (run_id);
CREATE INDEX IF NOT EXISTS idx_dqa_flags_category ON dqa.dqa_flags (category);
CREATE INDEX IF NOT EXISTS idx_dqa_flags_severity ON dqa.dqa_flags (severity);

CREATE TABLE IF NOT EXISTS dqa.dqa_measurements (
    id             BIGSERIAL PRIMARY KEY,
    run_id         BIGINT REFERENCES dqa.dqa_runs(id) ON DELETE CASCADE,
    table_id       TEXT NOT NULL,
    rule_code      TEXT NOT NULL,
    metric_value   DOUBLE PRECISION,
    metric_detail  JSONB,
    measured_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_dqa_meas_series
    ON dqa.dqa_measurements (table_id, rule_code, measured_at DESC);
`

// DQAStore is the DWH-side persistence for the v2 rule engine.
type DQAStore struct {
	db *sql.DB
}

// NewDQAStore self-migrates the dqa schema (idempotent) and returns a store.
// A nil db is tolerated (feature is inert) so callers can wire it the same
// way NewRepository tolerates a nil dwhDB.
func NewDQAStore(db *sql.DB) *DQAStore {
	s := &DQAStore{db: db}
	if db != nil {
		ctx := context.Background()
		// Ignored on purpose — see dqaPreambleDDL.
		_, _ = db.ExecContext(ctx, dqaPreambleDDL)
		if _, err := db.ExecContext(ctx, dqaSchemaDDL); err != nil {
			// Non-fatal: surfaced on first real use instead of at boot, matching
			// the existing ensureSchema() convention for hiv.issue.
			fmt.Printf("dqa: schema init warning: %v\n", err)
		}
		if _, err := db.ExecContext(ctx, completenessSchemaDDL); err != nil {
			fmt.Printf("dqa: completeness schema init warning: %v\n", err)
		}
		if _, err := db.ExecContext(ctx, scheduledRunsSchemaDDL); err != nil {
			fmt.Printf("dqa: scheduled runs schema init warning: %v\n", err)
		}
	}
	return s
}

// ── table registry ──────────────────────────────────────────────────────

// TableMapping is one table_id -> physical table registration. PeriodColumn,
// DimColumns and FilterColumns are chosen per table because the datasets differ:
// one carries a VHT, another a facility.
type TableMapping struct {
	TableID       string   `json:"table_id"`
	PhysicalTable string   `json:"physical_table"`
	Description   string   `json:"description,omitempty"`
	IsActive      bool     `json:"is_active"`
	PeriodColumn  string   `json:"period_column,omitempty"`
	DimColumns    []string `json:"dim_columns"`
	FilterColumns []string `json:"filter_columns"`
}

func (s *DQAStore) UpsertTable(ctx context.Context, m TableMapping) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO dqa.dqa_tables
		  (table_id, physical_table, description, is_active, period_column,
		   dim_columns, filter_columns, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7, now())
		ON CONFLICT (table_id) DO UPDATE SET
		  physical_table = EXCLUDED.physical_table,
		  description    = EXCLUDED.description,
		  is_active      = EXCLUDED.is_active,
		  period_column  = EXCLUDED.period_column,
		  dim_columns    = EXCLUDED.dim_columns,
		  filter_columns = EXCLUDED.filter_columns,
		  updated_at     = now()`,
		m.TableID, m.PhysicalTable, m.Description, m.IsActive,
		nullIfEmpty(m.PeriodColumn), textArray(m.DimColumns), textArray(m.FilterColumns))
	return err
}

// textArray keeps a nil slice from reaching a NOT NULL column as SQL NULL, which
// is what pq.Array does with one. A request may legitimately omit these fields.
func textArray(values []string) any {
	if values == nil {
		return pq.Array([]string{})
	}
	return pq.Array(values)
}

// GetTable returns one registration, or false when it is not registered.
func (s *DQAStore) GetTable(ctx context.Context, tableID string) (TableMapping, bool, error) {
	m, err := scanTableMapping(s.db.QueryRowContext(ctx,
		`SELECT `+tableMappingColumns+` FROM dqa.dqa_tables WHERE table_id = $1`, tableID))
	if errors.Is(err, sql.ErrNoRows) {
		return TableMapping{}, false, nil
	}
	if err != nil {
		return TableMapping{}, false, err
	}
	return m, true, nil
}

const tableMappingColumns = `table_id, physical_table, description, is_active,
	COALESCE(period_column, ''), dim_columns, filter_columns`

func scanTableMapping(row interface{ Scan(...any) error }) (TableMapping, error) {
	var m TableMapping
	if err := row.Scan(&m.TableID, &m.PhysicalTable, &m.Description, &m.IsActive,
		&m.PeriodColumn, pq.Array(&m.DimColumns), pq.Array(&m.FilterColumns)); err != nil {
		return TableMapping{}, err
	}
	if m.DimColumns == nil {
		m.DimColumns = []string{}
	}
	if m.FilterColumns == nil {
		m.FilterColumns = []string{}
	}
	return m, nil
}

func (s *DQAStore) DeleteTable(ctx context.Context, tableID string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM dqa.dqa_tables WHERE table_id = $1`, tableID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListTables returns every registered table_id -> physical table mapping.
func (s *DQAStore) ListTables(ctx context.Context) ([]TableMapping, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+tableMappingColumns+` FROM dqa.dqa_tables ORDER BY table_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TableMapping{}
	for rows.Next() {
		m, err := scanTableMapping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// TableMap returns table_id -> physical_table for every active mapping,
// the shape CompileRule needs.
func (s *DQAStore) TableMap(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT table_id, physical_table FROM dqa.dqa_tables WHERE is_active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, phys string
		if err := rows.Scan(&id, &phys); err != nil {
			return nil, err
		}
		out[id] = phys
	}
	return out, rows.Err()
}

// ── rules CRUD ───────────────────────────────────────────────────────────

// RuleRecord is a stored rule plus its DB-only metadata (id/identity/audit).
type RuleRecord struct {
	dqa.Rule
	ID          int64     `json:"id"`
	Identity    string    `json:"identity"`
	CompiledSQL string    `json:"compiled_sql,omitempty"`
	CreatedBy   string    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UpsertRule inserts or updates a rule keyed by (table_id, code).
func (s *DQAStore) UpsertRule(ctx context.Context, rule dqa.Rule, compiledSQL, createdBy string) (RuleRecord, error) {
	zonesJSON, err := json.Marshal(rule.Zones)
	if err != nil {
		return RuleRecord{}, err
	}
	defJSON, err := json.Marshal(rule.Definition)
	if err != nil {
		return RuleRecord{}, err
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO dqa.dqa_rules
		  (code, table_id, category, name, rule_type, enabled, row_filter,
		   zones, definition, compiled_sql, created_by, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10,$11, now())
		ON CONFLICT (table_id, code) DO UPDATE SET
		   category=EXCLUDED.category, name=EXCLUDED.name,
		   rule_type=EXCLUDED.rule_type, enabled=EXCLUDED.enabled,
		   row_filter=EXCLUDED.row_filter, zones=EXCLUDED.zones,
		   definition=EXCLUDED.definition, compiled_sql=EXCLUDED.compiled_sql,
		   updated_at=now()
		RETURNING id, identity, created_by, created_at, updated_at`,
		rule.Code, rule.TableID, rule.Category, rule.Name, string(rule.RuleType),
		rule.Enabled, nullIfEmpty(rule.RowFilter), string(zonesJSON), string(defJSON),
		nullIfEmpty(compiledSQL), nullIfEmpty(createdBy))

	var rec RuleRecord
	var identity sql.NullString
	var createdByVal sql.NullString
	if err := row.Scan(&rec.ID, &identity, &createdByVal, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return RuleRecord{}, err
	}
	rec.Rule = rule
	rec.CompiledSQL = compiledSQL
	rec.Identity = identity.String
	rec.CreatedBy = createdByVal.String
	return rec, nil
}

func (s *DQAStore) DeleteRule(ctx context.Context, tableID, code string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM dqa.dqa_rules WHERE table_id = $1 AND code = $2`, tableID, code)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListRules reads rules, optionally scoped to one table_id. Disabled rules
// are included so the UI can show/toggle them; callers filter Enabled when
// building the runnable set.
func (s *DQAStore) ListRules(ctx context.Context, tableID string) ([]RuleRecord, error) {
	q := `SELECT id, identity, code, table_id, category, name, rule_type, enabled,
	             row_filter, zones, definition, compiled_sql, created_by, created_at, updated_at
	      FROM dqa.dqa_rules`
	args := []any{}
	if tableID != "" {
		q += ` WHERE table_id = $1`
		args = append(args, tableID)
	}
	q += ` ORDER BY table_id, code`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RuleRecord{}
	for rows.Next() {
		rec, err := scanRuleRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRuleRow(rows rowScanner) (RuleRecord, error) {
	var (
		rec                                        RuleRecord
		identity, name, rowFilter, compiledSQL, cb sql.NullString
		zonesRaw, defRaw                           []byte
		ruleType                                   string
	)
	if err := rows.Scan(&rec.ID, &identity, &rec.Code, &rec.TableID, &rec.Category, &name,
		&ruleType, &rec.Enabled, &rowFilter, &zonesRaw, &defRaw, &compiledSQL, &cb,
		&rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return RuleRecord{}, err
	}
	rec.Identity = identity.String
	rec.Name = name.String
	rec.RowFilter = rowFilter.String
	rec.CompiledSQL = compiledSQL.String
	rec.CreatedBy = cb.String
	rec.RuleType = dqa.RuleType(ruleType)

	var zones dqa.Zone
	_ = json.Unmarshal(zonesRaw, &zones)
	rec.Zones = zones

	var def map[string]any
	_ = json.Unmarshal(defRaw, &def)
	rec.Definition = def
	return rec, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfNilInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

// ── run + flag persistence ───────────────────────────────────────────────

// StoreRun persists one scan: a dqa_runs row, one dqa_flags row per flag,
// and per-rule measurements for history. Returns the run id.
func (s *DQAStore) StoreRun(ctx context.Context, tableID string, rulesEvaluated int,
	flags []dqa.RuleFlag, measurements map[string]any, triggeredBy string,
	periodStart, periodEnd string) (int64, error) {

	fails, warns := 0, 0
	for _, f := range flags {
		if f.Severity == string(dqa.SeverityFail) {
			fails++
		} else if f.Severity == string(dqa.SeverityWarn) {
			warns++
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var runID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO dqa.dqa_runs
		  (table_id, rules_evaluated, total_flags, errors, warnings, triggered_by, period_start, period_end)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		tableID, rulesEvaluated, len(flags), fails, warns, nullIfEmpty(triggeredBy),
		nullIfEmpty(periodStart), nullIfEmpty(periodEnd)).Scan(&runID)
	if err != nil {
		return 0, err
	}

	if len(flags) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO dqa.dqa_flags
			  (run_id, table_id, rule_code, severity, category, detail,
			   entity_id, period_date, district, entity_name, row_id,
			   year, month, region, subcounty, village, facility, vht, dims)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19::jsonb)`)
		if err != nil {
			return 0, err
		}
		defer stmt.Close()
		for _, f := range flags {
			dimsJSON, _ := json.Marshal(f.Dims)
			if _, err := stmt.ExecContext(ctx, runID, f.TableID, f.RuleCode, f.Severity,
				f.Category, f.Detail, nullIfEmpty(f.EntityID), nullIfEmpty(f.PeriodDate),
				nullIfEmpty(f.District), nullIfEmpty(f.EntityName), nullIfEmpty(f.RowID),
				nullIfNilInt(f.Year), nullIfNilInt(f.Month), nullIfEmpty(f.Region),
				nullIfEmpty(f.Subcounty), nullIfEmpty(f.Village), nullIfEmpty(f.Facility),
				nullIfEmpty(f.VHT), string(dimsJSON)); err != nil {
				return 0, err
			}
		}
	}

	if len(measurements) > 0 {
		mstmt, err := tx.PrepareContext(ctx, `
			INSERT INTO dqa.dqa_measurements (run_id, table_id, rule_code, metric_value, metric_detail)
			VALUES ($1,$2,$3,$4,$5::jsonb)`)
		if err != nil {
			return 0, err
		}
		defer mstmt.Close()
		for code, value := range measurements {
			var metricValue sql.NullFloat64
			var detail []byte
			switch v := value.(type) {
			case float64:
				metricValue = sql.NullFloat64{Float64: v, Valid: true}
			case int64:
				metricValue = sql.NullFloat64{Float64: float64(v), Valid: true}
			case nil:
				// leave both null (engine-error rule)
			default:
				detail, _ = json.Marshal(v)
			}
			if _, err := mstmt.ExecContext(ctx, runID, tableID, code, metricValue, nullBytes(detail)); err != nil {
				return 0, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return runID, nil
}

func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

// RunSummary is one row of a scan's run history.
type RunSummary struct {
	ID             int64     `json:"id"`
	TableID        string    `json:"table_id"`
	RunAt          time.Time `json:"run_at"`
	RulesEvaluated int       `json:"rules_evaluated"`
	TotalFlags     int       `json:"total_flags"`
	Errors         int       `json:"errors"`
	Warnings       int       `json:"warnings"`
	TriggeredBy    string    `json:"triggered_by,omitempty"`
}

func (s *DQAStore) ListRuns(ctx context.Context, tableID string, limit, offset int) ([]RunSummary, error) {
	q := `SELECT id, table_id, run_at, rules_evaluated, total_flags, errors, warnings,
	             COALESCE(triggered_by, '')
	      FROM dqa.dqa_runs`
	args := []any{}
	if tableID != "" {
		q += ` WHERE table_id = $1`
		args = append(args, tableID)
	}
	q += fmt.Sprintf(` ORDER BY run_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RunSummary{}
	for rows.Next() {
		var r RunSummary
		if err := rows.Scan(&r.ID, &r.TableID, &r.RunAt, &r.RulesEvaluated, &r.TotalFlags,
			&r.Errors, &r.Warnings, &r.TriggeredBy); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FlagRecord is one persisted violation.
type FlagRecord struct {
	ID         int64          `json:"id"`
	RunID      int64          `json:"run_id"`
	TableID    string         `json:"table_id"`
	RuleCode   string         `json:"rule_code"`
	Severity   string         `json:"severity"`
	Category   string         `json:"category"`
	Detail     string         `json:"detail"`
	EntityID   string         `json:"entity_id,omitempty"`
	PeriodDate string         `json:"period_date,omitempty"`
	District   string         `json:"district,omitempty"`
	EntityName string         `json:"entity_name,omitempty"`
	RowID      string         `json:"row_id,omitempty"`
	Year       *int           `json:"year,omitempty"`
	Month      *int           `json:"month,omitempty"`
	Region     string         `json:"region,omitempty"`
	Subcounty  string         `json:"subcounty,omitempty"`
	Village    string         `json:"village,omitempty"`
	Facility   string         `json:"facility,omitempty"`
	VHT        string         `json:"vht,omitempty"`
	Dims       map[string]any `json:"dims,omitempty"`
	Comment    string         `json:"comment,omitempty"`
}

// FlagFilter narrows a flag listing. Zero values mean "no filter".
type FlagFilter struct {
	Severity  string
	Year      *int
	Month     *int
	District  string
	Facility  string
	VHT       string
	Region    string
	Subcounty string
}

func (s *DQAStore) ListFlags(ctx context.Context, runID int64, filter FlagFilter, limit, offset int) ([]FlagRecord, error) {
	q := `SELECT id, run_id, table_id, rule_code, severity, category, detail,
	             COALESCE(entity_id,''), COALESCE(period_date,''), COALESCE(district,''),
	             COALESCE(entity_name,''), COALESCE(row_id,''),
	             year, month, COALESCE(region,''), COALESCE(subcounty,''),
	             COALESCE(village,''), COALESCE(facility,''), COALESCE(vht,''),
	             dims, COALESCE(comment,'')
	      FROM dqa.dqa_flags WHERE run_id = $1`
	args := []any{runID}

	addStr := func(col, val string) {
		if val == "" {
			return
		}
		q += fmt.Sprintf(` AND %s = $%d`, col, len(args)+1)
		args = append(args, val)
	}
	addInt := func(col string, val *int) {
		if val == nil {
			return
		}
		q += fmt.Sprintf(` AND %s = $%d`, col, len(args)+1)
		args = append(args, *val)
	}

	addStr("severity", filter.Severity)
	addStr("district", filter.District)
	addStr("facility", filter.Facility)
	addStr("vht", filter.VHT)
	addStr("region", filter.Region)
	addStr("subcounty", filter.Subcounty)
	addInt("year", filter.Year)
	addInt("month", filter.Month)

	q += fmt.Sprintf(` ORDER BY id LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []FlagRecord{}
	for rows.Next() {
		var f FlagRecord
		var dimsRaw []byte
		var year, month sql.NullInt64
		if err := rows.Scan(&f.ID, &f.RunID, &f.TableID, &f.RuleCode, &f.Severity, &f.Category,
			&f.Detail, &f.EntityID, &f.PeriodDate, &f.District, &f.EntityName, &f.RowID,
			&year, &month, &f.Region, &f.Subcounty, &f.Village, &f.Facility, &f.VHT,
			&dimsRaw, &f.Comment); err != nil {
			return nil, err
		}
		if year.Valid {
			v := int(year.Int64)
			f.Year = &v
		}
		if month.Valid {
			v := int(month.Int64)
			f.Month = &v
		}
		if len(dimsRaw) > 0 {
			_ = json.Unmarshal(dimsRaw, &f.Dims)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FlagFilterOptions are the distinct values available to filter a run's flags
// by, so the dashboard can populate its dropdowns from real data.
type FlagFilterOptions struct {
	Years       []int    `json:"years"`
	Months      []int    `json:"months"`
	Districts   []string `json:"districts"`
	Facilities  []string `json:"facilities"`
	VHTs        []string `json:"vhts"`
	Regions     []string `json:"regions"`
	Subcounties []string `json:"subcounties"`
}

func (s *DQAStore) FlagFilterOptions(ctx context.Context, runID int64) (FlagFilterOptions, error) {
	out := FlagFilterOptions{
		Years: []int{}, Months: []int{}, Districts: []string{},
		Facilities: []string{}, VHTs: []string{}, Regions: []string{},
		Subcounties: []string{},
	}

	ints := func(col string) ([]int, error) {
		rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
			`SELECT DISTINCT %s FROM dqa.dqa_flags WHERE run_id=$1 AND %s IS NOT NULL ORDER BY 1`, col, col), runID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		vals := []int{}
		for rows.Next() {
			var v int
			if err := rows.Scan(&v); err != nil {
				return nil, err
			}
			vals = append(vals, v)
		}
		return vals, rows.Err()
	}
	strs := func(col string) ([]string, error) {
		rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
			`SELECT DISTINCT %s FROM dqa.dqa_flags WHERE run_id=$1 AND %s <> '' AND %s IS NOT NULL ORDER BY 1`, col, col, col), runID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		vals := []string{}
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				return nil, err
			}
			vals = append(vals, v)
		}
		return vals, rows.Err()
	}

	var err error
	if out.Years, err = ints("year"); err != nil {
		return out, err
	}
	if out.Months, err = ints("month"); err != nil {
		return out, err
	}
	if out.Districts, err = strs("district"); err != nil {
		return out, err
	}
	if out.Facilities, err = strs("facility"); err != nil {
		return out, err
	}
	if out.VHTs, err = strs("vht"); err != nil {
		return out, err
	}
	if out.Regions, err = strs("region"); err != nil {
		return out, err
	}
	if out.Subcounties, err = strs("subcounty"); err != nil {
		return out, err
	}
	return out, nil
}
