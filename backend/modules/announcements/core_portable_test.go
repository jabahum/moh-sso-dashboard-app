package announcements

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type portableRepository struct {
	Repository
	ctx       context.Context
	created   CreateAnnouncementParams
	id, actor uuid.UUID
	item      Announcement
	err       error
}

func (r *portableRepository) Create(ctx context.Context, params CreateAnnouncementParams) (Announcement, error) {
	r.ctx, r.created = ctx, params
	return r.item, r.err
}
func (r *portableRepository) GetByID(context.Context, uuid.UUID) (Announcement, error) {
	return r.item, r.err
}
func (r *portableRepository) PublishNow(ctx context.Context, id, actor uuid.UUID) (Announcement, error) {
	r.ctx, r.id, r.actor = ctx, id, actor
	return r.item, r.err
}

type portablePublisher struct {
	ctx   context.Context
	items []Notification
}

func (p *portablePublisher) Publish(ctx context.Context, item Notification) error {
	p.ctx = ctx
	p.items = append(p.items, item)
	return errors.New("best effort failure")
}

type portableNormalizer struct {
	items []Attachment
	err   error
}

func (p *portableNormalizer) NormalizeAttachments(items []Attachment) ([]Attachment, error) {
	p.items = items
	return items, p.err
}

func TestPortableCoreCreateAndPublishUseOnlyFeatureContracts(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	actor := uuid.New()
	id := uuid.New()
	repo := &portableRepository{item: Announcement{ID: id, Title: "Title", Status: AnnouncementStatusDRAFT, CreatedBy: actor}}
	publisher := &portablePublisher{}
	core := NewServiceWithSettings(repo, ServicePorts{Notifications: publisher}, nil)
	item, err := core.CreateAnnouncementFromInput(ctx, CreateAnnouncementInput{
		Title: "Title", Message: "Message", Level: "warning", Status: "draft", AudienceType: "ALL_USERS",
		CreatedBy: actor, Summary: sql.NullString{String: "unused", Valid: false},
	})
	if err != nil || item.ID != id || repo.ctx != ctx || repo.created.Level != AnnouncementLevelWARNING ||
		repo.created.Status != AnnouncementStatusDRAFT || repo.created.AudienceType != AnnouncementAudienceTypeALLUSERS ||
		repo.created.Summary.Valid || repo.created.Summary.String != "unused" {
		t.Fatalf("create contract changed: %+v %v", repo.created, err)
	}
	repo.item.Status = AnnouncementStatusPUBLISHED
	if _, err := core.PublishAnnouncementNow(ctx, id, actor); err != nil {
		t.Fatal(err)
	}
	if repo.id != id || repo.actor != actor || publisher.ctx != ctx || len(publisher.items) != 2 ||
		publisher.items[0].Type != string(AnnouncementCreated) || publisher.items[1].Type != string(AnnouncementPublished) {
		t.Fatal("lifecycle delivery contract changed")
	}
	var metadata map[string]any
	if err := json.Unmarshal(publisher.items[1].Metadata, &metadata); err != nil || metadata["announcement_id"] != id.String() || metadata["published_by"] != actor.String() {
		t.Fatalf("publication metadata changed: %s", publisher.items[1].Metadata)
	}
	cause := errors.New("persistence unavailable")
	repo.err = cause
	if _, err := core.CreateAnnouncement(ctx, CreateAnnouncementParams{}); !errors.Is(err, cause) {
		t.Fatalf("repository cause lost: %v", err)
	}
}

func TestPortableEmailNormalizationIsDelegatedAndPreservesScheduling(t *testing.T) {
	normalizer := &portableNormalizer{}
	core := NewServiceWithSettings(nil, ServicePorts{EmailAttachments: normalizer}, nil)
	when := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	items := []Attachment{{FileName: "fixture.pdf", DataBase64: "YQ=="}}
	result, err := core.normalizeAnnouncementEmailOptions(AnnouncementEmailOptions{Attachments: items, IncludeAttachmentsInEmail: true, ScheduledAt: &when})
	if err != nil || len(normalizer.items) != 1 || result.ScheduledAt != &when {
		t.Fatalf("normalizer boundary changed: %+v %v", result, err)
	}
	cause := errors.New("attachment rejected")
	normalizer.err = cause
	if _, err := core.normalizeAnnouncementEmailOptions(AnnouncementEmailOptions{Attachments: items, IncludeAttachmentsInEmail: true}); !errors.Is(err, cause) {
		t.Fatalf("normalizer cause lost: %v", err)
	}
}
