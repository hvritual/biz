package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ce09Operation struct {
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

func TestCE09DeclaredChangeSecurityAndTransportAreExact(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "contracts", "generated", "operation-plans.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Operations []ce09Operation `json:"operations"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}

	platformSeen := map[string]bool{}
	tenantSeen := map[string]bool{}
	internalSeen := map[string]bool{}
	for _, op := range manifest.Operations {
		if !strings.HasPrefix(op.ID, "commercial.subscription.change.") {
			continue
		}
		if op.ID == "commercial.subscription.change.prepared" || op.ID == "commercial.subscription.change.preparation.cancel" {
			internalSeen[op.ID] = true
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

		switch op.ID {
		case "commercial.subscription.change.preview", "commercial.subscription.change.confirm", "commercial.subscription.change.preview.get", "commercial.subscription.change.get":
			platformSeen[op.ID] = true
			assertCE09PlatformOperation(t, op)
		case "commercial.subscription.change.targets_my", "commercial.subscription.change.preview_my", "commercial.subscription.change.preview_my.get", "commercial.subscription.change.confirm_my", "commercial.subscription.change.get_my":
			tenantSeen[op.ID] = true
			assertCE09TenantOperation(t, op)
		default:
			t.Fatalf("unexpected subscription-change operation %s", op.ID)
		}
	}
	if len(internalSeen) != 2 {
		t.Fatalf("CE10 internal operation count=%d", len(internalSeen))
	}
	if len(platformSeen) != 4 {
		t.Fatalf("platform change operations=%d", len(platformSeen))
	}
	if len(tenantSeen) != 5 {
		t.Fatalf("tenant change operations=%d", len(tenantSeen))
	}

	descriptor := v1.File_commercial_v1_subscription_change_proto.Services().ByName("SubscriptionChangesApplication")
	if descriptor == nil || descriptor.Methods().Len() != 9 {
		t.Fatalf("subscription change RPC count=%v, want 9", func() int { if descriptor == nil { return -1 }; return descriptor.Methods().Len() }())
	}
	for _, name := range []protoreflect.Name{"CompletePreparedSubscriptionChange", "CancelPreparedSubscriptionChange"} {
		if descriptor.Methods().ByName(name) != nil {
			t.Fatal("private operation gained transport")
		}
	}

	platformMessages := []protoreflect.MessageDescriptor{
		(&v1.ConfirmSubscriptionChangeRequest{}).ProtoReflect().Descriptor(),
		(&v1.PreviewSubscriptionChangeRequest{}).ProtoReflect().Descriptor(),
	}
	for _, message := range platformMessages {
		assertNoCE09AuthorityFields(t, message, false)
	}
	tenantMessages := []protoreflect.MessageDescriptor{
		(&v1.ListMySubscriptionChangeTargetsRequest{}).ProtoReflect().Descriptor(),
		(&v1.PreviewMySubscriptionChangeRequest{}).ProtoReflect().Descriptor(),
		(&v1.ReadMySubscriptionChangePreviewRequest{}).ProtoReflect().Descriptor(),
		(&v1.ConfirmMySubscriptionChangeRequest{}).ProtoReflect().Descriptor(),
		(&v1.ReadMySubscriptionChangeReceiptRequest{}).ProtoReflect().Descriptor(),
	}
	for _, message := range tenantMessages {
		assertNoCE09AuthorityFields(t, message, true)
	}
}

func assertCE09PlatformOperation(t *testing.T, op ce09Operation) {
	t.Helper()
	if len(op.Bindings.HTTP) != 1 || !strings.HasPrefix(op.Bindings.HTTP[0].Path, "/v1/platform/tenants/") {
		t.Fatalf("platform change route changed: %+v", op)
	}
	if op.Security.Mode != "all" {
		t.Fatal("platform change permission closure relaxed")
	}
	write := op.ID == "commercial.subscription.change.preview" || op.ID == "commercial.subscription.change.confirm"
	if write {
		permission := "platform.subscription.manage"
		if strings.HasSuffix(op.ID, "confirm") {
			permission = "platform.subscription.confirm"
		}
		want := []string{"commercial.catalog.read", "platform.plan.read", permission, "platform.tenant.read"}
		if !reflect.DeepEqual(op.Security.Permissions, want) || op.Execution.Transaction != "local" || op.Execution.Idempotency != "required" || op.Composition.Boundary != "local" || len(op.Composition.Requires) != 3 {
			t.Fatalf("wrong platform mutation boundary %+v", op)
		}
		return
	}
	if op.Execution.Transaction != "read_only" || op.Execution.Idempotency != "none" {
		t.Fatalf("platform readback is not read-only %+v", op)
	}
}

func assertCE09TenantOperation(t *testing.T, op ce09Operation) {
	t.Helper()
	if len(op.Bindings.HTTP) != 1 || !strings.HasPrefix(op.Bindings.HTTP[0].Path, "/v1/tenant/subscription/") || strings.Contains(op.Bindings.HTTP[0].Path, "{tenant_id}") {
		t.Fatalf("tenant change route is not trusted-tenant scoped: %+v", op)
	}
	if op.Security.Mode != "all" || !containsCE09(op.Security.Permissions, "tenant.subscription.manage") {
		t.Fatalf("tenant change permission closure relaxed: %+v", op)
	}
	switch op.ID {
	case "commercial.subscription.change.targets_my":
		if op.Bindings.HTTP[0].Method != "GET" || op.Execution.Transaction != "local" || op.Execution.Idempotency != "none" || !reflect.DeepEqual(op.Composition.Requires, []string{"commercial.plan.change_target.list"}) {
			t.Fatalf("wrong tenant target discovery boundary %+v", op)
		}
	case "commercial.subscription.change.preview_my", "commercial.subscription.change.confirm_my":
		if op.Bindings.HTTP[0].Method != "POST" || op.Execution.Transaction != "local" || op.Execution.Idempotency != "required" || op.Composition.Boundary != "local" || !reflect.DeepEqual(op.Composition.Requires, []string{"commercial.module.plan_catalog", "commercial.plan.change_target.resolve", "commercial.plan.subscription_snapshot"}) {
			t.Fatalf("wrong tenant mutation boundary %+v", op)
		}
	case "commercial.subscription.change.preview_my.get", "commercial.subscription.change.get_my":
		if op.Bindings.HTTP[0].Method != "GET" || op.Execution.Transaction != "read_only" || op.Execution.Idempotency != "none" || len(op.Composition.Requires) != 0 {
			t.Fatalf("wrong tenant readback boundary %+v", op)
		}
	}
}

func assertNoCE09AuthorityFields(t *testing.T, message protoreflect.MessageDescriptor, tenantScoped bool) {
	t.Helper()
	for _, name := range []protoreflect.Name{"paid", "payment_status", "used", "entitlement_version", "source_version", "effective_entitlements"} {
		if message.Fields().ByName(name) != nil {
			t.Fatalf("client authority field %s", name)
		}
	}
	if tenantScoped && message.Fields().ByName("tenant_id") != nil {
		t.Fatalf("tenant self-service request %s must not accept tenant_id", message.FullName())
	}
}

func containsCE09(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
