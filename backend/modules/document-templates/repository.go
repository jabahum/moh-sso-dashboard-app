package documenttemplates

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/document-templates/internal/postgres"
)

type DocumentTemplateRepository interface {
	Create(ctx context.Context, arg db.CreateDocumentTemplateParams) (db.DocumentTemplate, error)

	GetByID(ctx context.Context, id uuid.UUID) (db.DocumentTemplate, error)

	GetByCode(ctx context.Context, code string) (db.DocumentTemplate, error)

	GetByDocumentID(ctx context.Context, documentID uuid.UUID) (db.DocumentTemplate, error)

	List(ctx context.Context) ([]db.DocumentTemplate, error)

	ListActive(ctx context.Context) ([]db.DocumentTemplate, error)

	ListVersions(ctx context.Context, code string) ([]db.DocumentTemplate, error)

	Update(ctx context.Context, arg db.UpdateDocumentTemplateParams) (db.DocumentTemplate, error)

	Archive(ctx context.Context, id uuid.UUID) error

	Delete(ctx context.Context, id uuid.UUID) error

	ExistsCode(ctx context.Context, code string) (bool, error)
}

type Repository = DocumentTemplateRepository

type DocumentTemplateSheetRepository interface {
	Create(ctx context.Context, arg db.CreateDocumentTemplateSheetParams) (db.DocumentTemplateSheet, error)

	GetByID(ctx context.Context, id uuid.UUID) (db.DocumentTemplateSheet, error)

	GetByCode(ctx context.Context, templateID uuid.UUID, code string) (db.DocumentTemplateSheet, error)

	List(ctx context.Context, templateID uuid.UUID) ([]db.DocumentTemplateSheet, error)

	ListRequired(ctx context.Context, templateID uuid.UUID) ([]db.DocumentTemplateSheet, error)

	Update(ctx context.Context, arg db.UpdateDocumentTemplateSheetParams) (db.DocumentTemplateSheet, error)

	Archive(ctx context.Context, id uuid.UUID) error

	Delete(ctx context.Context, id uuid.UUID) error

	ExistsCode(ctx context.Context, templateID uuid.UUID, code string) (bool, error)

	ExistsName(ctx context.Context, templateID uuid.UUID, name string) (bool, error)
}

type SheetRepository = DocumentTemplateSheetRepository

type DocumentTemplateColumnRepository interface {
	Create(ctx context.Context, arg db.CreateDocumentTemplateColumnParams) (db.DocumentTemplateColumn, error)

	GetByID(ctx context.Context, id uuid.UUID) (db.DocumentTemplateColumn, error)

	GetByKey(ctx context.Context, sheetID uuid.UUID, key string) (db.DocumentTemplateColumn, error)

	List(ctx context.Context, sheetID uuid.UUID) ([]db.DocumentTemplateColumn, error)

	ListRequired(ctx context.Context, sheetID uuid.UUID) ([]db.DocumentTemplateColumn, error)

	ListUnique(ctx context.Context, sheetID uuid.UUID) ([]db.DocumentTemplateColumn, error)

	Update(ctx context.Context, arg db.UpdateDocumentTemplateColumnParams) (db.DocumentTemplateColumn, error)

	Archive(ctx context.Context, id uuid.UUID) error

	Delete(ctx context.Context, id uuid.UUID) error

	ExistsKey(ctx context.Context, sheetID uuid.UUID, key string) (bool, error)

	ExistsName(ctx context.Context, sheetID uuid.UUID, name string) (bool, error)
}

type ColumnRepository = DocumentTemplateColumnRepository

var (
	ErrNotFound = errors.New("resource not found")
)

type DocumentTemplateRepositoryImpl struct {
	q *db.Queries
}

func newDocumentTemplateRepository(q *db.Queries) DocumentTemplateRepository {
	return &DocumentTemplateRepositoryImpl{
		q: q,
	}
}

func (r *DocumentTemplateRepositoryImpl) Create(
	ctx context.Context,
	arg db.CreateDocumentTemplateParams,
) (db.DocumentTemplate, error) {

	return r.q.CreateDocumentTemplate(ctx, arg)
}

func (r *DocumentTemplateRepositoryImpl) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (db.DocumentTemplate, error) {

	template, err := r.q.GetDocumentTemplateByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return db.DocumentTemplate{}, ErrNotFound
		}
		return db.DocumentTemplate{}, err
	}

	return template, nil
}

