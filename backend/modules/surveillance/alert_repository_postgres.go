package surveillance

import (
	"context"
	"errors"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type postgresAlertRepository struct {
	db db.Store
}

func NewAlertRepository(db db.Store) AlertRepository {
	return &postgresAlertRepository{db: db}
}

func (r *postgresAlertRepository) Create(ctx context.Context, arg db.CreateAlertParams) (db.Alert, error) {
	alert, err := r.db.CreateAlert(ctx, arg)
	if err != nil {
		return db.Alert{}, err
	}
	return alert, nil
}

func (r *postgresAlertRepository) UpsertByExternalID(ctx context.Context, arg db.UpsertAlertByExternalIDParams) (db.Alert, error) {
	alert, err := r.db.UpsertAlertByExternalID(ctx, arg)
	if err != nil {
		return db.Alert{}, err
	}
	return alert, nil
}

func (r *postgresAlertRepository) GetByID(ctx context.Context, id uuid.UUID) (db.Alert, error) {
	if id == uuid.Nil {
		return db.Alert{}, errors.New("alert id is required")
	}

	alert, err := r.db.GetAlertByID(ctx, id)
	if err != nil {
		return db.Alert{}, err
	}

	return alert, nil
}

func (r *postgresAlertRepository) ListAlerts(ctx context.Context, arg db.ListAlertsParams) ([]db.ListAlertsRow, error) {
	return r.db.ListAlerts(ctx, arg)
}

func (r *postgresAlertRepository) ListAlertsInHealthContext(
	ctx context.Context,
	arg db.ListAlertsParams,
	scope HealthContextScope,
) ([]db.ListAlertsRow, error) {
	allowedIDs, err := scope.allowedIDs(ctx, "surveillance-district", false)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.ListAlertsInHealthContext(ctx, db.ListAlertsInHealthContextParams{
		EpiWeekID: arg.EpiWeekID, DiseaseID: arg.DiseaseID, DistrictID: arg.DistrictID,
		RegionID: arg.RegionID, AllowedDistrictIds: allowedIDs,
	})
	if err != nil {
		return nil, err
	}
	items := make([]db.ListAlertsRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, db.ListAlertsRow(row))
	}
	return items, nil
}

func (r *postgresAlertRepository) ListByDisease(ctx context.Context, diseaseID uuid.UUID) ([]db.ListAlertsByDiseaseRow, error) {

	if diseaseID == uuid.Nil {
		return []db.ListAlertsByDiseaseRow{}, errors.New("disease id is required")
	}

	alerts, err := r.db.ListAlertsByDisease(ctx, diseaseID)

	if err != nil {
		return []db.ListAlertsByDiseaseRow{}, err
	}

	return alerts, nil
}

func (r *postgresAlertRepository) ListByDistrict(
	ctx context.Context,
	districtID uuid.UUID,
) ([]db.ListAlertsByDistrictRow, error) {
	if districtID == uuid.Nil {
		return []db.ListAlertsByDistrictRow{}, errors.New("district id is required")
	}

	param := uuid.NullUUID{
		UUID:  districtID,
		Valid: true,
	}

	alerts, err := r.db.ListAlertsByDistrict(ctx, param)
	if err != nil {
		return []db.ListAlertsByDistrictRow{}, err
	}

	return alerts, nil
}

func (r *postgresAlertRepository) ListByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.ListAlertsByWeekRow, error) {
	if epiWeekID == uuid.Nil {
		return []db.ListAlertsByWeekRow{}, errors.New("epi_week id is required")
	}
	param := uuid.NullUUID{
		UUID:  epiWeekID,
		Valid: true,
	}
	alerts, err := r.db.ListAlertsByWeek(ctx, param)
	if err != nil {
		return []db.ListAlertsByWeekRow{}, err
	}

	return alerts, nil
}

func (r *postgresAlertRepository) UpdateStatus(ctx context.Context, arg db.UpdateAlertStatusParams) (db.Alert, error) {
	return r.db.UpdateAlertStatus(ctx, arg)
}

func (r *postgresAlertRepository) WithTx(ctx context.Context, fn func(q db.Querier) error) error {
	return r.db.ExecTx(ctx, fn)
}
