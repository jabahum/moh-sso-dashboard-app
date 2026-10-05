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

type ScheduleTiming struct {
	TimeOfDay  string `json:"timeOfDay"`
	Weekday    int    `json:"weekday,omitempty"`
	DayOfMonth int    `json:"dayOfMonth,omitempty"`
}

type ResolvedPeriod struct {
	Strategy string    `json:"strategy"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
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
	Timing           ScheduleTiming `json:"timing"`
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
	Timing           ScheduleTiming `json:"timing"`
	HealthContext    HealthContext  `json:"healthContext"`
	Recipients       []ScheduleRecipient `json:"recipients"`
	Enabled          *bool          `json:"enabled"`
}

type UpdateScheduleRequest = CreateScheduleRequest

type ListOptions struct {
	Limit   int
	Status  string
	Search  string
	Enabled *bool
}

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
	ScheduledFor  *time.Time     `json:"scheduledFor,omitempty"`
	ResolvedPeriod *ResolvedPeriod `json:"resolvedPeriod,omitempty"`
	GenerationAttempts int         `json:"generationAttempts"`
	MaxGenerationAttempts int      `json:"maxGenerationAttempts"`
	NextRetryAt    *time.Time      `json:"nextRetryAt,omitempty"`
	LastAttemptAt  *time.Time      `json:"lastAttemptAt,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
}

type Delivery struct {
	ID              string     `json:"id"`
	ExecutionID     string     `json:"executionId"`
	ArtifactID      string     `json:"artifactId,omitempty"`
	RecipientType   string     `json:"recipientType"`
	RecipientValue  string     `json:"recipientValue"`
	DeliveryChannel string     `json:"deliveryChannel"`
	Status          string     `json:"status"`
	Attempts        int        `json:"attempts"`
	MaxAttempts     int        `json:"maxAttempts"`
	LastError       string     `json:"lastError,omitempty"`
	NextRetryAt     *time.Time `json:"nextRetryAt,omitempty"`
	LastAttemptAt   *time.Time `json:"lastAttemptAt,omitempty"`
	SentAt          *time.Time `json:"sentAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type ExecutionDetail struct {
	Execution  Execution  `json:"execution"`
	Schedule   *Schedule  `json:"schedule,omitempty"`
	Artifacts  []Artifact `json:"artifacts"`
	Deliveries []Delivery `json:"deliveries"`
}

type Artifact struct {
	ID          string    `json:"id"`
	ExecutionID string    `json:"executionId"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType,omitempty"`
	ObjectKey   string    `json:"-"`
	ExternalURL string    `json:"-"`
	DownloadURL string    `json:"downloadUrl,omitempty"`
	SizeBytes   int64     `json:"sizeBytes,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type SchedulerStatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type SchedulerOverview struct {
	TotalSchedules       int64                  `json:"totalSchedules"`
	EnabledSchedules     int64                  `json:"enabledSchedules"`
	Executions24h        int64                  `json:"executions24h"`
	Completed24h         int64                  `json:"completed24h"`
	Failed24h            int64                  `json:"failed24h"`
	RetryingNow          int64                  `json:"retryingNow"`
	DeliveryFailures24h  int64                  `json:"deliveryFailures24h"`
	SuccessRate24h       float64                `json:"successRate24h"`
	ExecutionStatuses    []SchedulerStatusCount `json:"executionStatuses"`
	DeliveryStatuses     []SchedulerStatusCount `json:"deliveryStatuses"`
	RecentFailures       []Execution            `json:"recentFailures"`
	WorkerLastHeartbeatAt *time.Time             `json:"workerLastHeartbeatAt,omitempty"`
	WorkerLastCycleError  string                 `json:"workerLastCycleError,omitempty"`
	WorkerHealthy         bool                   `json:"workerHealthy"`
	GeneratedAt          time.Time              `json:"generatedAt"`
}

type PortalReport struct {
	DeliveryID string    `json:"deliveryId"`
	ExecutionID string   `json:"executionId"`
	ReportID    string   `json:"reportId"`
	ReportName  string   `json:"reportName"`
	Artifact    Artifact  `json:"artifact"`
	DeliveredAt time.Time `json:"deliveredAt"`
}
