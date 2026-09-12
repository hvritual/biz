package architecture

import (
	"encoding/json"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCE09DeclaredChangeSecurityAndTransportAreExact(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "contracts", "generated", "operation-plans.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Operations []struct {
			ID       string `json:"operationId"`
			Security struct {
				Permissions []string `json:"permissions"`
				Mode        string   `json:"permissionMode"`
			} `json:"security"`
			Execution struct {
				Transaction string `json:"transaction"`
				Idempotency string `json:"idempotency"`
			} `json:"execution"`
			Bindings struct {
				HTTP []struct{ Method, Path string }
			} `json:"bindings"`
			Composition struct {
				Boundary string   `json:"boundary"`
				Requires []string `json:"requiresOperations"`
			} `json:"composition"`
		}
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	internal := map[string]bool{}
	for _, op := range manifest.Operations {
		if !strings.HasPrefix(op.ID, "commercial.subscription.change.") {
			continue
		}
		if op.ID == "commercial.subscription.change.prepared" || op.ID == "commercial.subscription.change.preparation.cancel" {
			internal[op.ID] = true
			permission := "platform.provisioning.execute"
			if op.ID == "commercial.subscription.change.preparation.cancel" {
				permission = "platform.provisioning.cancel"
			}
			if len(op.Bindings.HTTP) != 0 || op.Execution.Transaction != "local" || op.Execution.Idempotency != "none" || op.Security.Mode != "all" || !reflect.DeepEqual(op.Security.Permissions, []string{"commercial.catalog.read", "platform.plan.read", permission}) {
				t.Fatalf("CE10 private boundary changed: %+v", op)
			}
			if op.ID == "commercial.subscription.change.prepared" && (op.Composition.Boundary != "local" || !reflect.DeepEqual(op.Composition.Requires, []string{"commercial.module.plan_catalog", "commercial.plan.eligibility", "commercial.plan.get"})) {
				t.Fatalf("CE10 activation child closure=%+v", op)
			}
			if op.ID == "commercial.subscription.change.preparation.cancel" && len(op.Composition.Requires) != 0 {
				t.Fatal("unexpected cancellation child")
			}
			continue
		}
		found[op.ID] = true
		if len(op.Bindings.HTTP) != 1 || !strings.HasPrefix(op.Bindings.HTTP[0].Path, "/v1/platform/tenants/") {
			t.Fatalf("unscoped change route %v", op)
		}
		if op.Security.Mode != "all" {
			t.Fatal("change permission closure relaxed")
		}
		write := op.ID == "commercial.subscription.change.preview" || op.ID == "commercial.subscription.change.confirm"
		if write {
			permission := "platform.subscription.manage"
			if strings.HasSuffix(op.ID, "confirm") {
				permission = "platform.subscription.confirm"
			}
			want := []string{"commercial.catalog.read", "platform.plan.read", permission, "platform.tenant.read"}
			if !reflect.DeepEqual(op.Security.Permissions, want) || op.Execution.Transaction != "local" || op.Execution.Idempotency != "required" || op.Composition.Boundary != "local" || len(op.Composition.Requires) != 3 {
				t.Fatalf("wrong mutation boundary %v", op)
			}
		} else if op.Execution.Transaction != "read_only" || op.Execution.Idempotency != "none" {
			t.Fatalf("readback is not read-only %v", op)
		}
	}
	if len(internal) != 2 {
		t.Fatalf("CE10 internal operation count=%d", len(internal))
	}
	descriptor := v1.File_commercial_v1_subscription_change_proto.Services().ByName("SubscriptionChangesApplication")
	if descriptor == nil || descriptor.Methods().Len() != 4 {
		t.Fatal("private child was exposed as RPC")
	}
	for _, name := range []protoreflect.Name{"CompletePreparedSubscriptionChange", "CancelPreparedSubscriptionChange"} {
		if descriptor.Methods().ByName(name) != nil {
			t.Fatal("private operation gained transport")
		}
	}
	if len(found) != 4 {
		t.Fatalf("change operations=%d", len(found))
	}
	for _, message := range []protoreflect.MessageDescriptor{(&v1.ConfirmSubscriptionChangeRequest{}).ProtoReflect().Descriptor(), (&v1.PreviewSubscriptionChangeRequest{}).ProtoReflect().Descriptor()} {
		for _, name := range []protoreflect.Name{"paid", "payment_status", "used", "entitlement_version", "source_version", "effective_entitlements"} {
			if message.Fields().ByName(name) != nil {
				t.Fatalf("client authority field %s", name)
			}
		}
	}
}
