package documents

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// Notification types and the shared title/severity the host gave them.
const (
	notificationDocumentCreated = "DOCUMENT_CREATED"
	notificationDocumentEdited  = "DOCUMENT_EDITED"
	notificationDocumentDeleted = "DOCUMENT_DELETED"
	notificationTitle           = "System notification"
	notificationSeverity        = "info"
)

type CreateDocumentInput struct {
	OriginalFilename string
	ContentType      string
	SizeBytes        int64
	ChecksumSHA256   *string
	StorageLocation  uuid.UUID
	ObjectKey        string
	UploadedBy       uuid.UUID
	ProcessType      ProcessType
	QueueProcessing  bool
	Status           string
	IsTemplate       bool
	Metadata         map[string]any
	HealthContextID  *uuid.UUID
}

type EditDocumentInput struct {
	ID               uuid.UUID
	OriginalFilename string
	ContentType      string
}

// Service manages documents and their processing jobs.
type Service struct {
	repo     Repository
	notifier modulekit.Notifier
	platform modulekit.Platform
}

// NewService builds the service. notifier may be nil.
func NewService(repo Repository, notifier modulekit.Notifier, platform modulekit.Platform) *Service {
	return &Service{repo: repo, notifier: notifier, platform: platform}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (s *Service) ready() error {
	if s == nil {
		return errors.New("document service is nil")
	}
	if s.repo == nil {
		return errors.New("document repository is nil")
	}
	return nil
}

func (s *Service) CreateDocument(ctx context.Context, input CreateDocumentInput) (Document, error) {
	if err := s.ready(); err != nil {
		return Document{}, err
	}

	if strings.TrimSpace(input.OriginalFilename) == "" {
		return Document{}, errors.New("original filename is required")
	}

	if input.StorageLocation == uuid.Nil {
		return Document{}, errors.New("storage location is required")
	}

	if input.UploadedBy == uuid.Nil {
		return Document{}, errors.New("uploaded by is required")
	}

	if strings.TrimSpace(input.ObjectKey) == "" {
		return Document{}, errors.New("object key is required")
	}

	docID := uuid.New()

	var checksum sql.NullString
	if input.ChecksumSHA256 != nil && strings.TrimSpace(*input.ChecksumSHA256) != "" {
		checksum = sql.NullString{
			String: strings.TrimSpace(*input.ChecksumSHA256),
			Valid:  true,
		}
	}

	processable := requiresProcessing(input.ContentType, input.OriginalFilename)
	queueProcessing := input.QueueProcessing && processable && !input.IsTemplate

	if input.QueueProcessing && !queueProcessing {
		return Document{}, errors.New("processing can only be queued for processable non-template documents")
	}

	status := input.Status
	if input.Status == "" {
		if queueProcessing {
			status = DocumentStatusPending
		} else {
			status = DocumentStatusCompleted
		}
	}

	if queueProcessing && !input.ProcessType.IsValid() {
		return Document{}, errors.New("invalid process type for processable document")
	}

	metadata := json.RawMessage([]byte(`{}`))
	if len(input.Metadata) > 0 {
		raw, err := json.Marshal(input.Metadata)
		if err != nil {
			return Document{}, fmt.Errorf("marshal document metadata: %w", err)
		}

		metadata = raw
	}

	doc, err := s.repo.CreateDocument(ctx, NewDocument{
		ID:               docID,
		OriginalFilename: strings.TrimSpace(input.OriginalFilename),
		ContentType: sql.NullString{
			String: strings.TrimSpace(input.ContentType),
			Valid:  strings.TrimSpace(input.ContentType) != "",
		},
		SizeBytes:         input.SizeBytes,
		ChecksumSha256:    checksum,
		StorageLocationID: input.StorageLocation,
		ObjectKey:         strings.TrimSpace(input.ObjectKey),
		UploadedBy:        input.UploadedBy,
		Status:            status,
		Metadata:          metadata,
		IsTemplate:        input.IsTemplate,
		HealthContextID:   nullableUUID(input.HealthContextID),
	})
	if err != nil {
		return Document{}, fmt.Errorf("create document: %w", err)
	}

	if queueProcessing {
		_, err = s.repo.CreateProcess(ctx, NewProcess{
			ID:          uuid.New(),
			DocumentID:  doc.ID,
			ProcessType: string(input.ProcessType),
			CreatedBy:   input.UploadedBy,
		})
		if err != nil {
			return Document{}, fmt.Errorf("create document process: %w", err)
		}
	}

	notification := modulekit.Notification{
		Type:       notificationDocumentCreated,
		Title:      notificationTitle,
		Severity:   notificationSeverity,
		Message:    buildDocumentCreatedMessage(queueProcessing, input.IsTemplate),
		TargetRole: "admin",
		UserID:     input.UploadedBy.String(),
		Metadata: mustJSON(map[string]any{
			"document_id":       doc.ID.String(),
			"filename":          doc.OriginalFilename,
			"content_type":      nullStringValue(doc.ContentType),
			"size_bytes":        doc.SizeBytes,
			"status":            doc.Status,
			"object_key":        doc.ObjectKey,
			"uploaded_by":       input.UploadedBy.String(),
			"is_template":       doc.IsTemplate,
			"needs_processing":  queueProcessing,
			"process_type":      string(input.ProcessType),
			"storage_location":  input.StorageLocation.String(),
			"checksum_provided": checksum.Valid,
		}),
	}

	// Email only when the document has been queued for processing.
	// Simple uploads and template uploads remain in-app only.
	if queueProcessing {
		s.attachAdminEmailDelivery(
			&notification,
			"document-created",
			"Document created and queued for processing",
			fmt.Sprintf("Document %s was created and queued for processing.", doc.OriginalFilename),
			map[string]any{
				"Name":         s.platform.AdminName(),
				"Platform":     s.platform.DisplayName(),
				"DocumentName": doc.OriginalFilename,
				"DocumentType": nullStringValue(doc.ContentType),
				"Status":       doc.Status,
				"ActionURL":    s.adminDocumentsURL(),
				"Details": fmt.Sprintf(
					"Document ID: %s\nFilename: %s\nContent Type: %s\nProcess Type: %s\nUploaded By: %s\nObject Key: %s",
					doc.ID.String(),
					doc.OriginalFilename,
					nullStringValue(doc.ContentType),
					string(input.ProcessType),
					input.UploadedBy.String(),
					doc.ObjectKey,
				),
			},
		)
	}

	s.notify(ctx, notification)

	return doc, nil
}

func (s *Service) GetDocumentInScope(ctx context.Context, id uuid.UUID, scope *HealthContextScope) (Document, error) {
	if scope == nil {
		return s.GetDocument(ctx, id)
	}
	if scope.ID == uuid.Nil {
		return Document{}, errors.New("health context id is required")
	}
	doc, err := s.repo.GetDocumentInHealthContext(ctx, id, *scope)
	if err != nil {
		return Document{}, fmt.Errorf("get document in health context: %w", err)
	}
	return doc, nil
}

func (s *Service) GetDocument(ctx context.Context, id uuid.UUID) (Document, error) {
	if err := s.ready(); err != nil {
		return Document{}, err
	}

	if id == uuid.Nil {
		return Document{}, errors.New("document id is required")
	}

	doc, err := s.repo.GetDocument(ctx, id)
	if err != nil {
		return Document{}, fmt.Errorf("get document: %w", err)
	}

	return doc, nil
}

func (s *Service) EditDocument(ctx context.Context, input EditDocumentInput) (Document, error) {
	if err := s.ready(); err != nil {
		return Document{}, err
	}

	if input.ID == uuid.Nil {
		return Document{}, errors.New("document id is required")
	}

	if strings.TrimSpace(input.OriginalFilename) == "" {
		return Document{}, errors.New("original filename is required")
	}

	doc, err := s.repo.EditDocument(ctx, DocumentEdit{
		ID:               input.ID,
		OriginalFilename: strings.TrimSpace(input.OriginalFilename),
		ContentType: sql.NullString{
			String: strings.TrimSpace(input.ContentType),
			Valid:  strings.TrimSpace(input.ContentType) != "",
		},
	})
	if err != nil {
		return Document{}, fmt.Errorf("edit document: %w", err)
	}

	// In-app only.
	s.notify(ctx, modulekit.Notification{
		Type:       notificationDocumentEdited,
		Title:      notificationTitle,
		Severity:   notificationSeverity,
		Message:    "Document edited",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"document_id":  doc.ID.String(),
			"filename":     doc.OriginalFilename,
			"content_type": nullStringValue(doc.ContentType),
			"status":       doc.Status,
		}),
	})

	return doc, nil
}

