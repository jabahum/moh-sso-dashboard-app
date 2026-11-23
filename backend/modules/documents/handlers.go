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

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
	"github.com/xuri/excelize/v2"
)

type handlers struct {
	service   *Service
	locations StorageLocations
	stores    FileStores
	remoteDB  *sql.DB
}

func selectedHealthContextScope(c *modulekit.Context) *HealthContextScope {
	selection := c.HealthContext()
	if selection == nil {
		return nil
	}
	contextID, err := uuid.Parse(selection.NodeID)
	if err != nil || contextID == uuid.Nil {
		return nil
	}
	return &HealthContextScope{ID: contextID, IncludeDescendants: selection.IncludeDescendants}
}

func selectedHealthContextID(c *modulekit.Context) *uuid.UUID {
	scope := selectedHealthContextScope(c)
	if scope == nil {
		return nil
	}
	return &scope.ID
}

// fileStoreFor resolves the provider that holds a document's object.
func (h handlers) fileStoreFor(c *modulekit.Context, locationID string) (FileStore, error, error) {
	loc, err := h.locations.GetByID(c.Context(), locationID)
	if err != nil {
		return nil, err, nil
	}
	store, err := h.stores.Get(loc.Provider)
	return store, nil, err
}

func (h handlers) createDocument(c *modulekit.Context) {
	ctx := c.Context()

	userID, err := uuid.Parse(c.Subject())
	if err != nil {
		c.Fail(http.StatusUnauthorized, "INVALID_USER", "invalid user id")
		return
	}

	if err := c.Request.ParseMultipartForm(modulekit.MaxMultipartMemory); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_MULTIPART", "invalid multipart form")
		return
	}

	storageLocationStr := c.PostForm("storage_location")

	storageLocationID, err := uuid.Parse(storageLocationStr)
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_STORAGE_LOCATION", "invalid storage location")
		return
	}

	isTemplate := strings.EqualFold(strings.TrimSpace(c.PostForm("is_template")), "true")

	metadata := map[string]any{}

	metadataRaw := strings.TrimSpace(c.PostForm("metadata"))

	if metadataRaw != "" {
		if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
			c.Fail(http.StatusBadRequest, "INVALID_METADATA", "metadata must be valid JSON")
			return
		}
	}

	templateCode := ""
	if value, ok := metadata["template_code"].(string); ok {
		templateCode = strings.TrimSpace(value)
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.Fail(http.StatusBadRequest, "FILE_REQUIRED", "file is required")
		return
	}
	defer file.Close()

	if header.Size == 0 {
		c.Fail(http.StatusBadRequest, "EMPTY_FILE", "file is empty")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	queueProcessing := shouldQueueProcessing(contentType, header.Filename, isTemplate, templateCode)

	var processType ProcessType

	if queueProcessing {
		processTypeValue := strings.TrimSpace(c.PostForm("process_type"))

		if processTypeValue == "" {
			c.Fail(http.StatusBadRequest, "PROCESS_TYPE_REQUIRED", "process type is required for this file type")
			return
		}

		parsedProcessType, ok := ParseProcessType(processTypeValue)
		if !ok {
			c.Fail(http.StatusBadRequest, "INVALID_PROCESS_TYPE", "invalid process type")
			return
		}

		processType = parsedProcessType
	}

	loc, err := h.locations.GetByID(ctx, storageLocationStr)
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_STORAGE", "request failed")
		return
	}

	storageProvider, err := h.stores.Get(loc.Provider)
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_PROVIDER", "request failed")
		return
	}

	objectKey := uuid.New().String()

	hasher := sha256.New()
	teeReader := io.TeeReader(file, hasher)

	if err := storageProvider.Upload(ctx, objectKey, teeReader, header.Size, contentType); err != nil {
		c.Fail(http.StatusInternalServerError, "UPLOAD_FAILED", "failed to upload file")
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

	doc, err := h.service.CreateDocument(ctx, input)
	if err != nil {
		_ = storageProvider.Delete(ctx, objectKey)

		c.Fail(http.StatusInternalServerError, "CREATE_FAILED", "request failed")
		return
	}

	objectURL, _ := storageProvider.GetObjectURL(ctx, objectKey)
	viewURL, _ := storageProvider.GetViewURL(ctx, objectKey, header.Filename)
	downloadURL, _ := storageProvider.GetDownloadURL(ctx, objectKey, header.Filename)

	c.OK(http.StatusCreated, toDocumentResponse(doc, objectURL, viewURL, downloadURL))
}

