package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/moh-sso-dashboard/internal/config"
	emailutil "github.com/moh-sso-dashboard/internal/email"
	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/moh-sso-dashboard/internal/model"
	"github.com/moh-sso-dashboard/internal/utils"
	mail "github.com/wneessen/go-mail"
)

type SMTPService struct {
	cfg       *config.Config
	templates *TemplateManager
	logger    *logger.Logger
}

func NewSMTPService(
	cfg *config.Config,
	tm *TemplateManager,
	logger *logger.Logger,
) (*SMTPService, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	if logger == nil {
		return nil, errors.New("logger is required")
	}

	if strings.TrimSpace(cfg.SMTP.Host) == "" {
		return nil, errors.New("smtp host is required")
	}

	if cfg.SMTP.Port <= 0 {
		return nil, errors.New("smtp port is required")
	}

	if strings.TrimSpace(cfg.SMTP.FromEmail) == "" {
		return nil, errors.New("smtp from email is required")
	}

	if strings.TrimSpace(cfg.SMTP.FromName) == "" {
		return nil, errors.New("smtp from name is required")
	}

	return &SMTPService{
		cfg:       cfg,
		templates: tm,
		logger:    logger,
	}, nil
}

func (s *SMTPService) Send(ctx context.Context, msg model.Message) (sendErr error) {
	logSubject := msg.Subject
	safeLogError := func(err error) error { return err }
	if msg.Metadata["application_id"] != "" {
		logSubject = "[platform notification]"
		safeLogError = func(error) error { return errors.New("platform SMTP operation failed") }
		defer func() {
			if sendErr != nil {
				sendErr = errors.New("platform SMTP operation failed")
			}
		}()
	}
	if s == nil {
		return errors.New("smtp service is nil")
	}

	if s.cfg == nil {
		return errors.New("smtp service config is nil")
	}

	if err := utils.ValidateMessage(msg); err != nil {
		s.logger.Error(
			"email message validation failed",
			"error", safeLogError(err),
			"subject", logSubject,
		)

		return fmt.Errorf("validate email message: %w", err)
	}

	attachments, err := emailutil.NormalizeAttachments(s.cfg, msg.Attachments)
	if err != nil {
		s.logger.Error(
			"email attachment validation failed",
			"error", safeLogError(err),
			"subject", logSubject,
		)

		return fmt.Errorf("validate email attachments: %w", err)
	}
	msg.Attachments = attachments

	if strings.TrimSpace(msg.TemplateName) != "" {
		if s.templates == nil {
			err := errors.New("template manager not configured")

			s.logger.Error(
				"failed to render email template",
				"error", safeLogError(err),
				"template_name", msg.TemplateName,
				"subject", logSubject,
			)

			return err
		}

		rendered, err := s.templates.Render(msg.TemplateName, msg.TemplateData)
		if err != nil {
			s.logger.Error(
				"failed to render email template",
				"error", safeLogError(err),
				"template_name", msg.TemplateName,
				"subject", logSubject,
			)

			return fmt.Errorf("render email template %q: %w", msg.TemplateName, err)
		}

		msg.HTMLBody = rendered

		if strings.TrimSpace(msg.TextBody) == "" {
			msg.TextBody = "Please use an email client that supports HTML."
		}
	}

	m := mail.NewMsg()

	from := normalizeFromAddress(msg.From, s.cfg)
	if err := setFromAddress(m, from); err != nil {
		s.logger.Error(
			"failed to set email from address",
			"error", safeLogError(err),
			"from_email", from.Email,
			"subject", logSubject,
		)

		return err
	}

	if err := addRecipients(m, "to", msg.To); err != nil {
		s.logger.Error(
			"failed to add email to recipients",
			"error", safeLogError(err),
			"subject", logSubject,
		)

		return err
	}

	if err := addRecipients(m, "cc", msg.Cc); err != nil {
		s.logger.Error(
			"failed to add email cc recipients",
			"error", safeLogError(err),
			"subject", logSubject,
		)

		return err
	}

	if err := addRecipients(m, "bcc", msg.Bcc); err != nil {
		s.logger.Error(
			"failed to add email bcc recipients",
			"error", safeLogError(err),
			"subject", logSubject,
		)

		return err
	}

	if err := addReplyToRecipients(m, msg.ReplyTo); err != nil {
		s.logger.Error(
			"failed to add email reply-to recipients",
			"error", safeLogError(err),
			"subject", logSubject,
		)

		return err
	}

	m.Subject(strings.TrimSpace(msg.Subject))

	if strings.TrimSpace(msg.TextBody) != "" {
		m.SetBodyString(mail.TypeTextPlain, msg.TextBody)
	}

	if strings.TrimSpace(msg.HTMLBody) != "" {
		if strings.TrimSpace(msg.TextBody) == "" {
			m.SetBodyString(mail.TypeTextHTML, msg.HTMLBody)
		} else {
			m.AddAlternativeString(mail.TypeTextHTML, msg.HTMLBody)
		}
	}

	for k, v := range msg.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}

		m.SetGenHeader(mail.Header(k), v)
	}

	if err := addAttachments(m, msg.Attachments); err != nil {
		s.logger.Error(
			"failed to add email attachments",
			"error", safeLogError(err),
			"subject", logSubject,
		)

		return err
	}

	host := strings.TrimSpace(s.cfg.SMTP.Host)
	hostLower := strings.ToLower(host)

	isLocalSMTP := hostLower == "mailhog" ||
		hostLower == "localhost" ||
		hostLower == "127.0.0.1"

	clientOptions := []mail.Option{
		mail.WithPort(s.cfg.SMTP.Port),
	}

	tlsPolicy := "TLSMandatory"
	if isLocalSMTP {
		clientOptions = append(clientOptions, mail.WithTLSPolicy(mail.NoTLS))
		tlsPolicy = "NoTLS"
	} else {
		clientOptions = append(clientOptions, mail.WithTLSPolicy(mail.TLSMandatory))
	}

	if strings.TrimSpace(s.cfg.SMTP.Username) != "" {
		clientOptions = append(
			clientOptions,
			mail.WithUsername(strings.TrimSpace(s.cfg.SMTP.Username)),
			mail.WithPassword(s.cfg.SMTP.Password),
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
		)
	}

	s.logger.Info(
		"creating smtp client",
		"smtp_host", host,
		"smtp_port", s.cfg.SMTP.Port,
		"tls_policy", tlsPolicy,
		"auth_enabled", strings.TrimSpace(s.cfg.SMTP.Username) != "",
	)

	client, err := mail.NewClient(
		host,
		clientOptions...,
	)
	if err != nil {
		s.logger.Error(
			"failed to create smtp client",
			"error", safeLogError(err),
			"smtp_host", host,
			"smtp_port", s.cfg.SMTP.Port,
			"tls_policy", tlsPolicy,
		)

		return fmt.Errorf("create smtp client: %w", err)
	}

	if err := client.DialAndSendWithContext(ctx, m); err != nil {
		s.logger.Error(
			"failed to send email",
			"error", safeLogError(err),
			"subject", logSubject,
			"smtp_host", host,
			"smtp_port", s.cfg.SMTP.Port,
			"tls_policy", tlsPolicy,
		)

		return fmt.Errorf("send email: %w", err)
	}

	s.logger.Info(
		"email sent successfully",
		"subject", logSubject,
		"from_email", from.Email,
		"to_count", len(msg.To),
		"cc_count", len(msg.Cc),
		"bcc_count", len(msg.Bcc),
		"smtp_host", host,
		"smtp_port", s.cfg.SMTP.Port,
		"tls_policy", tlsPolicy,
	)

	return nil
}

