package rbac

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/moh-sso-dashboard/internal/accesswarnings"
	"github.com/moh-sso-dashboard/internal/model"
)

// warningUserLookup reports an incomplete identity-sync revocation the way the
// users service does when UpdateUserClientRoles removes an access role.
type warningUserLookup struct{ testUserLookup }

func (l *warningUserLookup) UpdateUserClientRoles(ctx context.Context, userID uuid.UUID, clientID string, _ string, _ []string, _ uuid.UUID) error {
	accesswarnings.Add(ctx, accesswarnings.Warning{Code: accesswarnings.CodeRevocationIncomplete, ClientID: clientID, UserID: userID.String(), Message: "incomplete"})
	return nil
}

func TestUpdateUserAccessReturnsRevocationWarnings(t *testing.T) {
	user := uuid.New()
	repo := newTestRBACRepository() // one assignable system: data-statistics
	users := &warningUserLookup{testUserLookup{user: &model.User{ID: user.String(), Username: "person"}}}
	service := NewService(repo, users)
	result, err := service.UpdateUserAccess(context.Background(), user.String(), UpdateUserAccessRequest{}, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].ClientID != "data-statistics" || result.Warnings[0].Code != accesswarnings.CodeRevocationIncomplete {
		t.Fatalf("warnings = %+v", result.Warnings)
	}
}