func (s *Service) EditDocumentInScope(ctx context.Context, input EditDocumentInput, scope *HealthContextScope) (Document, error) {
	if _, err := s.GetDocumentInScope(ctx, input.ID, scope); err != nil {
		return Document{}, err
	}
	return s.EditDocument(ctx, input)
}

func (s *Service) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	if err := s.ready(); err != nil {
		return err
	}

	if id == uuid.Nil {
		return errors.New("document id is required")
	}

	doc, err := s.repo.GetDocument(ctx, id)
	if err != nil {
		return fmt.Errorf("get document before delete: %w", err)
	}

	if err := s.repo.DeleteDocument(ctx, id); err != nil {
		return fmt.Errorf("delete document: %w", err)
	}

	// In-app only. Add email here later if document deletion must be escalated.
	s.notify(ctx, modulekit.Notification{
		Type:       notificationDocumentDeleted,
		Title:      notificationTitle,
		Severity:   notificationSeverity,
		Message:    "Document deleted",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"document_id":  id.String(),
			"filename":     doc.OriginalFilename,
			"content_type": nullStringValue(doc.ContentType),
			"status":       doc.Status,
			"object_key":   doc.ObjectKey,
			"is_template":  doc.IsTemplate,
		}),
	})

	return nil
}

func (s *Service) DeleteDocumentInScope(ctx context.Context, id uuid.UUID, scope *HealthContextScope) error {
	if _, err := s.GetDocumentInScope(ctx, id, scope); err != nil {
		return err
	}
	return s.DeleteDocument(ctx, id)
}

