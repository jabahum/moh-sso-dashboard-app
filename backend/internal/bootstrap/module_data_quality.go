package bootstrap

import (
	"context"
	"errors"
	"reflect"

	kitadapter "github.com/moh-sso-dashboard/internal/adapters/modulekit"
	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/email"
	"github.com/moh-sso-dashboard/internal/keycloak"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/service"
	dataquality "github.com/mohuganda/moh-sso-dashboard/backend/modules/data-quality"
)

// buildDataQualityModule wires the data-quality module to the host databases,
// the Keycloak admin client and the issue-tracker email path.
func buildDataQualityModule(deps handlerDependencies) *dataquality.Module {
	var directory dataquality.UserDirectory
	if deps.AdminKeycloak != nil {
		directory = dataQualityDirectory{client: deps.AdminKeycloak}
	}
	frontendBaseURL := ""
	if deps.Config != nil {
		frontendBaseURL = deps.Config.FrontendBaseURL
	}
	module, err := dataquality.NewModule(dataquality.Dependencies{
		Primary:         deps.Databases.Primary,
		Warehouse:       deps.Databases.DWH,
		DQA:             deps.Databases.DQA,
		Directory:       directory,
		Mailer:          dataQualityMailer{config: deps.Config, fallback: deps.Services.Email},
		CallerName:      kitadapter.CallerName,
		FrontendBaseURL: frontendBaseURL,
	})
	if err != nil {
		panic(err)
	}
	return module
}

// dataQualityDirectory adapts the Keycloak admin client to the module's port.
type dataQualityDirectory struct{ client *keycloak.KeyAdminClient }

func (d dataQualityDirectory) ListGroups(ctx context.Context) ([]dataquality.DirectoryGroup, error) {
	groups, err := d.client.ListGroups(ctx)
	return toDirectoryGroups(groups), err
}

func (d dataQualityDirectory) GetUserGroups(ctx context.Context, userID string) ([]dataquality.DirectoryGroup, error) {
	groups, err := d.client.GetUserGroups(ctx, userID)
	return toDirectoryGroups(groups), err
}

func (d dataQualityDirectory) ListGroupMembers(ctx context.Context, groupID string) ([]dataquality.DirectoryUser, error) {
	members, err := d.client.ListGroupMembers(ctx, groupID)
	if members == nil {
		return nil, err
	}
	out := make([]dataquality.DirectoryUser, len(members))
	for i, m := range members {
		out[i] = dataquality.DirectoryUser{ID: m.ID, Username: m.Username, Email: m.Email, FirstName: m.FirstName, LastName: m.LastName}
	}
	return out, err
}

func (d dataQualityDirectory) FindUsers(ctx context.Context, query string, exact bool) ([]dataquality.DirectoryUser, error) {
	users, err := d.client.FindUsers(ctx, query, exact)
	if users == nil {
		return nil, err
	}
	out := make([]dataquality.DirectoryUser, len(users))
	for i, u := range users {
		out[i] = dataquality.DirectoryUser{ID: u.ID, Username: u.Username, Email: u.Email}
	}
	return out, err
}

func (d dataQualityDirectory) GetUser(userID string) (*dataquality.DirectoryUser, error) {
	user, err := d.client.GetUser(userID)
	if user == nil {
		return nil, err
	}
	return &dataquality.DirectoryUser{ID: user.ID, Username: user.Username, Email: user.Email, FirstName: user.FirstName, LastName: user.LastName}, err
}

func toDirectoryGroups(groups []keycloak.GroupRep) []dataquality.DirectoryGroup {
	if groups == nil {
		return nil
	}
	out := make([]dataquality.DirectoryGroup, len(groups))
	for i, g := range groups {
		out[i] = dataquality.DirectoryGroup{ID: g.ID, Name: g.Name, Path: g.Path}
	}
	return out
}

// dataQualityMailer sends through the dedicated issue-tracker SMTP account
// (SMTP_*_GM, then the configured SMTP settings) and falls back to the
// platform email service, exactly as the feature did in the host.
type dataQualityMailer struct {
	config   *config.Config
	fallback service.EmailService
}

func (m dataQualityMailer) Send(ctx context.Context, message dataquality.EmailMessage) error {
	return email.SendEmailViaGM(ctx, toEmailModel(message), m.config)
}

func (m dataQualityMailer) Fallback(ctx context.Context, message dataquality.EmailMessage) error {
	if isNilEmailService(m.fallback) {
		return errors.New("email service is unavailable")
	}
	return m.fallback.Send(ctx, toEmailModel(message))
}

// isNilEmailService also treats a typed nil service as absent, like the
// feature's reflection check did.
func isNilEmailService(s service.EmailService) bool {
	if s == nil {
		return true
	}
	v := reflect.ValueOf(s)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func toEmailModel(message dataquality.EmailMessage) model.Message {
	to := make([]model.Address, 0, len(message.To))
	for _, address := range message.To {
		to = append(to, model.Address{Email: address})
	}
	return model.Message{To: to, Subject: message.Subject, HTMLBody: message.HTMLBody, TextBody: message.TextBody}
}
