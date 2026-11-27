package bootstrap

// This composition root supplies shared capabilities to the published module.
// It contains no Announcements routes, DTOs, SQL or feature policy.
import (
	"context"
	"net/http"

	"github.com/google/uuid"
	emailutil "github.com/moh-sso-dashboard/internal/email"
	models "github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/modulehttp"
	a "github.com/mohuganda/moh-sso-dashboard/backend/modules/announcements"
)

func buildAnnouncementsModule(deps handlerDependencies) *a.Module {
	if deps.Config.CoreOnly() || !deps.Config.ModuleEnabled(a.ID) {
		return nil
	}
	mapUser := func(user models.User) a.AudienceUser {
		return a.AudienceUser{ID: user.ID, Username: user.Username, Email: user.Email, FirstName: user.FirstName,
			LastName: user.LastName, FullName: user.FullName, Enabled: user.Enabled}
	}
	mapUsers := func(users []models.User) []a.AudienceUser {
		if users == nil {
			return nil
		}
		out := make([]a.AudienceUser, len(users))
		for i, user := range users {
			out[i] = mapUser(user)
		}
		return out
	}
	mapAttachments := func(items []a.Attachment) []models.Attachment {
		if items == nil {
			return nil
		}
		out := make([]models.Attachment, len(items))
		for i, item := range items {
			out[i] = models.Attachment(item)
		}
		return out
	}
	users := deps.Repositories.Users
	groups := deps.Repositories.RBAC
	cfg := deps.Config
	settings := &a.Settings{}
	settings.SMS.Enabled, settings.SMS.MaxLength = cfg.SMS.Enabled, cfg.SMS.MaxLength
	settings.Announcement.MaxAttachments, settings.Announcement.MaxAttachmentBytes = cfg.Announcement.MaxAttachments, cfg.Announcement.MaxAttachmentBytes
	settings.Announcement.AllowedAttachmentTypes = cfg.Announcement.AllowedAttachmentTypes
	settings.Email.AllowedAttachmentTypes = cfg.Email.AllowedAttachmentTypes
	settings.Notification.PlatformName, settings.Notification.SystemAdminName = cfg.Notification.PlatformName, cfg.Notification.SystemAdminName
	settings.Notification.SystemAdminEmail, settings.Notification.AdminDashboardURL = cfg.Notification.SystemAdminEmail, cfg.Notification.AdminDashboardURL
	settings.StorageProvider, settings.AppBaseURL = cfg.StorageProvider, cfg.AppBaseURL
	settings.FrontendBaseURL, settings.FrontendRedirectURI = cfg.FrontendBaseURL, cfg.FrontendRedirectURI

	platform := modulePlatform(deps)
	module, err := a.NewModule(a.Dependencies{
		Database: deps.Databases.Primary, Settings: settings,
		Audit: moduleAudit{recorder: deps.Services.Audit}, RequireAdmin: platform.RequireAdmin, AuthorizeAny: platform.AuthorizeAny,
		Limit:     platform.Limit,
		RequestID: platform.RequestID, StreamError: platform.ReportError,
		Actor: func(r *http.Request) (a.Actor, error) {
			verified, valid := modulehttp.RequestAccess(r)
			if !valid {
				return a.Actor{}, a.ErrUnauthorized
			}
			id, err := uuid.Parse(verified.Subject)
			if err != nil || id == uuid.Nil {
				return a.Actor{}, a.ErrUnauthorized
			}
			actor := a.Actor{UserID: id, RealmRoles: verified.RealmRoles}
			for client, roles := range verified.ClientRoles {
				if len(roles) > 0 {
					actor.ClientIDs = append(actor.ClientIDs, client)
				}
			}
			actor.ClientIDs = append(actor.ClientIDs, verified.Systems...)
			// Both identifiers supplied by the shared identity directory remain usable.
			assignments, err := users.GetUserClientRoles(r.Context(), verified.Subject)
			if err != nil {
				return a.Actor{}, err
			}
			for _, assignment := range assignments {
				if len(assignment.Roles) > 0 {
					actor.ClientIDs = append(actor.ClientIDs, assignment.Id, assignment.ClientID)
				}
			}
			return actor, nil
		},
		Ports: a.ServicePorts{
			Storage: deps.FileStorage, HealthContexts: deps.Services.HealthContexts,
			Users: a.UserDirectoryFuncs{
				List: func(ctx context.Context) ([]a.AudienceUser, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					items, err := users.ListUsers()
					return mapUsers(items), err
				},
				Get: func(ctx context.Context, id uuid.UUID) (*a.AudienceUser, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					item, err := users.GetUserByID(id)
					if item == nil {
						return nil, err
					}
					out := mapUser(*item)
					return &out, err
				},
				ForRealmRole: func(ctx context.Context, role string) ([]a.AudienceUser, error) {
					items, err := users.GetUsersByRealmRole(ctx, role)
					return mapUsers(items), err
				},
				ClientRoles: func(ctx context.Context, id string) ([]a.ClientAccess, error) {
					items, err := users.GetUserClientRoles(ctx, id)
					if items == nil {
						return nil, err
					}
					out := make([]a.ClientAccess, len(items))
					for i, item := range items {
						out[i] = a.ClientAccess{ID: item.Id, ClientID: item.ClientID}
					}
					return out, err
				},
			},
			Groups: a.GroupDirectoryFuncs{
				Validate: func(ctx context.Context, id string) error { _, err := groups.GetGroup(ctx, id); return err },
				Members: func(ctx context.Context, id string) ([]a.AudienceGroupMember, error) {
					items, err := groups.ListGroupMembers(ctx, id)
					if items == nil {
						return nil, err
					}
					out := make([]a.AudienceGroupMember, len(items))
					for i, item := range items {
						out[i] = a.AudienceGroupMember{UserID: item.UserID, Username: item.Username, Email: item.Email}
					}
					return out, err
				},
			},
			Notifications: a.NotificationPublisherFunc(func(ctx context.Context, item a.Notification) error {
				out := models.Notification{ID: item.ID, Type: item.Type, Title: item.Title, Message: item.Message, Severity: item.Severity,
					TargetRole: item.TargetRole, ClientID: item.ClientID, UserID: item.UserID, Metadata: item.Metadata, Read: item.Read, CreatedAt: item.CreatedAt}
				if item.Deliveries != nil {
					out.Deliveries = make([]models.NotificationDeliveryRequest, len(item.Deliveries))
					for i, d := range item.Deliveries {
						out.Deliveries[i] = models.NotificationDeliveryRequest{
							Channel: models.NotificationChannel(d.Channel), Recipient: d.Recipient, TemplateName: d.TemplateName, TemplateData: d.TemplateData,
							Payload: d.Payload, Attachments: mapAttachments(d.Attachments), ScheduledAt: d.ScheduledAt, MaxAttempts: d.MaxAttempts,
						}
					}
				}
				_, err := deps.Services.Notifications.Notify(ctx, out)
				return err
			}),
			EmailAttachments: a.EmailAttachmentNormalizerFunc(func(items []a.Attachment) ([]a.Attachment, error) {
				normalized, err := emailutil.NormalizeAttachments(cfg, mapAttachments(items))
				if normalized == nil {
					return nil, err
				}
				out := make([]a.Attachment, len(normalized))
				for i, item := range normalized {
					out[i] = a.Attachment(item)
				}
				return out, err
			}),
		},
	})
	if err != nil {
		panic(err)
	}
	return module
}
