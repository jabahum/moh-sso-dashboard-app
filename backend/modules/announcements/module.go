package announcements

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const ID = "announcements"
const ReadPermission = "announcements:read"
const WritePermission = "announcements:write"
const PublishPermission = "announcements:publish"
const WritePolicy = "announcements-write"
const PublishPolicy = "announcements-publish"
const WriteLimit = 60
const PublishLimit = 30
const PolicyWindow = time.Minute

var ErrInvalidDependencies = errors.New("invalid Announcements dependencies")

type accessFailure int

func (e accessFailure) Error() string   { return http.StatusText(int(e)) }
func (e accessFailure) StatusCode() int { return int(e) }

var ErrUnauthorized error = accessFailure(401)
var ErrForbidden error = accessFailure(403)

// Actor contains only verified platform identity/claims, never request parameters.
// ClientIDs may contain either the platform UUID or the OIDC client identifier.
type Actor struct {
	UserID     uuid.UUID
	RealmRoles []string
	ClientIDs  []string
}

// AuditRecorder is satisfied by a shared core event recorder; the module does not
// own core audit persistence. Recording retains the existing best-effort behavior.
type AuditRecorder interface {
	Log(context.Context, uuid.NullUUID, string, map[string]any) error
}

// Dependencies are composed once before serving. Pools and providers remain
// caller-owned. Exactly one Database or Repository must be supplied.
// Capabilities must inspect verified identity; raw headers are never trusted here.
type Dependencies struct {
	Database     *sql.DB
	Repository   Repository
	Ports        ServicePorts
	Settings     *Settings
	Actor        func(*http.Request) (Actor, error)
	RequireAdmin func(*http.Request) error
	AuthorizeAny func(*http.Request, ...string) error
	Limit        func(*http.Request, string, int, time.Duration) (http.Header, error)
	Audit        AuditRecorder
	RequestID    func(*http.Request) string
	StreamError  func(*http.Request, error)
	// MaxRequestBytes bounds JSON and multipart bodies; zero selects 16 MiB.
	MaxRequestBytes int64
}
type Registrar interface{ Handle(string, http.Handler) }
type Module struct {
	mux      *http.ServeMux
	patterns []string
}

func NewModule(deps Dependencies) (*Module, error) {
	if deps.Actor == nil || deps.RequireAdmin == nil || deps.AuthorizeAny == nil || deps.Limit == nil {
		return nil, fmt.Errorf("%w: actor, admin, permission and rate capabilities are required", ErrInvalidDependencies)
	}
	if (deps.Database == nil) == (deps.Repository == nil) {
		return nil, fmt.Errorf("%w: provide exactly one database or repository", ErrInvalidDependencies)
	}
	if deps.MaxRequestBytes < 0 {
		return nil, fmt.Errorf("%w: invalid request body limit", ErrInvalidDependencies)
	}
	if deps.MaxRequestBytes == 0 {
		deps.MaxRequestBytes = 16 << 20
	}
	repo := deps.Repository
	if repo == nil {
		var err error
		repo, err = NewRepository(deps.Database)
		if err != nil {
			return nil, err
		}
	}
	h := newHTTPHandler(NewServiceWithSettings(repo, deps.Ports, deps.Settings), deps.Audit)
	m := &Module{mux: http.NewServeMux()}
	for _, endpoint := range endpoints(h) {
		pattern := endpoint.method + " " + endpoint.path
		m.patterns = append(m.patterns, pattern)
		m.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			c := &httpRequest{Request: r, Writer: w, admin: endpoint.admin, streamError: deps.StreamError,
				authorize: func(permissions ...string) error { return deps.AuthorizeAny(r, permissions...) }}
			if deps.RequestID != nil {
				c.requestID = deps.RequestID(r)
			}
			if r.Method != endpoint.method {
				w.Header().Set("Allow", endpoint.method)
				w.WriteHeader(405)
				return
			}
			if endpoint.permission != "" {
				actor, err := deps.Actor(r)
				if err != nil {
					c.reject(err)
					return
				}
				if actor.UserID == uuid.Nil {
					c.reject(ErrUnauthorized)
					return
				}
				c.actor = actor
				if endpoint.admin {
					if err := deps.RequireAdmin(r); err != nil {
						c.reject(err)
						return
					}
				}
				if err := deps.AuthorizeAny(r, endpoint.permission); err != nil {
					c.reject(err)
					return
				}
				if endpoint.policy != "" {
					limit := WriteLimit
					if endpoint.policy == PublishPolicy {
						limit = PublishLimit
					}
					headers, err := deps.Limit(r, endpoint.policy, limit, PolicyWindow)
					for key, values := range headers {
						w.Header().Del(key)
						for _, value := range values {
							w.Header().Add(key, value)
						}
					}
					if err != nil {
						c.reject(err)
						return
					}
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, deps.MaxRequestBytes)
			defer func() {
				if r.MultipartForm != nil {
					_ = r.MultipartForm.RemoveAll()
				}
			}()
			endpoint.serve(c)
		})
	}
	return m, nil
}
func Register(registrar Registrar, deps Dependencies) (*Module, error) {
	if registrar == nil {
		return nil, fmt.Errorf("%w: registrar is required", ErrInvalidDependencies)
	}
	m, err := NewModule(deps)
	if err != nil {
		return nil, err
	}
	for _, pattern := range m.patterns {
		registrar.Handle(pattern, m)
	}
	return m, nil
}
func (m *Module) Patterns() []string                               { return append([]string(nil), m.patterns...) }
func (m *Module) ServeHTTP(w http.ResponseWriter, r *http.Request) { m.mux.ServeHTTP(w, r) }

