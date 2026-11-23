package documents

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/internal/model"
	sharedservice "github.com/moh-sso-dashboard/internal/service"
	"github.com/moh-sso-dashboard/internal/storage"
	storagelocationfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/storage-locations"
	"github.com/xuri/excelize/v2"
)

type Handler struct {
	documentService        *Service
	auditService           *sharedservice.AuditService
	storageLocationService storagelocationfeature.Service
	storage                storage.Storage
	storageFactory         *storage.StorageFactory
	db                     *sql.DB
	remoteDB               *sql.DB
}

func selectedHealthContextScope(c *gin.Context) *HealthContextScope {
	value, exists := c.Get("health_context_id")
	if !exists {
		return nil
	}
	contextID, ok := value.(uuid.UUID)
	if !ok || contextID == uuid.Nil {
		return nil
	}

	scopeMode, _ := c.Get("health_context_scope_mode")
	return &HealthContextScope{
		ID: contextID,
		IncludeDescendants: strings.EqualFold(
			strings.TrimSpace(fmt.Sprint(scopeMode)),
			"NODE_AND_DESCENDANTS",
		),
	}
}

func selectedHealthContextID(c *gin.Context) *uuid.UUID {
	scope := selectedHealthContextScope(c)
	if scope == nil {
		return nil
	}
	return &scope.ID
}

func NewHandler(
	documentService *Service,
	auditService *sharedservice.AuditService,
	storageLocationService storagelocationfeature.Service,
	storage storage.Storage,
	storageFactory *storage.StorageFactory,
	db *sql.DB,
	remoteDB *sql.DB,
) *Handler {
	return &Handler{
		documentService:        documentService,
		auditService:           auditService,
		storageLocationService: storageLocationService,
		storage:                storage,
		storageFactory:         storageFactory,
		db:                     db,
		remoteDB:               remoteDB,
	}
}

func (h *Handler) CreateDocument(c *gin.Context) {
	ctx := c.Request.Context()

	userIDStr := c.GetString("user_id")

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "INVALID_USER", "invalid user id")
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_MULTIPART", "invalid multipart form")
		return
	}

	storageLocationStr := c.PostForm("storage_location")

	storageLocationID, err := uuid.Parse(storageLocationStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_STORAGE_LOCATION", "invalid storage location")
		return
	}

	isTemplate := strings.EqualFold(
		strings.TrimSpace(c.PostForm("is_template")),
		"true",
	)

	metadata := map[string]any{}

	metadataRaw := strings.TrimSpace(c.PostForm("metadata"))

	if metadataRaw != "" {
		if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
			response.Fail(
				c,
				http.StatusBadRequest,
				"INVALID_METADATA",
				"metadata must be valid JSON",
			)
			return
		}
	}

	templateCode := ""
	if value, ok := metadata["template_code"].(string); ok {
		templateCode = strings.TrimSpace(value)
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "FILE_REQUIRED", "file is required")
		return
	}
	defer file.Close()

	if header.Size == 0 {
		response.Fail(c, http.StatusBadRequest, "EMPTY_FILE", "file is empty")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	queueProcessing := shouldQueueProcessing(
		contentType,
		header.Filename,
		isTemplate,
		templateCode,
	)

	var processType model.ProcessType

	if queueProcessing {

		processTypeValue := strings.TrimSpace(c.PostForm("process_type"))

		if processTypeValue == "" {
			response.Fail(
				c,
				http.StatusBadRequest,
				"PROCESS_TYPE_REQUIRED",
				"process type is required for this file type",
			)
			return
		}

		parsedProcessType, ok := model.ParseProcessType(processTypeValue)
		if !ok {
			response.Fail(
				c,
				http.StatusBadRequest,
				"INVALID_PROCESS_TYPE",
				"invalid process type",
			)
			return
		}

		processType = parsedProcessType
	}

	loc, err := h.storageLocationService.GetByID(ctx, storageLocationStr)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_STORAGE", "request failed")
		return
	}

	storageProvider, err := h.storageFactory.Get(loc.Provider)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_PROVIDER", "request failed")
		return
	}

	objectKey := uuid.New().String()

	hasher := sha256.New()
	teeReader := io.TeeReader(file, hasher)

	if err := storageProvider.Upload(
		ctx,
		objectKey,
		teeReader,
		header.Size,
		contentType,
	); err != nil {
		response.Fail(
			c,
			http.StatusInternalServerError,
			"UPLOAD_FAILED",
			"failed to upload file",
		)
		return
	}

	checksumStr := hex.EncodeToString(hasher.Sum(nil))

	status := DocumentStatusCompleted
	if queueProcessing {
		status = DocumentStatusPending
	}

	if templateCode != "" {
		metadata["template_code"] = templateCode
	}

	input := CreateDocumentInput{
		OriginalFilename: header.Filename,
		ContentType:      contentType,
		SizeBytes:        header.Size,
		ChecksumSHA256:   &checksumStr,
		StorageLocation:  storageLocationID,
		ObjectKey:        objectKey,
		UploadedBy:       userID,
		Status:           status,
		IsTemplate:       isTemplate,
		QueueProcessing:  queueProcessing,
		Metadata:         metadata,
		HealthContextID:  selectedHealthContextID(c),
	}

	if queueProcessing {
		input.ProcessType = processType
	}

	doc, err := h.documentService.CreateDocument(ctx, input)
	if err != nil {
		_ = storageProvider.Delete(ctx, objectKey)

		response.Fail(
			c,
			http.StatusInternalServerError,
			"CREATE_FAILED",
			"request failed",
		)
		return
	}

	objectURL, _ := storageProvider.GetObjectURL(ctx, objectKey)
	viewURL, _ := storageProvider.GetViewURL(ctx, objectKey, header.Filename)
	downloadURL, _ := storageProvider.GetDownloadURL(ctx, objectKey, header.Filename)

	response.OK(
		c,
		http.StatusCreated,
		toDocumentResponse(doc, objectURL, viewURL, downloadURL),
	)
}

