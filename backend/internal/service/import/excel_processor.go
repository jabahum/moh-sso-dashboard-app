package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	documentRepository "github.com/moh-sso-dashboard/internal/repository/document_imports"
	processRepository "github.com/moh-sso-dashboard/internal/repository/processes"
	"github.com/moh-sso-dashboard/internal/storage"
	documenttemplates "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates"
	"github.com/xuri/excelize/v2"
)

type ExcelProcessor struct {
	documentRepository      documentRepository.DocumentRepository
	documentTemplateService documenttemplates.Service
	fileRepository          documentRepository.FileRepository
	processRepository       processRepository.ProcessRepository
	storage                 storage.Storage
	remoteDB                *sql.DB
}

func NewExcelProcessor(
	documentRepository documentRepository.DocumentRepository,
	fileRepository documentRepository.FileRepository,
	processRepository processRepository.ProcessRepository,
	documentTemplateService documenttemplates.Service,
	storage storage.Storage,
	remoteDB *sql.DB,
) *ExcelProcessor {
	return &ExcelProcessor{
		documentRepository:      documentRepository,
		fileRepository:          fileRepository,
		processRepository:       processRepository,
		documentTemplateService: documentTemplateService,
		storage:                 storage,
		remoteDB:                remoteDB,
	}
}

func (c *ExcelProcessor) Process(ctx context.Context, p db.Process) error {
	document, err := c.documentRepository.GetDocument(ctx, p.DocumentID)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}

	if document.IsTemplate {
		log.Printf("[ExcelProcessor] document %s is a template, skipping data extraction", document.ID)
		return nil
	}

	templateCode, err := getTemplateCodeFromDocument(document)
	if err != nil {
		return err
	}
	templateCode = strings.ToUpper(strings.TrimSpace(templateCode))

	reportDate, err := getReportDateFromDocument(document)
	if err != nil {
		return err
	}

	replaceDocumentID, err := getReplaceDocumentIDFromDocument(document)
	if err != nil {
		return err
	}

	template, err := c.documentTemplateService.GetTemplateStructure(ctx, templateCode)
	if err != nil {
		return fmt.Errorf("get template structure %q: %w", templateCode, err)
	}

	log.Printf("[ExcelProcessor] document=%s template=%s sheets=%d", document.ID, templateCode, len(template.Sheets))

	if len(template.Sheets) == 0 {
		log.Printf("[ExcelProcessor] WARNING: template %q has no sheets configured — no data will be imported", templateCode)
	}

	reader, err := c.storage.Download(ctx, document.ObjectKey)
	if err != nil {
		return fmt.Errorf("download document: %w", err)
	}
	defer reader.Close()

	fileBytes, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read downloaded document: %w", err)
	}

	workbook, err := excelize.OpenReader(bytes.NewReader(fileBytes))
	if err != nil {
		return fmt.Errorf("open excel workbook: %w", err)
	}
	defer workbook.Close()

	workbookSheets := workbook.GetSheetList()
	log.Printf("[ExcelProcessor] workbook sheets: %v", workbookSheets)

	tx, err := c.remoteDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remote tx: %w", err)
	}
	defer tx.Rollback()

	// If replacing a previous upload, invalidate its rows and file record first.
	if replaceDocumentID != nil {
		if err := c.fileRepository.InvalidateByDocument(ctx, tx, *replaceDocumentID); err != nil {
			return fmt.Errorf("invalidate previous document rows: %w", err)
		}
		if err := c.fileRepository.InvalidateFile(ctx, tx, *replaceDocumentID); err != nil {
			return fmt.Errorf("invalidate previous file record: %w", err)
		}
	}

	// Create the file record once per document.
	fileKey, err := c.fileRepository.CreateFile(ctx, tx, document.ID, templateCode, document.OriginalFilename, document.ObjectKey, reportDate)
	if err != nil {
		return fmt.Errorf("create custom file record: %w", err)
	}

	// sheetHashes tracks hashes per sheet_code for the post-commit soft-delete pass.
	sheetHashes := make(map[string][]string)
	totalInserted := 0

	for _, sheet := range template.Sheets {
		sheetCode := strings.ToLower(strings.TrimSpace(sheet.Code))
		resolvedName := resolveWorkbookSheetName(workbook, sheet)
		log.Printf("[ExcelProcessor] sheet code=%q name=%q resolved_workbook_sheet=%q header_row=%d required=%v",
			sheetCode, sheet.Name, resolvedName, sheet.HeaderRow, sheet.Required)

		positionalOverrides := blankHeaderOverrides(sheet)
		rows, err := c.extractSheetRows(ctx, workbook, sheet, positionalOverrides)
		if err != nil {
			return fmt.Errorf("extract sheet %q: %w", sheet.Name, err)
		}

		log.Printf("[ExcelProcessor] sheet %q extracted %d rows", sheetCode, len(rows))

		if len(rows) == 0 {
			log.Printf("[ExcelProcessor] sheet %q: no data rows found — skipping", sheetCode)
			continue
		}

		// Apply forward-fill if the sheet configuration requests it.
		rows = applyForwardFill(rows, sheet)

		const batchSize = 1000

		importRows := make([]documentRepository.CustomImportRow, 0, len(rows))
		for _, row := range rows {
			importRows = append(importRows, documentRepository.CustomImportRow{
				FileKey:      fileKey,
				DocumentID:   document.ID,
				TemplateCode: templateCode,
				SheetCode:    sheetCode,
				RowNumber:    row.RowNumber,
				ReportDate:   reportDate,
				Data:         row.Data,
				RowHash:      hashRawData(document.ID, sheetCode, row.RowNumber, row.RawData),
			})
		}

		importRows = deduplicateByHash(importRows, func(r documentRepository.CustomImportRow) string {
			return r.RowHash
		})

		for _, batch := range chunkSlice(importRows, batchSize) {
			if err := c.fileRepository.UpsertRowsBatch(ctx, tx, batch); err != nil {
				return fmt.Errorf("upsert rows for sheet %q: %w", sheet.Name, err)
			}
		}

		hashes := make([]string, len(importRows))
		for i, r := range importRows {
			hashes[i] = r.RowHash
		}
		sheetHashes[sheetCode] = hashes
		totalInserted += len(importRows)
		log.Printf("[ExcelProcessor] sheet %q: inserted %d rows (after dedup)", sheetCode, len(importRows))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit remote tx: %w", err)
	}

	log.Printf("[ExcelProcessor] document=%s total rows inserted=%d", document.ID, totalInserted)

	if totalInserted == 0 {
		log.Printf("[ExcelProcessor] WARNING: document=%s completed but zero rows were imported for template=%s", document.ID, templateCode)
	}

	// Soft-delete rows from previous uploads that are no longer in this file.
	sdTx, err := c.remoteDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin soft-delete tx: %w", err)
	}
	defer sdTx.Rollback()

	for sheetCode, hashes := range sheetHashes {
		if err := c.fileRepository.SoftDeleteBySheet(ctx, sdTx, document.ID, templateCode, sheetCode, hashes); err != nil {
			return fmt.Errorf("soft-delete sheet %q: %w", sheetCode, err)
		}
	}

	if err := sdTx.Commit(); err != nil {
		return fmt.Errorf("commit soft-delete tx: %w", err)
	}

	return nil
}

