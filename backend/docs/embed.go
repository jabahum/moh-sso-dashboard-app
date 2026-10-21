// Package docs serves the embedded OpenAPI contract and self-hosted Swagger UI.
package docs

import (
	"embed"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.yaml.in/yaml/v3"
)

//go:embed openapi.yaml swagger/*
var files embed.FS

func YAML() []byte {
	data, err := files.ReadFile("openapi.yaml")
	if err != nil {
		panic(err)
	}
	return data
}

func JSON() ([]byte, error) {
	var spec map[string]any
	if err := yaml.Unmarshal(YAML(), &spec); err != nil {
		return nil, err
	}
	return json.Marshal(spec)
}

// Register does not change API authentication; Try it out uses the normal
// bearer token or an existing same-origin HttpOnly session cookie.
func Register(r *gin.Engine) {
	r.GET("/swagger", func(c *gin.Context) {
		// http.Redirect normalizes relative locations against the stripped backend
		// path, losing reverse-proxy prefixes. Let the browser resolve this one.
		c.Header("Location", "swagger/")
		c.Status(http.StatusTemporaryRedirect)
	})
	r.GET("/swagger/*path", func(c *gin.Context) {
		name := c.Param("path")
		switch name {
		case "/openapi.yaml":
			c.Data(http.StatusOK, "application/yaml; charset=utf-8", YAML())
			return
		case "/openapi.json":
			data, err := JSON()
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			c.Data(http.StatusOK, "application/json; charset=utf-8", data)
			return
		case "/", "/index.html":
			name = "/index.html"
		case "/swagger-ui-bundle.js", "/swagger-ui.css", "/swagger-initializer.js", "/LICENSE", "/swagger-ui-bundle.js.LICENSE.txt":
		default:
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		data, err := files.ReadFile("swagger" + name)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		contentType := "text/html; charset=utf-8"
		if name == "/swagger-ui.css" {
			contentType = "text/css; charset=utf-8"
		}
		if name == "/swagger-ui-bundle.js" || name == "/swagger-initializer.js" {
			contentType = "text/javascript; charset=utf-8"
		}
		if name == "/LICENSE" || name == "/swagger-ui-bundle.js.LICENSE.txt" {
			contentType = "text/plain; charset=utf-8"
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, contentType, data)
	})
}
