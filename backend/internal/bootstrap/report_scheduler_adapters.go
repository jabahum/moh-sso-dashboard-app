package bootstrap

import (
	"context"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/service"
	reportscheduler "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

// Host adapters keep the scheduler independent of shared identity and delivery models.
type reportEmailAdapter struct {
	backend interface {
		Queue(context.Context, model.Message) error
		ResolveGroupEmailRecipients(context.Context, []string, []string) ([]model.Address, error)
	}
}

func (a reportEmailAdapter) Queue(ctx context.Context, message reportscheduler.Message) error {
	addresses := make([]model.Address, len(message.To))
	for i, value := range message.To {
		addresses[i] = model.Address{Name: value.Name, Email: value.Email}
	}
	attachments := make([]model.Attachment, len(message.Attachments))
	for i, value := range message.Attachments {
		attachments[i] = model.Attachment{FileName: value.FileName, ContentType: value.ContentType, DataBase64: value.DataBase64}
	}
	return a.backend.Queue(ctx, model.Message{To: addresses, Subject: message.Subject, TextBody: message.TextBody, HTMLBody: message.HTMLBody, Attachments: attachments, Metadata: message.Metadata})
}

func (a reportEmailAdapter) ResolveGroupEmailRecipients(ctx context.Context, ids, paths []string) ([]reportscheduler.Address, error) {
	values, err := a.backend.ResolveGroupEmailRecipients(ctx, ids, paths)
	if err != nil {
		return nil, err
	}
	result := make([]reportscheduler.Address, len(values))
	for i, value := range values {
		result[i] = reportscheduler.Address{Name: value.Name, Email: value.Email}
	}
	return result, nil
}

type reportUserAdapter struct {
	backend interface {
		GetUserByID(uuid.UUID) (*model.User, error)
	}
}

func (a reportUserAdapter) GetUserByID(id uuid.UUID) (*reportscheduler.User, error) {
	value, err := a.backend.GetUserByID(id)
	if err != nil || value == nil {
		return nil, err
	}
	return &reportscheduler.User{FullName: value.FullName, Email: value.Email, Enabled: value.Enabled}, nil
}

type reportNotificationAdapter struct{ backend service.NotificationsService }

func (a reportNotificationAdapter) Notify(ctx context.Context, value reportscheduler.Notification) error {
	_, err := a.backend.Notify(ctx, model.Notification{Type: value.Type, Title: value.Title, Message: value.Message, Severity: value.Severity, TargetRole: value.TargetRole, Metadata: value.Metadata})
	return err
}
