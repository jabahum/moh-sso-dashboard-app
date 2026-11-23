package documenttemplates

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

// handlers serves the document-template routes. Responses, status codes and
// error codes are unchanged from the host handler they replace, including the
// route/parameter mismatches noted on getTemplate, getTemplateStructure and
// listSheets.
type handlers struct {
	service       Service
	sheetService  SheetService
	columnService ColumnService
	warehouse     *sql.DB
}

// failWithAPIError writes an error that carries its own status and code (the
// host's apierror.APIError, now modulekit.Error) and reports whether it did.
func failWithAPIError(c *modulekit.Context, err error) bool {
	var apiErr *modulekit.Error
	if errors.As(err, &apiErr) && apiErr != nil {
		c.Fail(apiErr.Status, apiErr.Code, apiErr.Message)
		return true
	}
	return false
}

func (h handlers) createTemplate(c *modulekit.Context) {
	var req CreateTemplateRequest

	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "VALIDATION_FAILED", "Invalid request payload")
		return
	}

	userID, err := uuid.Parse(c.Subject())
	if err != nil {
		c.Fail(http.StatusUnauthorized, "INVALID_USER", "Invalid authenticated user")
		return
	}

	req.CreatedBy = userID

	template, err := h.service.CreateTemplate(c.Context(), req)
	if err != nil {
		if failWithAPIError(c, err) {
			return
		}

		c.Fail(http.StatusInternalServerError, "CREATE_TEMPLATE_FAILED", "Failed to create template")
		return
	}

	c.OK(http.StatusCreated, toTemplateResponse(template))
}

// getTemplate serves GET /document-templates/{id}. It reads a "code" path
// parameter the route does not declare, so it always answers 400
// MISSING_TEMPLATE_CODE; kept as it was before extraction.
func (h handlers) getTemplate(c *modulekit.Context) {
	code := c.Param("code")

	if code == "" {
		c.Fail(http.StatusBadRequest, "MISSING_TEMPLATE_CODE", "Template code is required")
		return
	}

	template, err := h.service.GetTemplateRuntime(c.Context(), code)
	if err != nil {
		c.Fail(http.StatusNotFound, "TEMPLATE_NOT_FOUND", "Template not found")
		return
	}

	c.OK(http.StatusOK, toTemplateRuntimeResponse(template))
}

// getTemplateStructure serves both GET /document-templates/code/{code}/structure
// and GET /document-templates/{id}/structure. The {id} route has no "code"
// parameter and therefore always answers 400, as before extraction.
func (h handlers) getTemplateStructure(c *modulekit.Context) {
	code := c.Param("code")

	if code == "" {
		c.Fail(http.StatusBadRequest, "MISSING_TEMPLATE_CODE", "Template code is required")
		return
	}

	structure, err := h.service.GetTemplateStructure(c.Context(), code)
	if err != nil {
		c.Fail(http.StatusNotFound, "TEMPLATE_NOT_FOUND", "Template structure not found")
		return
	}

	c.OK(http.StatusOK, toTemplateStructureResponse(structure))
}

func (h handlers) listTemplates(c *modulekit.Context) {
	ctx := c.Context()
	activeOnly := strings.EqualFold(c.Query("active"), "true")

	var (
		templates []DocumentTemplate
		err       error
	)

	if activeOnly {
		templates, err = h.service.ListActiveTemplates(ctx)
	} else {
		templates, err = h.service.ListTemplates(ctx)
	}

	if err != nil {
		c.Fail(http.StatusInternalServerError, "LIST_TEMPLATES_FAILED", "Failed to list templates")
		return
	}

	if templates == nil {
		templates = []DocumentTemplate{}
	}

	c.OK(http.StatusOK, templates)
}

func (h handlers) updateTemplate(c *modulekit.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_TEMPLATE_ID", "Invalid template ID")
		return
	}

	var req UpdateTemplateRequest

	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "VALIDATION_FAILED", "Invalid request payload")
		return
	}

	req.ID = id

	template, err := h.service.UpdateTemplate(c.Context(), req)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "UPDATE_TEMPLATE_FAILED", "Failed to update template")
		return
	}

	c.OK(http.StatusOK, toTemplateResponse(template))
}

