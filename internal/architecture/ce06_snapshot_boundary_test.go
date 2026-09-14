package architecture_test

import (
	"encoding/json"
	"github.com/hvritual/biz/internal/commercial/capabilitymap"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCE06AllBusinessWritesJoinInvocationBarrier(t *testing.T) {
	root := filepath.Join("..", "..")
	doc, err := capabilitymap.BuildRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "contracts/generated/operation-plans.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plans struct {
		Operations []struct {
			ID        string `json:"operationId"`
			Execution struct {
				Transaction string `json:"transaction"`
			} `json:"execution"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(b, &plans); err != nil {
		t.Fatal(err)
	}
	writes := map[string]bool{}
	for _, o := range plans.Operations {
		writes[o.ID] = o.Execution.Transaction == "local"
	}

	entitlementFiles, err := filepath.Glob(filepath.Join(root, "internal", "bizruntime", "*_entitlements.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entitlementFiles) == 0 {
		t.Fatal("no runtime entitlement barrier files discovered")
	}
	sort.Strings(entitlementFiles)
	var joined strings.Builder
	for _, path := range entitlementFiles {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		joined.Write(b)
		joined.WriteByte('\n')
	}

	count := 0
	for _, o := range doc.Operations {
		if o.Classification == capabilitymap.TenantBusiness && writes[o.OperationID] {
			count++
			if !strings.Contains(joined.String(), `enforcement.RequireExecuted(ctx, "`+o.OperationID+`")`) {
				t.Fatalf("unfenced business write %s", o.OperationID)
			}
		}
	}
	if count == 0 {
		t.Fatal("no business writes tested")
	}
}
