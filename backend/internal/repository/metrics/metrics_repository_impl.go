package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/config"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
	logger "github.com/moh-sso-dashboard/internal/log"
	"github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics/model"
)

type metricsRepository struct {
	db     db.Store
	config *config.Config
	logger *logger.Logger
}

func NewMetricsRepository(
	cfg *config.Config,
	store db.Store,
	log logger.Logger,
) MetricsRepository {

	return &metricsRepository{
		db:     store,
		config: cfg,
		logger: &log,
	}
}

func (r *metricsRepository) CountUsers(ctx context.Context) (int64, error) {
	return r.db.CountUsers(ctx)
}

func (r *metricsRepository) CountDisabledUsers(ctx context.Context) (int64, error) {
	return r.db.CountDisabledUsers(ctx)
}

func (r *metricsRepository) CountActiveUsers(ctx context.Context) (int64, error) {
	return r.db.CountActiveUsers(ctx)
}

func (r *metricsRepository) CountActiveUsersInRange(ctx context.Context, start, end time.Time) (int64, error) {
	return r.db.CountActiveUsersInRange(ctx, db.CountActiveUsersInRangeParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
}

func (r *metricsRepository) ActiveUsersToday(ctx context.Context) (int64, error) {
	return r.db.ActiveUsersToday(ctx)
}

func (r *metricsRepository) ActiveUsersThisWeek(ctx context.Context) (int64, error) {
	return r.db.ActiveUsersThisWeek(ctx)
}

func (r *metricsRepository) LoginTrend(ctx context.Context) ([]model.LoginTrendRow, error) {
	rows, err := r.db.LoginTrend(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]model.LoginTrendRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.LoginTrendRow(row))
	}
	return result, nil
}

func (r *metricsRepository) LoginTrendByDay(ctx context.Context, start, end time.Time) ([]model.LoginTrendByDayRow, error) {
	rows, err := r.db.LoginTrendByDay(ctx, db.LoginTrendByDayParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}
	result := make([]model.LoginTrendByDayRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.LoginTrendByDayRow(row))
	}
	return result, nil
}

func (r *metricsRepository) LoginSuccessFailureInRange(ctx context.Context, start, end time.Time) (model.LoginSuccessFailureInRangeRow, error) {
	row, err := r.db.LoginSuccessFailureInRange(ctx, db.LoginSuccessFailureInRangeParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
	return model.LoginSuccessFailureInRangeRow(row), err
}

func (r *metricsRepository) TotalLoginsInRange(ctx context.Context, start, end time.Time) (int64, error) {
	return r.db.TotalLoginsInRange(ctx, db.TotalLoginsInRangeParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
}

func (r *metricsRepository) ApproximateActiveSessions(ctx context.Context) (int64, error) {
	return r.db.ApproximateActiveSessions(ctx)
}

func (r *metricsRepository) CountFailedLogins(ctx context.Context) (int64, error) {
	return r.db.CountFailedLogins(ctx)
}

func (r *metricsRepository) CountFailedLoginsInRange(ctx context.Context, start, end time.Time) (int64, error) {
	return r.db.CountFailedLoginsInRange(ctx, db.CountFailedLoginsInRangeParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
}

func (r *metricsRepository) CountPasswordResetsInRange(ctx context.Context, start, end time.Time) (int64, error) {
	return r.db.CountPasswordResetsInRange(ctx, db.CountPasswordResetsInRangeParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
}

func (r *metricsRepository) SuspiciousLoginsInRange(ctx context.Context, start, end time.Time, homeCountry string) ([]model.SuspiciousLoginsInRangeRow, error) {
	rows, err := r.db.SuspiciousLoginsInRange(ctx, db.SuspiciousLoginsInRangeParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
		HomeCountry: homeCountry,
	})
	if err != nil {
		return nil, err
	}
	result := make([]model.SuspiciousLoginsInRangeRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.SuspiciousLoginsInRangeRow(row))
	}
	return result, nil
}

func (r *metricsRepository) CountClients(ctx context.Context) (int64, error) {
	return r.db.CountClients(ctx)
}

func (r *metricsRepository) CountEnabledClients(ctx context.Context) (int64, error) {
	return r.db.CountEnabledClients(ctx)
}

func (r *metricsRepository) MostActiveClients(ctx context.Context) ([]model.MostActiveClientsRow, error) {
	rows, err := r.db.MostActiveClients(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]model.MostActiveClientsRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.MostActiveClientsRow(row))
	}
	return result, nil
}

