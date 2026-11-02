package platform

import "context"

type Repository interface {
	SubmitNotification(context.Context, Principal, string, NotificationInput) (NotificationStatus, error)
	GetNotification(context.Context, string, string) (NotificationStatus, error)
	ResolveIdentity(context.Context, Token) (Principal, error)
	SaveIdentity(context.Context, string, string, string, IdentityInput) (Identity, error)
	UserAccess(context.Context, string, Token) (UserAccess, error)
}