// applyForwardFill forward-fills blank cell values for columns listed in the
// sheet configuration under "forward_fill_columns". This replaces the old
// hard-coded normalizeGHSCRows function.
func applyForwardFill(rows []extractedExcelRow, sheet documenttemplates.TemplateSheetStructure) []extractedExcelRow {
	raw, ok := sheet.Configuration["forward_fill_columns"]
	if !ok {
		return rows
	}

	var fillKeys []string
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				fillKeys = append(fillKeys, s)
			}
		}
	case []string:
		fillKeys = v
	}

	if len(fillKeys) == 0 {
		return rows
	}

	carry := make(map[string]any)
	for i := range rows {
		for _, key := range fillKeys {
			if isBlankValue(rows[i].Data[key]) && !isBlankValue(carry[key]) {
				rows[i].Data[key] = carry[key]
			}
		}
		for _, key := range fillKeys {
			if !isBlankValue(rows[i].Data[key]) {
				carry[key] = rows[i].Data[key]
			}
		}
	}

	return rows
}

// blankHeaderOverrides reads positional blank-header overrides from the sheet
// configuration key "blank_header_overrides" (map of 1-based col index → column_key).
// This replaces the old hard-coded blankHeaderOverrides() switch.
func blankHeaderOverrides(sheet documenttemplates.TemplateSheetStructure) map[int]string {
	raw, ok := sheet.Configuration["blank_header_overrides"]
	if !ok {
		return nil
	}

	asMap, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	result := make(map[int]string, len(asMap))
	for k, v := range asMap {
		idx, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		if s, ok := v.(string); ok {
			result[idx] = s
		}
	}
	return result
}

