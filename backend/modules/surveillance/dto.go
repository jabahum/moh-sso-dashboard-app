package surveillance

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AlertListParams struct {
	EpiWeekID  uuid.UUID
	DiseaseID  uuid.UUID
	DistrictID uuid.UUID
	RegionID   uuid.UUID
}

type WeeklyStatusListInput struct {
	EpiWeekID   uuid.UUID
	RegionID    uuid.UUID
	DistrictID  uuid.UUID
	SubCountyID uuid.UUID
	DiseaseID   uuid.UUID
	IndicatorID uuid.UUID
	Status      string
}

type CreateImportBatchInput struct {
	SourceName  string
	FileName    *string
	DatasetType string
	ImportedBy  *string
	Status      string
	Notes       *string
	DocumentID  *uuid.UUID
}

type UpdateImportBatchStatusInput struct {
	ID     uuid.UUID
	Status string
	Notes  *string
}

type AlertResponse struct {
	ID           uuid.UUID  `json:"id"`
	ExternalID   *string    `json:"external_id,omitempty"`
	DiseaseID    uuid.UUID  `json:"disease_id"`
	DiseaseName  *string    `json:"disease_name,omitempty"`
	DistrictID   *uuid.UUID `json:"district_id,omitempty"`
	DistrictName *string    `json:"district_name,omitempty"`
	RegionID     *uuid.UUID `json:"region_id,omitempty"`
	EpiWeekID    *uuid.UUID `json:"epi_week_id,omitempty"`
	EpiYear      *int32     `json:"epi_year,omitempty"`
	EpiWeek      *int32     `json:"epi_week,omitempty"`
	OccurredOn   *time.Time `json:"occurred_on,omitempty"`
	CreatedOn    *time.Time `json:"created_on,omitempty"`
	Narrative    string     `json:"narrative"`
	SubmittedBy  *string    `json:"submitted_by,omitempty"`
	Status       string     `json:"status"`
	SourceName   *string    `json:"source_name,omitempty"`
	ImportedAt   time.Time  `json:"imported_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type WeeklyStatusDetailedResponse struct {
	ID            uuid.UUID  `json:"id"`
	RegionID      *uuid.UUID `json:"region_id,omitempty"`
	DistrictID    *uuid.UUID `json:"district_id,omitempty"`
	SubCountyID   *uuid.UUID `json:"sub_county_id,omitempty"`
	DiseaseID     *uuid.UUID `json:"disease_id,omitempty"`
	IndicatorID   *uuid.UUID `json:"indicator_id,omitempty"`
	EpiWeekID     uuid.UUID  `json:"epi_week_id"`
	Status        string     `json:"status"`
	SourceName    string     `json:"source_name"`
	ImportedAt    time.Time  `json:"imported_at"`
	CreatedAt     time.Time  `json:"created_at"`
	RegionName    string     `json:"region_name"`
	DistrictName  string     `json:"district_name"`
	SubCountyName string     `json:"sub_county_name"`
	DiseaseName   string     `json:"disease_name"`
	IndicatorName string     `json:"indicator_name"`
}

type WeeklyStatusResponse struct {
	ID          uuid.UUID  `json:"id"`
	RegionID    *uuid.UUID `json:"region_id,omitempty"`
	DistrictID  *uuid.UUID `json:"district_id,omitempty"`
	SubCountyID *uuid.UUID `json:"sub_county_id,omitempty"`
	DiseaseID   *uuid.UUID `json:"disease_id,omitempty"`
	IndicatorID *uuid.UUID `json:"indicator_id,omitempty"`
	EpiWeekID   uuid.UUID  `json:"epi_week_id"`
	Status      string     `json:"status"`
	SourceName  *string    `json:"source_name,omitempty"`
	ImportedAt  time.Time  `json:"imported_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type FacilityWeeklyMetricResponse struct {
	ID             uuid.UUID `json:"id"`
	SourceRecordID *string   `json:"source_record_id,omitempty"`

	FacilityID   uuid.UUID `json:"facility_id"`
	FacilityName string    `json:"facility_name"`

	SubCountyID   *uuid.UUID `json:"sub_county_id,omitempty"`
	SubCountyName string     `json:"sub_county_name"`

	DistrictID   *uuid.UUID `json:"district_id,omitempty"`
	DistrictName string     `json:"district_name"`

	RegionID   *uuid.UUID `json:"region_id,omitempty"`
	RegionName string     `json:"region_name"`

	DiseaseID     *uuid.UUID `json:"disease_id,omitempty"`
	DiseaseName   string     `json:"disease_name"`
	IndicatorID   *uuid.UUID `json:"indicator_id,omitempty"`
	IndicatorName string     `json:"indicator_name"`

	EpiWeekID uuid.UUID `json:"epi_week_id"`
	EpiYear   int32     `json:"epi_year"`
	EpiWeek   int32     `json:"epi_week"`

	MetricValue string    `json:"metric_value"`
	SourceName  string    `json:"source_name"`
	ImportedAt  time.Time `json:"imported_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type EpiWeekResponse struct {
	ID            uuid.UUID  `json:"id"`
	EpiYear       int32      `json:"epi_year"`
	EpiWeek       int32      `json:"epi_week"`
	WeekStartDate *time.Time `json:"week_start_date,omitempty"`
	WeekEndDate   *time.Time `json:"week_end_date,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type DiseaseResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	ShortName *string   `json:"short_name,omitempty"`
	Code      *string   `json:"code,omitempty"`
	Category  *string   `json:"category,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RegionResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Code      *string   `json:"code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DistrictResponse struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	RegionID   *uuid.UUID `json:"region_id,omitempty"`
	Code       *string    `json:"code,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	RegionName *string    `json:"region_name,omitempty"`
}

type SubCountyResponse struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	DistrictID uuid.UUID `json:"district_id"`
	Code       *string   `json:"code,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type UpsertDistrictRequest struct {
	Name     string     `json:"name" binding:"required"`
	RegionID *uuid.UUID `json:"region_id,omitempty"`
	Code     *string    `json:"code,omitempty"`
}

