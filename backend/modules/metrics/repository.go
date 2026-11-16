package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics/model"
)

// Repository defines all data access needed by the metrics service.
type Repository interface {
	CountUsers(ctx context.Context) (int64, error)
	CountDisabledUsers(ctx context.Context) (int64, error)
	CountActiveUsers(ctx context.Context) (int64, error)
	CountActiveUsersInRange(ctx context.Context, start, end time.Time) (int64, error)
	ActiveUsersToday(ctx context.Context) (int64, error)
	ActiveUsersThisWeek(ctx context.Context) (int64, error)
	LoginTrend(ctx context.Context) ([]db.LoginTrendRow, error)
	LoginTrendByDay(ctx context.Context, start, end time.Time) ([]db.LoginTrendByDayRow, error)
	LoginSuccessFailureInRange(ctx context.Context, start, end time.Time) (db.LoginSuccessFailureInRangeRow, error)
	TotalLoginsInRange(ctx context.Context, start, end time.Time) (int64, error)
	ApproximateActiveSessions(ctx context.Context) (int64, error)
	CountFailedLogins(ctx context.Context) (int64, error)
	CountFailedLoginsInRange(ctx context.Context, start, end time.Time) (int64, error)
	CountPasswordResetsInRange(ctx context.Context, start, end time.Time) (int64, error)
	SuspiciousLoginsInRange(ctx context.Context, start, end time.Time, homeCountry string) ([]db.SuspiciousLoginsInRangeRow, error)
	CountClients(ctx context.Context) (int64, error)
	CountEnabledClients(ctx context.Context) (int64, error)
	MostActiveClients(ctx context.Context) ([]db.MostActiveClientsRow, error)
	MostAccessedClients(ctx context.Context, start, end time.Time, rowLimit int32) ([]db.MostAccessedClientsRow, error)
	LoginCountForClientInRange(ctx context.Context, clientID string, start, end time.Time) (int64, error)
	ActiveUsersPerClientToday(ctx context.Context) ([]db.ActiveUsersPerClientTodayRow, error)
	NewClientsInRange(ctx context.Context, start, end time.Time) ([]db.NewClientsInRangeRow, error)
	RecentlyCreatedClients(ctx context.Context, rowLimit int32) ([]db.RecentlyCreatedClientsRow, error)
	NewUsersInRange(ctx context.Context, start, end time.Time) ([]db.User, error)
	NewUsersTrend(ctx context.Context, start, end time.Time) ([]db.NewUsersTrendRow, error)
	CountNewUsersToday(ctx context.Context) (int64, error)
	CountNewUsersThisWeek(ctx context.Context) (int64, error)
	NeverLoggedInUsers(ctx context.Context) ([]db.User, error)
	LastLoginForUser(ctx context.Context, userID uuid.UUID) (sql.NullTime, error)
	LastLoginForAllUsers(ctx context.Context) ([]db.LastLoginForAllUsersRow, error)
	LoginCountPerUserInRange(ctx context.Context, start, end time.Time) ([]db.LoginCountPerUserInRangeRow, error)
	ClientUsageForUserInRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.ClientUsageForUserInRangeRow, error)
}