// respondWithDocument resolves a document's URLs and writes it.
func (h handlers) respondWithDocument(c *modulekit.Context, doc Document) {
	ctx := c.Context()
	storageProvider, lookupErr, providerErr := h.fileStoreFor(c, doc.StorageLocationID.String())
	if lookupErr != nil {
		c.Fail(http.StatusInternalServerError, "STORAGE_LOOKUP_FAILED", "request failed")
		return
	}
	if providerErr != nil {
		c.Fail(http.StatusInternalServerError, "INVALID_PROVIDER", "request failed")
		return
	}

	objectURL, _ := storageProvider.GetObjectURL(ctx, doc.ObjectKey)
	viewURL, _ := storageProvider.GetViewURL(ctx, doc.ObjectKey, doc.OriginalFilename)
	downloadURL, _ := storageProvider.GetDownloadURL(ctx, doc.ObjectKey, doc.OriginalFilename)

	c.OK(http.StatusOK, toDocumentResponse(doc, objectURL, viewURL, downloadURL))
}

func (h handlers) getDocument(c *modulekit.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	doc, err := h.service.GetDocumentInScope(c.Context(), id, selectedHealthContextScope(c))
	if err != nil {
		c.Fail(http.StatusNotFound, "NOT_FOUND", "request failed")
		return
	}

	h.respondWithDocument(c, doc)
}

func (h handlers) editDocument(c *modulekit.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	var req UpdateDocumentRequest
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_REQUEST", "request failed")
		return
	}

	if req.OriginalFilename == nil || req.ContentType == nil {
		c.Fail(http.StatusBadRequest, "INVALID_REQUEST", "original_filename and content_type are required")
		return
	}

	doc, err := h.service.EditDocumentInScope(c.Context(), EditDocumentInput{
		ID:               id,
		OriginalFilename: *req.OriginalFilename,
		ContentType:      *req.ContentType,
	}, selectedHealthContextScope(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "UPDATE_FAILED", "request failed")
		return
	}

	h.respondWithDocument(c, doc)
}

func (h handlers) deleteDocument(c *modulekit.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	if err := h.service.DeleteDocumentInScope(c.Context(), id, selectedHealthContextScope(c)); err != nil {
		c.Fail(http.StatusInternalServerError, "DELETE_FAILED", "request failed")
		return
	}

	c.OK(http.StatusOK, messageResponse{Message: "document deleted successfully"})
}

// streamDocument writes a stored file with the given Content-Disposition type.
func (h handlers) streamDocument(c *modulekit.Context, disposition, failureCode string) {
	ctx := c.Context()

	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_UUID", "invalid uuid")
		return
	}

	doc, err := h.service.GetDocumentInScope(ctx, uid, selectedHealthContextScope(c))
	if err != nil {
		c.Fail(http.StatusNotFound, "NOT_FOUND", "document not found")
		return
	}

	storageProvider, lookupErr, providerErr := h.fileStoreFor(c, doc.StorageLocationID.String())
	if lookupErr != nil {
		c.Fail(http.StatusInternalServerError, "STORAGE_LOOKUP_FAILED", "request failed")
		return
	}
	if providerErr != nil {
		c.Fail(http.StatusInternalServerError, "INVALID_PROVIDER", "request failed")
		return
	}

	reader, err := storageProvider.Download(ctx, doc.ObjectKey)
	if err != nil {
		c.Fail(http.StatusInternalServerError, failureCode, "failed to retrieve file")
		return
	}
	defer reader.Close()

	contentType := nullStringValue(doc.ContentType)
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, doc.OriginalFilename))
	c.Header("Content-Type", contentType)

	_, _ = io.Copy(c.Writer, reader)
}

