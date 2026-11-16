package metrics

import (
	"github.com/moh-sso-dashboard/internal/service"
	metrics "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics"
)

type Handler = metrics.Handler

func NewHandler(s *service.MetricsService) *Handler {
	return metrics.NewHandler(s)
}
