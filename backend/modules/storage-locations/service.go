package storagelocations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// Service manages storage locations. Other modules (Documents) use GetByID to
// resolve where a file lives.
type Service interface {
	Create(ctx context.Context, input CreateInput) (Location, error)
	GetByID(ctx context.Context, id string) (Location, error)
	GetByCode(ctx context.Context, code string) (Location, error)
	ListActive(ctx context.Context) ([]Location, error)
	Update(ctx context.Context, id string, input UpdateStorageLocationInput) (Location, error)
	Delete(ctx context.Context, id string) error
}

// NewService builds the service. notifier may be nil.
func NewService(repo Repository, notifier modulekit.Notifier, platform modulekit.Platform) Service {
	return &service{repo: repo, notifier: notifier, platform: platform}
}

type service struct {
	repo     Repository
	notifier modulekit.Notifier
	platform modulekit.Platform
}

func (s *service) ready() error {
	if s == nil {
		return errors.New("storage location service is nil")
	}
	if s.repo == nil {
		return errors.New("storage location repository is nil")
	}
	return nil
}

func (s *service) Create(ctx context.Context, input CreateInput) (Location, error) {
	if err := s.ready(); err != nil {
		return Location{}, err
	}
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Provider = strings.TrimSpace(input.Provider)
	input.BaseUri = strings.TrimSpace(input.BaseUri)
	if input.Code == "" {
		return Location{}, errors.New("storage location code is required")
	}
	if input.Name == "" {
		return Location{}, errors.New("storage location name is required")
	}
	if input.Provider == "" {
		return Location{}, errors.New("storage location provider is required")
	}
	existing, err := s.repo.GetByCode(ctx, input.Code)
	if err == nil && existing.ID != uuid.Nil {
		return Location{}, fmt.Errorf("storage location code already exists: %s", input.Code)
	}
	location, err := s.repo.Create(ctx, CreateInput{
		Code: input.Code, Name: input.Name, Provider: input.Provider, BaseUri: input.BaseUri, IsActive: input.IsActive,
	})
	if err != nil {
		return Location{}, fmt.Errorf("create storage location: %w", err)
	}
	// In-app only.
	s.notify(ctx, modulekit.Notification{
		Type: "STORAGE_LOCATION_CREATED", Title: "Storage location created", Severity: "info",
		Message: "Storage location created", TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"storage_location_id": location.ID.String(), "code": location.Code, "name": location.Name,
			"provider": location.Provider, "base_uri": location.BaseUri, "is_active": location.IsActive,
		}),
	})
	return location, nil
}

func (s *service) GetByID(ctx context.Context, id string) (Location, error) {
	if err := s.ready(); err != nil {
		return Location{}, err
	}
	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return Location{}, fmt.Errorf("invalid storage location id: %w", err)
	}
	location, err := s.repo.GetByID(ctx, parsedID)
	if err != nil {
		return Location{}, fmt.Errorf("get storage location by id: %w", err)
	}
	return location, nil
}

func (s *service) GetByCode(ctx context.Context, code string) (Location, error) {
	if err := s.ready(); err != nil {
		return Location{}, err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return Location{}, errors.New("storage location code is required")
	}
	location, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return Location{}, fmt.Errorf("get storage location by code: %w", err)
	}
	return location, nil
}

func (s *service) ListActive(ctx context.Context) ([]Location, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	locations, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active storage locations: %w", err)
	}
	return locations, nil
}

