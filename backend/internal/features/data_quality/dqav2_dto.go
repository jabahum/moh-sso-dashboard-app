package data_quality

import "github.com/moh-sso-dashboard/internal/features/data_quality/dqa"

// dqaRuleRequest is the wire shape for creating/updating a v2 rule. It maps
// 1:1 onto dqa.RuleFromMap's expectations.
type dqaRuleRequest struct {
	Code       string         `json:"code" binding:"required"`
	TableID    string         `json:"table_id" binding:"required"`
	Type       string         `json:"type" binding:"required"`
	Category   string         `json:"category"`
	Name       string         `json:"name"`
	Enabled    *bool          `json:"enabled"`
	RowFilter  string         `json:"row_filter"`
	GroupBy    []string       `json:"group_by"`
	Zones      dqaZonesDTO    `json:"zones"`
	Definition map[string]any `json:"definition"`
}

type dqaZonesDTO struct {
	Warn string `json:"warn"`
	Fail string `json:"fail"`
}

func (r dqaRuleRequest) toRule() dqa.Rule {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return dqa.Rule{
		Code: r.Code, TableID: r.TableID, RuleType: dqa.RuleType(r.Type),
		Category: r.Category, Name: r.Name, Enabled: enabled, RowFilter: r.RowFilter,
		GroupBy: r.GroupBy, Zones: dqa.Zone{Warn: r.Zones.Warn, Fail: r.Zones.Fail},
		Definition: r.Definition,
	}
}

// dqaRuleResponse is what the API returns for a stored rule.
type dqaRuleResponse struct {
	ID          int64          `json:"id"`
	Identity    string         `json:"identity"`
	Code        string         `json:"code"`
	TableID     string         `json:"table_id"`
	Type        string         `json:"type"`
	Category    string         `json:"category"`
	Name        string         `json:"name"`
	Enabled     bool           `json:"enabled"`
	RowFilter   string         `json:"row_filter,omitempty"`
	GroupBy     []string       `json:"group_by,omitempty"`
	Zones       dqaZonesDTO    `json:"zones"`
	Definition  map[string]any `json:"definition"`
	CompiledSQL string         `json:"compiled_sql,omitempty"`
	CreatedBy   string         `json:"created_by,omitempty"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

func ruleRecordToResponse(rec RuleRecord) dqaRuleResponse {
	return dqaRuleResponse{
		ID: rec.ID, Identity: rec.Identity, Code: rec.Code, TableID: rec.TableID,
		Type: string(rec.RuleType), Category: rec.Category, Name: rec.Name,
		Enabled: rec.Enabled, RowFilter: rec.RowFilter, GroupBy: rec.GroupBy,
		Zones:       dqaZonesDTO{Warn: rec.Zones.Warn, Fail: rec.Zones.Fail},
		Definition:  rec.Definition,
		CompiledSQL: rec.CompiledSQL, CreatedBy: rec.CreatedBy,
		CreatedAt: rec.CreatedAt.Format(rfc3339Milli), UpdatedAt: rec.UpdatedAt.Format(rfc3339Milli),
	}
}

const rfc3339Milli = "2006-01-02T15:04:05.000Z07:00"

// dqaTableRequest registers/updates a table_id -> physical table mapping.
type dqaTableRequest struct {
	TableID       string   `json:"table_id" binding:"required"`
	PhysicalTable string   `json:"physical_table" binding:"required"`
	Description   string   `json:"description"`
	IsActive      *bool    `json:"is_active"`
	PeriodColumn  string   `json:"period_column"`
	DimColumns    []string `json:"dim_columns"`
	FilterColumns []string `json:"filter_columns"`
}

func (r dqaTableRequest) toMapping() TableMapping {
	active := true
	if r.IsActive != nil {
		active = *r.IsActive
	}
	return TableMapping{
		TableID: r.TableID, PhysicalTable: r.PhysicalTable,
		Description: r.Description, IsActive: active,
		PeriodColumn: r.PeriodColumn,
		DimColumns:   r.DimColumns, FilterColumns: r.FilterColumns,
	}
}

// dqaCompilePreviewResponse is the compiled-SQL preview shown before saving.
type dqaCompilePreviewResponse struct {
	Kind       string `json:"kind"`
	MeasureSQL string `json:"measure_sql"`
	SampleSQL  string `json:"sample_sql,omitempty"`
}

// dqaRunRequest triggers a scan of one registered table, optionally narrowed to
// chosen periods and filter values.
type dqaRunRequest struct {
	TableID string   `json:"table_id" binding:"required"`
	Scope   RunScope `json:"scope"`
}