func (h *Handler) GetDocument(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	doc, err := h.documentService.GetDocumentInScope(ctx, id, selectedHealthContextScope(c))
	if err != nil {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "request failed")
		return
	}

	loc, err := h.storageLocationService.GetByID(ctx, doc.StorageLocationID.String())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "STORAGE_LOOKUP_FAILED", "request failed")
		return
	}

	storageProvider, err := h.storageFactory.Get(loc.Provider)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INVALID_PROVIDER", "request failed")
		return
	}

	objectURL, _ := storageProvider.GetObjectURL(ctx, doc.ObjectKey)
	viewURL, _ := storageProvider.GetViewURL(ctx, doc.ObjectKey, doc.OriginalFilename)
	downloadURL, _ := storageProvider.GetDownloadURL(ctx, doc.ObjectKey, doc.OriginalFilename)

	response.OK(c, http.StatusOK, toDocumentResponse(doc, objectURL, viewURL, downloadURL))
}

func (h *Handler) EditDocument(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	var req UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "request failed")
		return
	}

	if req.OriginalFilename == nil || req.ContentType == nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "original_filename and content_type are required")
		return
	}

	doc, err := h.documentService.EditDocumentInScope(ctx, EditDocumentInput{
		ID:               id,
		OriginalFilename: *req.OriginalFilename,
		ContentType:      *req.ContentType,
	}, selectedHealthContextScope(c))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "UPDATE_FAILED", "request failed")
		return
	}

	loc, err := h.storageLocationService.GetByID(ctx, doc.StorageLocationID.String())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "STORAGE_LOOKUP_FAILED", "request failed")
		return
	}

	storageProvider, err := h.storageFactory.Get(loc.Provider)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INVALID_PROVIDER", "request failed")
		return
	}

	objectURL, _ := storageProvider.GetObjectURL(ctx, doc.ObjectKey)
	viewURL, _ := storageProvider.GetViewURL(ctx, doc.ObjectKey, doc.OriginalFilename)
	downloadURL, _ := storageProvider.GetDownloadURL(ctx, doc.ObjectKey, doc.OriginalFilename)

	response.OK(c, http.StatusOK, toDocumentResponse(doc, objectURL, viewURL, downloadURL))
}