func (r *DocumentTemplateRepositoryImpl) GetByCode(
	ctx context.Context,
	code string,
) (db.DocumentTemplate, error) {

	template, err := r.q.GetDocumentTemplateByCode(ctx, code)
	if err != nil {
		if err == sql.ErrNoRows {
			return db.DocumentTemplate{}, ErrNotFound
		}
		return db.DocumentTemplate{}, err
	}

	return template, nil
}

func (r *DocumentTemplateRepositoryImpl) List(
	ctx context.Context,
) ([]db.DocumentTemplate, error) {

	return r.q.ListDocumentTemplates(ctx)
}

func (r *DocumentTemplateRepositoryImpl) ListActive(
	ctx context.Context,
) ([]db.DocumentTemplate, error) {

	return r.q.ListActiveDocumentTemplates(ctx)
}

func (r *DocumentTemplateRepositoryImpl) ListVersions(
	ctx context.Context,
	code string,
) ([]db.DocumentTemplate, error) {

	return r.q.ListDocumentTemplateVersions(ctx, code)
}

func (r *DocumentTemplateRepositoryImpl) Update(
	ctx context.Context,
	arg db.UpdateDocumentTemplateParams,
) (db.DocumentTemplate, error) {

	return r.q.UpdateDocumentTemplate(ctx, arg)
}

func (r *DocumentTemplateRepositoryImpl) Archive(
	ctx context.Context,
	id uuid.UUID,
) error {

	return r.q.ArchiveDocumentTemplate(ctx, id)
}

func (r *DocumentTemplateRepositoryImpl) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {

	return r.q.DeleteDocumentTemplate(ctx, id)
}

func (r *DocumentTemplateRepositoryImpl) ExistsCode(
	ctx context.Context,
	code string,
) (bool, error) {

	exists, err := r.q.ExistsDocumentTemplateCode(ctx, code)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *DocumentTemplateRepositoryImpl) GetByDocumentID(
	ctx context.Context,
	documentID uuid.UUID,
) (db.DocumentTemplate, error) {
	return r.q.GetDocumentTemplateByDocumentID(ctx, uuid.NullUUID{
		UUID:  documentID,
		Valid: documentID != uuid.Nil,
	})
}

type DocumentTemplateSheetRepositoryImpl struct {
	q *db.Queries
}

func newDocumentTemplateSheetRepository(q *db.Queries) DocumentTemplateSheetRepository {
	return &DocumentTemplateSheetRepositoryImpl{
		q: q,
	}
}

func (r *DocumentTemplateSheetRepositoryImpl) Create(
	ctx context.Context,
	arg db.CreateDocumentTemplateSheetParams,
) (db.DocumentTemplateSheet, error) {

	return r.q.CreateDocumentTemplateSheet(ctx, arg)
}

func (r *DocumentTemplateSheetRepositoryImpl) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (db.DocumentTemplateSheet, error) {

	sheet, err := r.q.GetDocumentTemplateSheetByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return db.DocumentTemplateSheet{}, ErrNotFound
		}
		return db.DocumentTemplateSheet{}, err
	}

	return sheet, nil
}

func (r *DocumentTemplateSheetRepositoryImpl) GetByCode(
	ctx context.Context,
	templateID uuid.UUID,
	code string,
) (db.DocumentTemplateSheet, error) {

	sheet, err := r.q.GetDocumentTemplateSheetByCode(ctx, db.GetDocumentTemplateSheetByCodeParams{
		TemplateID: templateID,
		Code:       code,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return db.DocumentTemplateSheet{}, ErrNotFound
		}
		return db.DocumentTemplateSheet{}, err
	}

	return sheet, nil
}

func (r *DocumentTemplateSheetRepositoryImpl) List(
	ctx context.Context,
	templateID uuid.UUID,
) ([]db.DocumentTemplateSheet, error) {

	return r.q.ListDocumentTemplateSheets(ctx, templateID)
}

func (r *DocumentTemplateSheetRepositoryImpl) ListRequired(
	ctx context.Context,
	templateID uuid.UUID,
) ([]db.DocumentTemplateSheet, error) {

	return r.q.ListRequiredDocumentTemplateSheets(ctx, templateID)
}

func (r *DocumentTemplateSheetRepositoryImpl) Update(
	ctx context.Context,
	arg db.UpdateDocumentTemplateSheetParams,
) (db.DocumentTemplateSheet, error) {

	return r.q.UpdateDocumentTemplateSheet(ctx, arg)
}

func (r *DocumentTemplateSheetRepositoryImpl) Archive(
	ctx context.Context,
	id uuid.UUID,
) error {

	return r.q.ArchiveDocumentTemplateSheet(ctx, id)
}

