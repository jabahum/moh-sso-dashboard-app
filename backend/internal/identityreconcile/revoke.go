package identityreconcile

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/identitysync"
)

// Access removal (Track C). Removing a person's access to one application runs
// these steps synchronously, so existing sessions end immediately:
//
//  1. record an active revocation (ihp_identity_sync_revocations);
//  2. remove the application's access role from the Keycloak user;
//  3. revoke the user's consents and offline tokens for the application's
//     interactive client only (never a global logout);
//  4. ask the application to end its local sessions (POST /sessions/revoke).
//
// A failing step never stops the others: each step's status is recorded and
// Resume retries pending and failed steps. A 1.0 application cannot end its
// sessions; that step is recorded as unsupported and reported, never hidden.

// StepStatus is the status of one removal step.
type StepStatus string

const (
	StepPending     StepStatus = "pending"
	StepDone        StepStatus = "done"
	StepFailed      StepStatus = "failed"
	StepUnsupported StepStatus = "unsupported"
)

// needsRun reports whether a step should be (re)tried.
func (s StepStatus) needsRun() bool { return s == StepPending || s == StepFailed || s == "" }

// Stable error codes stored in last_error. They never contain upstream bodies.
const (
	CodeRoleRemovalFailed       = "role_removal_failed"
	CodeAccessRoleInherited     = "access_role_inherited"
	CodeConsentRevocationFailed = "consent_revocation_failed"
	CodeIntegrationDisabled     = "integration_disabled"
	CodeConnectorUnavailable    = "connector_unavailable"
	CodeSessionsAppUnavailable  = "sessions_app_unavailable"
	CodeSessionsAppRejected     = "sessions_app_rejected"
)

// Revocation errors.
var (
	ErrRevocationNotFound = errors.New("no active identity-sync revocation")
	ErrInvalidRevocation  = errors.New("invalid identity-sync revocation")
	// ErrAccessRoleInherited reports that the person still holds the access role
	// through a group or composite role after the direct mapping was removed.
	ErrAccessRoleInherited = errors.New("access role is still granted through a group or composite role")
)

