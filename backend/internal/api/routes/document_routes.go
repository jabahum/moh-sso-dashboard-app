package routes

import (
	"github.com/gin-gonic/gin"

	documentsfeature "github.com/moh-sso-dashboard/internal/features/documents"
)

func registerDocumentRoutes(protected *gin.RouterGroup, deps Dependencies) {
	documentsfeature.RegisterProtectedRoutes(protected, deps.Documents, deps.Limiter)
}
