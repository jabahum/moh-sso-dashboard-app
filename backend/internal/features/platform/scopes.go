package platform

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// scopes.json is the single list of platform capability scopes: their names,
// what they allow, which token types carry them and the endpoints that require
// them. The admin API, binding validation, the docs and the Keycloak scope
// reconciliation (infra/scripts/reconcile-platform-scopes.py) all read it.
//
//go:embed scopes.json
var scopeCatalogueJSON []byte

// Scope is one platform capability scope.
type Scope struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Tokens says which tokens carry the scope: "machine" (a bound service
	// client) and/or "user" (a signed-in user's token at the application).
	Tokens    []string `json:"tokens"`
	Endpoints []string `json:"endpoints"`
}

// ScopeCatalogue is the embedded catalogue.
type ScopeCatalogue struct {
	Version int     `json:"version"`
	Scopes  []Scope `json:"scopes"`
}

var (
	scopeNamePattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*:[a-z][a-z0-9-]*$`)
	scopeEndpointPattern = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE) /api/v1/platform/\S+$`)
	scopeCatalogue       = mustParseScopeCatalogue(scopeCatalogueJSON)
)

func mustParseScopeCatalogue(data []byte) ScopeCatalogue {
	catalogue, err := parseScopeCatalogue(data)
	if err != nil {
		panic(err)
	}
	return catalogue
}

// parseScopeCatalogue reads and checks a catalogue: version 1, unique
// resource:action names, a description, valid token types and endpoints.
func parseScopeCatalogue(data []byte) (ScopeCatalogue, error) {
	var catalogue ScopeCatalogue
	if err := json.Unmarshal(data, &catalogue); err != nil {
		return catalogue, fmt.Errorf("platform scope catalogue: %w", err)
	}
	if catalogue.Version != 1 || len(catalogue.Scopes) == 0 {
		return catalogue, fmt.Errorf("platform scope catalogue: unsupported version or no scopes")
	}
	seen := map[string]bool{}
	for _, scope := range catalogue.Scopes {
		switch {
		case !scopeNamePattern.MatchString(scope.Name):
			return catalogue, fmt.Errorf("platform scope catalogue: invalid name %q", scope.Name)
		case seen[scope.Name]:
			return catalogue, fmt.Errorf("platform scope catalogue: duplicate %s", scope.Name)
		case strings.TrimSpace(scope.Description) == "":
			return catalogue, fmt.Errorf("platform scope catalogue: %s has no description", scope.Name)
		case len(scope.Tokens) == 0 || len(scope.Endpoints) == 0:
			return catalogue, fmt.Errorf("platform scope catalogue: %s needs token types and endpoints", scope.Name)
		}
		for _, token := range scope.Tokens {
			if token != "machine" && token != "user" {
				return catalogue, fmt.Errorf("platform scope catalogue: %s has unknown token type %q", scope.Name, token)
			}
		}
		for _, endpoint := range scope.Endpoints {
			if !scopeEndpointPattern.MatchString(endpoint) {
				return catalogue, fmt.Errorf("platform scope catalogue: %s has invalid endpoint %q", scope.Name, endpoint)
			}
		}
		seen[scope.Name] = true
	}
	return catalogue, nil
}

// Scopes returns the platform scope catalogue.
func Scopes() []Scope { return append([]Scope(nil), scopeCatalogue.Scopes...) }

// KnownScope reports whether name is in the catalogue.
func KnownScope(name string) bool {
	for _, scope := range scopeCatalogue.Scopes {
		if scope.Name == name {
			return true
		}
	}
	return false
}

// MachineScope reports whether name is in the catalogue and can be granted to
// a bound machine client.
func MachineScope(name string) bool {
	for _, scope := range scopeCatalogue.Scopes {
		if scope.Name == name {
			return contains(scope.Tokens, "machine")
		}
	}
	return false
}

func machineScopeNames() []string {
	var names []string
	for _, scope := range scopeCatalogue.Scopes {
		if contains(scope.Tokens, "machine") {
			names = append(names, scope.Name)
		}
	}
	return names
}
