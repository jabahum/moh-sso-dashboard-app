package visualiser

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type Repository interface {
	ListDatasets(ctx context.Context) ([]DatasetResponse, error)
	ListDataElements(ctx context.Context, dataSetID *string) ([]DataElementResponse, error)
	ListDataValues(ctx context.Context, req DataValuesRequest) (DataValuesResponse, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) ListDatasets(ctx context.Context) ([]DatasetResponse, error) {
	query := `
		SELECT dataset_key, dataset_id, display_name, is_current, create_date
		FROM dwh.dim_dataset
		WHERE is_current = true
		AND dataset_key <> -1
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]DatasetResponse, 0)
	for rows.Next() {
		var dataset DatasetResponse
		if err := rows.Scan(&dataset.DatasetKey, &dataset.DatasetID, &dataset.DisplayName, &dataset.IsCurrent, &dataset.CreateDate); err != nil {
			continue
		}
		results = append(results, dataset)
	}
	return results, rows.Err()
}

func (r *postgresRepository) ListDataElements(ctx context.Context, dataSetID *string) ([]DataElementResponse, error) {
	query := `
		SELECT 
			dim_data_element_map_key, 
			data_element_id, 
			data_set_id, 
			data_element_short_name, 
			data_element_long_name,
			row_version, 
			is_current
		FROM dwh.dim_hmis_data_element_map
		WHERE dim_data_element_map_key <> -1
		AND is_current = true
	`

	var (
		rows *sql.Rows
		err  error
	)
	if dataSetID != nil && *dataSetID != "" {
		query += ` AND data_set_id = $1`
		rows, err = r.db.QueryContext(ctx, query, *dataSetID)
	} else {
		rows, err = r.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]DataElementResponse, 0)
	for rows.Next() {
		var element DataElementResponse
		if err := rows.Scan(&element.DimDataElementMapKey, &element.DataElementID, &element.DataSetID, &element.DataElementShortName, &element.DataElementLongName, &element.RowVersion, &element.IsCurrent); err != nil {
			continue
		}
		results = append(results, element)
	}
	return results, rows.Err()
}

func (r *postgresRepository) ListDataValues(ctx context.Context, req DataValuesRequest) (DataValuesResponse, error) {
	query, values := buildDataValuesQuery(req)

	rows, err := r.db.QueryContext(ctx, query, values...)
	if err != nil {
		return DataValuesResponse{}, err
	}
	defer rows.Close()

	rowsList := make([]DataValueRowResponse, 0)
	for rows.Next() {
		var row DataValueRowResponse
		if err := rows.Scan(
			&row.OrgUnitID,
			&row.OrgUnitName,
			&row.DataElementID,
			&row.Period,
			&row.Value,
			&row.Level,
			&row.LevelOfCare,
			&row.Ownership,
			&row.Dataelement,
		); err != nil {
			return DataValuesResponse{}, err
		}
		rowsList = append(rowsList, row)
	}
	if err := rows.Err(); err != nil {
		return DataValuesResponse{}, err
	}
	return DataValuesResponse{Rows: rowsList}, nil
}

// buildDataValuesQuery keeps SQL construction separate from execution for plan inspection
// and database-backed regression tests.
func buildDataValuesQuery(req DataValuesRequest) (string, []interface{}) {
	var conditions []string
	var values []interface{}
	paramCounter := 0
	requestedLevel := normalizeOrgunitLevel(req.OrgunitLevel)

	if len(req.OU) > 0 {
		paramCounter++
		values = append(values, pq.Array(req.OU))
	}

	if len(req.DX) > 0 {
		paramCounter++
		values = append(values, pq.Array(req.DX))
		conditions = append(conditions, fmt.Sprintf("hs.data_element_id = ANY($%d)", paramCounter))
	}

	if len(req.PE) > 0 {
		paramCounter++
		values = append(values, pq.Array(req.PE))
		conditions = append(conditions, fmt.Sprintf(`hs."period" = ANY($%d)`, paramCounter))
	} else if req.StartDate != "" && req.EndDate != "" {
		paramCounter++
		values = append(values, req.StartDate)
		conditions = append(conditions, fmt.Sprintf("hs.tperiod >= $%d", paramCounter))

		paramCounter++
		values = append(values, req.EndDate)
		conditions = append(conditions, fmt.Sprintf("hs.tperiod <= $%d", paramCounter))
	}

	orgUnitFilterClause := ""
	if len(req.LevelOfCare) > 0 {
		paramCounter++
		values = append(values, pq.Array(req.LevelOfCare))
		orgUnitFilterClause += fmt.Sprintf(" AND h.level_of_care = ANY($%d)", paramCounter)
	}
	if len(req.Ownership) > 0 {
		paramCounter++
		values = append(values, pq.Array(req.Ownership))
		orgUnitFilterClause += fmt.Sprintf(" AND h.ownership = ANY($%d)", paramCounter)
	}
	aggregateAllCareAndOwnership := len(req.LevelOfCare) == 0 && len(req.Ownership) == 0
	levelOfCareExpr := "COALESCE(NULLIF(oa.level_of_care, ''), 'ALL')"
	ownershipExpr := "COALESCE(NULLIF(oa.ownership, ''), 'ALL')"
	if aggregateAllCareAndOwnership {
		levelOfCareExpr = "'ALL'"
		ownershipExpr = "'ALL'"
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	var query string
	if len(req.OU) > 0 {
		selectedLevelClause := ""
		if requestedLevel != nil {
			paramCounter++
			values = append(values, *requestedLevel)
			selectedLevelClause = fmt.Sprintf("WHERE su.selected_level = $%d", paramCounter)
		}
		attributesCTE, attributesJoin := "", ""
		if !aggregateAllCareAndOwnership {
			attributesCTE = `org_unit_attrs AS (
			  SELECT COALESCE(NULLIF(h.facility_uid, ''), h.org_unit_id) AS facility_uid,
			    MAX(NULLIF(h.level_of_care, '')) AS level_of_care,
			    MAX(NULLIF(h.ownership, '')) AS ownership
			  FROM hiv.organisation_unit h
			  GROUP BY 1
			),`
			attributesJoin = "LEFT JOIN org_unit_attrs oa ON oa.facility_uid = hs.org_unit_id"
		}
		outputID, outputName, outputLevel := "sf.selected_uid", "sf.selected_name", "sf.selected_level"
		outputJoin := "JOIN selected_facilities sf ON hs.org_unit_id = sf.facility_uid"
		facilityNameSelect := ""
		if req.AggregationLevel == "6" {
			// facility_values already contains each facility once, even for overlapping parents.
			outputID, outputName, outputLevel = "hs.org_unit_id", "hs.facility_name", "'6'"
			outputJoin = ""
			facilityNameSelect = ", COALESCE(MIN(NULLIF(hs.facility, '')), hs.org_unit_id) AS facility_name"
		}
		query = `WITH ` + attributesCTE + `
		   selected_input AS (
		     SELECT DISTINCT unnest($1::text[]) AS selected_uid
		   ),
		   selected_candidates AS (
		     SELECT
		       si.selected_uid,
		       '6'::text AS selected_level,
		       COALESCE(MIN(NULLIF(h.facility_name, '')), MIN(NULLIF(h.org_unit_name, '')), si.selected_uid) AS selected_name,
		       1 AS priority
		     FROM selected_input si
		     JOIN hiv.organisation_unit h
		       ON (h.facility_uid = si.selected_uid OR (h.level = '6' AND h.org_unit_id = si.selected_uid))
		     GROUP BY si.selected_uid

		     UNION ALL

		     SELECT
		       si.selected_uid,
		       '5'::text AS selected_level,
		       COALESCE(MIN(NULLIF(h.sub_county, '')), MIN(NULLIF(h.org_unit_name, '')), si.selected_uid) AS selected_name,
		       2 AS priority
		     FROM selected_input si
		     JOIN hiv.organisation_unit h ON
		      (h.sub_county_uid = si.selected_uid OR (h.level = '5' AND h.org_unit_id = si.selected_uid))
		     GROUP BY si.selected_uid

		     UNION ALL

		     SELECT
		       si.selected_uid,
		       '3'::text AS selected_level,
		       COALESCE(MIN(NULLIF(h.district, '')), si.selected_uid) AS selected_name,
		       3 AS priority
		     FROM selected_input si
		     JOIN hiv.organisation_unit h
		      ON h.district_uid = si.selected_uid
		     GROUP BY si.selected_uid

		     UNION ALL

		     SELECT
		       si.selected_uid,
		       '2'::text AS selected_level,
		       COALESCE(MIN(NULLIF(h.region, '')), si.selected_uid) AS selected_name,
		       4 AS priority
		     FROM selected_input si
		     JOIN hiv.organisation_unit h
		      ON h.region_uid = si.selected_uid
		     GROUP BY si.selected_uid

		     UNION ALL

		     SELECT
		       si.selected_uid,
		       '1'::text AS selected_level,
		       COALESCE(MIN(CASE WHEN h.level = '1' THEN NULLIF(h.org_unit_name, '') END), 'MoH - Uganda') AS selected_name,
		       5 AS priority
		     FROM selected_input si
		     JOIN hiv.organisation_unit h
		      ON (h.country_uid = si.selected_uid OR (h.level = '1' AND h.org_unit_id = si.selected_uid))
		     GROUP BY si.selected_uid

		     UNION ALL

		     SELECT
		       si.selected_uid,
		       '6'::text AS selected_level,
		       si.selected_uid AS selected_name,
		       99 AS priority
		     FROM selected_input si
		   ),
		   selected_units AS (
		     SELECT DISTINCT ON (su.selected_uid)
		       su.selected_uid,
		       su.selected_level,
		       su.selected_name
		     FROM selected_candidates su
		     ` + selectedLevelClause + `
		     ORDER BY su.selected_uid, su.priority
		   ),
		   selected_facilities AS (
		     SELECT DISTINCT selected_uid, selected_level, selected_name, facility_uid
		     FROM (
		       ` + selectedFacilityBranches(orgUnitFilterClause) + `
		     ) memberships
		   ),
		   facility_values AS (
		     SELECT hs.org_unit_id, hs.data_element_id, hs."period", hs.dataelement` + facilityNameSelect + `,
		       SUM(CASE
		         WHEN hs.value ~ '^\s*-?\d+(\.\d+)?\s*$' THEN TRIM(hs.value)::numeric
		         ELSE 0
		       END) AS value
		     FROM report.hmis_summary hs
		     JOIN (SELECT DISTINCT facility_uid FROM selected_facilities) selected
		       ON selected.facility_uid = hs.org_unit_id
		     ` + whereClause + `
		     GROUP BY 1, 2, 3, 4
		   )
		   SELECT ` + outputID + ` AS org_unit_id, ` + outputName + ` AS org_unit_name,
		     hs.data_element_id, hs."period", SUM(hs.value)::bigint AS value,
		     ` + outputLevel + ` AS "level",
		     ` + levelOfCareExpr + ` AS level_of_care,
		     ` + ownershipExpr + ` AS ownership,
		     hs.dataelement
		   FROM facility_values hs
		   ` + outputJoin + `
		   ` + attributesJoin + `
		   GROUP BY 1, 2, 3, 4, 6, 7, 8, 9
		   ORDER BY 4, 6, 2, 3
		`
	} else {
		aggregationLevel := resolveRequestedAggregationLevel(requestedLevel)
		if req.AggregationLevel == "6" {
			aggregationLevel = "6"
		}
		orgUnitExpr, facilityExpr, levelExpr, _, _, _ := aggregationExpressions(aggregationLevel)
		if req.AggregationLevel == "6" {
			levelExpr = "'6'"
		}
		orgUnitFilterJoin := ""
		if orgUnitFilterClause != "" {
			orgUnitFilterJoin = `
	       JOIN (
	         SELECT DISTINCT COALESCE(NULLIF(h.facility_uid, ''), h.org_unit_id) AS facility_uid
	         FROM hiv.organisation_unit h
	         WHERE COALESCE(NULLIF(h.facility_uid, ''), h.org_unit_id) IS NOT NULL
	           ` + orgUnitFilterClause + `
	       ) ou_filter
	         ON ou_filter.facility_uid = hs.org_unit_id
	       `
		}

		query = `
		   SELECT 
		      ` + orgUnitExpr + ` AS org_unit_id,
		      ` + facilityExpr + ` AS org_unit_name,
		      hs.data_element_id,
		      hs."period",
		      SUM(
		        CASE
		          WHEN hs.value ~ '^\s*-?\d+(\.\d+)?\s*$' THEN TRIM(hs.value)::numeric
		          ELSE 0
		        END
		      )::bigint AS value,
		      ` + levelExpr + ` AS "level",
	          ` + levelOfCareExpr + ` AS level_of_care,
	          ` + ownershipExpr + ` AS ownership,
	          hs.dataelement
	       FROM report.hmis_summary hs
	       LEFT JOIN (
	         SELECT
	           COALESCE(NULLIF(h.facility_uid, ''), h.org_unit_id) AS facility_uid,
	           MAX(NULLIF(h.level_of_care, '')) AS level_of_care,
	           MAX(NULLIF(h.ownership, '')) AS ownership
	         FROM hiv.organisation_unit h
	         WHERE COALESCE(NULLIF(h.facility_uid, ''), h.org_unit_id) IS NOT NULL
	         GROUP BY 1
	       ) oa
	         ON oa.facility_uid = hs.org_unit_id
	       ` + orgUnitFilterJoin + `
	       ` + whereClause + `
	       GROUP BY 1, 2, 3, 4, 6, 7, 8, 9
	       ORDER BY 
	          4,
	          1,
	          3
	    `
	}

	return query, values
}

// Separate equality joins allow PostgreSQL to choose indexes/hash joins per level.
// Deduplicate memberships before joining facts so duplicate hierarchy rows cannot
// multiply values. Overlapping selections still each receive their own total.
func selectedFacilityBranches(filters string) string {
	type membership struct{ level, predicate string }
	memberships := []membership{
		{"1", "h.country_uid = su.selected_uid"},
		{"1", "h.level = '1' AND h.org_unit_id = su.selected_uid"},
		{"2", "h.region_uid = su.selected_uid"},
		{"3", "h.district_uid = su.selected_uid"},
		{"5", "h.sub_county_uid = su.selected_uid"},
		{"5", "h.level = '5' AND h.org_unit_id = su.selected_uid"},
		{"6", "h.facility_uid = su.selected_uid"},
		{"6", "h.level = '6' AND h.org_unit_id = su.selected_uid"},
	}
	branches := make([]string, 0, len(memberships))
	for _, m := range memberships {
		branches = append(branches, `SELECT su.selected_uid, su.selected_level, su.selected_name,
		  COALESCE(NULLIF(h.facility_uid, ''), h.org_unit_id) AS facility_uid
		  FROM selected_units su JOIN hiv.organisation_unit h ON `+m.predicate+`
		  WHERE su.selected_level = '`+m.level+`'
		    AND COALESCE(NULLIF(h.facility_uid, ''), h.org_unit_id) IS NOT NULL`+filters)
	}
	return strings.Join(branches, " UNION ALL ")
}

func resolveRequestedAggregationLevel(requested *string) string {
	if requested != nil {
		switch *requested {
		case "1", "2", "3", "5", "6":
			return *requested
		}
	}
	return "6"
}