func (h *Handler) DeleteDocument(c *gin.Context) {
	idParam := c.Param("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	if err := h.documentService.DeleteDocumentInScope(
		c.Request.Context(),
		id,
		selectedHealthContextScope(c),
	); err != nil {
		response.Fail(c, http.StatusInternalServerError, "DELETE_FAILED", "request failed")
		return
	}

	response.OK(c, http.StatusOK, struct {
		Message string `json:"message"`
	}{Message: "document deleted successfully"})
}

func (h *Handler) DownloadDocument(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	uid, err := uuid.Parse(id)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "invalid uuid")
		return
	}

	doc, err := h.documentService.GetDocumentInScope(ctx, uid, selectedHealthContextScope(c))
	if err != nil {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "document not found")
		return
	}

	loc, err := h.storageLocationService.GetByID(ctx, doc.StorageLocationID.String())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "STORAGE_LOOKUP_FAILED", "request failed")
		return
	}

	storageProvider, err := h.storageFactory.Get(loc.Provider)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INVALID_PROVIDER", "request failed")
		return
	}

	reader, err := storageProvider.Download(ctx, doc.ObjectKey)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "DOWNLOAD_FAILED", "failed to retrieve file")
		return
	}
	defer reader.Close()

	contentType := nullStringValue(doc.ContentType)
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, doc.OriginalFilename))
	c.Header("Content-Type", contentType)

	_, _ = io.Copy(c.Writer, reader)
}

func (h *Handler) ViewDocument(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	uid, err := uuid.Parse(id)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "invalid uuid")
		return
	}

	doc, err := h.documentService.GetDocumentInScope(ctx, uid, selectedHealthContextScope(c))
	if err != nil {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "document not found")
		return
	}

	loc, err := h.storageLocationService.GetByID(ctx, doc.StorageLocationID.String())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "STORAGE_LOOKUP_FAILED", "request failed")
		return
	}

	storageProvider, err := h.storageFactory.Get(loc.Provider)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INVALID_PROVIDER", "request failed")
		return
	}

	reader, err := storageProvider.Download(ctx, doc.ObjectKey)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "VIEW_FAILED", "failed to retrieve file")
		return
	}
	defer reader.Close()

	contentType := nullStringValue(doc.ContentType)
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, doc.OriginalFilename))
	c.Header("Content-Type", contentType)

	_, _ = io.Copy(c.Writer, reader)
}

func (h *Handler) GetDocumentStats(c *gin.Context) {
	ctx := c.Request.Context()

	query := `
		SELECT
			COUNT(*)                                              AS total,
			COALESCE(SUM(size_bytes), 0)                         AS total_size,
			COUNT(*) FILTER (WHERE status = 'PENDING')           AS pending,
			COUNT(*) FILTER (WHERE status = 'PROCESSING')        AS processing,
			COUNT(*) FILTER (WHERE status = 'COMPLETED')         AS completed,
			COUNT(*) FILTER (WHERE status = 'FAILED')            AS failed
		FROM documents
		WHERE is_template = false
	`
	args := []any{}
	if scope := selectedHealthContextScope(c); scope != nil {
		query += `
			AND (
				health_context_id = $1
				OR (
					$2::boolean
					AND EXISTS (
						SELECT 1
						FROM health_context_closure hc
						WHERE hc.ancestor_id = $1
						  AND hc.descendant_id = documents.health_context_id
					)
				)
			)
		`
		args = append(args, scope.ID, scope.IncludeDescendants)
	}
	row := h.db.QueryRowContext(ctx, query, args...)

	var stats struct {
		Total      int64 `json:"total"`
		TotalSize  int64 `json:"total_size"`
		Pending    int64 `json:"pending"`
		Processing int64 `json:"processing"`
		Completed  int64 `json:"completed"`
		Failed     int64 `json:"failed"`
	}

	if err := row.Scan(
		&stats.Total, &stats.TotalSize,
		&stats.Pending, &stats.Processing,
		&stats.Completed, &stats.Failed,
	); err != nil {
		response.Fail(c, http.StatusInternalServerError, "STATS_FAILED", "failed to compute stats")
		return
	}

	response.OK(c, http.StatusOK, stats)
}

