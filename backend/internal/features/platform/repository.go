package platform

import "context"

type Repository interface {
	ResolveIdentity(context.Context, Token) (Principal, error)
	SaveIdentity(context.Context, string, string, string, IdentityInput) (Identity, error)
	UserAccess(context.Context, string, Token) (UserAccess, error)
}