type UpsertSubCountyRequest struct {
	Name       string     `json:"name" binding:"required"`
	DistrictID *uuid.UUID `json:"district_id" binding:"required"`
	Code       *string    `json:"code,omitempty"`
}

type DeleteResponse struct {
	Deleted bool `json:"deleted"`
}

type DiseaseWeeklyTrendAggregateResponse struct {
	EpiWeekID  uuid.UUID `json:"epi_week_id"`
	EpiYear    int32     `json:"epi_year"`
	EpiWeek    int32     `json:"epi_week"`
	TotalCases int64     `json:"total_cases"`
}

type CreateImportBatchRequest struct {
	SourceName  string     `json:"source_name" binding:"required"`
	FileName    *string    `json:"file_name,omitempty"`
	DatasetType string     `json:"dataset_type" binding:"required"`
	ImportedBy  *string    `json:"imported_by,omitempty"`
	Status      string     `json:"status,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
	DocumentID  *uuid.UUID `json:"document_id,omitempty"`
}

type UpdateImportBatchStatusRequest struct {
	Status string  `json:"status" binding:"required"`
	Notes  *string `json:"notes,omitempty"`
}

type ImportBatchResponse struct {
	ID          uuid.UUID  `json:"id"`
	SourceName  string     `json:"source_name"`
	FileName    *string    `json:"file_name,omitempty"`
	DatasetType string     `json:"dataset_type"`
	ImportedBy  *string    `json:"imported_by,omitempty"`
	ImportedAt  time.Time  `json:"imported_at"`
	Status      string     `json:"status"`
	Notes       *string    `json:"notes,omitempty"`
	TotalRows   int32      `json:"total_rows"`
	SuccessRows int32      `json:"success_rows"`
	FailedRows  int32      `json:"failed_rows"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DocumentID  *uuid.UUID `json:"document_id,omitempty"`
}

type ImportRawRowResponse struct {
	ID           uuid.UUID       `json:"id"`
	BatchID      uuid.UUID       `json:"batch_id"`
	RowNumber    int32           `json:"row_number"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAt    time.Time       `json:"created_at"`
	Status       string          `json:"status"`
	ErrorMessage *string         `json:"error_message,omitempty"`
}