func (r *metricsRepository) MostAccessedClients(ctx context.Context, start, end time.Time, rowLimit int32) ([]model.MostAccessedClientsRow, error) {
	rows, err := r.db.MostAccessedClients(ctx, db.MostAccessedClientsParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
		RowLimit: rowLimit,
	})
	if err != nil {
		return nil, err
	}
	result := make([]model.MostAccessedClientsRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.MostAccessedClientsRow(row))
	}
	return result, nil
}

func (r *metricsRepository) LoginCountForClientInRange(ctx context.Context, clientID string, start, end time.Time) (int64, error) {
	return r.db.LoginCountForClientInRange(ctx, db.LoginCountForClientInRangeParams{
		ClientID: clientID,
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
}

func (r *metricsRepository) ActiveUsersPerClientToday(ctx context.Context) ([]model.ActiveUsersPerClientTodayRow, error) {
	rows, err := r.db.ActiveUsersPerClientToday(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]model.ActiveUsersPerClientTodayRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.ActiveUsersPerClientTodayRow(row))
	}
	return result, nil
}

func (r *metricsRepository) NewClientsInRange(
	ctx context.Context,
	start, end time.Time,
) ([]model.NewClientsInRangeRow, error) {
	rows, err := r.db.NewClientsInRange(ctx, db.NewClientsInRangeParams{
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		return nil, err
	}
	result := make([]model.NewClientsInRangeRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.NewClientsInRangeRow(row))
	}
	return result, nil
}

func (r *metricsRepository) RecentlyCreatedClients(
	ctx context.Context,
	rowLimit int32,
) ([]model.RecentlyCreatedClientsRow, error) {
	rows, err := r.db.RecentlyCreatedClients(ctx, rowLimit)
	if err != nil {
		return nil, err
	}
	result := make([]model.RecentlyCreatedClientsRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.RecentlyCreatedClientsRow(row))
	}
	return result, nil
}

func (r *metricsRepository) NewUsersInRange(ctx context.Context, start, end time.Time) ([]model.User, error) {
	rows, err := r.db.NewUsersInRange(ctx, db.NewUsersInRangeParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}
	result := make([]model.User, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.User(row))
	}
	return result, nil
}

func (r *metricsRepository) NewUsersTrend(ctx context.Context, start, end time.Time) ([]model.NewUsersTrendRow, error) {
	rows, err := r.db.NewUsersTrend(ctx, db.NewUsersTrendParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}
	result := make([]model.NewUsersTrendRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.NewUsersTrendRow(row))
	}
	return result, nil
}

func (r *metricsRepository) CountNewUsersToday(ctx context.Context) (int64, error) {
	return r.db.CountNewUsersToday(ctx)
}

func (r *metricsRepository) CountNewUsersThisWeek(ctx context.Context) (int64, error) {
	return r.db.CountNewUsersThisWeek(ctx)
}

func (r *metricsRepository) NeverLoggedInUsers(ctx context.Context) ([]model.User, error) {
	rows, err := r.db.NeverLoggedInUsers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]model.User, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.User(row))
	}
	return result, nil
}

func (r *metricsRepository) LastLoginForUser(ctx context.Context, userID uuid.UUID) (sql.NullTime, error) {
	return r.db.LastLoginForUser(ctx, userID)
}

func (r *metricsRepository) LastLoginForAllUsers(ctx context.Context) ([]model.LastLoginForAllUsersRow, error) {
	rows, err := r.db.LastLoginForAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]model.LastLoginForAllUsersRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.LastLoginForAllUsersRow(row))
	}
	return result, nil
}

func (r *metricsRepository) LoginCountPerUserInRange(ctx context.Context, start, end time.Time) ([]model.LoginCountPerUserInRangeRow, error) {
	rows, err := r.db.LoginCountPerUserInRange(ctx, db.LoginCountPerUserInRangeParams{
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}
	result := make([]model.LoginCountPerUserInRangeRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.LoginCountPerUserInRangeRow(row))
	}
	return result, nil
}

func (r *metricsRepository) ClientUsageForUserInRange(
	ctx context.Context,
	userID uuid.UUID,
	start, end time.Time,
) ([]model.ClientUsageForUserInRangeRow, error) {

	rows, err := r.db.ClientUsageForUserInRange(ctx, db.ClientUsageForUserInRangeParams{
		UserID: userID,
		StartTime: sql.NullTime{
			Time:  start,
			Valid: true,
		},
		EndTime: sql.NullTime{
			Time:  end,
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}
	result := make([]model.ClientUsageForUserInRangeRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.ClientUsageForUserInRangeRow(row))
	}
	return result, nil
}
