package authorization

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"yunka.io/gateway/authz"
)

func TestEnterprise174GeneratedCatalogMatchesOperationPlans(t *testing.T) {
	path := filepath.Join("..", "..", "..", "contracts", "generated", "operation-plans.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Operations []struct {
			OperationID string `json:"operationId"`
			Domain string `json:"domain"`
			Application string `json:"application"`
			UseCase string `json:"useCase"`
			Security struct {
				TenantRequired bool `json:"tenantRequired"`
				Authentication []string `json:"authentication"`
				Permissions []string `json:"permissions"`
				PermissionMode string `json:"permissionMode"`
			} `json:"security"`
			Bindings struct {
				RPC string `json:"rpc"`
				HTTP []struct {
					Method string `json:"method"`
					Path string `json:"path"`
				} `json:"http"`
			} `json:"bindings"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(payload, &source); err != nil {
		t.Fatal(err)
	}
	expected := make([]Action, 0, len(source.Operations))
	for _, operation := range source.Operations {
		permissions := make([]authz.PermissionKey, 0, len(operation.Security.Permissions))
		for _, permission := range operation.Security.Permissions {
			permissions = append(permissions, authz.PermissionKey(permission))
		}
		httpBindings := make([]HTTPBinding, 0, len(operation.Bindings.HTTP))
		for _, binding := range operation.Bindings.HTTP {
			httpBindings = append(httpBindings, HTTPBinding{Method: binding.Method, Path: binding.Path})
		}
		mode := operation.Security.PermissionMode
		if mode == "" {
			mode = "all"
		}
		expected = append(expected, Action{
			Code: operation.OperationID,
			Domain: operation.Domain,
			Application: operation.Application,
			UseCase: operation.UseCase,
			TenantRequired: operation.Security.TenantRequired,
			Authentication: append([]string(nil), operation.Security.Authentication...),
			Permissions: permissions,
			PermissionMode: mode,
			RPC: operation.Bindings.RPC,
			HTTP: httpBindings,
		})
	}
	if !reflect.DeepEqual(stripCommercialFacts(Catalog()), expected) {
		t.Fatal("generated action catalog is stale; regenerate from contracts/generated/operation-plans.json")
	}
}

func TestEnterprise174GeneratedCatalogMatchesCommercialCapabilityMapping(t *testing.T) {
	path := filepath.Join("..", "..", "..", "contracts", "commercial", "operation-capabilities.v1.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		MappingVersion string `json:"mapping_version"`
		Operations []struct {
			OperationID string `json:"operation_id"`
			Classification string `json:"classification"`
			ModuleCode string `json:"module_code"`
			CapabilityCodes []string `json:"capability_codes"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(payload, &source); err != nil {
		t.Fatal(err)
	}
	if source.MappingVersion != CommercialCapabilityMappingVersion {
		t.Fatalf("commercial mapping version mismatch: generated=%s source=%s", CommercialCapabilityMappingVersion, source.MappingVersion)
	}
	byCode := map[string]Action{}
	for _, action := range Catalog() {
		byCode[action.Code] = action
	}
	if len(byCode) != len(source.Operations) {
		t.Fatalf("commercial operation count mismatch: catalog=%d source=%d", len(byCode), len(source.Operations))
	}
	for _, operation := range source.Operations {
		action, ok := byCode[operation.OperationID]
		if !ok {
			t.Fatalf("commercial mapping operation missing from action catalog: %s", operation.OperationID)
		}
		if action.Classification != operation.Classification || action.ModuleCode != operation.ModuleCode || !reflect.DeepEqual(action.CapabilityCodes, operation.CapabilityCodes) {
			t.Fatalf("commercial mapping mismatch for %s: action=%+v source=%+v", operation.OperationID, action, operation)
		}
	}
}

func stripCommercialFacts(actions []Action) []Action {
	out := make([]Action, len(actions))
	for index, action := range actions {
		action.Classification = ""
		action.ModuleCode = ""
		action.CapabilityCodes = nil
		out[index] = action
	}
	return out
}

func TestEnterprise174TenantRolePermissionCatalogDerivesFromActions(t *testing.T) {
	definitions := TenantRolePermissions()
	if len(definitions) == 0 {
		t.Fatal("tenant role permission catalog is empty")
	}
	seen := map[authz.PermissionKey]bool{}
	for _, definition := range definitions {
		if definition.Permission == "" || len(definition.Actions) == 0 || len(definition.Groups) == 0 {
			t.Fatalf("invalid derived permission definition: %+v", definition)
		}
		if seen[definition.Permission] {
			t.Fatalf("duplicate permission definition: %s", definition.Permission)
		}
		seen[definition.Permission] = true
	}
}

func TestEnterprise174BrandingMembershipPermissionHasSingleOperation(t *testing.T) {
	var operations []string
	for _, action := range Catalog() {
		for _, permission := range action.Permissions {
			if permission == "tenant.branding.read" {
				operations = append(operations, action.Code)
			}
		}
	}
	if len(operations) != 1 || operations[0] != "tenant.branding.get" {
		t.Fatalf("tenant.branding.read membership grant expanded beyond its frozen operation: %v", operations)
	}
}
