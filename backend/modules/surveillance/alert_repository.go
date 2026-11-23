package surveillance

import (
	"context"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
)

type AlertRepository interface {
	Create(ctx context.Context, arg db.CreateAlertParams) (db.Alert, error)
	UpsertByExternalID(ctx context.Context, arg db.UpsertAlertByExternalIDParams) (db.Alert, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.Alert, error)
	ListAlerts(ctx context.Context, arg db.ListAlertsParams) ([]db.ListAlertsRow, error)
	ListAlertsInHealthContext(
		ctx context.Context,
		arg db.ListAlertsParams,
		scope HealthContextScope,
	) ([]db.ListAlertsRow, error)
	ListByDisease(ctx context.Context, diseaseID uuid.UUID) ([]db.ListAlertsByDiseaseRow, error)
	ListByDistrict(ctx context.Context, districtID uuid.UUID) ([]db.ListAlertsByDistrictRow, error)
	ListByWeek(ctx context.Context, epiWeekID uuid.UUID) ([]db.ListAlertsByWeekRow, error)
	UpdateStatus(ctx context.Context, arg db.UpdateAlertStatusParams) (db.Alert, error)
	WithTx(ctx context.Context, fn func(q db.Querier) error) error
}
