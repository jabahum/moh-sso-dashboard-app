package announcements

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// schedulingRepository implements only what scheduling touches. Any other
// repository call (for example marking email sent) panics via the nil
// embedded interface, which the tests rely on.
type schedulingRepository struct {
	Repository
	current   Announcement
	scheduled []ScheduleAnnouncementParams
	published []uuid.UUID
	due       []Announcement
	claimed   int32
}

func (r *schedulingRepository) GetByID(context.Context, uuid.UUID) (Announcement, error) {
	return r.current, nil
}

func (r *schedulingRepository) Schedule(_ context.Context, params ScheduleAnnouncementParams) (Announcement, error) {
	r.scheduled = append(r.scheduled, params)
	item := r.current
	item.Status = AnnouncementStatusSCHEDULED
	item.PublishAt = params.PublishAt
	return item, nil
}

func (r *schedulingRepository) PublishNow(_ context.Context, id, _ uuid.UUID) (Announcement, error) {
	r.published = append(r.published, id)
	item := r.current
	item.Status = AnnouncementStatusPUBLISHED
	return item, nil
}

func (r *schedulingRepository) ClaimDueScheduled(_ context.Context, limit int32) ([]Announcement, error) {
	r.claimed = limit
	due := r.due
	r.due = nil
	return due, nil
}

type recordingPublisher struct{ items []Notification }

func (p *recordingPublisher) Publish(_ context.Context, item Notification) error {
	p.items = append(p.items, item)
	return nil
}

func at(t time.Time) sql.NullTime { return sql.NullTime{Time: t, Valid: true} }

func TestScheduleDoesNotQueueAudienceDeliveries(t *testing.T) {
	id := uuid.New()
	repo := &schedulingRepository{current: Announcement{ID: id, Title: "Clinic hours", Status: AnnouncementStatusDRAFT, NotifyByEmail: true, NotifyBySms: true}}
	publisher := &recordingPublisher{}
	core := NewServiceWithSettings(repo, ServicePorts{Notifications: publisher}, &Settings{})

	item, err := core.ScheduleAnnouncementByUser(context.Background(), id, time.Now().Add(24*time.Hour), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != AnnouncementStatusSCHEDULED || len(repo.scheduled) != 1 || len(repo.published) != 0 {
		t.Fatalf("expected a scheduled announcement, got %+v", item)
	}
	if len(publisher.items) != 1 || publisher.items[0].Type != string(AnnouncementScheduled) || len(publisher.items[0].Deliveries) != 0 {
		t.Fatalf("schedule must only notify admins, got %+v", publisher.items)
	}
}

func TestScheduleInThePastPublishesImmediately(t *testing.T) {
	id := uuid.New()
	repo := &schedulingRepository{current: Announcement{ID: id, Title: "Now", Status: AnnouncementStatusDRAFT}}
	core := NewServiceWithSettings(repo, ServicePorts{Notifications: &recordingPublisher{}}, nil)

	item, err := core.ScheduleAnnouncementByUser(context.Background(), id, time.Now().Add(-time.Minute), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != AnnouncementStatusPUBLISHED || len(repo.published) != 1 || len(repo.scheduled) != 0 {
		t.Fatalf("past publish time should publish now: %+v", item)
	}
}

func TestScheduleRejectsPublishAfterExpiryAndRequestAttachments(t *testing.T) {
	id := uuid.New()
	expires := time.Now().Add(time.Hour)
	repo := &schedulingRepository{current: Announcement{ID: id, ExpiresAt: at(expires)}}
	core := NewServiceWithSettings(repo, ServicePorts{}, nil)

	if _, err := core.ScheduleAnnouncementByUser(context.Background(), id, expires.Add(time.Hour), uuid.New()); !errors.Is(err, ErrAnnouncementExpired) {
		t.Fatalf("expected ErrAnnouncementExpired, got %v", err)
	}
	_, err := core.ScheduleAnnouncementByUser(context.Background(), id, time.Now().Add(time.Minute), uuid.New(),
		AnnouncementEmailOptions{Attachments: []Attachment{{FileName: "a.pdf", DataBase64: "YQ=="}}})
	if !errors.Is(err, ErrScheduleAttachmentsUnsupported) {
		t.Fatalf("expected ErrScheduleAttachmentsUnsupported, got %v", err)
	}
	if len(repo.scheduled) != 0 {
		t.Fatal("rejected schedule requests must not change the announcement")
	}
}

func TestPublishRejectsExpiredAnnouncement(t *testing.T) {
	id := uuid.New()
	repo := &schedulingRepository{current: Announcement{ID: id, ExpiresAt: at(time.Now().Add(-time.Minute))}}
	core := NewServiceWithSettings(repo, ServicePorts{}, nil)

	if _, err := core.PublishAnnouncementNow(context.Background(), id, uuid.New()); !errors.Is(err, ErrAnnouncementExpired) {
		t.Fatalf("expected ErrAnnouncementExpired, got %v", err)
	}
	if len(repo.published) != 0 {
		t.Fatal("expired announcement was published")
	}
}

func TestPublishDueScheduledNotifiesEachClaimedAnnouncement(t *testing.T) {
	scheduler := uuid.New()
	first := Announcement{ID: uuid.New(), Title: "First", Status: AnnouncementStatusPUBLISHED, PublishedBy: uuid.NullUUID{UUID: scheduler, Valid: true}}
	second := Announcement{ID: uuid.New(), Title: "Second", Status: AnnouncementStatusPUBLISHED}
	repo := &schedulingRepository{due: []Announcement{first, second}}
	publisher := &recordingPublisher{}
	core := NewServiceWithSettings(repo, ServicePorts{Notifications: publisher}, nil)

	published, err := core.PublishDueScheduled(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if published != 2 || repo.claimed != DefaultScheduledPublishBatch {
		t.Fatalf("published %d with batch %d", published, repo.claimed)
	}
	if len(publisher.items) != 2 || publisher.items[0].Type != string(AnnouncementPublished) || publisher.items[1].Type != string(AnnouncementPublished) {
		t.Fatalf("each due announcement needs a publish notification: %+v", publisher.items)
	}

	again, err := core.PublishDueScheduled(context.Background(), 10)
	if err != nil || again != 0 || len(publisher.items) != 2 {
		t.Fatalf("already-claimed announcements were notified again: %d %v", again, err)
	}
}

type signalPublisher struct{ published chan Notification }

func (p signalPublisher) Publish(_ context.Context, item Notification) error {
	p.published <- item
	return nil
}

func TestRunScheduledPublisherStopsOnCancel(t *testing.T) {
	repo := &schedulingRepository{due: []Announcement{{ID: uuid.New(), Status: AnnouncementStatusPUBLISHED}}}
	publisher := signalPublisher{published: make(chan Notification, 1)}
	core := NewServiceWithSettings(repo, ServicePorts{Notifications: publisher}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		core.RunScheduledPublisher(ctx, 10*time.Millisecond, func(err error) { t.Error(err) })
		close(done)
	}()
	select {
	case item := <-publisher.published:
		if item.Type != string(AnnouncementPublished) {
			t.Fatalf("unexpected notification %+v", item)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("publisher never ran")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publisher did not stop after cancel")
	}
}
