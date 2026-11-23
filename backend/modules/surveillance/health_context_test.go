package surveillance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/mohuganda/moh-sso-dashboard/backend/core/modulekit"
)

func TestScopeAuthorizationIDsSeparateHierarchyFromDataAccess(t *testing.T) {
	facility, parentRegion := uuid.New(), uuid.New()
	node, parent := uuid.New(), uuid.New()
	directory := &memoryDirectory{
		nodes:    map[string]modulekit.HealthContextNode{node.String(): {ID: node.String()}, parent.String(): {ID: parent.String()}},
		parentOf: map[string]string{node.String(): parent.String()},
		aliases: []modulekit.HealthContextAlias{
			{NodeID: node.String(), Namespace: healthContextNamespaceFacility, ExternalID: facility.String()},
			{NodeID: node.String(), Namespace: healthContextNamespaceFacility, ExternalID: facility.String()},
			{NodeID: parent.String(), Namespace: healthContextNamespaceRegion, ExternalID: parentRegion.String()},
		},
	}
	scope := HealthContextScope{ID: node, service: directory}
	ids, err := scope.allowedIDs(context.Background(), healthContextNamespaceFacility, false)
	if err != nil || len(ids) != 1 || ids[0] != facility {
		t.Fatalf("facility scope: %v %v", ids, err)
	}
	ids, err = scope.allowedIDs(context.Background(), healthContextNamespaceRegion, false)
	if err != nil || len(ids) != 0 {
		t.Fatalf("metric scope included an ancestor: %v %v", ids, err)
	}
	ids, err = scope.allowedIDs(context.Background(), healthContextNamespaceRegion, true)
	if err != nil || len(ids) != 1 || ids[0] != parentRegion {
		t.Fatalf("hierarchy selector lost ancestor: %v %v", ids, err)
	}
	directory.failure = errors.New("primary database unavailable")
	if _, err := scope.allowedIDs(context.Background(), healthContextNamespaceFacility, false); err == nil {
		t.Fatal("resolver failure must fail closed")
	}
	if _, err := (HealthContextScope{}).allowedIDs(context.Background(), healthContextNamespaceFacility, false); err == nil {
		t.Fatal("missing resolver must fail closed")
	}
}

func TestScopeContainsAliasAndNationalNode(t *testing.T) {
	node, child, root := uuid.New(), uuid.New(), uuid.New()
	district := uuid.New()
	directory := &memoryDirectory{
		nodes:    map[string]modulekit.HealthContextNode{node.String(): {ID: node.String()}, child.String(): {ID: child.String()}, root.String(): {ID: root.String(), Code: "UG"}},
		parentOf: map[string]string{child.String(): node.String()},
		aliases:  []modulekit.HealthContextAlias{{NodeID: child.String(), Namespace: healthContextNamespaceDistrict, ExternalID: district.String()}},
	}
	exact := HealthContextScope{ID: node, service: directory}
	if ok, err := exact.containsAlias(context.Background(), directory, healthContextNamespaceDistrict, district); err != nil || ok {
		t.Fatalf("node-only scope included a descendant: %v %v", ok, err)
	}
	exact.IncludeDescendants = true
	if ok, err := exact.containsAlias(context.Background(), directory, healthContextNamespaceDistrict, district); err != nil || !ok {
		t.Fatalf("descendant scope lost its child: %v %v", ok, err)
	}
	if national, err := (HealthContextScope{ID: root, service: directory}).isNational(context.Background()); err != nil || !national {
		t.Fatalf("national node: %v %v", national, err)
	}
	if national, err := exact.isNational(context.Background()); err != nil || national {
		t.Fatalf("district node reported national: %v %v", national, err)
	}
}
