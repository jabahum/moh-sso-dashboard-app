package storagelocations

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

var (
	ErrInvalidID         = errors.New("invalid id")
	ErrStorageCodeExists = errors.New("storage location code already exists")
	ErrStorageNotFound   = errors.New("storage location not found")
)

type handlers struct{ service Service }

func (h handlers) create(c *modulekit.Context) {
	var req createStorageLocationRequest
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "invalid payload")
		return
	}
	location, err := h.service.Create(c.Context(), CreateInput{
		Code: req.Code, Name: req.Name, Provider: req.Provider, BaseUri: req.BaseUri,
		// Valid stays false, so NULL is stored, unchanged from before extraction.
		IsActive: sql.NullBool{Bool: req.IsActive},
	})
	if err != nil {
		if errors.Is(err, ErrStorageCodeExists) {
			c.Fail(http.StatusConflict, "CODE_EXISTS", "storage location code already exists")
			return
		}
		c.Fail(http.StatusInternalServerError, "CREATE_FAILED", "request failed")
		return
	}
	c.OK(http.StatusCreated, toResponse(location))
}

func (h handlers) getByID(c *modulekit.Context) {
	location, err := h.service.GetByID(c.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrInvalidID) {
			c.Fail(http.StatusBadRequest, "INVALID_ID", "invalid id")
			return
		}
		c.Fail(http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	c.OK(http.StatusOK, toResponse(location))
}

func (h handlers) listActive(c *modulekit.Context) {
	locations, err := h.service.ListActive(c.Context())
	if err != nil {
		c.Fail(http.StatusInternalServerError, "LIST_FAILED", "request failed")
		return
	}
	c.OK(http.StatusOK, toResponses(locations))
}

func (h handlers) update(c *modulekit.Context) {
	var req UpdateStorageLocationInput
	if err := c.BindJSON(&req); err != nil {
		c.Fail(http.StatusBadRequest, "INVALID_PAYLOAD", "invalid payload")
		return
	}
	location, err := h.service.Update(c.Context(), c.Param("id"), req)
	if err != nil {
		c.Fail(http.StatusInternalServerError, "UPDATE_FAILED", "request failed")
		return
	}
	c.OK(http.StatusOK, toResponse(location))
}

func (h handlers) delete(c *modulekit.Context) {
	if err := h.service.Delete(c.Context(), c.Param("id")); err != nil {
		c.Fail(http.StatusInternalServerError, "DELETE_FAILED", "request failed")
		return
	}
	c.OK(http.StatusOK, deleteStorageLocationResponse{Message: "deleted"})
}
