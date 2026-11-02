package platform

import (
	"context"
	"time"
)

type Repository interface {
	Operations(context.Context, string) (ApplicationOperations, error)
	WorkerHealth(context.Context, time.Duration) ([]WorkerHealth, error)
	SubmitNotification(context.Context, Principal, string, NotificationInput) (NotificationStatus, error)
	GetNotification(context.Context, string, string) (NotificationStatus, error)
	ResolveIdentity(context.Context, Token) (Principal, error)
	SaveIdentity(context.Context, string, string, string, IdentityInput) (Identity, error)
	UserAccess(context.Context, string, Token) (UserAccess, error)
}
