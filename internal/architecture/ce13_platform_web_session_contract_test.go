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
			Authentication []string `json:"authentication"`
		} `json:"security"`
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
		"commercial.module.create": {},
		"commercial.module.get": {},
		"commercial.module.list": {},
		"commercial.module.update": {},
		"commercial.module.set_sales_status": {},
		"commercial.module.set_technical_status": {},
		"commercial.module.delete": {},
		"commercial.plan.create": {},
		"commercial.plan.clone": {},
		"commercial.plan.update": {},
		"commercial.plan.publish": {},
		"commercial.plan.retire": {},
		"commercial.plan.get": {},
		"commercial.plan.list": {},
		"commercial.plan.eligibility": {},
		"commercial.entitlement.override.create": {},
		"commercial.entitlement.override.revoke": {},
		"commercial.entitlement.override.list": {},
		"commercial.entitlement.explain": {},
		"commercial.subscription.get": {},
		"commercial.subscription.change.preview": {},
		"commercial.subscription.change.confirm": {},
		"commercial.subscription.change.preview.get": {},
		"commercial.subscription.change.get": {},
	}
	allowedCommercialWeb := map[string]struct{}{"commercial.entitlement.get_my": {}}
	for id := range expectedPlatformWeb {
		allowedCommercialWeb[id] = struct{}{}
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
		if hasWeb {
			if _, allowed := allowedCommercialWeb[operation.OperationID]; !allowed {
				t.Fatalf("commercial operation %s unexpectedly admits web-session", operation.OperationID)
			}
		}
		if strings.HasPrefix(operation.OperationID, "commercial.provisioning.") && hasWeb {
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