func (h handlers) downloadDocument(c *modulekit.Context) {
	h.streamDocument(c, "attachment", "DOWNLOAD_FAILED")
}

func (h handlers) viewDocument(c *modulekit.Context) {
	h.streamDocument(c, "inline", "VIEW_FAILED")
}

func (h handlers) getDocumentStats(c *modulekit.Context) {
	stats, err := h.service.Stats(c.Context(), selectedHealthContextScope(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "STATS_FAILED", "failed to compute stats")
		return
	}

	c.OK(http.StatusOK, stats)
}

func (h handlers) listDocuments(c *modulekit.Context) {
	ctx := c.Context()

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "1000"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	page := Pagination{Limit: int32(limit), Offset: int32(offset)}

	docs, err := h.service.ListDocumentsInScope(ctx, selectedHealthContextScope(c), page)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "LIST_FAILED", "failed to list documents")
		return
	}

	out := make([]DocumentResponse, 0, len(docs))
	for _, doc := range docs {
		objectURL := ""
		viewURL := ""
		downloadURL := ""

		loc, err := h.locations.GetByID(ctx, doc.StorageLocationID.String())
		if err == nil {
			storageProvider, err := h.stores.Get(loc.Provider)
			if err == nil {
				objectURL, _ = storageProvider.GetObjectURL(ctx, doc.ObjectKey)
				viewURL, _ = storageProvider.GetViewURL(ctx, doc.ObjectKey, doc.OriginalFilename)
				downloadURL, _ = storageProvider.GetDownloadURL(ctx, doc.ObjectKey, doc.OriginalFilename)
			}
		}

		out = append(out, toDocumentResponse(doc, objectURL, viewURL, downloadURL))
	}

	c.OK(http.StatusOK, out)
}

func (h handlers) listDocumentProcesses(c *modulekit.Context) {
	idParam := c.Param("id")
	if idParam == "" {
		c.Fail(http.StatusBadRequest, "INVALID_DOCUMENT_ID", "document id is required")
		return
	}

	processes, err := h.service.ListProcessesByDocumentInScope(c.Context(), idParam, selectedHealthContextScope(c))
	if err != nil {
		c.Fail(http.StatusInternalServerError, "PROCESS_LIST_FAILED", "failed to list processes")
		return
	}

	out := make([]ProcessResponse, 0, len(processes))
	for _, p := range processes {
		out = append(out, toProcessResponse(p))
	}

	c.OK(http.StatusOK, out)
}

func (h handlers) reprocessDocument(c *modulekit.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "error", "invalid document id")
		return
	}

	if err := h.service.ReprocessInScope(c.Context(), documentID, selectedHealthContextScope(c)); err != nil {
		c.Fail(http.StatusBadRequest, "error", "request failed")
		return
	}

	c.OK(http.StatusOK, messageResponse{Message: "reprocessing started"})
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

