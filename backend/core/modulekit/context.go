package modulekit

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/mohuganda/moh-sso-dashboard/backend/core"
)

// MaxMultipartMemory matches the host's default in-memory multipart budget.
const MaxMultipartMemory = 32 << 20

// Context gives a handler its request, verified caller and response helpers.
type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request
}

// NewContext wraps a request for handlers used outside a Router (tests, adapters).
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{Writer: w, Request: r}
}

// Context returns the request context.
func (c *Context) Context() context.Context { return c.Request.Context() }

// Access returns the verified caller. ok is false for anonymous requests.
func (c *Context) Access() (core.RequestAccess, bool) { return AccessFromRequest(c.Request) }

// Subject returns the verified caller's subject (user ID), or "".
func (c *Context) Subject() string {
	access, _ := c.Access()
	return access.Subject
}

// HealthContext returns the selected, already-authorized health-context node,
// or nil when none was selected.
func (c *Context) HealthContext() *HealthContextSelection {
	request, _ := FromRequest(c.Request)
	return request.HealthContext
}

// RequestID returns the request correlation ID.
func (c *Context) RequestID() string {
	request, _ := FromRequest(c.Request)
	return request.RequestID
}

// Param returns a path parameter declared in the endpoint pattern.
func (c *Context) Param(name string) string { return c.Request.PathValue(name) }

// Query returns the first value of a query parameter, or "".
func (c *Context) Query(name string) string { return c.Request.URL.Query().Get(name) }

// GetQuery returns the first value of a query parameter and whether it was sent.
func (c *Context) GetQuery(name string) (string, bool) {
	values, ok := c.Request.URL.Query()[name]
	if !ok || len(values) == 0 {
		return "", false
	}
	return values[0], true
}

// DefaultQuery returns a query parameter, or fallback when it was not sent.
func (c *Context) DefaultQuery(name, fallback string) string {
	if value, ok := c.GetQuery(name); ok {
		return value
	}
	return fallback
}

// QueryArray returns every value of a query parameter.
func (c *Context) QueryArray(name string) []string { return c.Request.URL.Query()[name] }

// GetHeader returns a request header.
func (c *Context) GetHeader(name string) string { return c.Request.Header.Get(name) }

// Header sets a response header.
func (c *Context) Header(name, value string) {
	if value == "" {
		c.Writer.Header().Del(name)
		return
	}
	c.Writer.Header().Set(name, value)
}

// BindJSON decodes the JSON body into target and applies its binding rules.
func (c *Context) BindJSON(target any) error { return BindJSON(c.Request, target) }

// FormFile returns the first uploaded file for a multipart field.
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(MaxMultipartMemory); err != nil {
			return nil, err
		}
	}
	file, header, err := c.Request.FormFile(name)
	if err != nil {
		return nil, err
	}
	_ = file.Close()
	return header, nil
}

// PostForm returns a multipart or URL-encoded form value.
func (c *Context) PostForm(name string) string {
	if c.Request.MultipartForm == nil && strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/") {
		_ = c.Request.ParseMultipartForm(MaxMultipartMemory)
	}
	return c.Request.PostFormValue(name)
}

// OK writes a success envelope.
func (c *Context) OK(status int, data any) { WriteOK(c.Writer, c.Request, status, data) }

// Fail writes an error envelope.
func (c *Context) Fail(status int, code, message string) {
	WriteFail(c.Writer, c.Request, status, code, message)
}

// Error writes err as an error envelope (see WriteError).
func (c *Context) Error(err error) { WriteError(c.Writer, c.Request, err) }

// JSON writes a raw JSON value without the envelope.
func (c *Context) JSON(status int, value any) { WriteJSON(c.Writer, status, value) }

// Data writes raw bytes with a content type.
func (c *Context) Data(status int, contentType string, body []byte) {
	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.WriteHeader(status)
	_, _ = c.Writer.Write(body)
}

// DataFromReader streams a body with a content type and optional extra headers.
// A negative length omits Content-Length.
func (c *Context) DataFromReader(status int, length int64, contentType string, body io.Reader, headers map[string]string) {
	for name, value := range headers {
		c.Writer.Header().Set(name, value)
	}
	if length >= 0 {
		c.Writer.Header().Set("Content-Length", formatInt(length))
	}
	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.WriteHeader(status)
	_, _ = io.Copy(c.Writer, body)
}

// Status writes only a status code.
func (c *Context) Status(status int) { c.Writer.WriteHeader(status) }
