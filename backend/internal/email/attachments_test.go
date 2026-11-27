package email

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/model"
)

func configWithBaseDir(dir string) *config.Config {
	cfg := &config.Config{}
	cfg.Email.AttachmentBaseDir = dir
	return cfg
}

func TestPathAttachmentsDisabledWithoutBaseDir(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret.pdf")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := NormalizeAttachments(nil, []model.Attachment{{FileName: "a.pdf", Path: secret}})
	if !errors.Is(err, ErrPathAttachmentsDisabled) {
		t.Fatalf("expected path attachments to be disabled, got %v", err)
	}
}

func TestPathAttachmentOutsideBaseDirIsRejected(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.pdf")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "link.pdf")); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		outside,                        // absolute, outside
		"/proc/self/environ",           // absolute system file disguised by file_name
		"../" + filepath.Base(outside), // relative escape
		"link.pdf",                     // symlink escaping the base dir
	} {
		_, err := NormalizeAttachments(configWithBaseDir(base), []model.Attachment{{FileName: "a.pdf", Path: path}})
		if err == nil {
			t.Fatalf("path %q outside the base dir was accepted", path)
		}
	}
}

func TestPathAttachmentTypeIsCheckedOnTheFileRead(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "server.env"), []byte("KEY=value"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := NormalizeAttachments(configWithBaseDir(base), []model.Attachment{{FileName: "report.pdf", Path: "server.env"}})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected disallowed path type, got %v", err)
	}
}

func TestPathAttachmentInsideBaseDirIsReadIntoData(t *testing.T) {
	base := t.TempDir()
	content := []byte("%PDF-1.7 report")
	if err := os.MkdirAll(filepath.Join(base, "reports"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "reports", "monthly.pdf"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"reports/monthly.pdf", filepath.Join(base, "reports", "monthly.pdf")} {
		out, err := NormalizeAttachments(configWithBaseDir(base), []model.Attachment{{FileName: "monthly.pdf", Path: path}})
		if err != nil {
			t.Fatalf("path %q: %v", path, err)
		}
		if out[0].Path != "" {
			t.Fatalf("path %q was kept; downstream senders must not reopen it", path)
		}
		if string(out[0].Data) != string(content) || out[0].DataBase64 == "" {
			t.Fatalf("path %q was not read into the attachment data", path)
		}
	}
}

func TestPathAttachmentLargerThanLimitIsRejected(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "big.pdf"), make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := configWithBaseDir(base)
	cfg.Email.MaxAttachmentBytes = 16

	if _, err := NormalizeAttachments(cfg, []model.Attachment{{FileName: "big.pdf", Path: "big.pdf"}}); err == nil {
		t.Fatal("oversized path attachment was accepted")
	}
}