// Revocation is one row of ihp_identity_sync_revocations.
type Revocation struct {
	ID                  string     `json:"id"`
	ApplicationClientID string     `json:"applicationId"`
	Issuer              string     `json:"issuer"`
	Subject             string     `json:"subject"`
	RevokedBy           string     `json:"revokedBy"`
	Reason              string     `json:"reason"`
	RevokedAt           time.Time  `json:"revokedAt"`
	RestoredBy          string     `json:"restoredBy,omitempty"`
	RestoreReason       string     `json:"restoreReason,omitempty"`
	RestoredAt          *time.Time `json:"restoredAt,omitempty"`
	RoleRemoved         StepStatus `json:"roleRemoved"`
	ConsentsRevoked     StepStatus `json:"consentsRevoked"`
	SessionsRevoked     StepStatus `json:"sessionsRevoked"`
	Attempts            int        `json:"attempts"`
	LastError           string     `json:"lastError,omitempty"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// Active reports whether the revocation has not been restored.
func (r Revocation) Active() bool { return r.RestoredAt == nil }

// Complete reports whether no step is pending or failed. A complete revocation
// may still have SessionsRevoked=unsupported; see SessionsStillOpen.
func (r Revocation) Complete() bool {
	return !r.RoleRemoved.needsRun() && !r.ConsentsRevoked.needsRun() && !r.SessionsRevoked.needsRun()
}

// SessionsStillOpen reports that existing application sessions were not ended:
// the step failed, is pending, or the application cannot end sessions (1.0).
func (r Revocation) SessionsStillOpen() bool { return r.SessionsRevoked != StepDone }

// RevocationFilter selects revocations to list.
type RevocationFilter struct {
	Application     string // empty means every application
	IncludeRestored bool
	OnlyUnfinished  bool // active rows with a pending or failed step
	Limit           int
}

// RevocationRepository persists revocations. Every write records an audit event
// in the same transaction.
type RevocationRepository interface {
	// BeginRevocation inserts an active revocation, or returns the existing active
	// one for the same application, issuer and subject (created=false).
	BeginRevocation(ctx context.Context, revocation Revocation) (Revocation, bool, error)
	ActiveRevocation(ctx context.Context, application, issuer, subject string) (Revocation, error)
	// SaveRevocationAttempt stores the step statuses, attempts and last error of
	// an active revocation. It returns ErrRevocationNotFound if it was restored.
	SaveRevocationAttempt(ctx context.Context, revocation Revocation, actor string) error
	RestoreRevocation(ctx context.Context, application, issuer, subject, actor, reason string) (Revocation, error)
	ListRevocations(ctx context.Context, filter RevocationFilter) ([]Revocation, error)
}

// AccessDirectory is the Keycloak side of access removal. Every method is
// idempotent, and a user that no longer exists counts as already removed.
type AccessDirectory interface {
	// RemoveAccessRole removes the direct client-role mapping. It returns
	// ErrAccessRoleInherited when the user still holds the role afterwards.
	RemoveAccessRole(ctx context.Context, subject, clientID, role string) error
	// RevokeClientConsents revokes consents and offline tokens for one client
	// only. It must not end the user's SSO session or other clients' sessions.
	RevokeClientConsents(ctx context.Context, subject, clientID string) error
	// HasAccessRole reports whether the user effectively holds the role
	// (directly, by group or by composite).
	HasAccessRole(ctx context.Context, subject, clientID, role string) (bool, error)
}

// IntegrationLookup reads registered integrations. *RegistryService implements it.
type IntegrationLookup interface {
	Get(ctx context.Context, application string) (Integration, error)
}

// SessionRevokers returns the session revoker (the contract connector) for an
// enabled application.
type SessionRevokers func(ctx context.Context, application string) (identitysync.SessionRevoker, error)

// RegistrySessionRevokers builds session revokers from Track A's registry, which
// re-checks the sync client and access-role bindings each time.
func RegistrySessionRevokers(registry *RegistryService) SessionRevokers {
	return func(ctx context.Context, application string) (identitysync.SessionRevoker, error) {
		connector, _, err := registry.Connector(ctx, application)
		if err != nil {
			return nil, err
		}
		return connector, nil
	}
}

// RevokeService removes people's access to registered applications.
type RevokeService struct {
	Integrations IntegrationLookup
	Sessions     SessionRevokers
	Directory    AccessDirectory
	Repository   RevocationRepository
	// Issuer is the public issuer written into application links
	// (KEYCLOAK_EXTERNAL_URL/realms/<realm>).
	Issuer string
	// Locker serialises revocation with grants (see access_lock.go). Nil uses
	// the repository when it implements AccessLocker (PostgresRevocations does).
	Locker AccessLocker
}

var (
	_ RevocationChecker  = (*RevokeService)(nil)
	_ AccessLocker       = (*RevokeService)(nil)
	_ RevocationEnforcer = (*RevokeService)(nil)
)

func (s *RevokeService) locker() AccessLocker {
	if s == nil {
		return nil
	}
	if s.Locker != nil {
		return s.Locker
	}
	if locker, ok := s.Repository.(AccessLocker); ok {
		return locker
	}
	return nil
}

// WithAccessLock runs fn holding the (application, subject) access lock that
// grants and revocations share. Batch and single apply use it, through the
// RevocationChecker they are given, around "check revoked -> grant".
func (s *RevokeService) WithAccessLock(ctx context.Context, application, subject string, fn func(context.Context) error) error {
	return withAccessLock(ctx, s.locker(), application, subject, fn)
}

// NewRevokeService wires a revoke service to Track A's registry.
func NewRevokeService(registry *RegistryService, repository RevocationRepository, directory AccessDirectory, issuer string) *RevokeService {
	return &RevokeService{
		Integrations: registry,
		Sessions:     RegistrySessionRevokers(registry),
		Directory:    directory,
		Repository:   repository,
		Issuer:       strings.TrimRight(issuer, "/"),
	}
}

func (s *RevokeService) configured() error {
	if s == nil || s.Integrations == nil || s.Sessions == nil || s.Directory == nil || s.Repository == nil || s.Issuer == "" {
		return errors.New("identity-sync revocation is not configured")
	}
	return nil
}

func validText(value string, max int) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && len(trimmed) <= max
}

// Revoke removes subject's access to application and ends its sessions now. It
// is idempotent: repeating it for an active revocation retries only the steps
// that are not finished. The returned revocation shows each step's status; an
// error is returned only when nothing could be recorded.
func (s *RevokeService) Revoke(ctx context.Context, application, subject, revokedBy, reason string) (Revocation, error) {
	if err := s.configured(); err != nil {
		return Revocation{}, err
	}
	identity := identitysync.Identity{Issuer: s.Issuer, Subject: subject}
	switch {
	case !applicationIDPattern.MatchString(application):
		return Revocation{}, fmt.Errorf("%w: application", ErrInvalidRevocation)
	case !identitysync.ValidIdentity(identity, s.Issuer):
		return Revocation{}, fmt.Errorf("%w: subject must be a Keycloak user ID", ErrInvalidRevocation)
	case !validText(revokedBy, 255):
		return Revocation{}, fmt.Errorf("%w: revoked-by is required", ErrInvalidRevocation)
	case !validText(reason, 2000):
		return Revocation{}, fmt.Errorf("%w: a reason is required", ErrInvalidRevocation)
	}
	integration, err := s.Integrations.Get(ctx, application)
	if err != nil {
		return Revocation{}, err
	}
	var revocation Revocation
	var codes []string
	// Record the revocation and remove the role under the access lock, so a
	// grant that already passed its revocation check finishes first (and its
	// role is then removed here), and any later grant sees this revocation.
	err = s.WithAccessLock(ctx, application, subject, func(ctx context.Context) error {
		var created bool
		var err error
		revocation, created, err = s.Repository.BeginRevocation(ctx, Revocation{
			ApplicationClientID: application, Issuer: s.Issuer, Subject: subject,
			RevokedBy: strings.TrimSpace(revokedBy), Reason: strings.TrimSpace(reason),
			RoleRemoved: StepPending, ConsentsRevoked: StepPending, SessionsRevoked: StepPending,
		})
		if err != nil {
			return err
		}
		if !created {
			// A repeated revoke re-asserts every step: the role may have been granted
			// again by hand, or a 1.0 application may since have added sessions.revoke.
			// Every step is idempotent, and the existing row is reused.
			revocation.RoleRemoved, revocation.ConsentsRevoked, revocation.SessionsRevoked = StepPending, StepPending, StepPending
		}
		s.removeRoleStep(ctx, &revocation, integration, &codes)
		return nil
	})
	if err != nil {
		return Revocation{}, err
	}
	return s.finishSteps(ctx, revocation, integration, strings.TrimSpace(revokedBy), codes)
}

// EnforceRevocation is called by apply right after it granted the access role.
// If an active revocation exists for the identity (it was recorded by a path
// that did not take the access lock), the role removal is recorded as pending
// again, so resume handles it even if this process stops, and then, when the
// service has a directory, every removal step runs at once. It returns
// enforced=true when a revocation exists; err reports a step that could not
// finish (the pending or failed status is stored either way).
func (s *RevokeService) EnforceRevocation(ctx context.Context, application, issuer, subject, actor string) (bool, error) {
	if s == nil || s.Repository == nil {
		return false, errors.New("identity-sync revocation is not configured")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "identity-sync"
	}
	enforced := false
	var revocation Revocation
	var integration Integration
	var codes []string
	full := s.configured() == nil
	err := s.WithAccessLock(ctx, application, subject, func(ctx context.Context) error {
		active, err := s.Repository.ActiveRevocation(ctx, application, issuer, subject)
		if errors.Is(err, ErrRevocationNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("check revocation: %w", err)
		}
		enforced = true
		revocation = active
		revocation.RoleRemoved, revocation.ConsentsRevoked, revocation.SessionsRevoked = StepPending, StepPending, StepPending
		if err := s.Repository.SaveRevocationAttempt(ctx, revocation, actor); err != nil {
			return err
		}
		if !full {
			return nil
		}
		if integration, err = s.Integrations.Get(ctx, application); err != nil {
			return err
		}
		s.removeRoleStep(ctx, &revocation, integration, &codes)
		return nil
	})
	if err != nil || !enforced {
		return enforced, err
	}
	if !full {
		return true, fmt.Errorf("identity-sync revocation %s for %s: role removal recorded as pending; run 'identity-sync revocations --resume'", revocation.ID, application)
	}
	revocation, err = s.finishSteps(ctx, revocation, integration, actor, codes)
	if err != nil {
		return true, err
	}
	if !revocation.Complete() {
		return true, fmt.Errorf("identity-sync revocation %s for %s is incomplete (%s); run 'identity-sync revocations --resume'", revocation.ID, application, revocation.LastError)
	}
	return true, nil
}

// Resume retries the pending and failed steps of active revocations (of one
// application, or of all when application is empty). Unsupported steps are not
// retried. It returns every revocation it processed with its new status.
func (s *RevokeService) Resume(ctx context.Context, application, actor string) ([]Revocation, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	if application != "" && !applicationIDPattern.MatchString(application) {
		return nil, fmt.Errorf("%w: application", ErrInvalidRevocation)
	}
	if !validText(actor, 255) {
		return nil, fmt.Errorf("%w: actor is required", ErrInvalidRevocation)
	}
	pending, err := s.Repository.ListRevocations(ctx, RevocationFilter{Application: application, OnlyUnfinished: true, Limit: 500})
	if err != nil {
		return nil, err
	}
	integrations := map[string]Integration{}
	results := make([]Revocation, 0, len(pending))
	for _, revocation := range pending {
		integration, ok := integrations[revocation.ApplicationClientID]
		if !ok {
			integration, err = s.Integrations.Get(ctx, revocation.ApplicationClientID)
			if err != nil {
				return results, err
			}
			integrations[revocation.ApplicationClientID] = integration
		}
		updated, err := s.runSteps(ctx, revocation, integration, strings.TrimSpace(actor))
		if errors.Is(err, ErrRevocationNotFound) {
			continue // restored meanwhile
		}
		if err != nil {
			return results, err
		}
		results = append(results, updated)
	}
	return results, nil
}

// Restore ends the active revocation so later syncs may grant access again. It
// does not grant the access role.
func (s *RevokeService) Restore(ctx context.Context, application, subject, restoredBy, reason string) (Revocation, error) {
	if err := s.configured(); err != nil {
		return Revocation{}, err
	}
	switch {
	case !applicationIDPattern.MatchString(application):
		return Revocation{}, fmt.Errorf("%w: application", ErrInvalidRevocation)
	case !identitysync.ValidIdentity(identitysync.Identity{Issuer: s.Issuer, Subject: subject}, s.Issuer):
		return Revocation{}, fmt.Errorf("%w: subject must be a Keycloak user ID", ErrInvalidRevocation)
	case !validText(restoredBy, 255):
		return Revocation{}, fmt.Errorf("%w: restored-by is required", ErrInvalidRevocation)
	case !validText(reason, 2000):
		return Revocation{}, fmt.Errorf("%w: a reason is required", ErrInvalidRevocation)
	}
	return s.Repository.RestoreRevocation(ctx, application, s.Issuer, subject, strings.TrimSpace(restoredBy), strings.TrimSpace(reason))
}

// IsRevoked reports whether issuer/subject has an active revocation for
// application. Track B uses it (RevocationChecker) to propose "revoked" and to
// refuse re-granting access.
func (s *RevokeService) IsRevoked(ctx context.Context, application, issuer, subject string) (bool, error) {
	if s == nil || s.Repository == nil {
		return false, errors.New("identity-sync revocation is not configured")
	}
	_, err := s.Repository.ActiveRevocation(ctx, application, issuer, subject)
	if errors.Is(err, ErrRevocationNotFound) {
		return false, nil
	}
	return err == nil, err
}

// List returns revocations matching filter.
func (s *RevokeService) List(ctx context.Context, filter RevocationFilter) ([]Revocation, error) {
	if s == nil || s.Repository == nil {
		return nil, errors.New("identity-sync revocation is not configured")
	}
	if filter.Application != "" && !applicationIDPattern.MatchString(filter.Application) {
		return nil, fmt.Errorf("%w: application", ErrInvalidRevocation)
	}
	return s.Repository.ListRevocations(ctx, filter)
}

// runSteps runs every unfinished step, continuing past failures, then stores
// the outcome. Each step is idempotent. The role step holds the access lock.
func (s *RevokeService) runSteps(ctx context.Context, revocation Revocation, integration Integration, actor string) (Revocation, error) {
	var codes []string
	if revocation.RoleRemoved.needsRun() {
		err := s.WithAccessLock(ctx, revocation.ApplicationClientID, revocation.Subject, func(ctx context.Context) error {
			s.removeRoleStep(ctx, &revocation, integration, &codes)
			return nil
		})
		if err != nil {
			codes = append(codes, CodeRoleRemovalFailed)
			revocation.RoleRemoved = StepFailed
		}
	}
	return s.finishSteps(ctx, revocation, integration, actor, codes)
}

// removeRoleStep is step 2: the access role. The application's OIDC callback
// refuses new sign-ins once the role claim is gone. Callers hold the access lock.
func (s *RevokeService) removeRoleStep(ctx context.Context, revocation *Revocation, integration Integration, codes *[]string) {
	if !revocation.RoleRemoved.needsRun() {
		return
	}
	err := s.Directory.RemoveAccessRole(ctx, revocation.Subject, revocation.ApplicationClientID, integration.AccessRole)
	switch {
	case err == nil:
		revocation.RoleRemoved = StepDone
	case errors.Is(err, ErrAccessRoleInherited):
		*codes = append(*codes, CodeAccessRoleInherited)
		revocation.RoleRemoved = StepFailed
	default:
		*codes = append(*codes, CodeRoleRemovalFailed)
		revocation.RoleRemoved = StepFailed
	}
}

// finishSteps runs steps 3 and 4 when unfinished and stores the attempt. codes
// holds the error codes of earlier steps in this attempt.
func (s *RevokeService) finishSteps(ctx context.Context, revocation Revocation, integration Integration, actor string, codes []string) (Revocation, error) {
	fail := func(code string) StepStatus {
		codes = append(codes, code)
		return StepFailed
	}
	app, subject := revocation.ApplicationClientID, revocation.Subject

	// Step 3: consents and offline tokens for this application's client only.
	if revocation.ConsentsRevoked.needsRun() {
		if err := s.Directory.RevokeClientConsents(ctx, subject, app); err != nil {
			revocation.ConsentsRevoked = fail(CodeConsentRevocationFailed)
		} else {
			revocation.ConsentsRevoked = StepDone
		}
	}

	// Step 4: the application's own sessions (contract 1.1). The connector
	// retries transient failures under the contract's policy.
	if revocation.SessionsRevoked.needsRun() {
		revocation.SessionsRevoked = s.revokeSessions(ctx, integration, revocation, fail)
	}

	revocation.Attempts++
	revocation.LastError = ""
	if len(codes) > 0 {
		revocation.LastError = codes[0]
	}
	if err := s.Repository.SaveRevocationAttempt(ctx, revocation, actor); err != nil {
		return revocation, err
	}
	return revocation, nil
}

func (s *RevokeService) revokeSessions(ctx context.Context, integration Integration, revocation Revocation, fail func(string) StepStatus) StepStatus {
	if !integration.Enabled {
		return fail(CodeIntegrationDisabled)
	}
	revoker, err := s.Sessions(ctx, revocation.ApplicationClientID)
	if err != nil {
		return fail(CodeConnectorUnavailable)
	}
	err = revoker.RevokeSessions(ctx, identitysync.Identity{Issuer: revocation.Issuer, Subject: revocation.Subject})
	switch {
	case err == nil:
		return StepDone
	case errors.Is(err, identitysync.ErrUnsupported):
		return StepUnsupported
	case isClientRejection(err):
		return fail(CodeSessionsAppRejected)
	default:
		return fail(CodeSessionsAppUnavailable)
	}
}

// clientRejection matches the connector's error for a 4xx status it does not
// retry (HTTPAccounts.RevokeSessions reports "...: HTTP <status>"). Retried
// statuses that ran out of attempts are reported as "giving up after ...".
var clientRejection = regexp.MustCompile(`^revoke application sessions: HTTP 4[0-9]{2}$`)

// isClientRejection reports a non-retryable 4xx from the application (for
// example a 400 for an issuer the application is not configured for, or 401/403
// for a sync token it refuses).
func isClientRejection(err error) bool {
	return clientRejection.MatchString(err.Error())
}
