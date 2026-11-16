package service

import (
	repository "github.com/moh-sso-dashboard/internal/repository/metrics"
	metrics "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
)

type MetricsService = metrics.Service

func NewMetricsService(repo repository.MetricsRepository) *MetricsService {
	return metrics.NewService(repo)
}
