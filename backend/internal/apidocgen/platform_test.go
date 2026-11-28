package apidocgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const miniSpec = `# header comment
openapi: 3.0.3
paths:
  /api/v1/platform/identity:
    get:
      x-go-handler: internal.Handler
      responses:
        "200":
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/platform.Principal'
  /api/v1/admin/users:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/admin.User'
components:
  schemas:
    platform.Principal:
      type: object
      properties:
        worker:
          $ref: '#/components/schemas/core.Worker'
    core.Worker:
      type: object
    admin.User:
      type: object
`

func TestPlatformContractExtractsOnlyPlatformPaths(t *testing.T) {
	data, err := PlatformContract([]byte(miniSpec))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, platformContractHeader) {
		t.Fatal("missing generated header")
	}
	var contract object
	if err := yaml.Unmarshal(dropHeader(data), &contract); err != nil {
		t.Fatal(err)
	}
	paths := contract["paths"].(object)
	if _, ok := paths["/api/v1/platform/identity"]; !ok || len(paths) != 1 {
		t.Fatalf("paths = %v", paths)
	}
	schemas := contract["components"].(object)["schemas"].(object)
	if _, ok := schemas["core.Worker"]; !ok || len(schemas) != 2 {
		t.Fatalf("schemas = %v, want the referenced schema and its dependency only", schemas)
	}
	if strings.Contains(text, "x-go-handler") || strings.Contains(text, "admin.User") {
		t.Fatal("internal annotations or unrelated schemas leaked into the contract")
	}
	if contract["info"].(object)["version"] != PlatformContractVersion {
		t.Fatal("version not set")
	}
}

func TestPlatformContractErrors(t *testing.T) {
	if _, err := PlatformContract([]byte("openapi: 3.0.3\npaths: {}\n")); err == nil {
		t.Fatal("accepted a document without platform paths")
	}
	broken := strings.Replace(miniSpec, "    core.Worker:\n      type: object\n", "", 1)
	if _, err := PlatformContract([]byte(broken)); err == nil || !strings.Contains(err.Error(), "core.Worker") {
		t.Fatalf("missing schema: %v", err)
	}
}

func TestPublishedPlatformContractIsCurrent(t *testing.T) {
	root := backendRoot(t)
	full, err := os.ReadFile(filepath.Join(root, "docs/openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := PlatformContract(full)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, PlatformContractFile))
	if err != nil || string(got) != string(want) {
		t.Fatalf("%s is stale; run go run ./cmd/cli openapi generate", PlatformContractFile)
	}
	if changes, err := BreakingChanges(got, got); err != nil || len(changes) != 0 {
		t.Fatalf("contract not compatible with itself: %v %v", changes, err)
	}
}

func TestApplyOverridesRequiredParametersAndErrorResponses(t *testing.T) {
	op := object{
		"parameters": []any{object{"in": "header", "name": "Idempotency-Key", "required": false}},
		"responses":  object{"202": object{"description": "Accepted"}},
	}
	applyOverrides(op, object{
		"x-required-parameters": []any{"header Idempotency-Key"},
		"x-error-responses":     []any{"409", "202"},
	})
	if op["parameters"].([]any)[0].(object)["required"] != true {
		t.Fatal("parameter not marked required")
	}
	responses := op["responses"].(object)
	if _, ok := responses["409"]; !ok || responses["202"].(object)["description"] != "Accepted" {
		t.Fatalf("responses = %v", responses)
	}
}
