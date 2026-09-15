package architecture_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hvritual/biz/internal/commercial/capabilitymap"
)

func TestCE07PlanOwnerAndNoHardDelete(t *testing.T) {
	root := filepath.Join("..", "..")
	b, err := os.ReadFile(filepath.Join(root, "contracts/proto/commercial/v1/plan.proto"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "rpc Delete") || strings.Contains(string(b), "tenant_required: true") || strings.Contains(string(b), "/v1/tenant/") {
		t.Fatal("plan authoring leaks tenant/delete transport surface")
	}
	err = filepath.WalkDir(filepath.Join(root, "internal/commercial/application/planmanagement"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, forbidden := range []string{"gorm.io", "internal/access/infrastructure", "internal/commercial/modulecatalog", "operation.NewExecutor"} {
			if strings.Contains(string(b), forbidden) {
				t.Errorf("owner boundary %s contains %s", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	doc, err := capabilitymap.BuildRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	platform := map[string]bool{
		"commercial.plan.create": false, "commercial.plan.clone": false, "commercial.plan.update": false, "commercial.plan.publish": false, "commercial.plan.retire": false, "commercial.plan.get": false, "commercial.plan.list": false, "commercial.plan.eligibility": false, "commercial.plan.discover": false,
	}
	private := map[string]bool{
		"commercial.plan.subscription_snapshot": false,
		"commercial.plan.change_target.resolve": false,
		"commercial.plan.change_target.list":    false,
	}
	for _, op := range doc.Operations {
		if !strings.HasPrefix(op.OperationID, "commercial.plan.") {
			continue
		}
		if _, ok := platform[op.OperationID]; ok {
			platform[op.OperationID] = true
			if op.Classification != capabilitymap.PlatformManagement {
				t.Errorf("platform plan operation %s classification=%s", op.OperationID, op.Classification)
			}
			continue
		}
		if _, ok := private[op.OperationID]; ok {
			private[op.OperationID] = true
			if string(op.Classification) != "foundation_exempt" {
				t.Errorf("private plan helper %s classification=%s", op.OperationID, op.Classification)
			}
			continue
		}
		t.Errorf("unexpected plan operation %s", op.OperationID)
	}
	for id, seen := range platform {
		if !seen {
			t.Errorf("missing plan operation %s", id)
		}
	}
	for id, seen := range private {
		if !seen {
			t.Errorf("missing private plan helper %s", id)
		}
	}

	plansRaw, err := os.ReadFile(filepath.Join(root, "contracts", "generated", "operation-plans.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plans struct {
		Operations []struct {
			OperationID string `json:"operationId"`
			Bindings    struct {
				HTTP []struct{ Method, Path string }
			} `json:"bindings"`
			Security struct {
				Authentication []string `json:"authentication"`
			} `json:"security"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(plansRaw, &plans); err != nil {
		t.Fatal(err)
	}
	seenPrivate := map[string]bool{}
	for _, op := range plans.Operations {
		if _, ok := private[op.OperationID]; !ok {
			continue
		}
		seenPrivate[op.OperationID] = true
		if len(op.Bindings.HTTP) != 0 {
			t.Errorf("private plan helper %s gained HTTP transport", op.OperationID)
		}
		if len(op.Security.Authentication) != 1 || op.Security.Authentication[0] != "api-key" {
			t.Errorf("private plan helper %s authentication=%v, want api-key only", op.OperationID, op.Security.Authentication)
		}
	}
	for id := range private {
		if !seenPrivate[id] {
			t.Errorf("private plan helper %s missing from operation plans", id)
		}
	}
}
