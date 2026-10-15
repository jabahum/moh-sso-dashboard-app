package report_scheduler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	emailutil "github.com/moh-sso-dashboard/internal/email"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/storage"
)

type attachmentRepository struct {
	Repository
	delivery                   Delivery
	execution                  Execution
	artifact                   Artifact
	queuedToken, sent, retried bool
	retryError                 string
}

func (r *attachmentRepository) ClaimDueDelivery(context.Context, time.Time) (Delivery, bool, error) {
	return r.delivery, true, nil
}
func (r *attachmentRepository) GetExecution(context.Context, string) (Execution, error) {
	return r.execution, nil
}
func (r *attachmentRepository) GetSchedule(context.Context, string, string, bool) (Schedule, error) {
	return Schedule{ReportName: "Monthly <report>"}, nil
}
func (r *attachmentRepository) GetArtifact(context.Context, string) (Artifact, error) {
	return r.artifact, nil
}
func (r *attachmentRepository) SetDeliveryAccessToken(_ context.Context, _ string, hash string, _ time.Time) error {
	r.queuedToken = len(hash) == 64
	return nil
}
func (r *attachmentRepository) MarkDeliverySent(context.Context, string) error {
	r.sent = true
	return nil
}
func (r *attachmentRepository) ScheduleDeliveryRetry(_ context.Context, _ string, message string, _ time.Time) error {
	r.retried = true
	r.retryError = message
	return nil
}
func (r *attachmentRepository) FinalizeExecutionDeliveryState(context.Context, string) (string, bool, error) {
	return "completed", false, nil
}

type attachmentHealthBI struct {
	HealthBIClient
	data        []byte
	contentType string
	err         error
	calls       int
	limit       int64
	jobID       string
}

func (c *attachmentHealthBI) DownloadArtifact(_ context.Context, id string, limit int64) ([]byte, string, error) {
	c.calls++
	c.limit = limit
	c.jobID = id
	return c.data, c.contentType, c.err
}

type attachmentEmail struct {
	messages []model.Message
	err      error
}

func (e *attachmentEmail) Queue(_ context.Context, m model.Message) error {
	if e.err != nil {
		return e.err
	}
	e.messages = append(e.messages, m)
	return nil
}
func (e *attachmentEmail) ResolveGroupEmailRecipients(context.Context, []string, []string) ([]model.Address, error) {
	return nil, nil
}

func attachmentFixture() (*Service, *attachmentRepository, *attachmentHealthBI, *attachmentEmail) {
	scheduleID := "schedule-1"
	r := &attachmentRepository{delivery: Delivery{ID: "delivery-1", ExecutionID: "execution-1", ArtifactID: "artifact-1", DeliveryChannel: "email", RecipientValue: "test@example.invalid", MaxAttempts: 3}, execution: Execution{ID: "execution-1", ScheduleID: &scheduleID, HealthBIJobID: "job-1"}, artifact: Artifact{ID: "artifact-1", FileName: "monthly-report.pdf", ExternalURL: "https://untrusted.example/never-fetch-this"}}
	c := &attachmentHealthBI{data: []byte("%PDF-1.7\nsynthetic report bytes"), contentType: "application/pdf"}
	e := &attachmentEmail{}
	s := NewService(r, c, e, nil, nil, nil)
	s.SetPublicBaseURL("https://portal.example/ssobackend")
	return s, r, c, e
}

func TestReportEmailQueuesPDFAttachmentThatSurvivesSerialization(t *testing.T) {
	s, r, c, e := attachmentFixture()
	found, err := s.ProcessOneDelivery(context.Background())
	if err != nil || !found || !r.sent || !r.queuedToken || r.retried {
		t.Fatalf("delivery failed: found=%v error=%v repo=%+v", found, err, r)
	}
	if len(e.messages) != 1 || c.calls != 1 || c.jobID != "job-1" || c.limit != 10<<20 {
		t.Fatal("artifact not fetched with job identity and attachment bound")
	}
	msg := e.messages[0]
	if len(msg.Attachments) != 1 {
		t.Fatal("PDF missing from queued email")
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var restored model.Message
	if err := json.Unmarshal(payload, &restored); err != nil {
		t.Fatal(err)
	}
	attachments, err := emailutil.NormalizeAttachments(nil, restored.Attachments)
	if err != nil {
		t.Fatal(err)
	}
	if len(attachments) != 1 || attachments[0].FileName != "monthly-report.pdf" || attachments[0].ContentType != "application/pdf" || string(attachments[0].Data) != string(c.data) {
		t.Fatalf("PDF did not survive email queue serialization: %+v", attachments)
	}
	if !strings.Contains(msg.TextBody, "is attached") || !strings.Contains(msg.TextBody, "/public/deliveries/") || strings.Contains(msg.HTMLBody, "<strong>Monthly <report>") {
		t.Fatal("attachment email must retain the secure link and escape the report name")
	}
}

func TestReportAttachmentFailuresRetryWithoutSendingLinkOnlyEmail(t *testing.T) {
	for _, scenario := range []string{"download", "oversize", "html", "invalid-pdf", "empty", "queue"} {
		t.Run(scenario, func(t *testing.T) {
			s, r, c, e := attachmentFixture()
			switch scenario {
			case "download":
				c.err = errors.New("artifact unavailable")
			case "oversize":
				s.SetEmailAttachmentLimit(4)
			case "html":
				c.contentType = "text/html"
			case "invalid-pdf":
				c.data = []byte("not a PDF")
			case "empty":
				c.data = nil
			case "queue":
				e.err = errors.New("queue unavailable")
			}
			found, err := s.ProcessOneDelivery(context.Background())
			if err != nil || !found || r.sent || !r.retried || r.retryError == "" || len(e.messages) != 0 {
				t.Fatalf("attachment failure must retry, not mark sent: %+v %v", r, err)
			}
		})
	}
}

func TestPortalDeliveryDoesNotDownloadEmailAttachment(t *testing.T) {
	s, r, c, e := attachmentFixture()
	r.delivery.DeliveryChannel = "portal"
	if _, err := s.ProcessOneDelivery(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !r.sent || c.calls != 0 || len(e.messages) != 0 {
		t.Fatal("portal delivery unexpectedly fetched or emailed an attachment")
	}
}

type attachmentStorage struct{ storage.Storage }

func (attachmentStorage) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("%PDF-1.7\nstored PDF")), nil
}
func TestReportEmailUsesStoredArtifactWithoutReGenerating(t *testing.T) {
	s, r, c, _ := attachmentFixture()
	s.fileStorage = attachmentStorage{}
	r.artifact.ObjectKey = "reports/stored.pdf"
	attachment, err := s.reportEmailAttachment(context.Background(), r.execution, r.artifact)
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(attachment.DataBase64)
	if err != nil || !strings.Contains(string(data), "stored PDF") || c.calls != 0 {
		t.Fatal("stored attachment was not used")
	}
}

func TestReportEmailAttachesSelectedCSVFormat(t *testing.T) {
	s, r, c, e := attachmentFixture()
	r.artifact.FileName = "monthly-report.csv"
	c.data = []byte("Report,Value\nMonthly,42\n")
	c.contentType = "text/csv; charset=utf-8"
	if _, err := s.ProcessOneDelivery(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !r.sent || len(e.messages) != 1 || len(e.messages[0].Attachments) != 1 || e.messages[0].Attachments[0].ContentType != "text/csv" {
		t.Fatal("CSV attachment was not queued")
	}
}