func (s *Service) ListDocuments(ctx context.Context, page Pagination) ([]Document, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}

	return s.repo.ListDocuments(ctx, page)
}

func (s *Service) ListDocumentsInScope(ctx context.Context, scope *HealthContextScope, page Pagination) ([]Document, error) {
	if scope == nil {
		return s.ListDocuments(ctx, page)
	}
	if scope.ID == uuid.Nil {
		return nil, errors.New("health context id is required")
	}
	return s.repo.ListDocumentsInHealthContext(ctx, *scope, page)
}

// Stats summarises non-template documents in the scope (all when nil).
func (s *Service) Stats(ctx context.Context, scope *HealthContextScope) (Stats, error) {
	if err := s.ready(); err != nil {
		return Stats{}, err
	}
	return s.repo.Stats(ctx, scope)
}

func (s *Service) ListProcessesByDocument(ctx context.Context, documentID string) ([]Process, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}

	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document id: %w", err)
	}

	_, err = s.repo.GetDocument(ctx, docUUID)
	if err != nil {
		return nil, fmt.Errorf("get document before listing processes: %w", err)
	}

	return s.repo.ListProcessesByDocument(ctx, docUUID)
}

func (s *Service) ListProcessesByDocumentInScope(ctx context.Context, documentID string, scope *HealthContextScope) ([]Process, error) {
	docUUID, err := uuid.Parse(documentID)
	if err != nil {
		return nil, fmt.Errorf("invalid document id: %w", err)
	}
	if _, err = s.GetDocumentInScope(ctx, docUUID, scope); err != nil {
		return nil, err
	}
	return s.repo.ListProcessesByDocument(ctx, docUUID)
}

