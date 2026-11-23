package storagelocations

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// Repository persists storage locations.
type Repository interface {
	Create(ctx context.Context, input CreateInput) (Location, error)
	GetByID(ctx context.Context, id uuid.UUID) (Location, error)
	GetByCode(ctx context.Context, code string) (Location, error)
	ListActive(ctx context.Context) ([]Location, error)
	Update(ctx context.Context, input UpdateStorageLocationInput) (Location, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// NewPostgresRepository stores locations in the primary database's
// storage_locations table.
func NewPostgresRepository(db *sql.DB) Repository { return &postgresRepository{db: db} }

type postgresRepository struct{ db *sql.DB }

const locationColumns = `id, code, name, provider, base_uri, is_active, created_at`

type scanner interface{ Scan(...any) error }

func scanLocation(row scanner) (Location, error) {
	var location Location
	err := row.Scan(&location.ID, &location.Code, &location.Name, &location.Provider,
		&location.BaseUri, &location.IsActive, &location.CreatedAt)
	return location, err
}

func (r *postgresRepository) Create(ctx context.Context, input CreateInput) (Location, error) {
	return scanLocation(r.db.QueryRowContext(ctx, `INSERT INTO storage_locations (
    id, code, name, provider, base_uri, is_active
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING `+locationColumns,
		input.ID, input.Code, input.Name, input.Provider, input.BaseUri, input.IsActive))
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (Location, error) {
	return scanLocation(r.db.QueryRowContext(ctx, `SELECT `+locationColumns+`
FROM storage_locations
WHERE id = $1`, id))
}

func (r *postgresRepository) GetByCode(ctx context.Context, code string) (Location, error) {
	return scanLocation(r.db.QueryRowContext(ctx, `SELECT `+locationColumns+`
FROM storage_locations
WHERE code = $1
  AND is_active = TRUE`, code))
}

func (r *postgresRepository) ListActive(ctx context.Context) ([]Location, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+locationColumns+`
FROM storage_locations
WHERE is_active = TRUE
ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Location{}
	for rows.Next() {
		location, err := scanLocation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, location)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return items, rows.Err()
}

func (r *postgresRepository) Update(ctx context.Context, input UpdateStorageLocationInput) (Location, error) {
	return scanLocation(r.db.QueryRowContext(ctx, `UPDATE storage_locations
SET name = $2,
    provider = $3,
    base_uri = $4,
    is_active = $5
WHERE id = $1
RETURNING `+locationColumns,
		input.ID, input.Name, input.Provider, input.BaseUri, input.IsActive))
}

func (r *postgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM storage_locations
WHERE id = $1`, id)
	return err
}
