package report_scheduler

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/moh-sso-dashboard/internal/model"
)

func (s *Service) reportEmailAttachment(ctx context.Context, execution Execution, artifact Artifact) (model.Attachment, error) {
	limit := s.emailAttachmentLimit
	if limit <= 0 {
		limit = 10 << 20
	}
	var data []byte
	var contentType string
	var err error
	if artifact.ObjectKey != "" {
		if s.fileStorage == nil {
			return model.Attachment{}, fmt.Errorf("report attachment storage is not configured")
		}
		reader, downloadErr := s.fileStorage.Download(ctx, artifact.ObjectKey)
		if downloadErr != nil {
			return model.Attachment{}, fmt.Errorf("download report attachment: %w", downloadErr)
		}
		defer reader.Close()
		data, err = io.ReadAll(io.LimitReader(reader, limit+1))
		contentType = artifact.ContentType
	} else {
		if s.healthBI == nil {
			return model.Attachment{}, fmt.Errorf("Health BI is not configured for report attachments")
		}
		data, contentType, err = s.healthBI.DownloadArtifact(ctx, execution.HealthBIJobID, limit)
	}
	if err != nil {
		return model.Attachment{}, err
	}
	if len(data) == 0 {
		return model.Attachment{}, fmt.Errorf("report attachment is empty")
	}
	if int64(len(data)) > limit {
		return model.Attachment{}, fmt.Errorf("report exceeds email attachment limit (%d bytes)", limit)
	}
	expected := mime.TypeByExtension(strings.ToLower(filepath.Ext(artifact.FileName)))
	expected, _, _ = mime.ParseMediaType(expected)
	actual, _, _ := mime.ParseMediaType(contentType)
	if actual == "" {
		actual = expected
	}
	if expected != "application/pdf" && expected != "text/csv" {
		return model.Attachment{}, fmt.Errorf("unsupported report attachment format")
	}
	if actual != expected {
		return model.Attachment{}, fmt.Errorf("report attachment content type does not match its file format")
	}
	if expected == "application/pdf" && !bytes.HasPrefix(data, []byte("%PDF-")) {
		return model.Attachment{}, fmt.Errorf("report attachment is not a valid PDF")
	}
	return model.Attachment{FileName: filepath.Base(artifact.FileName), ContentType: actual, DataBase64: base64.StdEncoding.EncodeToString(data)}, nil
}

func (s *Service) queueReportEmail(ctx context.Context, schedule Schedule, execution Execution, delivery Delivery, artifact Artifact) error {
	if s.email == nil {
		return fmt.Errorf("email service is not configured")
	}
	if s.publicBaseURL == "" {
		return fmt.Errorf("APP_BASE_URL is required for secure report delivery links")
	}
	attachment, err := s.reportEmailAttachment(ctx, execution, artifact)
	if err != nil {
		return err
	}
	token, err := newDeliveryToken()
	if err != nil {
		return err
	}
	if err = s.repo.SetDeliveryAccessToken(ctx, delivery.ID, hashDeliveryToken(token), time.Now().Add(s.deliveryLinkTTL)); err != nil {
		return err
	}
	downloadURL := s.publicBaseURL + "/api/v1/report-scheduler/public/deliveries/" + token + "/download"
	return s.email.Queue(ctx, model.Message{
		To:          []model.Address{{Email: delivery.RecipientValue}},
		Subject:     fmt.Sprintf("%s report is ready", schedule.ReportName),
		TextBody:    fmt.Sprintf("Your scheduled report %q is attached. You can also download it here: %s", schedule.ReportName, downloadURL),
		HTMLBody:    fmt.Sprintf("<p>Your scheduled report <strong>%s</strong> is attached.</p><p><a href=%q>Download report</a></p>", html.EscapeString(schedule.ReportName), downloadURL),
		Attachments: []model.Attachment{attachment},
		Metadata:    map[string]string{"report_execution_id": execution.ID, "report_artifact_id": artifact.ID},
	})
}