func (h *Handler) ListDocuments(c *gin.Context) {
	ctx := c.Request.Context()

	limitStr := c.DefaultQuery("limit", "1000")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	page := model.Pagination{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	docs, err := h.documentService.ListDocumentsInScope(ctx, selectedHealthContextScope(c), page)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "LIST_FAILED", "failed to list documents")
		return
	}

	out := make([]DocumentResponse, 0, len(docs))
	for _, doc := range docs {
		objectURL := ""
		viewURL := ""
		downloadURL := ""

		loc, err := h.storageLocationService.GetByID(ctx, doc.StorageLocationID.String())
		if err == nil {
			storageProvider, err := h.storageFactory.Get(loc.Provider)
			if err == nil {
				objectURL, _ = storageProvider.GetObjectURL(ctx, doc.ObjectKey)
				viewURL, _ = storageProvider.GetViewURL(ctx, doc.ObjectKey, doc.OriginalFilename)
				downloadURL, _ = storageProvider.GetDownloadURL(ctx, doc.ObjectKey, doc.OriginalFilename)
			}
		}

		out = append(out, toDocumentResponse(doc, objectURL, viewURL, downloadURL))
	}

	response.OK(c, http.StatusOK, out)
}

func (h *Handler) ListDocumentProcesses(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	if idParam == "" {
		response.Fail(c, http.StatusBadRequest, "INVALID_DOCUMENT_ID", "document id is required")
		return
	}

	processes, err := h.documentService.ListProcessesByDocumentInScope(
		ctx,
		idParam,
		selectedHealthContextScope(c),
	)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "PROCESS_LIST_FAILED", "failed to list processes")
		return
	}

	out := make([]ProcessResponse, 0, len(processes))
	for _, p := range processes {
		out = append(out, toProcessResponse(p))
	}

	response.OK(c, http.StatusOK, out)
}

func (h *Handler) ReprocessDocument(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "error", "invalid document id")
		return
	}

	err = h.documentService.ReprocessInScope(
		c.Request.Context(),
		documentID,
		selectedHealthContextScope(c),
	)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "error", "request failed")
		return
	}

	response.OK(c, http.StatusOK, struct {
		Message string `json:"message"`
	}{Message: "reprocessing started"})
}

// ── Data Preview ──────────────────────────────────────────────────────────────

type previewSheet struct {
	Code     string                   `json:"code"`
	Name     string                   `json:"name"`
	RowCount int                      `json:"row_count"`
	Columns  []string                 `json:"columns"`
	Rows     []map[string]interface{} `json:"rows"`
}

type dataPreviewResponse struct {
	TemplateCode string         `json:"template_code"`
	ReportDate   string         `json:"report_date,omitempty"`
	Sheets       []previewSheet `json:"sheets"`
}