func (h handlers) publishTemplate(c *modulekit.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_TEMPLATE_ID", "Invalid template ID")
		return
	}

	if err := h.service.PublishTemplate(c.Context(), id); err != nil {
		c.Fail(http.StatusInternalServerError, "PUBLISH_TEMPLATE_FAILED", "Failed to publish template")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h handlers) archiveTemplate(c *modulekit.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_TEMPLATE_ID", "Invalid template ID")
		return
	}

	if err := h.service.ArchiveTemplate(c.Context(), id); err != nil {
		c.Fail(http.StatusInternalServerError, "ARCHIVE_TEMPLATE_FAILED", "Failed to archive template")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h handlers) deleteTemplate(c *modulekit.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_TEMPLATE_ID", "Invalid template ID")
		return
	}

	if err := h.service.DeleteTemplate(c.Context(), id); err != nil {
		c.Fail(http.StatusInternalServerError, "DELETE_TEMPLATE_FAILED", "Failed to delete template")
		return
	}

	c.Status(http.StatusNoContent)
}

// listSheets serves GET /document-templates/{id}/sheets. It reads a
// "templateId" path parameter the route does not declare, so it always
// answers 400 INVALID_TEMPLATE_ID; kept as it was before extraction.
func (h handlers) listSheets(c *modulekit.Context) {
	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_TEMPLATE_ID", "Invalid template ID")
		return
	}

	sheets, err := h.sheetService.ListSheets(c.Context(), templateID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "LIST_SHEETS_FAILED", "Failed to list sheets")
		return
	}

	c.OK(http.StatusOK, toSheetResponses(sheets))
}

func (h handlers) listColumns(c *modulekit.Context) {
	sheetID, err := uuid.Parse(c.Param("sheetId"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_SHEET_ID", "Invalid sheet ID")
		return
	}

	columns, err := h.columnService.ListColumns(c.Context(), sheetID)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "LIST_COLUMNS_FAILED", "Failed to list columns")
		return
	}

	c.OK(http.StatusOK, toColumnResponses(columns))
}

type hasDataResponse struct {
	HasData bool `json:"has_data"`
}

// hasData reports whether the warehouse holds current imported data for a
// template code.
func (h handlers) hasData(c *modulekit.Context) {
	code := c.Param("code")
	if code == "" {
		c.Fail(http.StatusBadRequest, "MISSING_TEMPLATE_CODE", "Template code is required")
		return
	}

	ctx := c.Context()
	var hasData bool

	err := h.warehouse.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM import.custom_data_files
			WHERE template_code = $1 AND is_current = 'Y'
		)
	`, strings.ToUpper(strings.TrimSpace(code))).Scan(&hasData)

	if err != nil {
		c.Fail(http.StatusInternalServerError, "HAS_DATA_CHECK_FAILED", "Failed to check template data")
		return
	}

	c.OK(http.StatusOK, hasDataResponse{HasData: hasData})
}

type replaceStructureRequest struct {
	Sheets []CreateTemplateSheetWithColumnsRequest `json:"sheets"`
}

func (h handlers) replaceStructure(c *modulekit.Context) {
	templateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_ID", "invalid template id")
		return
	}

	var req replaceStructureRequest
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "VALIDATION_FAILED", "invalid request payload")
		return
	}

	structure, err := h.service.ReplaceStructure(c.Context(), templateID, req.Sheets)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "REPLACE_FAILED", err.Error())
		return
	}

	c.OK(http.StatusOK, structure)
}

func (h handlers) createTemplateWithStructure(c *modulekit.Context) {
	var req CreateTemplateStructureRequest

	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "VALIDATION_FAILED", "Invalid request payload")
		return
	}

	userID, err := uuid.Parse(c.Subject())
	if err != nil {
		c.Fail(http.StatusUnauthorized, "INVALID_USER", "Invalid authenticated user")
		return
	}

	req.Template.CreatedBy = userID

	template, err := h.service.CreateTemplateStructure(c.Context(), req)
	if err != nil {
		if failWithAPIError(c, err) {
			return
		}

		if errors.Is(err, ErrTemplateCodeExists) {
			c.Fail(http.StatusConflict, "TEMPLATE_CODE_EXISTS", "A template with this code already exists")
			return
		}

		c.Fail(http.StatusInternalServerError, "CREATE_TEMPLATE_FAILED", "request failed")
		return
	}

	c.OK(http.StatusCreated, toTemplateStructureResponse(template))
}
