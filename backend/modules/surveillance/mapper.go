package surveillance

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

func toAlertResponse(row db.ListAlertsRow) AlertResponse {
	var externalID *string
	if row.ExternalID.Valid {
		externalID = &row.ExternalID.String
	}

	var diseaseName *string
	if row.DiseaseName.Valid {
		diseaseName = &row.DiseaseName.String
	}

	var districtID *uuid.UUID
	if row.DistrictID.Valid {
		districtID = &row.DistrictID.UUID
	}

	var districtName *string
	if row.DistrictName.Valid {
		districtName = &row.DistrictName.String
	}

	var regionID *uuid.UUID
	if row.RegionID.Valid {
		regionID = &row.RegionID.UUID
	}

	var epiWeekID *uuid.UUID
	if row.EpiWeekID.Valid {
		epiWeekID = &row.EpiWeekID.UUID
	}

	var occurredOn *time.Time
	if row.OccurredOn.Valid {
		occurredOn = &row.OccurredOn.Time
	}

	var createdOn *time.Time
	if row.CreatedOn.Valid {
		createdOn = &row.CreatedOn.Time
	}

	var submittedBy *string
	if row.SubmittedBy.Valid {
		submittedBy = &row.SubmittedBy.String
	}

	var sourceName *string
	if row.SourceName.Valid {
		sourceName = &row.SourceName.String
	}

	return AlertResponse{
		ID:           row.ID,
		ExternalID:   externalID,
		DiseaseID:    row.DiseaseID,
		DiseaseName:  diseaseName,
		DistrictID:   districtID,
		DistrictName: districtName,
		RegionID:     regionID,
		EpiWeekID:    epiWeekID,
		OccurredOn:   occurredOn,
		CreatedOn:    createdOn,
		Narrative:    row.Narrative,
		SubmittedBy:  submittedBy,
		Status:       string(row.Status),
		SourceName:   sourceName,
		ImportedAt:   row.ImportedAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func toAlertResponses(items []db.ListAlertsRow) []AlertResponse {
	out := make([]AlertResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toAlertResponse(item))
	}
	return out
}

func uuidPtr(v uuid.UUID) *uuid.UUID {
	id := v
	return &id
}

func sqlNullStringValue(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return strings.TrimSpace(ns.String)
}

func nullStringValue(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func nullUUIDPtr(id uuid.NullUUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	v := id.UUID
	return &v
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func sqlNullStringFromPtr(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{
		String: *value,
		Valid:  true,
	}
}

func uuidNullFromPtr(value *uuid.UUID) uuid.NullUUID {
	if value == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{
		UUID:  *value,
		Valid: true,
	}
}

func uuidNull(value uuid.UUID) uuid.NullUUID {
	if value == uuid.Nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: value, Valid: true}
}

func riskLevelNull(value string) db.NullRiskLevel {
	switch db.RiskLevel(value) {
	case db.RiskLevelMAROON, db.RiskLevelRED, db.RiskLevelYELLOW, db.RiskLevelGREEN:
		return db.NullRiskLevel{RiskLevel: db.RiskLevel(value), Valid: true}
	default:
		return db.NullRiskLevel{}
	}
}

func requiredTimePtr(value time.Time) *time.Time {
	return &value
}

func toEpiWeekResponse(item db.EpiWeek) EpiWeekResponse {
	return EpiWeekResponse{
		ID:            item.ID,
		EpiYear:       item.EpiYear,
		EpiWeek:       item.EpiWeek,
		WeekStartDate: &item.WeekStartDate,
		WeekEndDate:   &item.WeekEndDate,
		CreatedAt:     item.CreatedAt,
	}
}

func toEpiWeekResponses(items []db.EpiWeek) []EpiWeekResponse {
	out := make([]EpiWeekResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toEpiWeekResponse(item))
	}
	return out
}

func toDiseaseResponse(item db.Disease) DiseaseResponse {
	return DiseaseResponse{
		ID:        item.ID,
		Name:      item.Name,
		ShortName: nullStringPtr(item.ShortName),
		Code:      nullStringPtr(item.Code),
		Category:  nullStringPtr(item.Category),
		IsActive:  item.IsActive,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func toDiseaseResponses(items []db.Disease) []DiseaseResponse {
	out := make([]DiseaseResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toDiseaseResponse(item))
	}
	return out
}

func toRegionResponse(item db.Region) RegionResponse {
	return RegionResponse{
		ID:        item.ID,
		Name:      item.Name,
		Code:      nullStringPtr(item.Code),
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func toRegionResponses(items []db.Region) []RegionResponse {
	out := make([]RegionResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toRegionResponse(item))
	}
	return out
}

func toDistrictResponse(item db.District) DistrictResponse {
	return DistrictResponse{
		ID:        item.ID,
		Name:      item.Name,
		RegionID:  nullUUIDPtr(item.RegionID),
		Code:      nullStringPtr(item.Code),
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func toDistrictListResponse(item db.ListDistrictsRow) DistrictResponse {
	out := DistrictResponse{
		ID:         item.ID,
		Name:       item.Name,
		RegionID:   nullUUIDPtr(item.RegionID),
		Code:       nullStringPtr(item.Code),
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
		RegionName: nullStringPtr(item.RegionName),
	}
	return out
}

func toDistrictListResponses(items []db.ListDistrictsRow) []DistrictResponse {
	out := make([]DistrictResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toDistrictListResponse(item))
	}
	return out
}

func toDistrictResponses(items []db.District) []DistrictResponse {
	out := make([]DistrictResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toDistrictResponse(item))
	}
	return out
}

func toSubCountyResponse(item db.SubCounty) SubCountyResponse {
	return SubCountyResponse{
		ID:         item.ID,
		Name:       item.Name,
		DistrictID: item.DistrictID,
		Code:       nullStringPtr(item.Code),
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

func toSubCountyResponses(items []db.SubCounty) []SubCountyResponse {
	out := make([]SubCountyResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toSubCountyResponse(item))
	}
	return out
}

func toWeeklyStatusDetailedResponse(row db.ListWeeklyStatusesDetailedRow) WeeklyStatusDetailedResponse {
	var regionID *uuid.UUID
	if row.RegionID.Valid {
		regionID = uuidPtr(row.RegionID.UUID)
	}

	var districtID *uuid.UUID
	if row.DistrictID.Valid {
		districtID = uuidPtr(row.DistrictID.UUID)
	}

	var subCountyID *uuid.UUID
	if row.SubCountyID.Valid {
		subCountyID = uuidPtr(row.SubCountyID.UUID)
	}

	var diseaseID *uuid.UUID
	if row.DiseaseID.Valid {
		diseaseID = uuidPtr(row.DiseaseID.UUID)
	}

	var indicatorID *uuid.UUID
	if row.IndicatorID.Valid {
		indicatorID = uuidPtr(row.IndicatorID.UUID)
	}

	return WeeklyStatusDetailedResponse{
		ID:            row.ID,
		RegionID:      regionID,
		DistrictID:    districtID,
		SubCountyID:   subCountyID,
		DiseaseID:     diseaseID,
		IndicatorID:   indicatorID,
		EpiWeekID:     row.EpiWeekID,
		Status:        string(row.Status),
		SourceName:    sqlNullStringValue(row.SourceName),
		ImportedAt:    row.ImportedAt,
		CreatedAt:     row.CreatedAt,
		RegionName:    sqlNullStringValue(row.RegionName),
		DistrictName:  sqlNullStringValue(row.DistrictName),
		SubCountyName: sqlNullStringValue(row.SubCountyName),
		DiseaseName:   sqlNullStringValue(row.DiseaseName),
		IndicatorName: sqlNullStringValue(row.IndicatorName),
	}
}

func toWeeklyStatusResponse(item db.WeeklyStatus) WeeklyStatusResponse {
	return WeeklyStatusResponse{
		ID:          item.ID,
		RegionID:    nullUUIDPtr(item.RegionID),
		DistrictID:  nullUUIDPtr(item.DistrictID),
		SubCountyID: nullUUIDPtr(item.SubCountyID),
		DiseaseID:   nullUUIDPtr(item.DiseaseID),
		IndicatorID: nullUUIDPtr(item.IndicatorID),
		EpiWeekID:   item.EpiWeekID,
		Status:      string(item.Status),
		SourceName:  nullStringPtr(item.SourceName),
		ImportedAt:  item.ImportedAt,
		CreatedAt:   item.CreatedAt,
	}
}

func toWeeklyStatusResponses(items []db.WeeklyStatus) []WeeklyStatusResponse {
	out := make([]WeeklyStatusResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toWeeklyStatusResponse(item))
	}
	return out
}

func toFacilityWeeklyDiseaseMetricResponse(row db.ListFacilityWeeklyDiseaseMetricsByWeekRow) FacilityWeeklyMetricResponse {
	var sourceRecordID *string
	if row.SourceRecordID.Valid {
		sourceRecordID = &row.SourceRecordID.String
	}

	var subCountyID *uuid.UUID
	if row.SubCountyID.Valid {
		subCountyID = uuidPtr(row.SubCountyID.UUID)
	}

	var districtID *uuid.UUID
	if row.DistrictID.Valid {
		districtID = uuidPtr(row.DistrictID.UUID)
	}

	var regionID *uuid.UUID
	if row.RegionID.Valid {
		regionID = uuidPtr(row.RegionID.UUID)
	}

	var diseaseID *uuid.UUID
	if row.DiseaseID.Valid {
		diseaseID = uuidPtr(row.DiseaseID.UUID)
	}

	return FacilityWeeklyMetricResponse{
		ID:             row.ID,
		SourceRecordID: sourceRecordID,
		FacilityID:     row.FacilityID,
		FacilityName:   row.FacilityName,
		SubCountyID:    subCountyID,
		SubCountyName:  sqlNullStringValue(row.SubCountyName),
		DistrictID:     districtID,
		DistrictName:   sqlNullStringValue(row.DistrictName),
		RegionID:       regionID,
		RegionName:     sqlNullStringValue(row.RegionName),
		DiseaseID:      diseaseID,
		DiseaseName:    row.DiseaseName,
		IndicatorID:    nil,
		IndicatorName:  "",
		EpiWeekID:      row.EpiWeekID,
		EpiYear:        row.EpiYear,
		EpiWeek:        row.EpiWeek,
		MetricValue:    row.MetricValue,
		SourceName:     sqlNullStringValue(row.SourceName),
		ImportedAt:     row.ImportedAt,
		CreatedAt:      row.CreatedAt,
	}
}

func toFacilityWeeklyDiseaseMetricByWeekAndDiseaseResponse(row db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow) FacilityWeeklyMetricResponse {
	var sourceRecordID *string
	if row.SourceRecordID.Valid {
		sourceRecordID = &row.SourceRecordID.String
	}

	var subCountyID *uuid.UUID
	if row.SubCountyID.Valid {
		subCountyID = uuidPtr(row.SubCountyID.UUID)
	}

	var districtID *uuid.UUID
	if row.DistrictID.Valid {
		districtID = uuidPtr(row.DistrictID.UUID)
	}

	var regionID *uuid.UUID
	if row.RegionID.Valid {
		regionID = uuidPtr(row.RegionID.UUID)
	}

	var diseaseID *uuid.UUID
	if row.DiseaseID.Valid {
		diseaseID = uuidPtr(row.DiseaseID.UUID)
	}

	return FacilityWeeklyMetricResponse{
		ID:             row.ID,
		SourceRecordID: sourceRecordID,
		FacilityID:     row.FacilityID,
		FacilityName:   row.FacilityName,
		SubCountyID:    subCountyID,
		SubCountyName:  sqlNullStringValue(row.SubCountyName),
		DistrictID:     districtID,
		DistrictName:   sqlNullStringValue(row.DistrictName),
		RegionID:       regionID,
		RegionName:     sqlNullStringValue(row.RegionName),
		DiseaseID:      diseaseID,
		DiseaseName:    row.DiseaseName,
		IndicatorID:    nil,
		IndicatorName:  "",
		EpiWeekID:      row.EpiWeekID,
		EpiYear:        row.EpiYear,
		EpiWeek:        row.EpiWeek,
		MetricValue:    row.MetricValue,
		SourceName:     sqlNullStringValue(row.SourceName),
		ImportedAt:     row.ImportedAt,
		CreatedAt:      row.CreatedAt,
	}
}

func toFacilityMetricByFacilityResponse(row db.ListFacilityMetricsByFacilityRow) FacilityWeeklyMetricResponse {
	return FacilityWeeklyMetricResponse{
		ID:             row.ID,
		SourceRecordID: nullStringPtr(row.SourceRecordID),
		FacilityID:     row.FacilityID,
		FacilityName:   row.FacilityName,
		SubCountyID:    nullUUIDPtr(row.SubCountyID),
		SubCountyName:  sqlNullStringValue(row.SubCountyName),
		DistrictID:     nullUUIDPtr(row.DistrictID),
		DistrictName:   sqlNullStringValue(row.DistrictName),
		RegionID:       nullUUIDPtr(row.RegionID),
		RegionName:     sqlNullStringValue(row.RegionName),
		DiseaseID:      nullUUIDPtr(row.DiseaseID),
		DiseaseName:    subjectNameForType(row.SubjectType, "disease", row.SubjectName),
		IndicatorID:    nullUUIDPtr(row.IndicatorID),
		IndicatorName:  subjectNameForType(row.SubjectType, "indicator", row.SubjectName),
		EpiWeekID:      row.EpiWeekID,
		EpiYear:        row.EpiYear,
		EpiWeek:        row.EpiWeek,
		MetricValue:    row.MetricValue,
		SourceName:     sqlNullStringValue(row.SourceName),
		ImportedAt:     row.ImportedAt,
		CreatedAt:      row.CreatedAt,
	}
}

func toFacilityDiseaseMetricTrendResponse(row db.ListFacilityDiseaseMetricsTrendRow) FacilityWeeklyMetricResponse {
	return FacilityWeeklyMetricResponse{
		ID:             row.ID,
		SourceRecordID: nullStringPtr(row.SourceRecordID),
		FacilityID:     row.FacilityID,
		FacilityName:   row.FacilityName,
		SubCountyID:    nullUUIDPtr(row.SubCountyID),
		SubCountyName:  sqlNullStringValue(row.SubCountyName),
		DistrictID:     nullUUIDPtr(row.DistrictID),
		DistrictName:   sqlNullStringValue(row.DistrictName),
		RegionID:       nullUUIDPtr(row.RegionID),
		RegionName:     sqlNullStringValue(row.RegionName),
		DiseaseID:      nullUUIDPtr(row.DiseaseID),
		DiseaseName:    row.DiseaseName,
		EpiWeekID:      row.EpiWeekID,
		EpiYear:        row.EpiYear,
		EpiWeek:        row.EpiWeek,
		MetricValue:    row.MetricValue,
		SourceName:     sqlNullStringValue(row.SourceName),
		ImportedAt:     row.ImportedAt,
		CreatedAt:      row.CreatedAt,
	}
}

func toFacilityIndicatorMetricTrendResponse(row db.ListFacilityIndicatorMetricsTrendRow) FacilityWeeklyMetricResponse {
	return FacilityWeeklyMetricResponse{
		ID:             row.ID,
		SourceRecordID: nullStringPtr(row.SourceRecordID),
		FacilityID:     row.FacilityID,
		FacilityName:   row.FacilityName,
		SubCountyID:    nullUUIDPtr(row.SubCountyID),
		SubCountyName:  sqlNullStringValue(row.SubCountyName),
		DistrictID:     nullUUIDPtr(row.DistrictID),
		DistrictName:   sqlNullStringValue(row.DistrictName),
		RegionID:       nullUUIDPtr(row.RegionID),
		RegionName:     sqlNullStringValue(row.RegionName),
		IndicatorID:    nullUUIDPtr(row.IndicatorID),
		IndicatorName:  row.IndicatorName,
		EpiWeekID:      row.EpiWeekID,
		EpiYear:        row.EpiYear,
		EpiWeek:        row.EpiWeek,
		MetricValue:    row.MetricValue,
		SourceName:     sqlNullStringValue(row.SourceName),
		ImportedAt:     row.ImportedAt,
		CreatedAt:      row.CreatedAt,
	}
}

func toFacilityWeeklyDiseaseMetricResponses(items []db.ListFacilityWeeklyDiseaseMetricsByWeekRow) []FacilityWeeklyMetricResponse {
	out := make([]FacilityWeeklyMetricResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toFacilityWeeklyDiseaseMetricResponse(item))
	}
	return out
}

func toFacilityMetricByFacilityResponses(items []db.ListFacilityMetricsByFacilityRow) []FacilityWeeklyMetricResponse {
	out := make([]FacilityWeeklyMetricResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toFacilityMetricByFacilityResponse(item))
	}
	return out
}

func toFacilityDiseaseMetricTrendResponses(items []db.ListFacilityDiseaseMetricsTrendRow) []FacilityWeeklyMetricResponse {
	out := make([]FacilityWeeklyMetricResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toFacilityDiseaseMetricTrendResponse(item))
	}
	return out
}

