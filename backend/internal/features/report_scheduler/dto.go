package report_scheduler

import "time"

type ModuleResponse struct {
	Name              string `json:"name"`
	Status            string `json:"status"`
	SchedulingEnabled bool   `json:"schedulingEnabled"`
	HealthBIEnabled   bool   `json:"healthBiEnabled"`
	HealthContext     HealthContext `json:"healthContext"`
}

type HealthBIReport struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Description      string         `json:"description,omitempty"`
	Category         string         `json:"category,omitempty"`
	SupportedFormats []string       `json:"supportedFormats,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type HealthBIParameter struct {
	Name        string         `json:"name"`
	Label       string         `json:"label,omitempty"`
	Type        string         `json:"type"`
	Required    bool           `json:"required"`
	Description string         `json:"description,omitempty"`
	Options     []any          `json:"options,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type HealthContext struct {
	Level    string `json:"level"`
	District string `json:"district,omitempty"`
	Facility string `json:"facility,omitempty"`
}

type ScheduleRecipient struct {
	ID              string `json:"id,omitempty"`
	Type            string `json:"type" binding:"required"`
	Value           string `json:"value" binding:"required"`
	DeliveryChannel string `json:"deliveryChannel" binding:"required"`
}

type RecipientPreview struct {
	EmailRecipients  []string `json:"emailRecipients"`
	PortalRecipients []string `json:"portalRecipients"`
	EmailCount       int      `json:"emailCount"`
	PortalCount      int      `json:"portalCount"`
}

type OutputConfig struct {
	Format         string `json:"format"`
	DeliveryMode   string `json:"deliveryMode,omitempty"`
	FileNamePrefix string `json:"fileNamePrefix,omitempty"`
}

type GenerateReportRequest struct {
	Parameters map[string]any `json:"parameters,omitempty"`
	Format     string         `json:"format" binding:"required"`
}

type HealthBIJob struct {
	ID          string         `json:"id"`
	ReportID    string         `json:"reportId,omitempty"`
	Status      string         `json:"status"`
	ArtifactURL string         `json:"artifactUrl,omitempty"`
	Error       string         `json:"error,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Schedule struct {
	ID               string         `json:"id"`
	HealthBIReportID string         `json:"healthBiReportId"`
	ReportName       string         `json:"reportName"`
	Description      string         `json:"description,omitempty"`
	Frequency        string         `json:"frequency"`
	CronExpression   string         `json:"cronExpression,omitempty"`
	Timezone         string         `json:"timezone"`
	PeriodStrategy   string         `json:"periodStrategy"`
	Parameters       map[string]any `json:"parameters"`
	OutputFormat     string         `json:"outputFormat"`
	OutputConfig     OutputConfig   `json:"outputConfig"`
	HealthContext    HealthContext  `json:"healthContext"`
	Recipients       []ScheduleRecipient `json:"recipients"`
	Enabled          bool           `json:"enabled"`
	CreatedBy        string         `json:"createdBy"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	LastRunAt        *time.Time     `json:"lastRunAt,omitempty"`
	NextRunAt        *time.Time     `json:"nextRunAt,omitempty"`
}

type CreateScheduleRequest struct {
	HealthBIReportID string         `json:"healthBiReportId" binding:"required"`
	ReportName       string         `json:"reportName" binding:"required"`
	Description      string         `json:"description"`
	Frequency        string         `json:"frequency" binding:"required"`
	CronExpression   string         `json:"cronExpression"`
	Timezone         string         `json:"timezone" binding:"required"`
	PeriodStrategy   string         `json:"periodStrategy" binding:"required"`
	Parameters       map[string]any `json:"parameters"`
	OutputFormat     string         `json:"outputFormat" binding:"required"`
	OutputConfig     OutputConfig   `json:"outputConfig"`
	HealthContext    HealthContext  `json:"healthContext"`
	Recipients       []ScheduleRecipient `json:"recipients"`
	Enabled          *bool          `json:"enabled"`
}

type UpdateScheduleRequest = CreateScheduleRequest

type Execution struct {
	ID            string         `json:"id"`
	ScheduleID    *string        `json:"scheduleId,omitempty"`
	HealthBIJobID string         `json:"healthBiJobId,omitempty"`
	ReportID      string         `json:"reportId"`
	Status        string         `json:"status"`
	Parameters    map[string]any `json:"parameters"`
	OutputFormat  string         `json:"outputFormat"`
	TriggerType   string         `json:"triggerType"`
	TriggeredBy   string         `json:"triggeredBy,omitempty"`
	ErrorMessage  string         `json:"errorMessage,omitempty"`
	StartedAt     *time.Time     `json:"startedAt,omitempty"`
	FinishedAt    *time.Time     `json:"finishedAt,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
}

type Artifact struct {
	ID          string    `json:"id"`
	ExecutionID string    `json:"executionId"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType,omitempty"`
	ObjectKey   string    `json:"objectKey,omitempty"`
	ExternalURL string    `json:"externalUrl,omitempty"`
	SizeBytes   int64     `json:"sizeBytes,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type PortalReport struct {
	DeliveryID string    `json:"deliveryId"`
	ExecutionID string   `json:"executionId"`
	ReportID    string   `json:"reportId"`
	ReportName  string   `json:"reportName"`
	Artifact    Artifact  `json:"artifact"`
	DeliveredAt time.Time `json:"deliveredAt"`
}