func (h handlers) dataPreview(c *modulekit.Context) {
	ctx := c.Context()

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	doc, err := h.service.GetDocumentInScope(ctx, documentID, selectedHealthContextScope(c))
	if err != nil {
		c.Fail(http.StatusNotFound, "NOT_FOUND", "document not found")
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
		c.Fail(http.StatusServiceUnavailable, "NO_REMOTE_DB", "remote database not configured")
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
		c.Fail(http.StatusInternalServerError, "QUERY_FAILED", err.Error())
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
		c.OK(http.StatusOK, dataPreviewResponse{
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
		displayName, columns, err := h.service.repo.TemplateSheetColumns(ctx, templateCode, code)
		if err != nil {
			continue
		}
		if displayName != "" || len(columns) > 0 {
			sheetMetas[code] = &sheetMeta{displayName: displayName, columns: columns}
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

	c.OK(http.StatusOK, result)
}

/* =========================================================
 * exportDataPreview — download rows matching the caller's
 * current preview filter/search criteria as CSV. Runs against
 * the full server-side dataset (no row cap), unlike dataPreview.
 * ========================================================= */

var columnKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type exportDataPreviewRequest struct {
	SheetCode string              `json:"sheet_code"`
	Columns   []string            `json:"columns"`
	Search    []string            `json:"search"`
	Filters   map[string][]string `json:"filters"`
}

func (h handlers) exportDataPreview(c *modulekit.Context) {
	ctx := c.Context()

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_ID", "invalid document id")
		return
	}

	var req exportDataPreviewRequest
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_REQUEST", "invalid export request")
		return
	}

	sheetCode := strings.TrimSpace(req.SheetCode)
	if sheetCode == "" {
		c.Fail(http.StatusBadRequest, "MISSING_SHEET_CODE", "sheet_code is required")
		return
	}

	if h.remoteDB == nil {
		c.Fail(http.StatusServiceUnavailable, "NO_REMOTE_DB", "remote database not configured")
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
		c.Fail(http.StatusBadRequest, "MISSING_COLUMNS", "at least one valid column is required")
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
		c.Fail(http.StatusInternalServerError, "QUERY_FAILED", err.Error())
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
 * parseStructure — detect sheet/column layout of a template file
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

func (h handlers) parseStructure(c *modulekit.Context) {
	ctx := c.Context()

	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_UUID", "invalid document id")
		return
	}

	doc, err := h.service.GetDocumentInScope(ctx, uid, selectedHealthContextScope(c))
	if err != nil {
		c.Fail(http.StatusNotFound, "NOT_FOUND", "document not found")
		return
	}

	provider, lookupErr, providerErr := h.fileStoreFor(c, doc.StorageLocationID.String())
	if lookupErr != nil {
		c.Fail(http.StatusInternalServerError, "STORAGE_LOOKUP_FAILED", "failed to get storage location")
		return
	}
	if providerErr != nil {
		c.Fail(http.StatusInternalServerError, "INVALID_PROVIDER", "unsupported storage provider")
		return
	}

	reader, err := provider.Download(ctx, doc.ObjectKey)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "DOWNLOAD_FAILED", "failed to retrieve file")
		return
	}
	defer reader.Close()

	ext := strings.ToLower(filepath.Ext(doc.OriginalFilename))
	if ext != ".xlsx" && ext != ".xls" {
		c.Fail(http.StatusBadRequest, "UNSUPPORTED_FORMAT", "only .xlsx and .xls files are supported")
		return
	}

	sheets, err := detectSheetsFromReader(reader)
	if err != nil {
		c.Fail(http.StatusBadRequest, "PARSE_FAILED", "failed to parse spreadsheet: "+err.Error())
		return
	}

	c.OK(http.StatusOK, map[string]any{"sheets": sheets})
}

/* =========================================================
 * scanStructure — detect sheet/column layout from an uploaded file
 *   Accepts a multipart file upload, returns detected structure
 *   without creating any document record in the database.
 * ========================================================= */

func (h handlers) scanStructure(c *modulekit.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Fail(http.StatusBadRequest, "NO_FILE", "file is required")
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".xlsx" && ext != ".xls" {
		c.Fail(http.StatusBadRequest, "UNSUPPORTED_FORMAT", "only .xlsx and .xls files are supported")
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.Fail(http.StatusInternalServerError, "READ_FAILED", "failed to open uploaded file")
		return
	}
	defer f.Close()

	sheets, err := detectSheetsFromReader(f)
	if err != nil {
		c.Fail(http.StatusBadRequest, "PARSE_FAILED", "failed to parse spreadsheet: "+err.Error())
		return
	}

	c.OK(http.StatusOK, map[string]any{"sheets": sheets})
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
