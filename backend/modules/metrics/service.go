package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics/model"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CountUsers(ctx context.Context) (int64, error) {
	return s.repo.CountUsers(ctx)
}

func (s *Service) CountDisabledUsers(ctx context.Context) (int64, error) {
	return s.repo.CountDisabledUsers(ctx)
}

func (s *Service) CountActiveUsers(ctx context.Context) (int64, error) {
	return s.repo.CountActiveUsers(ctx)
}

func (s *Service) CountActiveUsersInRange(ctx context.Context, start, end time.Time) (int64, error) {
	return s.repo.CountActiveUsersInRange(ctx, start, end)
}

func (s *Service) ActiveUsersToday(ctx context.Context) (int64, error) {
	return s.repo.ActiveUsersToday(ctx)
}

func (s *Service) ActiveUsersThisWeek(ctx context.Context) (int64, error) {
	return s.repo.ActiveUsersThisWeek(ctx)
}

func (s *Service) LoginTrend(ctx context.Context) ([]db.LoginTrendRow, error) {
	return s.repo.LoginTrend(ctx)
}

func (s *Service) LoginTrendByDay(ctx context.Context, start, end time.Time) ([]db.LoginTrendByDayRow, error) {
	return s.repo.LoginTrendByDay(ctx, start, end)
}

func (s *Service) LoginSuccessFailure(ctx context.Context, start, end time.Time) (db.LoginSuccessFailureInRangeRow, error) {
	return s.repo.LoginSuccessFailureInRange(ctx, start, end)
}

func (s *Service) TotalLoginsInRange(ctx context.Context, start, end time.Time) (int64, error) {
	return s.repo.TotalLoginsInRange(ctx, start, end)
}

func (s *Service) ApproximateActiveSessions(ctx context.Context) (int64, error) {
	return s.repo.ApproximateActiveSessions(ctx)
}

func (s *Service) CountFailedLogins(ctx context.Context) (int64, error) {
	return s.repo.CountFailedLogins(ctx)
}

func (s *Service) CountFailedLoginsInRange(ctx context.Context, start, end time.Time) (int64, error) {
	return s.repo.CountFailedLoginsInRange(ctx, start, end)
}

func (s *Service) CountPasswordResetsInRange(ctx context.Context, start, end time.Time) (int64, error) {
	return s.repo.CountPasswordResetsInRange(ctx, start, end)
}

func (s *Service) SuspiciousLogins(ctx context.Context, start, end time.Time, homeCountry string) ([]db.SuspiciousLoginsInRangeRow, error) {
	return s.repo.SuspiciousLoginsInRange(ctx, start, end, homeCountry)
}

func (s *Service) CountClients(ctx context.Context) (int64, error) {
	return s.repo.CountClients(ctx)
}

func (s *Service) CountEnabledClients(ctx context.Context) (int64, error) {
	return s.repo.CountEnabledClients(ctx)
}

func (s *Service) MostActiveClients(ctx context.Context) ([]db.MostActiveClientsRow, error) {
	return s.repo.MostActiveClients(ctx)
}

func (s *Service) MostAccessedClients(ctx context.Context, start, end time.Time, limit int32) ([]db.MostAccessedClientsRow, error) {
	return s.repo.MostAccessedClients(ctx, start, end, limit)
}

func (s *Service) LoginCountForClient(ctx context.Context, clientID string, start, end time.Time) (int64, error) {
	return s.repo.LoginCountForClientInRange(ctx, clientID, start, end)
}

func (s *Service) ActiveUsersPerClientToday(ctx context.Context) ([]db.ActiveUsersPerClientTodayRow, error) {
	return s.repo.ActiveUsersPerClientToday(ctx)
}

func (s *Service) NewClientsInRange(ctx context.Context, start, end time.Time) ([]db.NewClientsInRangeRow, error) {
	return s.repo.NewClientsInRange(ctx, start, end)
}

func (s *Service) RecentlyCreatedClients(ctx context.Context, limit int32) ([]db.RecentlyCreatedClientsRow, error) {
	return s.repo.RecentlyCreatedClients(ctx, limit)
}

func (s *Service) NewUsersInRange(ctx context.Context, start, end time.Time) ([]db.User, error) {
	return s.repo.NewUsersInRange(ctx, start, end)
}

func (s *Service) NewUsersTrend(ctx context.Context, start, end time.Time) ([]db.NewUsersTrendRow, error) {
	return s.repo.NewUsersTrend(ctx, start, end)
}

func (s *Service) CountNewUsersToday(ctx context.Context) (int64, error) {
	return s.repo.CountNewUsersToday(ctx)
}

func (s *Service) CountNewUsersThisWeek(ctx context.Context) (int64, error) {
	return s.repo.CountNewUsersThisWeek(ctx)
}

func (s *Service) NeverLoggedInUsers(ctx context.Context) ([]db.User, error) {
	return s.repo.NeverLoggedInUsers(ctx)
}

func (s *Service) LastLoginForUser(ctx context.Context, userID uuid.UUID) (sql.NullTime, error) {
	return s.repo.LastLoginForUser(ctx, userID)
}

func (s *Service) LastLoginForAllUsers(ctx context.Context) ([]db.LastLoginForAllUsersRow, error) {
	return s.repo.LastLoginForAllUsers(ctx)
}

func (s *Service) LoginCountPerUser(ctx context.Context, start, end time.Time) ([]db.LoginCountPerUserInRangeRow, error) {
	return s.repo.LoginCountPerUserInRange(ctx, start, end)
}

func (s *Service) ClientUsageForUser(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.ClientUsageForUserInRangeRow, error) {
	return s.repo.ClientUsageForUserInRange(ctx, userID, start, end)
}
