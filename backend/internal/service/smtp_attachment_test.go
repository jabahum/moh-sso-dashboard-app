package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	netmail "net/mail"
	"strings"
	"testing"

	emailutil "github.com/moh-sso-dashboard/internal/email"
	"github.com/moh-sso-dashboard/internal/model"
	gomail "github.com/wneessen/go-mail"
)

func TestQueuedPDFAttachmentBecomesRealMIMEPart(t *testing.T) {
	pdf := []byte("%PDF-1.7\nsynthetic report attachment")
	queued := model.Message{Attachments: []model.Attachment{{FileName: "monthly-report.pdf", ContentType: "application/pdf", DataBase64: base64.StdEncoding.EncodeToString(pdf)}}}
	payload, err := json.Marshal(queued)
	if err != nil {
		t.Fatal(err)
	}
	var restored model.Message
	if err = json.Unmarshal(payload, &restored); err != nil {
		t.Fatal(err)
	}
	attachments, err := emailutil.NormalizeAttachments(nil, restored.Attachments)
	if err != nil {
		t.Fatal(err)
	}
	message := gomail.NewMsg()
	if err = message.From("sender@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if err = message.To("recipient@example.invalid"); err != nil {
		t.Fatal(err)
	}
	message.Subject("Report attachment fixture")
	message.SetBodyString(gomail.TypeTextPlain, "The report is attached.")
	if err = addAttachments(message, attachments); err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if _, err = message.WriteTo(&raw); err != nil {
		t.Fatal(err)
	}
	mail, err := netmail.ReadMessage(bytes.NewReader(raw.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	contentType, params, err := mime.ParseMediaType(mail.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "multipart/mixed" {
		t.Fatalf("email has no attachment multipart: %s", contentType)
	}
	parts := multipart.NewReader(mail.Body, params["boundary"])
	for {
		part, err := parts.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		disposition, parameters, _ := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		if disposition != "attachment" {
			continue
		}
		if parameters["filename"] != "monthly-report.pdf" {
			t.Fatalf("wrong attachment filename: %s", parameters["filename"])
		}
		if !strings.HasPrefix(part.Header.Get("Content-Type"), "application/pdf") {
			t.Fatalf("wrong PDF MIME type: %s", part.Header.Get("Content-Type"))
		}
		var reader io.Reader = part
		if strings.EqualFold(part.Header.Get("Content-Transfer-Encoding"), "base64") {
			reader = base64.NewDecoder(base64.StdEncoding, part)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, pdf) {
			t.Fatal("PDF bytes changed in generated email")
		}
		return
	}
	t.Fatal("email contains no PDF attachment")
}
