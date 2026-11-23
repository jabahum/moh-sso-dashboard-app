package dataquality

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Completeness score — eCHIS DQA framework dimension 2. Unit is the VHT-month,
// averaged to district with equal weight per VHT; a VHT has reported when it made
// at least one submission of any form that month.

const completenessSourceTables = "report.mv_97_fact_vht_expected, report.mv_97_dim_vht, report.mv_97_fact_data_record_forms"

const completenessSchemaDDL = `
CREATE TABLE IF NOT EXISTS dqa.completeness_runs (
    id                      BIGSERIAL PRIMARY KEY,
    period_date             DATE        NOT NULL,
    run_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    triggered_by            TEXT,
    expected                INTEGER     NOT NULL DEFAULT 0,
    reported                INTEGER     NOT NULL DEFAULT 0,
    score                   NUMERIC(5,1),
    districts               INTEGER     NOT NULL DEFAULT 0,
    submitters_not_expected INTEGER     NOT NULL DEFAULT 0,
    source_tables           TEXT        NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_completeness_runs_period ON dqa.completeness_runs (period_date, run_at DESC);

CREATE TABLE IF NOT EXISTS dqa.completeness_district (
    run_id      BIGINT  NOT NULL REFERENCES dqa.completeness_runs(id) ON DELETE CASCADE,
    period_date DATE    NOT NULL,
    district    TEXT    NOT NULL,
    expected    INTEGER NOT NULL,
    reported    INTEGER NOT NULL,
    missing     INTEGER NOT NULL,
    score       NUMERIC(5,1),
    PRIMARY KEY (run_id, district)
);

CREATE TABLE IF NOT EXISTS dqa.completeness_vht (
    run_id            BIGINT  NOT NULL REFERENCES dqa.completeness_runs(id) ON DELETE CASCADE,
    period_date       DATE    NOT NULL,
    vht_uuid          TEXT    NOT NULL,
    username          TEXT,
    district          TEXT,
    sub_county        TEXT,
    parish            TEXT,
    village           TEXT,
    facility          TEXT,
    dhis2_facility_id TEXT,
    submissions       INTEGER NOT NULL,
    reported          BOOLEAN NOT NULL,
    PRIMARY KEY (run_id, vht_uuid)
);
CREATE INDEX IF NOT EXISTS idx_completeness_vht_district ON dqa.completeness_vht (run_id, district, reported);
`

// eatZone is fixed rather than loaded: the Alpine image ships no tzdata, and
// Uganda has no daylight saving.
var eatZone = time.FixedZone("EAT", 3*60*60)

var errCompletenessRunNotFound = errors.New("completeness run not found")

type CompletenessRun struct {
	ID                    int64     `json:"id"`
	PeriodDate            string    `json:"period_date"`
	RunAt                 time.Time `json:"run_at"`
	TriggeredBy           string    `json:"triggered_by,omitempty"`
	Expected              int       `json:"expected"`
	Reported              int       `json:"reported"`
	Missing               int       `json:"missing"`
	Score                 *float64  `json:"score"`
	Districts             int       `json:"districts"`
	SubmittersNotExpected int       `json:"submitters_not_expected"`
	SourceTables          string    `json:"source_tables"`
}

type CompletenessDistrict struct {
	RunID      int64    `json:"run_id"`
	PeriodDate string   `json:"period_date"`
	District   string   `json:"district"`
	Expected   int      `json:"expected"`
	Reported   int      `json:"reported"`
	Missing    int      `json:"missing"`
	Score      *float64 `json:"score"`
}

type CompletenessVHT struct {
	RunID           int64   `json:"run_id"`
	PeriodDate      string  `json:"period_date"`
	VHTUUID         string  `json:"vht_uuid"`
	Username        string  `json:"username,omitempty"`
	District        string  `json:"district,omitempty"`
	SubCounty       string  `json:"sub_county,omitempty"`
	Parish          string  `json:"parish,omitempty"`
	Village         string  `json:"village,omitempty"`
	Facility        string  `json:"facility,omitempty"`
	DHIS2FacilityID string  `json:"dhis2_facility_id,omitempty"`
	Submissions     int     `json:"submissions"`
	Reported        bool    `json:"reported"`
	Score           float64 `json:"score"`
}

