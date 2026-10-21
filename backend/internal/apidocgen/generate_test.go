package apidocgen

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func backendRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate backend")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func operation(t *testing.T, spec object, path, method string) object {
	t.Helper()
	paths := spec["paths"].(object)
	methods, ok := paths[path].(object)
	if !ok {
		t.Fatalf("missing path %s", path)
	}
	op, ok := methods[method].(object)
	if !ok {
		t.Fatalf("missing %s %s", method, path)
	}
	return op
}

func TestCompleteContractIsCurrent(t *testing.T) {
	dir := backendRoot(t)
	generated, err := Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(dir, "docs/openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, actual) {
		t.Fatal("API documentation is stale: cd backend && go run ./cmd/cli openapi generate")
	}
	var spec object
	if err := yaml.Unmarshal(generated, &spec); err != nil {
		t.Fatal(err)
	}
	if err := Validate(spec); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range spec["paths"].(object) {
		for method := range item.(object) {
			if isHTTPMethod(strings.ToUpper(method)) {
				count++
			}
		}
	}
	if count != len(Routes()) {
		t.Fatalf("documented %d operations, registered %d", count, len(Routes()))
	}

	t.Run("current session authentication", func(t *testing.T) {
		schemes := spec["components"].(object)["securitySchemes"].(object)
		if schemes["sessionCookie"].(object)["name"] != "sso_session" {
			t.Fatal("session cookie documentation is stale")
		}
		public := operation(t, spec, "/api/v1/announcements/public", "get")
		if len(public["security"].([]any)) != 0 {
			t.Fatal("public announcements must not require auth")
		}
		protected := operation(t, spec, "/api/v1/users", "get")
		if _, overridden := protected["security"]; overridden {
			t.Fatal("protected operation must inherit authentication")
		}
	})
	t.Run("typed JSON and responses", func(t *testing.T) {
		op := operation(t, spec, "/api/v1/admin/users", "post")
		body := op["requestBody"].(object)["content"].(object)["application/json"].(object)["schema"].(object)
		if body["$ref"] != "#/components/schemas/users.CreateUserRequest" {
			t.Fatalf("untyped create user body: %v", body)
		}
		response := op["responses"].(object)["201"].(object)["content"].(object)["application/json"].(object)["schema"].(object)
		data := response["properties"].(object)["data"].(object)
		if data["$ref"] != "#/components/schemas/users.UserResponse" {
			t.Fatalf("untyped response: %v", data)
		}
	})
	t.Run("multipart plus JSON attachment alternatives", func(t *testing.T) {
		op := operation(t, spec, "/api/v1/admin/announcements/{id}/attachments", "post")
		content := op["requestBody"].(object)["content"].(object)
		if content["multipart/form-data"] == nil || content["application/json"] == nil {
			t.Fatal("both attachment upload formats must be documented")
		}
	})
	t.Run("required upload storage location", func(t *testing.T) {
		op := operation(t, spec, "/api/v1/documents", "post")
		body := op["requestBody"].(object)["content"].(object)["multipart/form-data"].(object)["schema"].(object)
		required := map[string]bool{}
		for _, item := range body["required"].([]any) {
			required[item.(string)] = true
		}
		if !required["file"] || !required["storage_location"] {
			t.Fatal("upload fields must be required")
		}
	})
	t.Run("redirect status codes follow actual handler", func(t *testing.T) {
		op := operation(t, spec, "/api/v1/auth/login", "get")
		if op["responses"].(object)["307"] == nil {
			t.Fatal("login redirect status must match http.StatusTemporaryRedirect")
		}
	})
	t.Run("unknown references are rejected", func(t *testing.T) {
		definition := spec["components"].(object)["schemas"].(object)
		definition["broken"] = object{"$ref": "#/components/schemas/does-not-exist"}
		if err := Validate(spec); err == nil {
			t.Fatal("unresolved reference was accepted")
		}
		delete(definition, "broken")
	})
}

func TestOpenAPIPath(t *testing.T) {
	if got := OpenAPIPath("/api/v1/users/:id/clients/:clientID/roles"); got != "/api/v1/users/{id}/clients/{clientID}/roles" {
		t.Fatal(got)
	}
}
