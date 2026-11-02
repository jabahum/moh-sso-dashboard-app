package service

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/moh-sso-dashboard/internal/config"
	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestPlatformSMTPDiagnosticsDoNotLogMessageSubject(t *testing.T) {
	var output bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&output)
	defer func() { log.Logger = previous }()
	sender := &SMTPService{cfg: &config.Config{}, logger: logger.NewLogger()}
	err := sender.Send(context.Background(), model.Message{Subject: "private-patient-subject", TextBody: "private-message-body", Metadata: map[string]string{"application_id": "pilot"}})
	if err == nil || err.Error() != "platform SMTP operation failed" {
		t.Fatalf("unsafe SMTP error: %v", err)
	}
	if strings.Contains(output.String(), "private-patient-subject") || strings.Contains(output.String(), "private-message-body") {
		t.Fatal("message content leaked into operational logs")
	}
	if !strings.Contains(output.String(), "[platform notification]") {
		t.Fatal("expected redacted platform diagnostics")
	}
}
