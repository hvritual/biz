package architecture_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const qualifiedYunkaRevision = "33b98ceba57494abda2299e4f0290a5651dab4bc"

type ce01OperationPlans struct {
	SchemaVersion int `json:"schemaVersion"`
	Operations    []struct {
		OperationID string `json:"operationId"`
		Domain      string `json:"domain"`
		Application string `json:"application"`
		Security    struct {
			TenantRequired bool `json:"tenantRequired"`
		} `json:"security"`
		Composition struct {
			RequiresOperations []string `json:"requiresOperations"`
		} `json:"composition"`
		Bindings struct {
			RPC  string `json:"rpc"`
			HTTP []struct {
				Method string `json:"method"`
				Path   string `json:"path"`
			} `json:"http"`
		} `json:"bindings"`
	} `json:"operations"`
}

func TestCE01(t *testing.T) {
	t.Run("qualified framework revision remains pinned", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if !strings.Contains(text, "yunka.io/framework v0.0.0-20260910080749-33b98ceba574") ||
			!strings.Contains(text, "yunka.io/gateway v0.0.0-20260910080749-33b98ceba574") ||
			!strings.Contains(text, "yunka.io/pkg v0.0.0-20260910080749-33b98ceba574") {
			t.Fatalf("CE01-BASELINE: Yunka dependencies no longer resolve to %s", qualifiedYunkaRevision)
		}
	})

	t.Run("generated operation plan is the traceable operation inventory", func(t *testing.T) {
		plan := loadCE01OperationPlans(t)
		if plan.SchemaVersion != 2 {
			t.Fatalf("CE01-OPERATIONS: schemaVersion=%d, want 2", plan.SchemaVersion)
		}
		if len(plan.Operations) == 0 {
			t.Fatal("CE01-OPERATIONS: generated operation inventory is empty")
		}
		seen := map[string]bool{}
		for _, operation := range plan.Operations {
			id := strings.TrimSpace(operation.OperationID)
			if id == "" {
				t.Fatal("CE01-OPERATIONS: blank operationId")
			}
			if seen[id] {
				t.Fatalf("CE01-OPERATIONS: duplicate operationId %q", id)
			}
			seen[id] = true
		}
		for _, required := range []string{"device.create", "device.transfer", "site.validate_transfer_target", "tenant.create", "tenant.member.bootstrap_owner", "tenant.role.bootstrap_owner"} {
			if !seen[required] {
				t.Fatalf("CE01-OPERATIONS: required baseline operation %q disappeared", required)
			}
		}
	})

	t.Run("device transfer composition remains explicit", func(t *testing.T) {
		plan := loadCE01OperationPlans(t)
		var requires []string
		for _, operation := range plan.Operations {
			if operation.OperationID == "device.transfer" {
				if !operation.Security.TenantRequired {
					t.Fatal("CE01-CHILD: device.transfer must remain tenant-required")
				}
				requires = append(requires, operation.Composition.RequiresOperations...)
				break
			}
		}
		sort.Strings(requires)
		want := []string{"device.update", "site.validate_transfer_target"}
		if strings.Join(requires, ",") != strings.Join(want, ",") {
			t.Fatalf("CE01-CHILD: device.transfer requires=%v, want %v", requires, want)
		}
	})

	t.Run("internal child operation stays transport private", func(t *testing.T) {
		plan := loadCE01OperationPlans(t)
		for _, operation := range plan.Operations {
			if operation.OperationID != "site.validate_transfer_target" {
				continue
			}
			if operation.Bindings.RPC != "" || len(operation.Bindings.HTTP) != 0 {
				t.Fatalf("CE01-CHILD: internal site validation unexpectedly has external bindings: %+v", operation.Bindings)
			}
			return
		}
		t.Fatal("CE01-CHILD: site.validate_transfer_target missing")
	})
}

func loadCE01OperationPlans(t *testing.T) ce01OperationPlans {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "contracts", "generated", "operation-plans.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result ce01OperationPlans
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