func (h *Handler) DataPreview(c *gin.Context) {
	ctx := c.Request.Context()

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	doc, err := h.documentService.GetDocumentInScope(
		ctx,
		documentID,
		selectedHealthContextScope(c),
	)
	if err != nil {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "document not found")
		return
	}

	var meta map[string]string
	if len(doc.Metadata) > 0 {
		_ = json.Unmarshal(doc.Metadata, &meta)
	}

	templateCode := strings.ToUpper(strings.TrimSpace(meta["template_code"]))
	reportDate := meta["report_date"]

	// No early return when templateCode is empty: ad-hoc (no-template) CSV
	// uploads still have real rows in import.custom_data_files, keyed by
	// document_id rather than template_code. The query below is already
	// scoped by document_id, and the "no sheets found" branch further down
	// already returns an empty preview correctly when there's truly no data.
	if h.remoteDB == nil {
		response.Fail(c, http.StatusServiceUnavailable, "NO_REMOTE_DB", "remote database not configured")
		return
	}

	// Distinct sheet codes present for this document, ordered by first row_number seen
	sheetCodeRows, err := h.remoteDB.QueryContext(ctx, `
		SELECT sheet_code
		FROM import.custom_data_files
		WHERE document_id = $1 AND is_current = 'Y'
		GROUP BY sheet_code
		ORDER BY MIN(row_number)
	`, documentID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	var sheetCodes []string
	for sheetCodeRows.Next() {
		var code string
		if err := sheetCodeRows.Scan(&code); err == nil {
			sheetCodes = append(sheetCodes, code)
		}
	}
	sheetCodeRows.Close()

	if len(sheetCodes) == 0 {
		response.OK(c, http.StatusOK, dataPreviewResponse{
			TemplateCode: templateCode,
			ReportDate:   reportDate,
			Sheets:       []previewSheet{},
		})
		return
	}

	// Load ordered column keys and sheet display names from template structure (primary DB)
	type sheetMeta struct {
		displayName string
		columns     []string // ordered by column_order
	}
	sheetMetas := map[string]*sheetMeta{}

	for _, code := range sheetCodes {
		colRows, err := h.db.QueryContext(ctx, `
			SELECT dts.display_name, dtc.column_key
			FROM document_template_sheets dts
			JOIN document_template_columns dtc ON dtc.sheet_id = dts.id
			JOIN document_templates dt ON dt.id = dts.template_id
			WHERE dt.code = $1 AND dts.code = $2
			ORDER BY dtc.column_order
		`, templateCode, code)
		if err != nil {
			continue
		}
		sm := &sheetMeta{}
		for colRows.Next() {
			var displayName, columnKey string
			if err := colRows.Scan(&displayName, &columnKey); err == nil {
				sm.displayName = displayName
				sm.columns = append(sm.columns, columnKey)
			}
		}
		colRows.Close()
		if sm.displayName != "" || len(sm.columns) > 0 {
			sheetMetas[code] = sm
		}
	}

	const previewRowLimit = 500

	result := dataPreviewResponse{TemplateCode: templateCode, ReportDate: reportDate}

	for _, sheetCode := range sheetCodes {
		var totalCount int
		_ = h.remoteDB.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM import.custom_data_files
			WHERE document_id = $1 AND sheet_code = $2 AND is_current = 'Y'
		`, documentID, sheetCode).Scan(&totalCount)

		dataRows, err := h.remoteDB.QueryContext(ctx, `
			SELECT file_data, row_hash FROM import.custom_data_files
			WHERE document_id = $1 AND sheet_code = $2 AND is_current = 'Y'
			ORDER BY row_number
			LIMIT $3
		`, documentID, sheetCode, previewRowLimit)
		if err != nil {
			continue
		}

		var rows []map[string]interface{}
		var fallbackColumns []string
		for dataRows.Next() {
			var raw []byte
			var rowHash sql.NullString
			if err := dataRows.Scan(&raw, &rowHash); err != nil {
				continue
			}
			var rowData map[string]interface{}
			if err := json.Unmarshal(raw, &rowData); err != nil {
				continue
			}
			// Derive column order from first row if template structure unavailable
			if fallbackColumns == nil {
				for k := range rowData {
					fallbackColumns = append(fallbackColumns, k)
				}
			}
			// Reserved key for row identity (selection/export); never added to
			// the visible column list above, so it stays hidden from the table.
			if rowHash.Valid {
				rowData["_row_hash"] = rowHash.String
			}
			rows = append(rows, rowData)
		}
		dataRows.Close()

		if rows == nil {
			rows = []map[string]interface{}{}
		}

		// Column list: prefer template definition order, fall back to first-row keys
		columns := fallbackColumns
		displayName := sheetCode
		if sm, ok := sheetMetas[sheetCode]; ok {
			if len(sm.columns) > 0 {
				columns = sm.columns
			}
			if sm.displayName != "" {
				displayName = sm.displayName
			}
		}

		result.Sheets = append(result.Sheets, previewSheet{
			Code:     sheetCode,
			Name:     displayName,
			RowCount: totalCount,
			Columns:  columns,
			Rows:     rows,
		})
	}

	response.OK(c, http.StatusOK, result)
}

/* =========================================================
 * ExportDataPreview — download rows matching the caller's
 * current preview filter/search criteria as CSV. Runs against
 * the full server-side dataset (no row cap), unlike DataPreview.
 * ========================================================= */

var columnKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type exportDataPreviewRequest struct {
	SheetCode string              `json:"sheet_code"`
	Columns   []string            `json:"columns"`
	Search    []string            `json:"search"`
	Filters   map[string][]string `json:"filters"`
}

func (h *Handler) ExportDataPreview(c *gin.Context) {
	ctx := c.Request.Context()

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	var req exportDataPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid export request")
		return
	}

	sheetCode := strings.TrimSpace(req.SheetCode)
	if sheetCode == "" {
		response.Fail(c, http.StatusBadRequest, "MISSING_SHEET_CODE", "sheet_code is required")
		return
	}

	if h.remoteDB == nil {
		response.Fail(c, http.StatusServiceUnavailable, "NO_REMOTE_DB", "remote database not configured")
		return
	}

	// Only allow-listed column keys (from the caller-supplied columns list)
	// are interpolated into the JSONB path expressions below — column keys
	// can't be parameterized like values, so this character check plus
	// deriving the list only from the caller's own request is what keeps
	// this safe from injection.
	var columns []string
	for _, col := range req.Columns {
		if columnKeyPattern.MatchString(col) {
			columns = append(columns, col)
		}
	}
	if len(columns) == 0 {
		response.Fail(c, http.StatusBadRequest, "MISSING_COLUMNS", "at least one valid column is required")
		return
	}
	allowedColumns := make(map[string]bool, len(columns))
	for _, col := range columns {
		allowedColumns[col] = true
	}

	query := strings.Builder{}
	query.WriteString(`SELECT file_data FROM import.custom_data_files WHERE document_id = $1 AND sheet_code = $2 AND is_current = 'Y'`)
	args := []any{documentID, sheetCode}

	if len(req.Search) > 0 {
		args = append(args, pq.Array(req.Search))
		idx := len(args)
		clauses := make([]string, len(columns))
		for i, col := range columns {
			clauses[i] = fmt.Sprintf(`file_data->>'%s' = ANY($%d)`, col, idx)
		}
		query.WriteString(" AND (" + strings.Join(clauses, " OR ") + ")")
	}

	for key, values := range req.Filters {
		if !allowedColumns[key] || len(values) == 0 {
			continue
		}
		args = append(args, pq.Array(values))
		query.WriteString(fmt.Sprintf(` AND file_data->>'%s' = ANY($%d)`, key, len(args)))
	}

	query.WriteString(" ORDER BY row_number")

	rows, err := h.remoteDB.QueryContext(ctx, query.String(), args...)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-export.csv"`, sheetCode))
	c.Status(http.StatusOK)

	writer := csv.NewWriter(c.Writer)

	if err := writer.Write(columns); err != nil {
		return
	}

	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var rowData map[string]interface{}
		if err := json.Unmarshal(raw, &rowData); err != nil {
			continue
		}
		record := make([]string, len(columns))
		for i, col := range columns {
			record[i] = formatCSVValue(rowData[col])
		}
		if err := writer.Write(record); err != nil {
			return
		}
	}

	writer.Flush()
}

