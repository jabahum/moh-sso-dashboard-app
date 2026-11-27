package announcements

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type markingRepo struct {
	Repository
	item        Announcement
	published   int
	emailMarked int
	snapshots   int
}

func (r *markingRepo) PublishNow(context.Context, uuid.UUID, uuid.UUID) (Announcement, error) {
	r.published++
	r.item.Status = AnnouncementStatusPUBLISHED
	return r.item, nil
}
func (r *markingRepo) SaveAudienceSnapshot(context.Context, uuid.UUID, string, []string) error {
	r.snapshots++
	return nil
}
func (r *markingRepo) MarkEmailNotificationSent(context.Context, uuid.UUID) (Announcement, error) {
	r.emailMarked++
	r.item.EmailNotificationSentAt.Valid = true
	return r.item, nil
}

type oneUser struct{ UserDirectory }

func (oneUser) ListUsers(context.Context) ([]AudienceUser, error) {
	return []AudienceUser{{ID: uuid.NewString(), Username: "nurse", Email: "nurse@example.invalid", Enabled: true}}, nil
}

type failingPublisher struct{ calls int }

func (p *failingPublisher) Publish(context.Context, Notification) error {
	p.calls++
	return errors.New("notification service down")
}

type deliveryRecorder struct{ items []Notification }

func (p *deliveryRecorder) Publish(_ context.Context, item Notification) error {
	p.items = append(p.items, item)
	return nil
}

type rejectingNormalizer struct{}

func (rejectingNormalizer) NormalizeAttachments([]Attachment) ([]Attachment, error) {
	return nil, errors.New("attachment type not allowed")
}

func emailAnnouncement() Announcement {
	return Announcement{ID: uuid.New(), Title: "Outage", Status: AnnouncementStatusDRAFT,
		AudienceType: AnnouncementAudienceTypeALLUSERS, NotifyByEmail: true}
}

func TestEmailMarkedOnlyAfterPublisherAcceptsIt(t *testing.T) {
	repo := &markingRepo{item: emailAnnouncement()}
	publisher := &failingPublisher{}
	core := NewServiceWithSettings(repo, ServicePorts{Users: oneUser{}, Notifications: publisher}, nil)

	item, err := core.PublishAnnouncementNow(context.Background(), repo.item.ID, uuid.New())
	if err != nil {
		t.Fatalf("publisher failure must stay best effort: %v", err)
	}
	if publisher.calls != 1 || repo.emailMarked != 0 || item.EmailNotificationSentAt.Valid {
		t.Fatalf("email marked sent although it was never handed off: calls=%d marked=%d", publisher.calls, repo.emailMarked)
	}

	// With the publisher back, publishing again sends and marks the email.
	ok := &deliveryRecorder{}
	core = NewServiceWithSettings(repo, ServicePorts{Users: oneUser{}, Notifications: ok}, nil)
	if _, err := core.PublishAnnouncementNow(context.Background(), repo.item.ID, uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(ok.items) != 1 || repo.emailMarked != 1 {
		t.Fatalf("retry did not resend: notifications=%d marked=%d", len(ok.items), repo.emailMarked)
	}
	hasEmail := false
	for _, delivery := range ok.items[0].Deliveries {
		hasEmail = hasEmail || delivery.Channel == NotificationChannelEmail
	}
	if !hasEmail {
		t.Fatal("retry notification carried no email delivery")
	}
}

func TestRejectedRequestAttachmentChangesNothing(t *testing.T) {
	repo := &markingRepo{item: emailAnnouncement()}
	core := NewServiceWithSettings(repo, ServicePorts{Users: oneUser{}, EmailAttachments: rejectingNormalizer{}, Notifications: &deliveryRecorder{}}, nil)

	_, err := core.PublishAnnouncementNow(context.Background(), repo.item.ID, uuid.New(), AnnouncementEmailOptions{
		Attachments: []Attachment{{FileName: "a.exe", DataBase64: "YQ=="}}, IncludeAttachmentsInEmail: true,
	})
	if !errors.Is(err, ErrInvalidEmailAttachments) {
		t.Fatalf("expected ErrInvalidEmailAttachments, got %v", err)
	}
	if repo.published != 0 || repo.snapshots != 0 {
		t.Fatalf("announcement changed despite rejected attachment: published=%d snapshots=%d", repo.published, repo.snapshots)
	}
}
