package bootstrap

import (
	"context"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

// moduleAudit adapts the audit interfaces of the currently published modules.
type moduleAudit struct{ recorder core.AuditRecorder }

func (a moduleAudit) Log(ctx context.Context, actor uuid.NullUUID, action string, metadata map[string]any) error {
	event := core.AuditEvent{Action: action, Metadata: metadata}
	if actor.Valid {
		event.Subject = actor.UUID.String()
	}
	return a.recorder.RecordAudit(ctx, event)
}
