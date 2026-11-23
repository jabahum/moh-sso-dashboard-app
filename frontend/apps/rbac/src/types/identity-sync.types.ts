// Identity-sync admin API (backend/internal/features/platform/identity_sync*.go).

export type IdentitySyncAccountIdFormat = "uuid" | "positive-integer" | "opaque";

export type IdentitySyncIntegration = {
  applicationId: string;
  origin: string;
  syncClientId: string;
  accessRole: string;
  credentialRef: string;
  accountIdFormat: IdentitySyncAccountIdFormat;
  contractVersion: string;
  enabled: boolean;
  updatedBy: string;
  createdAt: string;
  updatedAt: string;
};

export type IdentitySyncIntegrationPayload = {
  origin: string;
  syncClientId: string;
  accessRole: string;
  credentialRef: string;
  accountIdFormat: IdentitySyncAccountIdFormat;
  enabled: boolean;
  reason: string;
  allowContractDowngrade?: boolean;
};

export type IdentitySyncDescriptor = {
  contract: string;
  version: string;
  application: string;
  accountIdFormat: string;
  maxPageSize: number;
  capabilities: string[];
};

export type IdentitySyncProposal =
  | "reuse"
  | "create"
  | "review"
  | "linked"
  | "linked_no_access"
  | "revoked";

export type IdentitySyncRunStatus = "previewed" | "applying" | "completed" | "failed" | "cancelled";

export type IdentitySyncRun = {
  id: string;
  applicationId: string;
  issuer: string;
  status: IdentitySyncRunStatus;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
};

export type IdentitySyncRow = {
  id: string;
  runId: string;
  accountId: string;
  emailHint: string;
  proposal: IdentitySyncProposal;
  proposedSubject?: string;
  decision: "pending" | "approved" | "rejected";
  reviewedBy?: string;
  reviewedAt?: string;
  reviewNote?: string;
  userStatus: string;
  linkStatus: string;
  grantStatus: string;
  attempts: number;
  lastError?: string;
  updatedAt: string;
};

export type IdentitySyncRunDetail = {
  run: IdentitySyncRun;
  counts: Record<string, number>;
  rows: IdentitySyncRow[];
};

export type IdentitySyncJobKind = "start_run" | "apply_run" | "resume_run" | "resume_revocations";

export type IdentitySyncJob = {
  id: string;
  applicationId: string;
  kind: IdentitySyncJobKind;
  runId?: string;
  status: "running" | "succeeded" | "failed";
  actor: string;
  error?: string;
  errorMessage?: string;
  summary?: Record<string, number>;
  startedAt: string;
  finishedAt?: string;
};

export type IdentitySyncStepStatus = "pending" | "done" | "failed" | "unsupported";

export type IdentitySyncRevocation = {
  id: string;
  applicationId: string;
  issuer: string;
  subject: string;
  revokedBy: string;
  reason: string;
  revokedAt: string;
  restoredBy?: string;
  restoreReason?: string;
  restoredAt?: string;
  roleRemoved: IdentitySyncStepStatus;
  consentsRevoked: IdentitySyncStepStatus;
  sessionsRevoked: IdentitySyncStepStatus;
  attempts: number;
  lastError?: string;
  updatedAt: string;
  complete: boolean;
  sessionsStillOpen: boolean;
};

/** A registered service identity (ihp_application_identities) of an application. */
export type ServiceIdentity = {
  applicationId: string;
  clientId: string;
  subject: string;
  scopes: string[];
  enabled: boolean;
};