func (r *DocumentTemplateSheetRepositoryImpl) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {

	return r.q.DeleteDocumentTemplateSheet(ctx, id)
}

func (r *DocumentTemplateSheetRepositoryImpl) ExistsCode(
	ctx context.Context,
	templateID uuid.UUID,
	code string,
) (bool, error) {

	exists, err := r.q.ExistsDocumentTemplateSheetCode(ctx, db.ExistsDocumentTemplateSheetCodeParams{
		TemplateID: templateID,
		Code:       code,
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *DocumentTemplateSheetRepositoryImpl) ExistsName(
	ctx context.Context,
	templateID uuid.UUID,
	name string,
) (bool, error) {

	exists, err := r.q.ExistsDocumentTemplateSheetName(ctx, db.ExistsDocumentTemplateSheetNameParams{
		TemplateID: templateID,
		Name:       name,
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

type DocumentTemplateColumnRepositoryImpl struct {
	q *db.Queries
}

func newDocumentTemplateColumnRepository(q *db.Queries) DocumentTemplateColumnRepository {
	return &DocumentTemplateColumnRepositoryImpl{
		q: q,
	}
}

func (r *DocumentTemplateColumnRepositoryImpl) Create(
	ctx context.Context,
	arg db.CreateDocumentTemplateColumnParams,
) (db.DocumentTemplateColumn, error) {

	return r.q.CreateDocumentTemplateColumn(ctx, arg)
}

func (r *DocumentTemplateColumnRepositoryImpl) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (db.DocumentTemplateColumn, error) {

	col, err := r.q.GetDocumentTemplateColumnByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return db.DocumentTemplateColumn{}, ErrNotFound
		}
		return db.DocumentTemplateColumn{}, err
	}

	return col, nil
}

func (r *DocumentTemplateColumnRepositoryImpl) GetByKey(
	ctx context.Context,
	sheetID uuid.UUID,
	key string,
) (db.DocumentTemplateColumn, error) {

	col, err := r.q.GetDocumentTemplateColumnByKey(ctx, db.GetDocumentTemplateColumnByKeyParams{
		SheetID:   sheetID,
		ColumnKey: key,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return db.DocumentTemplateColumn{}, ErrNotFound
		}
		return db.DocumentTemplateColumn{}, err
	}

	return col, nil
}

func (r *DocumentTemplateColumnRepositoryImpl) List(
	ctx context.Context,
	sheetID uuid.UUID,
) ([]db.DocumentTemplateColumn, error) {

	return r.q.ListDocumentTemplateColumns(ctx, sheetID)
}

func (r *DocumentTemplateColumnRepositoryImpl) ListRequired(
	ctx context.Context,
	sheetID uuid.UUID,
) ([]db.DocumentTemplateColumn, error) {

	return r.q.ListRequiredDocumentTemplateColumns(ctx, sheetID)
}

func (r *DocumentTemplateColumnRepositoryImpl) ListUnique(
	ctx context.Context,
	sheetID uuid.UUID,
) ([]db.DocumentTemplateColumn, error) {

	return r.q.ListUniqueDocumentTemplateColumns(ctx, sheetID)
}

func (r *DocumentTemplateColumnRepositoryImpl) Update(
	ctx context.Context,
	arg db.UpdateDocumentTemplateColumnParams,
) (db.DocumentTemplateColumn, error) {

	return r.q.UpdateDocumentTemplateColumn(ctx, arg)
}

func (r *DocumentTemplateColumnRepositoryImpl) Archive(
	ctx context.Context,
	id uuid.UUID,
) error {

	return r.q.ArchiveDocumentTemplateColumn(ctx, id)
}

func (r *DocumentTemplateColumnRepositoryImpl) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {

	return r.q.DeleteDocumentTemplateColumn(ctx, id)
}

func (r *DocumentTemplateColumnRepositoryImpl) ExistsKey(
	ctx context.Context,
	sheetID uuid.UUID,
	key string,
) (bool, error) {

	exists, err := r.q.ExistsDocumentTemplateColumnKey(ctx, db.ExistsDocumentTemplateColumnKeyParams{
		SheetID:   sheetID,
		ColumnKey: key,
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *DocumentTemplateColumnRepositoryImpl) ExistsName(
	ctx context.Context,
	sheetID uuid.UUID,
	name string,
) (bool, error) {

	exists, err := r.q.ExistsDocumentTemplateColumnName(ctx, db.ExistsDocumentTemplateColumnNameParams{
		SheetID:    sheetID,
		ColumnName: name,
	})
	if err != nil {
		return false, err
	}

	return exists, nil
}
