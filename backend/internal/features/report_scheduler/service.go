package report_scheduler

import "context"

type Service struct {
	repo Repository
	healthBI HealthBIClient
}

func NewService(repo Repository, healthBI HealthBIClient) *Service { return &Service{repo: repo, healthBI: healthBI} }

func (s *Service) Module() ModuleResponse {
	enabled := s != nil && s.healthBI != nil && s.healthBI.Enabled()
	return ModuleResponse{Name: "Report Scheduler", Status: "ready", SchedulingEnabled: s != nil && s.repo != nil, HealthBIEnabled: enabled}
}

func (s *Service) ListReports(ctx context.Context) ([]HealthBIReport, error) { return s.healthBI.ListReports(ctx) }
func (s *Service) GetReport(ctx context.Context, id string) (HealthBIReport, error) { return s.healthBI.GetReport(ctx, id) }
func (s *Service) GetParameters(ctx context.Context, id string) ([]HealthBIParameter, error) { return s.healthBI.GetReportParameters(ctx, id) }
func (s *Service) Generate(ctx context.Context, id string, request GenerateReportRequest) (HealthBIJob, error) { return s.healthBI.GenerateReport(ctx, id, request) }
func (s *Service) GetJob(ctx context.Context, id string) (HealthBIJob, error) { return s.healthBI.GetJob(ctx, id) }
func (s *Service) CreateSchedule(ctx context.Context, input CreateScheduleRequest, userID string) (Schedule, error) { return s.repo.CreateSchedule(ctx, input, userID) }
func (s *Service) ListSchedules(ctx context.Context, userID string, all bool) ([]Schedule, error) { return s.repo.ListSchedules(ctx, userID, all) }
func (s *Service) GetSchedule(ctx context.Context, id, userID string, all bool) (Schedule, error) { return s.repo.GetSchedule(ctx, id, userID, all) }
func (s *Service) UpdateSchedule(ctx context.Context, id string, input UpdateScheduleRequest, userID string, all bool) (Schedule, error) { return s.repo.UpdateSchedule(ctx, id, input, userID, all) }
func (s *Service) DeleteSchedule(ctx context.Context, id, userID string, all bool) error { return s.repo.DeleteSchedule(ctx, id, userID, all) }
func (s *Service) ListExecutions(ctx context.Context, userID string, all bool) ([]Execution, error) { return s.repo.ListExecutions(ctx, userID, all) }
