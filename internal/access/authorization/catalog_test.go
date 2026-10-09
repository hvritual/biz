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
			Domain      string `json:"domain"`
			Application string `json:"application"`
			UseCase     string `json:"useCase"`
			Security    struct {
				TenantRequired bool     `json:"tenantRequired"`
				Authentication []string `json:"authentication"`
				Permissions    []string `json:"permissions"`
				PermissionMode string   `json:"permissionMode"`
			} `json:"security"`
			Bindings struct {
				RPC  string `json:"rpc"`
				HTTP []struct {
					Method string `json:"method"`
					Path   string `json:"path"`
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
			Code:           operation.OperationID,
			Domain:         operation.Domain,
			Application:    operation.Application,
			UseCase:        operation.UseCase,
			TenantRequired: operation.Security.TenantRequired,
			Authentication: append([]string(nil), operation.Security.Authentication...),
			Permissions:    permissions,
			PermissionMode: mode,
			RPC:            operation.Bindings.RPC,
			HTTP:           httpBindings,
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
		Operations     []struct {
			OperationID     string   `json:"operation_id"`
			Classification  string   `json:"classification"`
			ModuleCode      string   `json:"module_code"`
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

func TestEnterprise179RoleAssignableCatalogExcludesInternalChildOperations(t *testing.T) {
	assignable := RoleAssignableActions(Catalog())
	if len(assignable) == 0 {
		t.Fatal("role assignable action catalog is empty")
	}
	seen := map[string]bool{}
	for _, action := range assignable {
		seen[action.Code] = true
		if !action.TenantRequired || !containsString(action.Authentication, "web-session") {
			t.Fatalf("non-tenant/web action leaked into role catalog: %+v", action)
		}
		if action.RPC == "" && len(action.HTTP) == 0 {
			t.Fatalf("internal unbound action leaked into role catalog: %+v", action)
		}
	}
	if !seen["tenant.role.set_permissions"] || !seen["tenant.member.list"] {
		t.Fatalf("expected public role/member actions missing: %v", seen)
	}
	if seen["tenant.department.assert_member_assignment_allowed"] {
		t.Fatal("internal child operation became role-assignable")
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

func TestPlatformModuleWebActionsDeriveReadManageAndTechnicalAuthority(t *testing.T) {
	actions := PlatformWebActions()
	byCode := map[string]Action{}
	for _, action := range actions {
		byCode[action.Code] = action
		if action.TenantRequired || !containsString(action.Authentication, "web-session") {
			t.Fatalf("non-platform-browser action leaked into projection: %+v", action)
		}
	}
	for code, permission := range map[string]authz.PermissionKey{
		"commercial.module.list":                 "platform.module.read",
		"commercial.module.create":               "platform.module.manage",
		"commercial.module.set_sales_status":     "platform.module.manage",
		"commercial.module.set_technical_status": "platform.module.technical.manage",
	} {
		action, ok := byCode[code]
		if !ok {
			t.Fatalf("platform module action missing: %s", code)
		}
		if len(action.Permissions) != 1 || action.Permissions[0] != permission {
			t.Fatalf("platform module permission mismatch for %s: %v", code, action.Permissions)
		}
	}
	if _, ok := byCode["commercial.module.runtime.verify"]; ok {
		t.Fatal("API-key-only runtime verification leaked into platform web projection")
	}
}

func TestCE340ProvisioningWebActionsRequireCompletePlatformGrants(t *testing.T) {
	const read = "commercial.provisioning.task.get"
	const retry = "commercial.provisioning.task.retry"
	for _, action := range RoleAssignableActions(Catalog()) {
		if action.Code == read || action.Code == retry {
			t.Fatalf("platform provisioning action became tenant-role-assignable: %+v", action)
		}
	}
	for _, test := range []struct {
		name        string
		permissions []authz.PermissionKey
		want        []string
	}{
		{"no grants", nil, nil},
		{"tenant read only", []authz.PermissionKey{"platform.tenant.read"}, nil},
		{"provisioning read only", []authz.PermissionKey{"platform.provisioning.read"}, nil},
		{"provisioning manage only", []authz.PermissionKey{"platform.provisioning.manage"}, nil},
		{"both without tenant read", []authz.PermissionKey{"platform.provisioning.read", "platform.provisioning.manage"}, nil},
		{"task reader", []authz.PermissionKey{"platform.provisioning.read", "platform.tenant.read"}, []string{read}},
		{"task retry operator", []authz.PermissionKey{"platform.provisioning.manage", "platform.tenant.read"}, []string{retry}},
		{"task reader and retry operator", []authz.PermissionKey{"platform.provisioning.read", "platform.provisioning.manage", "platform.tenant.read"}, []string{read, retry}},
	} {
		t.Run(test.name, func(t *testing.T) {
			grants := make([]authz.Grant, 0, len(test.permissions))
			for _, permission := range test.permissions {
				grants = append(grants, authz.Grant{Permission: permission})
			}
			var got []string
			for _, action := range AuthorizedPlatformActions(grants) {
				if action.Code == read || action.Code == retry {
					got = append(got, action.Code)
				}
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("provisioning browser actions = %v, want %v for current grants %v", got, test.want, test.permissions)
			}
		})
	}
}

func TestAuthorizedPlatformActionsShrinkWithCurrentGrantSet(t *testing.T) {
	readOnly := AuthorizedPlatformActions([]authz.Grant{{Permission: "platform.module.read"}})
	readCodes := map[string]bool{}
	for _, action := range readOnly {
		readCodes[action.Code] = true
	}
	if !readCodes["commercial.module.list"] || !readCodes["commercial.module.get"] {
		t.Fatalf("read-only platform grants lost module reads: %v", readCodes)
	}
	if readCodes["commercial.module.create"] || readCodes["commercial.module.set_sales_status"] || readCodes["commercial.module.set_technical_status"] {
		t.Fatalf("read-only platform grant leaked write action: %v", readCodes)
	}

	manageOnly := AuthorizedPlatformActions([]authz.Grant{{Permission: "platform.module.manage"}})
	manageCodes := map[string]bool{}
	for _, action := range manageOnly {
		manageCodes[action.Code] = true
	}
	if !manageCodes["commercial.module.create"] || !manageCodes["commercial.module.set_sales_status"] {
		t.Fatalf("manage grant lost module management actions: %v", manageCodes)
	}
	if manageCodes["commercial.module.set_technical_status"] {
		t.Fatalf("manage grant incorrectly implied technical authority: %v", manageCodes)
	}

	technicalOnly := AuthorizedPlatformActions([]authz.Grant{{Permission: "platform.module.technical.manage"}})
	technicalCodes := map[string]bool{}
	for _, action := range technicalOnly {
		technicalCodes[action.Code] = true
	}
	if !technicalCodes["commercial.module.set_technical_status"] {
		t.Fatalf("technical grant lost technical action: %v", technicalCodes)
	}
	if technicalCodes["commercial.module.create"] || technicalCodes["commercial.module.set_sales_status"] {
		t.Fatalf("technical grant incorrectly implied general management: %v", technicalCodes)
	}
}