func normalizeFromAddress(from *model.Address, cfg *config.Config) *model.Address {
	if from != nil && strings.TrimSpace(from.Email) != "" {
		return &model.Address{
			Name:  strings.TrimSpace(from.Name),
			Email: strings.TrimSpace(from.Email),
		}
	}

	return &model.Address{
		Name:  strings.TrimSpace(cfg.SMTP.FromName),
		Email: strings.TrimSpace(cfg.SMTP.FromEmail),
	}
}

func setFromAddress(m *mail.Msg, from *model.Address) error {
	if from == nil {
		return errors.New("from address is required")
	}

	if strings.TrimSpace(from.Email) == "" {
		return errors.New("from email is required")
	}

	if strings.TrimSpace(from.Name) != "" {
		if err := m.FromFormat(strings.TrimSpace(from.Name), strings.TrimSpace(from.Email)); err != nil {
			return fmt.Errorf("set from: %w", err)
		}

		return nil
	}

	if err := m.From(strings.TrimSpace(from.Email)); err != nil {
		return fmt.Errorf("set from: %w", err)
	}

	return nil
}

func addRecipients(m *mail.Msg, kind string, recipients []model.Address) error {
	for _, recipient := range recipients {
		name := strings.TrimSpace(recipient.Name)
		email := strings.TrimSpace(recipient.Email)

		if email == "" {
			return fmt.Errorf("add %s recipient: email is required", kind)
		}

		switch kind {
		case "to":
			if name != "" {
				if err := m.AddToFormat(name, email); err != nil {
					return fmt.Errorf("add to: %w", err)
				}
			} else {
				if err := m.AddTo(email); err != nil {
					return fmt.Errorf("add to: %w", err)
				}
			}

		case "cc":
			if name != "" {
				if err := m.AddCcFormat(name, email); err != nil {
					return fmt.Errorf("add cc: %w", err)
				}
			} else {
				if err := m.AddCc(email); err != nil {
					return fmt.Errorf("add cc: %w", err)
				}
			}

		case "bcc":
			if name != "" {
				if err := m.AddBccFormat(name, email); err != nil {
					return fmt.Errorf("add bcc: %w", err)
				}
			} else {
				if err := m.AddBcc(email); err != nil {
					return fmt.Errorf("add bcc: %w", err)
				}
			}

		default:
			return fmt.Errorf("unsupported recipient kind %q", kind)
		}
	}

	return nil
}

func addReplyToRecipients(m *mail.Msg, recipients []model.Address) error {
	for _, recipient := range recipients {
		name := strings.TrimSpace(recipient.Name)
		email := strings.TrimSpace(recipient.Email)

		if email == "" {
			return errors.New("reply-to email is required")
		}

		if name != "" {
			if err := m.ReplyToFormat(name, email); err != nil {
				return fmt.Errorf("set reply-to: %w", err)
			}

			continue
		}

		if err := m.ReplyTo(email); err != nil {
			return fmt.Errorf("set reply-to: %w", err)
		}
	}

	return nil
}

func addAttachments(m *mail.Msg, attachments []model.Attachment) error {
	for _, att := range attachments {
		path := strings.TrimSpace(att.Path)
		fileName := strings.TrimSpace(att.FileName)

		switch {
		case path != "":
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("attachment path %q: %w", path, err)
			}

			if fileName != "" {
				m.AttachFile(path, mail.WithFileName(fileName))
			} else {
				m.AttachFile(path)
			}

		case len(att.Data) > 0:
			if fileName == "" {
				fileName = "attachment"
			}

			m.AttachReader(fileName, utils.NewReadSeeker(att.Data))

		default:
			return fmt.Errorf("attachment %q missing path or data", fileName)
		}
	}

	return nil
}
