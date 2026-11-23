package modulekit

import (
	"encoding/json"
	"fmt"
	"strings"
)

// HTTPModule is a module's routed handler plus the OpenAPI document that
// describes it. The host mounts every pattern and merges the document into the
// platform contract.
type HTTPModule struct {
	*Router
	document []byte
}

// NewHTTPModule pairs a router with its OpenAPI document. The document's paths
// are relative to the API base (for example "/storage-locations/{id}") and must
// describe every routed operation, and nothing else.
func NewHTTPModule(router *Router, document []byte) (*HTTPModule, error) {
	if router == nil {
		return nil, fmt.Errorf("module router is required")
	}
	var parsed struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(document, &parsed); err != nil {
		return nil, fmt.Errorf("invalid module OpenAPI document: %w", err)
	}
	declared := map[string]bool{}
	for path, methods := range parsed.Paths {
		for method := range methods {
			switch strings.ToLower(method) {
			case "get", "post", "put", "patch", "delete", "head", "options":
				declared[strings.ToUpper(method)+" "+path] = true
			}
		}
	}
	routed := map[string]bool{}
	for _, pattern := range router.Patterns() {
		routed[pattern] = true
		if !declared[pattern] {
			return nil, fmt.Errorf("operation %s is not described in the module OpenAPI document", pattern)
		}
	}
	for operation := range declared {
		if !routed[operation] {
			return nil, fmt.Errorf("OpenAPI operation %s has no route", operation)
		}
	}
	return &HTTPModule{Router: router, document: append([]byte(nil), document...)}, nil
}

// OpenAPI returns the module's OpenAPI document.
func (m *HTTPModule) OpenAPI() []byte { return append([]byte(nil), m.document...) }
