package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/http/response"
	"github.com/moh-sso-dashboard/module"
)

type ModuleHandler struct {
	Registry *module.Registry
}

func (h *ModuleHandler) List(c *gin.Context) {
	response.OK(c, http.StatusOK, h.Registry.Inventory())
}