func (s *Service) Reprocess(ctx context.Context, documentID uuid.UUID) error {
	if err := s.ready(); err != nil {
		return err
	}

	if documentID == uuid.Nil {
		return errors.New("document id is required")
	}

	doc, err := s.repo.GetDocument(ctx, documentID)
	if err != nil {
		return fmt.Errorf("get document before reprocess: %w", err)
	}

	if !requiresProcessing(
		nullStringValue(doc.ContentType),
		doc.OriginalFilename,
	) {
		return fmt.Errorf("document type does not support processing")
	}

	proc, err := s.repo.GetLatestProcessByDocumentID(ctx, documentID)
	if err != nil {
		return fmt.Errorf("get latest process by document id: %w", err)
	}

	if proc.Status == ProcessStatusProcessing {
		return fmt.Errorf("document already processing")
	}

	_, err = s.repo.MarkDocumentPending(ctx, documentID)
	if err != nil {
		return fmt.Errorf("mark document pending: %w", err)
	}

	_, err = s.repo.CreateProcess(ctx, NewProcess{
		ID:          uuid.New(),
		DocumentID:  documentID,
		ProcessType: proc.ProcessType,
		CreatedBy:   proc.CreatedBy,
	})
	if err != nil {
		return fmt.Errorf("create reprocess process: %w", err)
	}

	notification := modulekit.Notification{
		Type:       notificationDocumentEdited,
		Title:      "Document reprocessing queued",
		Severity:   notificationSeverity,
		Message:    "Document reprocessing queued",
		TargetRole: "admin",
		UserID:     proc.CreatedBy.String(),
		Metadata: mustJSON(map[string]any{
			"document_id":  doc.ID.String(),
			"filename":     doc.OriginalFilename,
			"content_type": nullStringValue(doc.ContentType),
			"process_type": proc.ProcessType,
			"created_by":   proc.CreatedBy.String(),
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"document-created",
		"Document reprocessing queued",
		fmt.Sprintf("Document %s was queued for reprocessing.", doc.OriginalFilename),
		map[string]any{
			"Name":         s.platform.AdminName(),
			"Platform":     s.platform.DisplayName(),
			"DocumentName": doc.OriginalFilename,
			"DocumentType": nullStringValue(doc.ContentType),
			"Status":       "PENDING",
			"ActionURL":    s.adminDocumentsURL(),
			"Details": fmt.Sprintf(
				"Document ID: %s\nFilename: %s\nContent Type: %s\nProcess Type: %s\nQueued By: %s",
				doc.ID.String(),
				doc.OriginalFilename,
				nullStringValue(doc.ContentType),
				proc.ProcessType,
				proc.CreatedBy.String(),
			),
		},
	)

	s.notify(ctx, notification)

	return nil
}

func (s *Service) ReprocessInScope(ctx context.Context, documentID uuid.UUID, scope *HealthContextScope) error {
	if _, err := s.GetDocumentInScope(ctx, documentID, scope); err != nil {
		return err
	}
	return s.Reprocess(ctx, documentID)
}

func (s *Service) MarkDocumentProcessing(ctx context.Context, documentID uuid.UUID) (Document, error) {
	if err := s.ready(); err != nil {
		return Document{}, err
	}

	if documentID == uuid.Nil {
		return Document{}, errors.New("document id is required")
	}

	doc, err := s.repo.MarkDocumentProcessing(ctx, documentID)
	if err != nil {
		return Document{}, fmt.Errorf("mark document processing: %w", err)
	}

	return doc, nil
}

func (s *Service) MarkDocumentCompleted(ctx context.Context, documentID uuid.UUID) (Document, error) {
	if err := s.ready(); err != nil {
		return Document{}, err
	}

	if documentID == uuid.Nil {
		return Document{}, errors.New("document id is required")
	}

	doc, err := s.repo.MarkDocumentCompleted(ctx, documentID)
	if err != nil {
		return Document{}, fmt.Errorf("mark document completed: %w", err)
	}

	notification := modulekit.Notification{
		Type:       notificationDocumentEdited,
		Title:      "Document processing completed",
		Severity:   "info",
		Message:    "Document processing completed",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"document_id":  doc.ID.String(),
			"filename":     doc.OriginalFilename,
			"content_type": nullStringValue(doc.ContentType),
			"status":       doc.Status,
			"object_key":   doc.ObjectKey,
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"document-processed",
		"Document processing completed",
		fmt.Sprintf("Document %s was processed successfully.", doc.OriginalFilename),
		map[string]any{
			"Name":         s.platform.AdminName(),
			"Platform":     s.platform.DisplayName(),
			"DocumentName": doc.OriginalFilename,
			"DocumentType": nullStringValue(doc.ContentType),
			"ActionURL":    s.adminDocumentsURL(),
			"Details": fmt.Sprintf(
				"Document ID: %s\nFilename: %s\nContent Type: %s\nStatus: %s\nObject Key: %s",
				doc.ID.String(),
				doc.OriginalFilename,
				nullStringValue(doc.ContentType),
				doc.Status,
				doc.ObjectKey,
			),
		},
	)

	s.notify(ctx, notification)

	return doc, nil
}

func (s *Service) MarkDocumentFailed(ctx context.Context, documentID uuid.UUID) (Document, error) {
	if err := s.ready(); err != nil {
		return Document{}, err
	}

	if documentID == uuid.Nil {
		return Document{}, errors.New("document id is required")
	}

	doc, err := s.repo.MarkDocumentFailed(ctx, documentID)
	if err != nil {
		return Document{}, fmt.Errorf("mark document failed: %w", err)
	}

	notification := modulekit.Notification{
		Type:       notificationDocumentDeleted,
		Title:      "Document processing failed",
		Severity:   "critical",
		Message:    "Document processing failed",
		TargetRole: "admin",
		Metadata: mustJSON(map[string]any{
			"document_id":  doc.ID.String(),
			"filename":     doc.OriginalFilename,
			"content_type": nullStringValue(doc.ContentType),
			"status":       doc.Status,
			"object_key":   doc.ObjectKey,
		}),
	}

	s.attachAdminEmailDelivery(
		&notification,
		"document-failed",
		"Document processing failed",
		fmt.Sprintf("Document %s failed during processing.", doc.OriginalFilename),
		map[string]any{
			"Name":         s.platform.AdminName(),
			"Platform":     s.platform.DisplayName(),
			"DocumentName": doc.OriginalFilename,
			"DocumentType": nullStringValue(doc.ContentType),
			"Reason":       "Processing failed. Please review process logs for details.",
			"ActionURL":    s.adminDocumentsURL(),
			"Details": fmt.Sprintf(
				"Document ID: %s\nFilename: %s\nContent Type: %s\nStatus: %s\nObject Key: %s",
				doc.ID.String(),
				doc.OriginalFilename,
				nullStringValue(doc.ContentType),
				doc.Status,
				doc.ObjectKey,
			),
		},
	)

	s.notify(ctx, notification)

	return doc, nil
}

func (s *Service) notify(ctx context.Context, notification modulekit.Notification) {
	if s == nil || s.notifier == nil {
		return
	}

	if strings.TrimSpace(notification.TargetRole) == "" {
		notification.TargetRole = "admin"
	}

	_ = s.notifier.Notify(ctx, notification)
}

func (s *Service) attachAdminEmailDelivery(
	notification *modulekit.Notification,
	templateName string,
	subject string,
	textBody string,
	templateData map[string]any,
) {
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
		templateData["ActionURL"] = s.adminDocumentsURL()
	}

	notification.Deliveries = []modulekit.NotificationDelivery{
		{
			Channel: modulekit.ChannelInApp,
			Recipient: map[string]any{
				"target_role": notification.TargetRole,
			},
			Payload: map[string]any{
				"title":    notification.Title,
				"message":  notification.Message,
				"type":     notification.Type,
				"severity": notification.Severity,
			},
			MaxAttempts: 1,
		},
		{
			Channel: modulekit.ChannelEmail,
			Recipient: map[string]any{
				"name":  s.platform.AdminName(),
				"email": adminEmail,
			},
			TemplateName: templateName,
			TemplateData: templateData,
			Payload: map[string]any{
				"subject":   subject,
				"text_body": textBody,
			},
			MaxAttempts: 5,
		},
	}
}

func (s *Service) adminDocumentsURL() string { return s.platform.AdminPageURL("documents") }

func buildDocumentCreatedMessage(needsProcessing bool, isTemplate bool) string {
	if isTemplate {
		return "Document template created"
	}

	if needsProcessing {
		return "Document created and queued for processing"
	}

	return "Document created"
}
