package routes

import (
	"github.com/gin-gonic/gin"

	documenttemplatesfeature "github.com/moh-sso-dashboard/internal/features/document_templates"
)

func registerDocumentRoutes(protected *gin.RouterGroup, deps Dependencies) {
	documenttemplatesfeature.RegisterProtectedRoutes(protected, deps.DocumentTemplates)
}
