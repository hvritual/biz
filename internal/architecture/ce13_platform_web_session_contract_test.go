package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type ce13OperationPlans struct {
	Operations []struct {
		OperationID string `json:"operationId"`
		Domain      string `json:"domain"`
		Security    struct {
			Public         bool     `json:"public"`
			TenantRequired bool     `json:"tenantRequired"`
			Authentication []string `json:"authentication"`
			Permissions    []string `json:"permissions"`
			PermissionMode string   `json:"permissionMode"`
		} `json:"security"`
		Execution struct {
			Transaction string `json:"transaction"`
			Idempotency string `json:"idempotency"`
		} `json:"execution"`
	} `json:"operations"`
}

func TestCE13PlatformCommercialWebSessionContract(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve CE-13 contract test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	payload, err := os.ReadFile(filepath.Join(root, "contracts", "generated", "operation-plans.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plans ce13OperationPlans
	if err := json.Unmarshal(payload, &plans); err != nil {
		t.Fatal(err)
	}

	expectedPlatformWeb := map[string]struct{}{
		"commercial.module.create":                   {},
		"commercial.module.get":                      {},
		"commercial.module.list":                     {},
		"commercial.module.update":                   {},
		"commercial.module.set_sales_status":         {},
		"commercial.module.set_technical_status":     {},
		"commercial.module.delete":                   {},
		"commercial.feature.create":                  {},
		"commercial.feature.get":                     {},
		"commercial.feature.list":                    {},
		"commercial.feature.publish":                 {},
		"commercial.feature.stop_selling":            {},
		"commercial.feature.plan_sunset":             {},
		"commercial.feature.complete_migration":      {},
		"commercial.feature.retire":                  {},
		"commercial.plan.create":                     {},
		"commercial.plan.clone":                      {},
		"commercial.plan.update":                     {},
		"commercial.plan.publish":                    {},
		"commercial.plan.retire":                     {},
		"commercial.plan.get":                        {},
		"commercial.plan.discover":                   {},
		"commercial.plan.list":                       {},
		"commercial.plan.eligibility":                {},
		"commercial.entitlement.override.create":     {},
		"commercial.entitlement.override.revoke":     {},
		"commercial.entitlement.override.list":       {},
		"commercial.entitlement.explain":             {},
		"commercial.subscription.get":                {},
		"commercial.subscription.change.preview":     {},
		"commercial.subscription.change.confirm":     {},
		"commercial.subscription.change.preview.get": {},
		"commercial.subscription.change.get":         {},
		"commercial.provisioning.task.get":           {},
		"commercial.provisioning.task.retry":         {},
	}
	allowedCommercialWeb := map[string]struct{}{
		"commercial.entitlement.get_my":                   {},
		"commercial.subscription.get_my":                  {},
		"commercial.subscription.get_my_usage":            {},
		"commercial.subscription.change.targets_my":       {},
		"commercial.subscription.change.preview_my":       {},
		"commercial.subscription.change.preview_my.get":   {},
		"commercial.subscription.change.confirm_my":       {},
		"commercial.subscription.change.get_my":           {},
		"commercial.subscription.change.list_my":          {},
		"commercial.subscription.payment.order.create_my": {},
		"commercial.subscription.payment.order.get_my":    {},
	}
	for id := range expectedPlatformWeb {
		allowedCommercialWeb[id] = struct{}{}
	}

	// CE-340 needs only task readback and retry in the existing platform
	// browser flow. Worker, delivery, list and cancel operations stay API-key-only.
	expectedProvisioningWeb := map[string]struct {
		permission  string
		transaction string
		idempotency string
	}{
		"commercial.provisioning.task.get":   {"platform.provisioning.read", "read_only", "none"},
		"commercial.provisioning.task.retry": {"platform.provisioning.manage", "local", "required"},
	}

	seen := map[string]bool{}
	for _, operation := range plans.Operations {
		if operation.Domain != "commercial" {
			continue
		}
		authentication := append([]string(nil), operation.Security.Authentication...)
		sort.Strings(authentication)
		hasWeb := containsCE13String(authentication, "web-session")
		if _, expected := expectedPlatformWeb[operation.OperationID]; expected {
			seen[operation.OperationID] = true
			want := []string{"api-key", "web-session"}
			if strings.Join(authentication, ",") != strings.Join(want, ",") {
				t.Fatalf("%s authentication = %v, want exactly %v", operation.OperationID, authentication, want)
			}
		}
		if strings.HasPrefix(operation.OperationID, "commercial.plan.change_target.") || operation.OperationID == "commercial.plan.subscription_snapshot" {
			if hasWeb || strings.Join(authentication, ",") != "api-key" {
				t.Fatalf("transport-private plan helper %s authentication = %v, want API-key-only", operation.OperationID, authentication)
			}
		}
		if hasWeb {
			if _, allowed := allowedCommercialWeb[operation.OperationID]; !allowed {
				t.Fatalf("commercial operation %s unexpectedly admits web-session", operation.OperationID)
			}
		}
		if expected, ok := expectedProvisioningWeb[operation.OperationID]; ok {
			permissions := append([]string(nil), operation.Security.Permissions...)
			sort.Strings(permissions)
			wantPermissions := []string{expected.permission, "platform.tenant.read"}
			if operation.Security.Public || operation.Security.TenantRequired || operation.Security.PermissionMode != "all" || strings.Join(permissions, ",") != strings.Join(wantPermissions, ",") {
				t.Fatalf("%s must retain its protected platform ALL permission boundary: %+v", operation.OperationID, operation.Security)
			}
			if operation.Execution.Transaction != expected.transaction || operation.Execution.Idempotency != expected.idempotency {
				t.Fatalf("%s execution = %+v, want transaction %s and idempotency %s", operation.OperationID, operation.Execution, expected.transaction, expected.idempotency)
			}
		} else if strings.HasPrefix(operation.OperationID, "commercial.provisioning.") && strings.Join(authentication, ",") != "api-key" {
			t.Fatalf("worker/operations provisioning operation %s must remain API-key-only", operation.OperationID)
		}
	}
	for operationID := range expectedPlatformWeb {
		if !seen[operationID] {
			t.Fatalf("expected CE-13 platform web operation %s missing from generated operation plans", operationID)
		}
	}
}

func containsCE13String(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
