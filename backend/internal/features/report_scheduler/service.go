package report_scheduler

// Service owns the module contract. Scheduling and delivery will be added here.
type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) Module() ModuleResponse {
	return ModuleResponse{Name: "Report Scheduler", Status: "setup", SchedulingEnabled: false}
}
