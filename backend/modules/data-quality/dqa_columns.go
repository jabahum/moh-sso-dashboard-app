package dataquality

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// TableColumn is one column of a physical DWH table, offered to the UI so a table
// can be configured against the columns it actually has.
type TableColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListPhysicalColumns reads a table's columns from the catalog. The table name is
// passed as a parameter cast to regclass, never interpolated.
func ListPhysicalColumns(ctx context.Context, db *sql.DB, physicalTable string) ([]TableColumn, error) {
	physicalTable = strings.TrimSpace(physicalTable)
	if physicalTable == "" {
		return nil, fmt.Errorf("physical table is required")
	}

	rows, err := db.QueryContext(ctx, `
		SELECT a.attname, format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a
		WHERE a.attrelid = $1::regclass AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, physicalTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TableColumn{}
	for rows.Next() {
		var c TableColumn
		if err := rows.Scan(&c.Name, &c.Type); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