// deduplicateByHash removes items with duplicate hashes, keeping the first occurrence.
// Required because PostgreSQL's ON CONFLICT DO UPDATE cannot target the same row twice
// within a single INSERT statement.
func deduplicateByHash[T any](items []T, getHash func(T) string) []T {
	seen := make(map[string]struct{}, len(items))
	result := make([]T, 0, len(items))
	for _, item := range items {
		h := getHash(item)
		if _, ok := seen[h]; !ok {
			seen[h] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func chunkSlice[T any](items []T, size int) [][]T {
	if size <= 0 {
		size = 1000
	}
	chunks := make([][]T, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}

type extractedExcelRow struct {
	RowNumber int
	Data      map[string]any
	RawData   map[string]any
}

func (c *ExcelProcessor) extractSheetRows(
	ctx context.Context,
	workbook *excelize.File,
	sheet documenttemplates.TemplateSheetStructure,
	positionalOverrides map[int]string,
) ([]extractedExcelRow, error) {
	_ = ctx

	sheetName := resolveWorkbookSheetName(workbook, sheet)

	rows, err := workbook.GetRows(sheetName)
	if err != nil {
		if sheet.Required {
			return nil, fmt.Errorf("%w (available sheets: %s)", err, strings.Join(workbook.GetSheetList(), ", "))
		}
		return nil, nil
	}

	headerIndex := sheet.HeaderRow - 1
	startIndex := sheet.StartRow - 1

	if headerIndex < 0 {
		headerIndex = 0
	}
	if startIndex <= headerIndex {
		startIndex = headerIndex + 1
	}

	// Anchor-based header detection: scan for the last row containing the anchor
	// value to locate the real data table's header when a sheet has multiple tables.
	// Configured via sheet.Configuration["header_anchor"] = "<column header value>".
	if anchor, ok := sheet.Configuration["header_anchor"].(string); ok && anchor != "" {
		normalizedAnchor := normalizeHeader(anchor)
		foundAt := -1
		for i, row := range rows {
			for _, cell := range row {
				if normalizeHeader(cell) == normalizedAnchor {
					foundAt = i
					break
				}
			}
		}
		if foundAt >= 0 {
			headerIndex = foundAt
			startIndex = headerIndex + 1
		}
	}

	if len(rows) <= headerIndex {
		if sheet.Required {
			return nil, fmt.Errorf("header row %d not found", sheet.HeaderRow)
		}
		return nil, nil
	}

	rows, err = expandMergedCellValues(workbook, sheetName, rows, headerIndex+1)
	if err != nil {
		return nil, fmt.Errorf("expand merged cells: %w", err)
	}

	headers := normalizeHeaders(rows[headerIndex])
	columnByHeader := buildColumnMap(sheet.Columns)

	log.Printf("[ExcelProcessor] sheet %q: header_row=%d start_row=%d headers=%v total_rows_in_file=%d",
		sheet.Name, headerIndex+1, startIndex+1, headers, len(rows))

	matchedHeaders := 0
	for _, h := range headers {
		if _, ok := columnByHeader[h]; ok {
			matchedHeaders++
		}
	}
	log.Printf("[ExcelProcessor] sheet %q: %d/%d headers matched template columns", sheet.Name, matchedHeaders, len(headers))

	result := make([]extractedExcelRow, 0)

	for rowIndex := startIndex; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]

		if isEmptyRow(row) {
			continue
		}

		data := make(map[string]any)
		rawData := make(map[string]any)

		for colIndex, header := range headers {
			value := ""
			if colIndex < len(row) {
				value = strings.TrimSpace(row[colIndex])
			}

			var column documenttemplates.TemplateColumnStructure
			var ok bool

			if header != "" {
				rawData[header] = value
				column, ok = columnByHeader[header]
			}

			if !ok && header == "" {
				if colKey, hasOverride := positionalOverrides[colIndex+1]; hasOverride {
					column, ok = columnByHeader[normalizeHeader(colKey)]
					if ok {
						rawData[colKey] = value
					}
				}
			}

			if !ok {
				if header != "" && sheet.AllowExtraColumns {
					data[header] = value
				}
				continue
			}

			parsedValue, err := parseValue(value, column.DataType)
			if err != nil {
				return nil, fmt.Errorf(
					"sheet=%s row=%d column=%s value=%q: %w",
					sheet.Name, rowIndex+1, column.ColumnName, value, err,
				)
			}

			data[column.ColumnKey] = parsedValue
		}

		result = append(result, extractedExcelRow{
			RowNumber: rowIndex + 1,
			Data:      data,
			RawData:   rawData,
		})
	}

	return result, nil
}

func expandMergedCellValues(
	workbook *excelize.File,
	sheetName string,
	rows [][]string,
	minRow int,
) ([][]string, error) {
	if minRow < 1 {
		minRow = 1
	}

	rowHasOwnData := make([]bool, len(rows))
	for i, row := range rows {
		rowHasOwnData[i] = !isEmptyRow(row)
	}

	mergedCells, err := workbook.GetMergeCells(sheetName)
	if err != nil {
		return rows, err
	}

	for _, mergedCell := range mergedCells {
		value := strings.TrimSpace(mergedCell.GetCellValue())
		if value == "" {
			continue
		}

		startCol, startRow, err := excelize.CellNameToCoordinates(mergedCell.GetStartAxis())
		if err != nil {
			return rows, err
		}
		endCol, endRow, err := excelize.CellNameToCoordinates(mergedCell.GetEndAxis())
		if err != nil {
			return rows, err
		}

		if startRow > endRow {
			startRow, endRow = endRow, startRow
		}
		if startCol > endCol {
			startCol, endCol = endCol, startCol
		}
		if endRow < minRow {
			continue
		}
		if startRow < minRow {
			startRow = minRow
		}

		for rowNumber := startRow; rowNumber <= endRow; rowNumber++ {
			rowIndex := rowNumber - 1
			if rowIndex < 0 || rowIndex >= len(rows) || !rowHasOwnData[rowIndex] {
				continue
			}
			if len(rows[rowIndex]) < endCol {
				expanded := make([]string, endCol)
				copy(expanded, rows[rowIndex])
				rows[rowIndex] = expanded
			}
			for colNumber := startCol; colNumber <= endCol; colNumber++ {
				colIndex := colNumber - 1
				if strings.TrimSpace(rows[rowIndex][colIndex]) == "" {
					rows[rowIndex][colIndex] = value
				}
			}
		}
	}

	return rows, nil
}

func resolveWorkbookSheetName(workbook *excelize.File, sheet documenttemplates.TemplateSheetStructure) string {
	candidates := sheetNameCandidates(sheet)

	visibleSheets := make([]string, 0)
	for _, sheetName := range workbook.GetSheetList() {
		visible, err := workbook.GetSheetVisible(sheetName)
		if err != nil {
			continue
		}
		if visible {
			visibleSheets = append(visibleSheets, sheetName)
		}
	}

	for _, candidate := range candidates {
		for _, available := range visibleSheets {
			if candidate == strings.TrimSpace(available) {
				return available
			}
		}
	}

	availableByNormalized := make(map[string]string, len(visibleSheets))
	for _, available := range visibleSheets {
		normalized := normalizeSheetName(available)
		if normalized == "" {
			continue
		}
		if _, exists := availableByNormalized[normalized]; !exists {
			availableByNormalized[normalized] = available
		}
	}

	for _, candidate := range candidates {
		if available, ok := availableByNormalized[normalizeSheetName(candidate)]; ok {
			return available
		}
	}

	// Fallback 1: sheet name starts with the candidate
	for _, candidate := range candidates {
		normalizedCandidate := normalizeSheetName(candidate)
		if normalizedCandidate == "" {
			continue
		}
		for normalizedSheet, available := range availableByNormalized {
			if strings.HasPrefix(normalizedSheet, normalizedCandidate) {
				return available
			}
		}
	}

	// Fallback 2: sheet name contains the candidate
	for _, candidate := range candidates {
		normalizedCandidate := normalizeSheetName(candidate)
		if normalizedCandidate == "" {
			continue
		}
		for normalizedSheet, available := range availableByNormalized {
			if strings.Contains(normalizedSheet, normalizedCandidate) {
				return available
			}
		}
	}

	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func sheetNameCandidates(sheet documenttemplates.TemplateSheetStructure) []string {
	values := []string{sheet.Code, sheet.Name, sheet.DisplayName}

	if aliases, ok := sheet.Configuration["aliases"]; ok {
		switch v := aliases.(type) {
		case []any:
			for _, a := range v {
				if s, ok := a.(string); ok {
					values = append(values, s)
				}
			}
		case []string:
			values = append(values, v...)
		}
	}

	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		candidate := strings.TrimSpace(value)
		if candidate == "" {
			continue
		}
		normalized := normalizeSheetName(candidate)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, candidate)
	}
	return result
}

func normalizeSheetName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func hashRawData(documentID uuid.UUID, sheetCode string, rowNumber int, rawData map[string]any) string {
	b, _ := json.Marshal(rawData)
	input := fmt.Sprintf("%s|%s|%d|%s", documentID, sheetCode, rowNumber, b)
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func buildColumnMap(columns []documenttemplates.TemplateColumnStructure) map[string]documenttemplates.TemplateColumnStructure {
	result := make(map[string]documenttemplates.TemplateColumnStructure)
	for _, column := range columns {
		result[normalizeHeader(column.ColumnName)] = column
		result[normalizeHeader(column.ColumnKey)] = column
		result[normalizeHeader(column.DisplayName)] = column
		for _, alias := range column.Aliases {
			result[normalizeHeader(alias)] = column
		}
	}
	return result
}

func normalizeHeaders(headers []string) []string {
	result := make([]string, 0, len(headers))
	for _, header := range headers {
		result = append(result, normalizeHeader(header))
	}
	return result
}

func isEmptyRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func isBlankValue(value any) bool {
	if value == nil {
		return true
	}
	return strings.TrimSpace(fmt.Sprint(value)) == ""
}

func parseValue(value string, dataType string) (any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	switch strings.ToLower(dataType) {
	case "text", "string":
		return value, nil

	case "number", "numeric", "decimal":
		cleaned := strings.ReplaceAll(value, ",", "")
		cleaned = strings.TrimSuffix(cleaned, "%")
		number, err := strconv.ParseFloat(cleaned, 64)
		if err != nil {
			return nil, err
		}
		return number, nil

	case "integer", "int":
		cleaned := strings.ReplaceAll(value, ",", "")
		number, err := strconv.Atoi(cleaned)
		if err != nil {
			return nil, err
		}
		return number, nil

	case "date", "datetime", "timestamp":
		return parseDate(value)

	case "boolean", "bool":
		switch strings.ToLower(value) {
		case "true", "yes", "1":
			return true, nil
		case "false", "no", "0":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid boolean")
		}

	default:
		return value, nil
	}
}

func parseDate(value string) (string, error) {
	parsed, err := parseDateToTime(value)
	if err != nil {
		return "", err
	}
	return parsed.Format("2006-01-02"), nil
}

func getFloatPtr(data map[string]any, key string) *float64 {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	switch v := value.(type) {
	case float64:
		return &v
	case float32:
		n := float64(v)
		return &n
	case int:
		n := float64(v)
		return &n
	case int64:
		n := float64(v)
		return &n
	case string:
		cleaned := strings.TrimSpace(v)
		cleaned = strings.ReplaceAll(cleaned, ",", "")
		cleaned = strings.TrimSuffix(cleaned, "%")
		if cleaned == "" {
			return nil
		}
		n, err := strconv.ParseFloat(cleaned, 64)
		if err != nil {
			return nil
		}
		return &n
	default:
		return nil
	}
}

func getTimePtr(data map[string]any, key string) *time.Time {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		return &v
	case float64:
		return excelSerialDateToTimePtr(v)
	case float32:
		return excelSerialDateToTimePtr(float64(v))
	case int:
		return excelSerialDateToTimePtr(float64(v))
	case int64:
		return excelSerialDateToTimePtr(float64(v))
	case string:
		parsed, err := parseDateToTime(v)
		if err != nil {
			return nil
		}
		return &parsed
	default:
		return nil
	}
}

func parseDateToTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("invalid date")
	}

	if parsed, ok := parseExcelSerialDate(value); ok {
		return parsed, nil
	}

	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"02-Jan-06",
		"02-Jan-2006",
		"2-Jan-06",
		"2-Jan-2006",
		"01/02/2006",
		"1/2/2006",
		"01/02/06",
		"1/2/06",
		"01-02-2006",
		"1-2-2006",
		"01-02-06",
		"1-2-06",
		"02/01/2006",
		"2/1/2006",
		"02/01/06",
		"2/1/06",
		"Jan-06",
		"Jan-2006",
		"January-06",
		"January-2006",
	}

	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date")
}

