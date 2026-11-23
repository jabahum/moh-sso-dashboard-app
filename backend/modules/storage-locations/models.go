package storagelocations

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Location is a storage destination row.
type Location struct {
	ID        uuid.UUID    `json:"id"`
	Code      string       `json:"code"`
	Name      string       `json:"name"`
	Provider  string       `json:"provider"`
	BaseUri   string       `json:"base_uri"`
	IsActive  sql.NullBool `json:"is_active"`
	CreatedAt sql.NullTime `json:"created_at"`
}

// CreateInput is the data stored for a new location.
type CreateInput struct {
	ID       uuid.UUID    `json:"id"`
	Code     string       `json:"code"`
	Name     string       `json:"name"`
	Provider string       `json:"provider"`
	BaseUri  string       `json:"base_uri"`
	IsActive sql.NullBool `json:"is_active"`
}

// UpdateStorageLocationInput is both the PUT request body and the update
// input. is_active keeps its historical {"Bool":..,"Valid":..} JSON shape.
type UpdateStorageLocationInput struct {
	ID       uuid.UUID    `json:"id"`
	Name     string       `json:"name"`
	Provider string       `json:"provider"`
	BaseUri  string       `json:"base_uri"`
	IsActive sql.NullBool `json:"is_active"`
}

type createStorageLocationRequest struct {
	Code     string `json:"code" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Provider string `json:"provider"`
	BaseUri  string `json:"base_uri"`
	IsActive bool   `json:"is_active" binding:"required"`
}

// StorageLocationResponse is the API representation of a location.
type StorageLocationResponse struct {
	ID        string     `json:"id"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Provider  string     `json:"provider"`
	BaseUri   string     `json:"base_uri"`
	IsActive  bool       `json:"is_active"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

type deleteStorageLocationResponse struct {
	Message string `json:"message"`
}

func toResponse(location Location) StorageLocationResponse {
	out := StorageLocationResponse{
		ID: location.ID.String(), Code: location.Code, Name: location.Name,
		Provider: location.Provider, BaseUri: location.BaseUri,
	}
	if location.IsActive.Valid {
		out.IsActive = location.IsActive.Bool
	}
	if location.CreatedAt.Valid {
		createdAt := location.CreatedAt.Time
		out.CreatedAt = &createdAt
	}
	return out
}

func toResponses(locations []Location) []StorageLocationResponse {
	out := make([]StorageLocationResponse, 0, len(locations))
	for _, location := range locations {
		out = append(out, toResponse(location))
	}
	return out
}