func formatCSVValue(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

/* =========================================================
 * ParseStructure — detect sheet/column layout of a template file
 * ========================================================= */

type detectedColumn struct {
	ColumnKey  string `json:"column_key"`
	ColumnName string `json:"column_name"`
}

type detectedSheet struct {
	Name      string           `json:"name"`
	HeaderRow int              `json:"header_row"`
	StartRow  int              `json:"start_row"`
	Columns   []detectedColumn `json:"columns"`
}

func (h *Handler) ParseStructure(c *gin.Context) {
	ctx := c.Request.Context()

	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "invalid document id")
		return
	}

	doc, err := h.documentService.GetDocumentInScope(ctx, uid, selectedHealthContextScope(c))
	if err != nil {
		response.Fail(c, http.StatusNotFound, "NOT_FOUND", "document not found")
		return
	}

	loc, err := h.storageLocationService.GetByID(ctx, doc.StorageLocationID.String())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "STORAGE_LOOKUP_FAILED", "failed to get storage location")
		return
	}

	provider, err := h.storageFactory.Get(loc.Provider)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INVALID_PROVIDER", "unsupported storage provider")
		return
	}

	reader, err := provider.Download(ctx, doc.ObjectKey)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "DOWNLOAD_FAILED", "failed to retrieve file")
		return
	}
	defer reader.Close()

	ext := strings.ToLower(filepath.Ext(doc.OriginalFilename))
	if ext != ".xlsx" && ext != ".xls" {
		response.Fail(c, http.StatusBadRequest, "UNSUPPORTED_FORMAT", "only .xlsx and .xls files are supported")
		return
	}

	sheets, err := detectSheetsFromReader(reader)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "PARSE_FAILED", "failed to parse spreadsheet: "+err.Error())
		return
	}

	response.OK(c, http.StatusOK, gin.H{"sheets": sheets})
}