type httpRequest struct {
	Request     *http.Request
	Writer      http.ResponseWriter
	actor       Actor
	admin       bool
	requestID   string
	streamError func(*http.Request, error)
	// authorize checks additional permissions inside a handler, for requests
	// whose body asks for more than the route's own permission.
	authorize func(permissions ...string) error
}

func (c *httpRequest) Query(key string) string { return c.Request.URL.Query().Get(key) }
func (c *httpRequest) DefaultQuery(key, fallback string) string {
	if values, ok := c.Request.URL.Query()[key]; ok && len(values) > 0 {
		return values[0]
	}
	return fallback
}
func (c *httpRequest) Param(key string) string { return c.Request.PathValue(key) }
func (c *httpRequest) GetString(key string) string {
	if key == "user_id" && c.actor.UserID != uuid.Nil {
		return c.actor.UserID.String()
	}
	return ""
}
func (c *httpRequest) Header(key, value string)    { c.Writer.Header().Set(key, value) }
func (c *httpRequest) GetHeader(key string) string { return c.Request.Header.Get(key) }
func (c *httpRequest) PostForm(key string) string  { return c.Request.PostFormValue(key) }
func (c *httpRequest) FormFile(key string) (*multipart.FileHeader, error) {
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		return nil, err
	}
	file, header, err := c.Request.FormFile(key)
	if err != nil {
		return nil, err
	}
	_ = file.Close()
	return header, nil
}
func (c *httpRequest) ShouldBindJSON(dst any) error {
	if err := json.NewDecoder(c.Request.Body).Decode(dst); err != nil {
		return err
	}
	// Preserve required-string validation from the existing HTTP DTOs.
	value := reflect.ValueOf(dst).Elem()
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		if typ.Field(i).Tag.Get("binding") == "required" && value.Field(i).IsZero() {
			return errors.New("required field missing")
		}
	}
	return nil
}
func (c *httpRequest) ok(status int, data any) {
	c.write(status, map[string]any{"success": true, "data": data})
}
func (c *httpRequest) fail(status int, code, message string) {
	c.write(status, map[string]any{"success": false, "error": map[string]string{"code": code, "message": message}})
}
func (c *httpRequest) write(status int, body map[string]any) {
	meta := map[string]string{}
	if c.requestID != "" {
		meta["requestId"] = c.requestID
	}
	body["meta"] = meta
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(status)
	_ = json.NewEncoder(c.Writer).Encode(body)
}
func (c *httpRequest) reject(err error) {
	status := 503
	var failure interface{ StatusCode() int }
	if errors.As(err, &failure) {
		switch failure.StatusCode() {
		case 401, 403, 429:
			status = failure.StatusCode()
		}
	}
	code, message := "CAPABILITY_UNAVAILABLE", "required platform capability unavailable"
	switch status {
	case 401:
		code, message = "UNAUTHORIZED", "unauthorized"
	case 403:
		code, message = "FORBIDDEN", "announcement access denied"
	case 429:
		code, message = "RATE_LIMIT_EXCEEDED", "rate limit exceeded"
		var retry interface{ RetryAfter() time.Duration }
		if errors.As(err, &retry) && retry.RetryAfter() > 0 {
			c.Header("Retry-After", strconv.FormatInt(int64(math.Ceil(retry.RetryAfter().Seconds())), 10))
		}
	}
	c.fail(status, code, message)
}
