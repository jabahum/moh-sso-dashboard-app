export type HealthContext = {
  level?: "national" | "district" | "facility" | string;
  district?: string;
  facility?: string;
};

export type ReportSchedulerModule = {
  name: string;
  status: "ready";
  schedulingEnabled: boolean;
  healthBiEnabled: boolean;
  healthContext: HealthContext;
};

export type HealthBIReport = {
  id: string;
  name: string;
  description?: string;
  category?: string;
  supportedFormats?: string[];
};

export type HealthBIParameter = {
  name: string;
  label?: string;
  type: string;
  required: boolean;
  description?: string;
  options?: unknown[];
};

export type ScheduleRecipient = {
  id?: string;
  type: "user" | "group" | "email";
  value: string;
  deliveryChannel: "email" | "portal";
};

export type OutputConfig = {
  format: string;
  deliveryMode?: string;
  fileNamePrefix?: string;
};

export type ScheduleTiming = {
  timeOfDay: string;
  weekday?: number;
  dayOfMonth?: number;
};

export type ReportSchedule = {
  id: string;
  healthBiReportId: string;
  reportName: string;
  description?: string;
  frequency: string;
  cronExpression?: string;
  timezone: string;
  periodStrategy: string;
  parameters: Record<string, unknown>;
  outputFormat: string;
  outputConfig: OutputConfig;
  timing: ScheduleTiming;
  healthContext: HealthContext;
  recipients: ScheduleRecipient[];
  enabled: boolean;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  lastRunAt?: string;
  nextRunAt?: string;
};

export type CreateScheduleRequest = {
  healthBiReportId: string;
  reportName: string;
  description?: string;
  frequency: string;
  cronExpression?: string;
  timezone: string;
  periodStrategy: string;
  parameters: Record<string, unknown>;
  outputFormat: string;
  outputConfig: OutputConfig;
  timing: ScheduleTiming;
  healthContext: HealthContext;
  recipients: ScheduleRecipient[];
  enabled: boolean;
};

export type RecipientPreview = {
  emailRecipients: string[];
  portalRecipients: string[];
  emailCount: number;
  portalCount: number;
};

export type PortalReport = {
  deliveryId: string;
  executionId: string;
  reportId: string;
  reportName: string;
  deliveredAt: string;
  artifact: {
    id: string;
    fileName: string;
    contentType?: string;
    externalUrl?: string;
  };
};

export type ReportExecution = {
  id: string;
  scheduleId?: string;
  healthBiJobId?: string;
  reportId: string;
  status: string;
  outputFormat: string;
  triggerType: "scheduled" | "manual" | string;
  scheduledFor?: string;
  generationAttempts: number;
  maxGenerationAttempts: number;
  nextRetryAt?: string;
  lastAttemptAt?: string;
  errorMessage?: string;
  resolvedPeriod?: {
    strategy: string;
    start: string;
    end: string;
  };
  createdAt: string;
};

export type ReportDelivery = {
  id: string;
  executionId: string;
  artifactId?: string;
  recipientType: string;
  recipientValue: string;
  deliveryChannel: string;
  status: string;
  attempts: number;
  maxAttempts: number;
  lastError?: string;
  nextRetryAt?: string;
  lastAttemptAt?: string;
  sentAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type ExecutionDetail = {
  execution: ReportExecution;
  schedule?: ReportSchedule;
  artifacts: Array<{
    id: string;
    executionId: string;
    fileName: string;
    contentType?: string;
    externalUrl?: string;
    sizeBytes?: number;
    createdAt: string;
  }>;
  deliveries: ReportDelivery[];
};
