package modulekit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/service"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

type recordingNotifications struct {
	service.NotificationsService
	got model.Notification
}

func (r *recordingNotifications) Notify(_ context.Context, n model.Notification) (*model.Notification, error) {
	r.got = n
	return &n, nil
}

func TestNotifierPreservesEveryField(t *testing.T) {
	recorder := &recordingNotifications{}
	err := Notifier{Service: recorder}.Notify(context.Background(), modulekit.Notification{
		Type: "T", Title: "Ti", Message: "M", Severity: "info", TargetRole: "admin", ClientID: "c", UserID: "u",
		Metadata: json.RawMessage(`{"a":1}`),
		Deliveries: []modulekit.NotificationDelivery{{
			Channel: modulekit.ChannelEmail, Recipient: map[string]any{"email": "x"}, TemplateName: "tpl",
			TemplateData: map[string]any{"k": "v"}, Payload: map[string]any{"subject": "s"}, MaxAttempts: 5,
			Attachments: []modulekit.NotificationAttachment{{FileName: "f.txt", ContentType: "text/plain", Data: []byte("d")}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := recorder.got
	if got.Type != "T" || got.TargetRole != "admin" || string(got.Metadata) != `{"a":1}` || len(got.Deliveries) != 1 {
		t.Fatalf("unexpected notification %+v", got)
	}
	delivery := got.Deliveries[0]
	if delivery.Channel != model.NotificationChannelEmail || delivery.MaxAttempts != 5 || delivery.TemplateName != "tpl" ||
		len(delivery.Attachments) != 1 || string(delivery.Attachments[0].Data) != "d" {
		t.Fatalf("unexpected delivery %+v", delivery)
	}
	if err := (Notifier{}).Notify(context.Background(), modulekit.Notification{}); err == nil {
		t.Fatal("missing service accepted")
	}
}