/* =========================================================
 * ScanStructure — detect sheet/column layout from an uploaded file
 *   Accepts a multipart file upload, returns detected structure
 *   without creating any document record in the database.
 * ========================================================= */

func (h *Handler) ScanStructure(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "NO_FILE", "file is required")
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".xlsx" && ext != ".xls" {
		response.Fail(c, http.StatusBadRequest, "UNSUPPORTED_FORMAT", "only .xlsx and .xls files are supported")
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "READ_FAILED", "failed to open uploaded file")
		return
	}
	defer f.Close()

	sheets, err := detectSheetsFromReader(f)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "PARSE_FAILED", "failed to parse spreadsheet: "+err.Error())
		return
	}

	response.OK(c, http.StatusOK, gin.H{"sheets": sheets})
}

// detectSheetsFromReader reads an Excel file stream and returns visible sheets
// with detected header columns. Reads at most 20 rows per sheet.
func detectSheetsFromReader(r io.Reader) ([]detectedSheet, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var sheets []detectedSheet

	for _, sheetName := range f.GetSheetList() {
		visible, err := f.GetSheetVisible(sheetName)
		if err != nil || !visible {
			continue
		}

		rowStream, err := f.Rows(sheetName)
		if err != nil {
			continue
		}

		var rawRows [][]string
		for rowStream.Next() {
			cols, _ := rowStream.Columns()
			rawRows = append(rawRows, cols)
			if len(rawRows) >= 20 {
				break
			}
		}
		_ = rowStream.Close()

		// Find first row with 4+ non-empty cells
		headerIdx := -1
		for i, row := range rawRows {
			nonEmpty := 0
			for _, cell := range row {
				if strings.TrimSpace(cell) != "" {
					nonEmpty++
				}
			}
			if nonEmpty >= 4 {
				headerIdx = i
				break
			}
		}
		if headerIdx < 0 || headerIdx >= len(rawRows) {
			continue
		}

		seen := map[string]int{}
		var columns []detectedColumn
		for _, cell := range rawRows[headerIdx] {
			name := strings.TrimSpace(cell)
			if name == "" {
				continue
			}
			key := toColumnKey(name)
			if key == "" {
				continue
			}
			seen[key]++
			if seen[key] > 1 {
				key = fmt.Sprintf("%s_%d", key, seen[key])
			}
			columns = append(columns, detectedColumn{ColumnKey: key, ColumnName: name})
		}

		if len(columns) == 0 {
			continue
		}

		sheets = append(sheets, detectedSheet{
			Name:      sheetName,
			HeaderRow: headerIdx + 1,
			StartRow:  headerIdx + 2,
			Columns:   columns,
		})
	}

	if sheets == nil {
		sheets = []detectedSheet{}
	}

	return sheets, nil
}

// toColumnKey converts a column header to a snake_case database key.
func toColumnKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var out []byte
	prevSep := true
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			out = append(out, ch)
			prevSep = false
		} else if !prevSep && len(out) > 0 {
			out = append(out, '_')
			prevSep = true
		}
	}
	for len(out) > 0 && out[len(out)-1] == '_' {
		out = out[:len(out)-1]
	}
	return string(out)
}