// parseCompletenessPeriod accepts YYYY-MM, defaulting to last month. Only ended
// months are allowed: a month still in progress would score its VHTs as missing.
func parseCompletenessPeriod(raw string, now time.Time) (time.Time, error) {
	local := now.In(eatZone)
	currentMonth := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC)

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return currentMonth.AddDate(0, -1, 0), nil
	}
	period, err := time.Parse("2006-01", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("period must be YYYY-MM, got %q", raw)
	}
	if !period.Before(currentMonth) {
		return time.Time{}, fmt.Errorf("period %s has not ended yet", raw)
	}
	return period, nil
}

// RunCompleteness scores one month and persists the VHT, district and run rows in
// a single transaction, computed inside the DWH.
func (s *DQAStore) RunCompleteness(ctx context.Context, period time.Time, triggeredBy string) (CompletenessRun, error) {
	periodDate := period.Format("2006-01-02")

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CompletenessRun{}, err
	}
	defer tx.Rollback()

	var runID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO dqa.completeness_runs (period_date, triggered_by, source_tables)
		VALUES ($1::date, $2, $3) RETURNING id`,
		periodDate, nullIfEmpty(triggeredBy), completenessSourceTables).Scan(&runID); err != nil {
		return CompletenessRun{}, fmt.Errorf("creating run: %w", err)
	}

	// Scored from the per-VHT facts, not report.cht_form_097b_with_vhts: that
	// matview crosses every roster VHT with every period and zero-fills, so a row
	// exists whether or not the VHT submitted. Inactive VHTs are excluded — the
	// roster carries ~15k contacts with no eCHIS login who never submit. The
	// roster's name column is PGP-encrypted, so VHTs are identified by username.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO dqa.completeness_vht
		  (run_id, period_date, vht_uuid, username, district, sub_county,
		   parish, village, facility, dhis2_facility_id, submissions, reported)
		SELECT $1, e.period_date, v.vht_uuid, NULLIF(v.username, ''), v.district,
		       v.sub_county, v.parish, v.village, v.facility, v.dhis2_facility_id,
		       COALESCE(f.num_data_record_forms, 0)::int,
		       f.vht_uuid IS NOT NULL
		FROM report.mv_97_fact_vht_expected e
		JOIN report.mv_97_dim_vht v ON v.vht_uuid = e.vht_uuid
		LEFT JOIN report.mv_97_fact_data_record_forms f
		       ON f.vht_uuid = e.vht_uuid AND f.period_date = e.period_date
		      AND f.num_data_record_forms > 0
		WHERE e.period_date = $2::date AND v.active IS TRUE`,
		runID, periodDate); err != nil {
		return CompletenessRun{}, fmt.Errorf("scoring VHTs: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO dqa.completeness_district
		  (run_id, period_date, district, expected, reported, missing, score)
		SELECT run_id, period_date, COALESCE(district, 'Undefined'),
		       count(*), count(*) FILTER (WHERE reported), count(*) FILTER (WHERE NOT reported),
		       round(100.0 * count(*) FILTER (WHERE reported) / NULLIF(count(*), 0), 1)
		FROM dqa.completeness_vht
		WHERE run_id = $1
		GROUP BY run_id, period_date, COALESCE(district, 'Undefined')`,
		runID); err != nil {
		return CompletenessRun{}, fmt.Errorf("rolling up districts: %w", err)
	}

	// Submitters with no expected row are reported beside the score so it cannot
	// hide them; they are mostly non-VHT eCHIS users.
	if _, err := tx.ExecContext(ctx, `
		UPDATE dqa.completeness_runs r
		SET expected  = t.expected,
		    reported  = t.reported,
		    score     = round(100.0 * t.reported / NULLIF(t.expected, 0), 1),
		    districts = t.districts,
		    submitters_not_expected = (
		      SELECT count(*) FROM report.mv_97_fact_data_record_forms f
		      WHERE f.period_date = $2::date AND f.num_data_record_forms > 0
		        AND NOT EXISTS (SELECT 1 FROM dqa.completeness_vht c
		                        WHERE c.run_id = r.id AND c.vht_uuid = f.vht_uuid))
		FROM (SELECT count(*) AS expected,
		             count(*) FILTER (WHERE reported) AS reported,
		             count(DISTINCT COALESCE(district, 'Undefined')) AS districts
		      FROM dqa.completeness_vht WHERE run_id = $1) t
		WHERE r.id = $1`,
		runID, periodDate); err != nil {
		return CompletenessRun{}, fmt.Errorf("finalising run: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return CompletenessRun{}, err
	}
	return s.GetCompletenessRun(ctx, runID)
}

const completenessRunColumns = `id, period_date::text, run_at, COALESCE(triggered_by, ''), expected, reported,
	score, districts, submitters_not_expected, source_tables`

func scanCompletenessRun(row interface{ Scan(...any) error }) (CompletenessRun, error) {
	var r CompletenessRun
	var score sql.NullFloat64
	if err := row.Scan(&r.ID, &r.PeriodDate, &r.RunAt, &r.TriggeredBy, &r.Expected, &r.Reported,
		&score, &r.Districts, &r.SubmittersNotExpected, &r.SourceTables); err != nil {
		return CompletenessRun{}, err
	}
	r.Missing = r.Expected - r.Reported
	r.Score = nullableScore(score)
	return r, nil
}

// nullableScore keeps N/A distinct from 0%, as the framework requires.
func nullableScore(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

func (s *DQAStore) GetCompletenessRun(ctx context.Context, runID int64) (CompletenessRun, error) {
	run, err := scanCompletenessRun(s.db.QueryRowContext(ctx,
		`SELECT `+completenessRunColumns+` FROM dqa.completeness_runs WHERE id = $1`, runID))
	if errors.Is(err, sql.ErrNoRows) {
		return CompletenessRun{}, errCompletenessRunNotFound
	}
	return run, err
}

func (s *DQAStore) ListCompletenessRuns(ctx context.Context, limit, offset int) ([]CompletenessRun, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+completenessRunColumns+` FROM dqa.completeness_runs
		 ORDER BY period_date DESC, run_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CompletenessRun{}
	for rows.Next() {
		run, err := scanCompletenessRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (s *DQAStore) ListCompletenessDistricts(ctx context.Context, runID int64) ([]CompletenessDistrict, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, period_date::text, district, expected, reported, missing, score
		FROM dqa.completeness_district WHERE run_id = $1
		ORDER BY score ASC NULLS FIRST, district`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CompletenessDistrict{}
	for rows.Next() {
		var d CompletenessDistrict
		var score sql.NullFloat64
		if err := rows.Scan(&d.RunID, &d.PeriodDate, &d.District, &d.Expected, &d.Reported,
			&d.Missing, &score); err != nil {
			return nil, err
		}
		d.Score = nullableScore(score)
		out = append(out, d)
	}
	return out, rows.Err()
}

// CompletenessVHTFilter narrows the VHT listing. Status is "missing", "reported"
// or empty for both.
type CompletenessVHTFilter struct {
	District string
	Status   string
}

func (s *DQAStore) ListCompletenessVHTs(ctx context.Context, runID int64, filter CompletenessVHTFilter, limit, offset int) ([]CompletenessVHT, error) {
	q := `SELECT run_id, period_date::text, vht_uuid, COALESCE(username, ''),
	             COALESCE(district, ''), COALESCE(sub_county, ''), COALESCE(parish, ''),
	             COALESCE(village, ''), COALESCE(facility, ''), COALESCE(dhis2_facility_id, ''),
	             submissions, reported
	      FROM dqa.completeness_vht WHERE run_id = $1`
	args := []any{runID}
	if filter.District != "" {
		args = append(args, filter.District)
		q += fmt.Sprintf(` AND district = $%d`, len(args))
	}
	switch filter.Status {
	case "missing":
		q += ` AND NOT reported`
	case "reported":
		q += ` AND reported`
	}
	args = append(args, limit, offset)
	q += fmt.Sprintf(` ORDER BY district, sub_county, username LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CompletenessVHT{}
	for rows.Next() {
		var v CompletenessVHT
		if err := rows.Scan(&v.RunID, &v.PeriodDate, &v.VHTUUID, &v.Username, &v.District,
			&v.SubCounty, &v.Parish, &v.Village, &v.Facility, &v.DHIS2FacilityID,
			&v.Submissions, &v.Reported); err != nil {
			return nil, err
		}
		if v.Reported {
			v.Score = 100
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