func toFacilityIndicatorMetricTrendResponses(items []db.ListFacilityIndicatorMetricsTrendRow) []FacilityWeeklyMetricResponse {
	out := make([]FacilityWeeklyMetricResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toFacilityIndicatorMetricTrendResponse(item))
	}
	return out
}

func toDiseaseWeeklyTrendAggregateResponse(row db.ListDiseaseWeeklyTrendAggregatedRow) DiseaseWeeklyTrendAggregateResponse {
	return DiseaseWeeklyTrendAggregateResponse{
		EpiWeekID:  row.EpiWeekID,
		EpiYear:    row.EpiYear,
		EpiWeek:    row.EpiWeek,
		TotalCases: row.TotalCases,
	}
}

func toDiseaseWeeklyTrendAggregateResponses(items []db.ListDiseaseWeeklyTrendAggregatedRow) []DiseaseWeeklyTrendAggregateResponse {
	out := make([]DiseaseWeeklyTrendAggregateResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toDiseaseWeeklyTrendAggregateResponse(item))
	}
	return out
}

func toImportBatchResponse(item db.SurveillanceImportBatch) ImportBatchResponse {
	return ImportBatchResponse{
		ID:          item.ID,
		SourceName:  item.SourceName,
		FileName:    nullStringPtr(item.FileName),
		DatasetType: item.DatasetType,
		ImportedBy:  nullStringPtr(item.ImportedBy),
		ImportedAt:  item.ImportedAt,
		Status:      item.Status,
		Notes:       nullStringPtr(item.Notes),
		TotalRows:   item.TotalRows,
		SuccessRows: item.SuccessRows,
		FailedRows:  item.FailedRows,
		CompletedAt: nullTimePtr(item.CompletedAt),
		DocumentID:  nullUUIDPtr(item.DocumentID),
	}
}

