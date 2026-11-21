package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	emailutil "github.com/moh-sso-dashboard/internal/email"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/service"
	reportscheduler "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

type reportEmailCapture struct {
	message    model.Message
	ids, paths []string
	err        error
}

func (c *reportEmailCapture) Queue(_ context.Context, value model.Message) error {
	c.message = value
	return c.err
}
func (c *reportEmailCapture) ResolveGroupEmailRecipients(_ context.Context, ids, paths []string) ([]model.Address, error) {
	c.ids, c.paths = ids, paths
	return []model.Address{{Name: "Recipient", Email: "recipient@example.invalid"}}, c.err
}

func TestReportEmailAdapterPreservesOutboxAttachment(t *testing.T) {
	capture := &reportEmailCapture{}
	adapter := reportEmailAdapter{capture}
	message := reportscheduler.Message{
		To:      []reportscheduler.Address{{Name: "Recipient", Email: "recipient@example.invalid"}},
		Subject: "Monthly report", TextBody: "Attached", HTMLBody: "<p>Attached</p>",
		Metadata:    map[string]string{"report_execution_id": "execution-1"},
		Attachments: []reportscheduler.Attachment{{FileName: "monthly.pdf", ContentType: "application/pdf", DataBase64: "JVBERi0xLjc="}},
	}
	if err := adapter.Queue(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(capture.message)
	if err != nil {
		t.Fatal(err)
	}
	var restored model.Message
	if err := json.Unmarshal(payload, &restored); err != nil {
		t.Fatal(err)
	}
	attachments, err := emailutil.NormalizeAttachments(nil, restored.Attachments)
	if err != nil || len(attachments) != 1 || string(attachments[0].Data) != "%PDF-1.7" {
		t.Fatalf("outbox attachment lost: %+v, %v", attachments, err)
	}
	if restored.Subject != message.Subject || restored.TextBody != message.TextBody || restored.HTMLBody != message.HTMLBody || !reflect.DeepEqual(restored.Metadata, message.Metadata) || restored.To[0].Name != "Recipient" {
		t.Fatalf("message fields lost: %+v", restored)
	}
	capture.err = errors.New("outbox unavailable")
	if err := adapter.Queue(context.Background(), message); !errors.Is(err, capture.err) {
		t.Fatal("queue failure swallowed")
	}
	_, err = adapter.ResolveGroupEmailRecipients(context.Background(), []string{"group-1"}, []string{"/district"})
	if !errors.Is(err, capture.err) || len(capture.ids) != 1 || len(capture.paths) != 1 {
		t.Fatal("group request or error lost")
	}
	capture.err = nil
	addresses, err := adapter.ResolveGroupEmailRecipients(context.Background(), nil, nil)
	if err != nil || len(addresses) != 1 || addresses[0].Email != "recipient@example.invalid" {
		t.Fatal("group contacts lost")
	}
}

type reportUserCapture struct {
	user *model.User
	err  error
}

func (c reportUserCapture) GetUserByID(uuid.UUID) (*model.User, error) { return c.user, c.err }

func TestReportUserAdapterPreservesMissingAndDisabledUsers(t *testing.T) {
	for _, user := range []*model.User{nil, {FullName: "Recipient", Email: "recipient@example.invalid", Enabled: false}, {Enabled: true}} {
		got, err := (reportUserAdapter{reportUserCapture{user: user}}).GetUserByID(uuid.New())
		if err != nil || (got == nil) != (user == nil) {
			t.Fatalf("lookup: %+v %v", got, err)
		}
		if user != nil && (got.FullName != user.FullName || got.Email != user.Email || got.Enabled != user.Enabled) {
			t.Fatal("user eligibility changed")
		}
	}
	failure := errors.New("directory unavailable")
	if _, err := (reportUserAdapter{reportUserCapture{err: failure}}).GetUserByID(uuid.New()); !errors.Is(err, failure) {
		t.Fatal("lookup failure swallowed")
	}
}

type reportNotificationCapture struct {
	service.NotificationsService
	value model.Notification
	err   error
}

func (c *reportNotificationCapture) Notify(_ context.Context, value model.Notification) (*model.Notification, error) {
	c.value = value
	return &value, c.err
}

func TestReportNotificationAdapterPreservesFailureContext(t *testing.T) {
	capture := &reportNotificationCapture{err: errors.New("notification unavailable")}
	value := reportscheduler.Notification{Type: "REPORT_SCHEDULER_FAILURE", Title: "Scheduled report failed", Message: "Report failed", Severity: "critical", TargetRole: "admin", Metadata: json.RawMessage(`{"execution_id":"execution-1"}`)}
	if err := (reportNotificationAdapter{capture}).Notify(context.Background(), value); !errors.Is(err, capture.err) {
		t.Fatal("notification failure swallowed")
	}
	if capture.value.Type != value.Type || capture.value.Title != value.Title || capture.value.Message != value.Message || capture.value.Severity != value.Severity || capture.value.TargetRole != value.TargetRole || string(capture.value.Metadata) != string(value.Metadata) {
		t.Fatal("failure notification context lost")
	}
}
