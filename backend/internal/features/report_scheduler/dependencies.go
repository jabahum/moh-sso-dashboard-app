package report_scheduler

import (
	"context"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/model"
)

// EmailDelivery supplies the email operations used by scheduled reports.
type EmailDelivery interface {
	Queue(ctx context.Context, message model.Message) error
	ResolveGroupEmailRecipients(ctx context.Context, groupIDs, groupPaths []string) ([]model.Address, error)
}

// UserLookup resolves report recipients and schedule owners.
type UserLookup interface {
	GetUserByID(id uuid.UUID) (*model.User, error)
}