func toImportBatchResponses(items []db.SurveillanceImportBatch) []ImportBatchResponse {
	out := make([]ImportBatchResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toImportBatchResponse(item))
	}
	return out
}

func toImportRawRowResponse(item db.SurveillanceImportRawRow) ImportRawRowResponse {
	return ImportRawRowResponse{
		ID:           item.ID,
		BatchID:      item.BatchID,
		RowNumber:    item.RowNumber,
		Payload:      item.Payload,
		CreatedAt:    item.CreatedAt,
		Status:       item.Status,
		ErrorMessage: nullStringPtr(item.ErrorMessage),
	}
}

func toImportRawRowResponses(items []db.SurveillanceImportRawRow) []ImportRawRowResponse {
	out := make([]ImportRawRowResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toImportRawRowResponse(item))
	}
	return out
}

func subjectNameForType(actual string, expected string, name string) string {
	if actual == expected {
		return name
	}
	return ""
}

func toFacilityWeeklyDiseaseMetricByWeekAndDiseaseResponses(
	rows []db.ListFacilityWeeklyDiseaseMetricsByWeekAndDiseaseRow,
) []FacilityWeeklyMetricResponse {
	out := make([]FacilityWeeklyMetricResponse, 0, len(rows))

	for _, item := range rows {
		out = append(out, toFacilityWeeklyDiseaseMetricByWeekAndDiseaseResponse(item))
	}
	return out
}
