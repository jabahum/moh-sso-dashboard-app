package users

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/moh-sso-dashboard/internal/accesswarnings"
	"github.com/moh-sso-dashboard/internal/keycloak"
	models "github.com/moh-sso-dashboard/internal/model"
	sharedservice "github.com/moh-sso-dashboard/internal/service"
)

type recordingNotifications struct {
	sharedservice.NotificationsService
	mu   sync.Mutex
	sent []models.Notification
}

func (r *recordingNotifications) Notify(_ context.Context, n models.Notification) (*models.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, n)
	return &n, nil
}

func (r *recordingNotifications) ofType(t models.NotificationType) []models.Notification {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Notification
	for _, n := range r.sent {
		if n.Type == string(t) {
			out = append(out, n)
		}
	}
	return out
}

type codedError struct{ code string }

func (e codedError) Error() string {
	return "identity-sync revocation for clims is incomplete (" + e.code + ")"
}
func (e codedError) WarningCode() string { return e.code }

// An incomplete identity-sync revocation after a portal role removal must not
// be only logged: the caller gets a warning and admins get a notification.
func TestIncompleteRevocationIsReportedToTheAdmin(t *testing.T) {
	for name, remove := range map[string]func(*Service, context.Context, uuid.UUID, uuid.UUID) error{
		"update client roles": func(s *Service, ctx context.Context, user, admin uuid.UUID) error {
			return s.UpdateUserClientRoles(ctx, user, "clims", "", nil, admin)
		},
		"remove client roles": func(s *Service, ctx context.Context, user, admin uuid.UUID) error {
			return s.RemoveUserClientRoles(ctx, user, "clims", "", []string{"clims_access"}, admin)
		},
	} {
		t.Run(name, func(t *testing.T) {
			notifications := &recordingNotifications{}
			repo := &clientRoleRepository{current: []keycloak.ClientRoleRep{{Name: "clims_access"}}}
			service := NewService(repo, notifications)
			service.SetAccessRoleRemovalHook(&recordingRemovalHook{err: codedError{code: "sessions_app_unavailable"}})
			ctx, collector := accesswarnings.WithCollector(context.Background())
			user := uuid.New()
			if err := remove(service, ctx, user, uuid.New()); err != nil {
				t.Fatalf("role removal must still succeed: %v", err)
			}
			warnings := collector.Warnings()
			if len(warnings) != 1 || warnings[0].ClientID != "clims" || warnings[0].UserID != user.String() ||
				warnings[0].Code != "sessions_app_unavailable" || !strings.Contains(warnings[0].Message, "incomplete") {
				t.Fatalf("warnings = %+v", warnings)
			}
			alerts := notifications.ofType(models.IdentitySyncRevocationIncomplete)
			if len(alerts) != 1 || alerts[0].TargetRole != "admin" || alerts[0].ClientID != "clims" || alerts[0].Severity != "critical" {
				t.Fatalf("admin notifications = %+v", notifications.sent)
			}
		})
	}
}

func TestRevocationWarningsAreNilSafe(t *testing.T) {
	// No collector in the context and no notifications service: nothing panics.
	repo := &clientRoleRepository{current: []keycloak.ClientRoleRep{{Name: "clims_access"}}}
	service := NewService(repo, nil)
	service.SetAccessRoleRemovalHook(&recordingRemovalHook{err: errors.New("incomplete")})
	if err := service.UpdateUserClientRoles(context.Background(), uuid.New(), "clims", "", nil, uuid.New()); err != nil {
		t.Fatal(err)
	}
	var nilCollector *accesswarnings.Collector
	if nilCollector.Warnings() != nil {
		t.Fatal("nil collector returned warnings")
	}
	// A plain error still produces a warning with the generic code.
	ctx, collector := accesswarnings.WithCollector(context.Background())
	if err := service.RemoveUserClientRoles(ctx, uuid.New(), "clims", "", []string{"clims_access"}, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if w := collector.Warnings(); len(w) != 1 || w[0].Code != accesswarnings.CodeRevocationIncomplete {
		t.Fatalf("warnings = %+v", w)
	}
}

type grantRecordingHook struct {
	recordingRemovalHook
	added []removalCall
	err   error
}

func (h *grantRecordingHook) ClientRolesAdded(_ context.Context, userID, clientID string, roles []string, actorID string) error {
	h.added = append(h.added, removalCall{userID: userID, clientID: clientID, actor: actorID, roles: roles})
	return h.err
}

// A portal grant tells the hook, after the grant succeeded, so an active
// identity-sync revocation can be restored explicitly by that admin.
func TestPortalGrantNotifiesTheRestoreHook(t *testing.T) {
	ctx := context.Background()
	user, admin := uuid.New(), uuid.New()
	hook := &grantRecordingHook{}
	repo := &clientRoleRepository{current: []keycloak.ClientRoleRep{{Name: "clims_viewer"}}}
	service := NewService(repo, nil)
	service.SetAccessRoleRemovalHook(hook)
	if err := service.UpdateUserClientRoles(ctx, user, "clims", "", []string{"clims_viewer", "clims_access"}, admin); err != nil {
		t.Fatal(err)
	}
	if err := service.AddUserClientRoles(ctx, user, "poesgo", "", []string{"poesgo_access"}, admin); err != nil {
		t.Fatal(err)
	}
	if len(hook.added) != 2 || hook.added[0].clientID != "clims" || strings.Join(hook.added[0].roles, ",") != "clims_access" ||
		hook.added[0].actor != admin.String() || hook.added[1].clientID != "poesgo" {
		t.Fatalf("grant hook calls = %+v", hook.added)
	}
	// A failing restore never fails the grant; it becomes a warning.
	hook.err = errors.New("database down")
	wctx, collector := accesswarnings.WithCollector(ctx)
	if err := service.AddUserClientRoles(wctx, user, "clims", "", []string{"clims_access"}, admin); err != nil {
		t.Fatal(err)
	}
	if w := collector.Warnings(); len(w) != 1 || w[0].Code != accesswarnings.CodeRestoreFailed {
		t.Fatalf("warnings = %+v", w)
	}
}