func parseExcelSerialDate(value string) (time.Time, bool) {
	cleaned := strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	serial, err := strconv.ParseFloat(cleaned, 64)
	if err != nil || serial <= 0 {
		return time.Time{}, false
	}
	parsed, err := excelize.ExcelDateToTime(serial, false)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func excelSerialDateToTimePtr(serial float64) *time.Time {
	if serial <= 0 {
		return nil
	}
	parsed, err := excelize.ExcelDateToTime(serial, false)
	if err != nil {
		return nil
	}
	return &parsed
}

func getReplaceDocumentIDFromDocument(document db.Document) (*uuid.UUID, error) {
	if len(document.Metadata) == 0 {
		return nil, nil
	}
	var metadata struct {
		ReplaceDocumentID string `json:"replace_document_id"`
	}
	if err := json.Unmarshal(document.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("decode document metadata: %w", err)
	}
	if metadata.ReplaceDocumentID == "" {
		return nil, nil
	}
	id, err := uuid.Parse(metadata.ReplaceDocumentID)
	if err != nil {
		return nil, fmt.Errorf("invalid replace_document_id %q: %w", metadata.ReplaceDocumentID, err)
	}
	return &id, nil
}

func getReportDateFromDocument(document db.Document) (*time.Time, error) {
	if len(document.Metadata) == 0 {
		return nil, fmt.Errorf("document metadata missing report_date")
	}
	var metadata struct {
		ReportDate string `json:"report_date"`
	}
	if err := json.Unmarshal(document.Metadata, &metadata); err != nil {
		return nil, fmt.Errorf("decode document metadata: %w", err)
	}
	if metadata.ReportDate == "" {
		return nil, fmt.Errorf("report_date is required in document metadata")
	}
	parsed, err := time.Parse("2006-01-02", metadata.ReportDate)
	if err != nil {
		return nil, fmt.Errorf("invalid report_date %q: expected YYYY-MM-DD", metadata.ReportDate)
	}
	if parsed.After(time.Now().Truncate(24 * time.Hour)) {
		return nil, fmt.Errorf("report_date cannot be a future date")
	}
	return &parsed, nil
}

// getOptionalReportDateFromDocument mirrors getReportDateFromDocument but
// returns nil instead of an error when report_date is absent or unparsable.
func getOptionalReportDateFromDocument(document db.Document) *time.Time {
	if len(document.Metadata) == 0 {
		return nil
	}
	var metadata struct {
		ReportDate string `json:"report_date"`
	}
	if err := json.Unmarshal(document.Metadata, &metadata); err != nil {
		return nil
	}
	if metadata.ReportDate == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", metadata.ReportDate)
	if err != nil {
		return nil
	}
	return &parsed
}

func getTemplateCodeFromDocument(document db.Document) (string, error) {
	if len(document.Metadata) == 0 {
		return "", fmt.Errorf("document metadata missing template_code")
	}
	var metadata struct {
		TemplateCode string `json:"template_code"`
	}
	if err := json.Unmarshal(document.Metadata, &metadata); err != nil {
		return "", fmt.Errorf("decode document metadata: %w", err)
	}
	templateCode := strings.TrimSpace(metadata.TemplateCode)
	if templateCode == "" {
		return "", fmt.Errorf("template_code is required in document metadata")
	}
	return templateCode, nil
}

// adHocTemplateCode is the sentinel template_code used for CSV uploads with
// no template selected. It's a fixed value (not per-document) because
// reconciliation for these uploads is scoped by document_id via
// InvalidateByDocument, not by template_code+sheet_code, so sharing the
// sentinel across unrelated ad-hoc uploads is safe.
const adHocTemplateCode = "_ADHOC"

// getTemplateCodeOrAdHoc is like getTemplateCodeFromDocument but falls back
// to adHocTemplateCode instead of erroring when no template_code is set.
// Malformed metadata is still a real error. Only CSV imports may use this
// fallback — Excel still requires a real template to know how to parse the
// workbook.
func getTemplateCodeOrAdHoc(document db.Document) (string, error) {
	if len(document.Metadata) == 0 {
		return adHocTemplateCode, nil
	}
	var metadata struct {
		TemplateCode string `json:"template_code"`
	}
	if err := json.Unmarshal(document.Metadata, &metadata); err != nil {
		return "", fmt.Errorf("decode document metadata: %w", err)
	}
	templateCode := strings.TrimSpace(metadata.TemplateCode)
	if templateCode == "" {
		return adHocTemplateCode, nil
	}
	return templateCode, nil
}