func (s *service) Update(ctx context.Context, id string, input UpdateStorageLocationInput) (Location, error) {
	if err := s.ready(); err != nil {
		return Location{}, err
	}
	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return Location{}, fmt.Errorf("invalid storage location id: %w", err)
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Provider = strings.TrimSpace(input.Provider)
	input.BaseUri = strings.TrimSpace(input.BaseUri)
	if input.Name == "" {
		return Location{}, errors.New("storage location name is required")
	}
	if input.Provider == "" {
		return Location{}, errors.New("storage location provider is required")
	}
	before, _ := s.repo.GetByID(ctx, parsedID)
	location, err := s.repo.Update(ctx, UpdateStorageLocationInput{
		ID: parsedID, Name: input.Name, Provider: input.Provider, BaseUri: input.BaseUri, IsActive: input.IsActive,
	})
	if err != nil {
		return Location{}, fmt.Errorf("update storage location: %w", err)
	}
	notification := modulekit.Notification{
		Type: "STORAGE_LOCATION_UPDATED", Title: "Storage location updated", Severity: "warning",
		Message: "Storage location updated", TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"storage_location_id": location.ID.String(), "code": location.Code, "name": location.Name,
			"provider": location.Provider, "base_uri": location.BaseUri, "is_active": location.IsActive,
			"previous": map[string]any{
				"name": before.Name, "provider": before.Provider, "base_uri": before.BaseUri, "is_active": before.IsActive,
			},
		}),
	}
	s.attachAdminEmailDelivery(&notification, "storage-location-updated", "Storage location updated",
		fmt.Sprintf("Storage location %s was updated.", location.Name),
		map[string]any{
			"Name": s.platform.AdminName(), "Platform": s.platform.DisplayName(), "Code": location.Code,
			"Provider": location.Provider, "ActionURL": s.adminStorageURL(),
			"Details": fmt.Sprintf(
				"Storage Location ID: %s\nCode: %s\nName: %s\nProvider: %s\nBase URI: %s\nIs Active: %v\n\nPrevious Name: %s\nPrevious Provider: %s\nPrevious Base URI: %s\nPrevious Active: %v",
				location.ID.String(), location.Code, location.Name, location.Provider, location.BaseUri, location.IsActive,
				before.Name, before.Provider, before.BaseUri, before.IsActive,
			),
		})
	s.notify(ctx, notification)
	return location, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	if err := s.ready(); err != nil {
		return err
	}
	parsedID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("invalid storage location id: %w", err)
	}
	location, _ := s.repo.GetByID(ctx, parsedID)
	if err := s.repo.Delete(ctx, parsedID); err != nil {
		return fmt.Errorf("delete storage location: %w", err)
	}
	if location.ID == uuid.Nil {
		return nil
	}
	notification := modulekit.Notification{
		Type: "STORAGE_LOCATION_DELETED", Title: "Storage location deleted", Severity: "critical",
		Message: "Storage location deleted", TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"storage_location_id": location.ID.String(), "code": location.Code, "name": location.Name,
			"provider": location.Provider, "base_uri": location.BaseUri, "is_active": location.IsActive,
		}),
	}
	s.attachAdminEmailDelivery(&notification, "storage-location-deleted", "Storage location deleted",
		fmt.Sprintf("Storage location %s was deleted.", location.Name),
		map[string]any{
			"Name": s.platform.AdminName(), "Platform": s.platform.DisplayName(), "Code": location.Code,
			"Provider": location.Provider, "ActionURL": s.adminStorageURL(),
			"Details": fmt.Sprintf(
				"Storage Location ID: %s\nCode: %s\nName: %s\nProvider: %s\nBase URI: %s\nWas Active: %v",
				location.ID.String(), location.Code, location.Name, location.Provider, location.BaseUri, location.IsActive,
			),
		})
	s.notify(ctx, notification)
	return nil
}

// notify is best effort, as before extraction.
func (s *service) notify(ctx context.Context, notification modulekit.Notification) {
	if s == nil || s.notifier == nil {
		return
	}
	if strings.TrimSpace(notification.TargetRole) == "" {
		notification.TargetRole = "admin"
	}
	_ = s.notifier.Notify(ctx, notification)
}

func (s *service) attachAdminEmailDelivery(notification *modulekit.Notification, templateName, subject, textBody string, templateData map[string]any) {
	if notification == nil {
		return
	}
	adminEmail := s.platform.AdminEmail()
	if adminEmail == "" {
		return
	}
	if templateData == nil {
		templateData = map[string]any{}
	}
	if _, ok := templateData["Name"]; !ok {
		templateData["Name"] = s.platform.AdminName()
	}
	if _, ok := templateData["Platform"]; !ok {
		templateData["Platform"] = s.platform.DisplayName()
	}
	if _, ok := templateData["ActionURL"]; !ok {
		templateData["ActionURL"] = s.adminStorageURL()
	}
	notification.Deliveries = []modulekit.NotificationDelivery{
		{
			Channel:   modulekit.ChannelInApp,
			Recipient: map[string]any{"target_role": notification.TargetRole},
			Payload: map[string]any{
				"title": notification.Title, "message": notification.Message,
				"type": notification.Type, "severity": notification.Severity,
			},
			MaxAttempts: 1,
		},
		{
			Channel:      modulekit.ChannelEmail,
			Recipient:    map[string]any{"name": s.platform.AdminName(), "email": adminEmail},
			TemplateName: templateName,
			TemplateData: templateData,
			Payload:      map[string]any{"subject": subject, "text_body": textBody},
			MaxAttempts:  5,
		},
	}
}

func (s *service) adminStorageURL() string { return s.platform.AdminPageURL("storage-locations") }

func mustJSON(value any) json.RawMessage {
	body, _ := json.Marshal(value)
	return body
}
