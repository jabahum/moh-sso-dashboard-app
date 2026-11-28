package apidocgen

import (
	"strings"
	"testing"
)

const compatBase = `openapi: 3.0.3
info: {version: 1.0.0}
paths:
  /api/v1/platform/notifications:
    post:
      parameters:
        - {in: header, name: Idempotency-Key, required: true, schema: {type: string}}
        - {in: query, name: dryRun, required: false, schema: {type: boolean}}
      requestBody:
        required: true
        content:
          application/json:
            schema: {$ref: '#/components/schemas/Input'}
      responses:
        "202":
          content:
            application/json:
              schema:
                type: object
                required: [data]
                properties:
                  data: {$ref: '#/components/schemas/Status'}
        "409": {description: Conflict}
  /api/v1/platform/identity:
    get:
      responses:
        "200": {description: OK}
components:
  schemas:
    Input:
      type: object
      required: [title]
      properties:
        title: {type: string}
        name: {type: string}
    Status:
      type: object
      required: [id, status]
      properties:
        id: {type: string}
        status: {type: string}
        attempts: {type: integer}
`

func TestBreakingChanges(t *testing.T) {
	for _, tc := range []struct {
		name, find, replace, want string
	}{
		{"identical", "", "", ""},
		{"additive response field", "        attempts: {type: integer}\n", "        attempts: {type: integer}\n        extra: {type: string}\n", ""},
		{"new optional request field", "        name: {type: string}\n", "        name: {type: string}\n        note: {type: string}\n", ""},
		{"error response removed is not breaking", "        \"409\": {description: Conflict}\n", "", ""},
		{"path removed", "  /api/v1/platform/identity:\n    get:\n      responses:\n        \"200\": {description: OK}\n", "", "/api/v1/platform/identity: path removed"},
		{"operation removed", "    get:\n      responses:\n        \"200\": {description: OK}\n", "    put:\n      responses:\n        \"200\": {description: OK}\n", "GET /api/v1/platform/identity: operation removed"},
		{"success response removed", "        \"202\":", "        \"201\":", "response 202 removed"},
		{"response field removed", "        attempts: {type: integer}\n", "", "response 202.data field attempts removed"},
		{"response field no longer guaranteed", "      required: [id, status]", "      required: [id]", "field status is no longer always present"},
		{"response type changed", "        id: {type: string}\n", "        id: {type: integer}\n", "response 202.data.id type changed from string to integer"},
		{"new required request field", "      required: [title]", "      required: [title, name]", "request field name became required"},
		{"optional parameter became required", "name: dryRun, required: false", "name: dryRun, required: true", "parameter query dryRun became required"},
		{"new required parameter", "        - {in: query, name: dryRun", "        - {in: query, name: tenant, required: true, schema: {type: string}}\n        - {in: query, name: dryRun", "new required parameter query tenant"},
		{"parameter type changed", "required: false, schema: {type: boolean}", "required: false, schema: {type: string}", "parameter query dryRun type changed"},
		{"major version accepts breaking changes", "info: {version: 1.0.0}\npaths:\n  /api/v1/platform/notifications:", "info: {version: 2.0.0}\npaths:\n  /api/v2/platform/notifications:", ""},
	} {
		current := compatBase
		if tc.find != "" {
			if !strings.Contains(current, tc.find) {
				t.Fatalf("%s: fixture text not found", tc.name)
			}
			current = strings.Replace(current, tc.find, tc.replace, 1)
		}
		changes, err := BreakingChanges([]byte(compatBase), []byte(current))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		joined := strings.Join(changes, "\n")
		if tc.want == "" && len(changes) != 0 || tc.want != "" && !strings.Contains(joined, tc.want) {
			t.Errorf("%s: changes = %q, want %q", tc.name, changes, tc.want)
		}
	}
	if _, err := BreakingChanges([]byte(":"), []byte(compatBase)); err == nil {
		t.Fatal("accepted an unparsable base")
	}
}
