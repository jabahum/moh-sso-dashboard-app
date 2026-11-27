package announcements

import (
	"context"
	"database/sql"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type storedAttachmentRepo struct {
	Repository
	items []AnnouncementAttachment
}

func (r storedAttachmentRepo) ListEmailAttachments(context.Context, uuid.UUID) ([]AnnouncementAttachment, error) {
	return r.items, nil
}

type memoryStorage struct{ AttachmentStorage }

func (memoryStorage) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("stored bytes")), nil
}

func storedAttachment(name, checksum string) AnnouncementAttachment {
	return AnnouncementAttachment{ID: uuid.New(), FileName: uuid.NewString() + ".pdf", OriginalFileName: name,
		StorageKey: uuid.NewString(), Checksum: sql.NullString{String: checksum, Valid: true}}
}

func TestEmailAttachmentsAreNotDuplicated(t *testing.T) {
	repo := storedAttachmentRepo{items: []AnnouncementAttachment{
		storedAttachment("Report.pdf", "aaa"), // also sent in the request
		storedAttachment("annex.pdf", "bbb"),
		storedAttachment("annex.pdf", "bbb"), // identical stored copy
		storedAttachment("annex.pdf", "ccc"), // same name, different file
	}}
	core := NewServiceWithSettings(repo, ServicePorts{Storage: memoryStorage{}}, nil)

	out, err := core.withPersistedAnnouncementEmailAttachments(context.Background(), uuid.New(), AnnouncementEmailOptions{
		IncludeAttachmentsInEmail: true,
		Attachments:               []Attachment{{FileName: "report.pdf", DataBase64: "YQ=="}},
	})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(out.Attachments))
	for _, attachment := range out.Attachments {
		names = append(names, attachment.FileName)
	}
	if got := strings.Join(names, ","); got != "report.pdf,annex.pdf,annex.pdf" {
		t.Fatalf("unexpected email attachments: %s", got)
	}
}
