package report_scheduler

import (
 "net/http"
 "github.com/gin-gonic/gin"
 "github.com/moh-sso-dashboard/internal/http/response"
)

type Handler struct { service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) GetModule(c *gin.Context) {
 response.OK(c, http.StatusOK, h.service.Module())
}
