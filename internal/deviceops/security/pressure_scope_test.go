package security

import (
	"context"
	"testing"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

type pressureSites struct{}

func (pressureSites) ResolveMemberSites(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func TestPressureOperationsKeepResourceSpecificScope(t *testing.T) {
	guard, err := NewGuard(pressureSites{})
	if err != nil {
		t.Fatal(err)
	}
	for operation, permission := range map[string]string{"device.transfer.local": "device.update", "device.provision.remote": "device.create"} {
		authorized := authz.AuthorizedOperation{Principal: identity.Principal{Authenticated: true, TenantID: "t", UserID: "u"}, Policy: authz.Policy{Operation: authz.OperationID(operation)}, Decision: authz.Decision{Allowed: true, Grants: []authz.Grant{{Permission: "device.read", Scope: "all"}, {Permission: authz.PermissionKey(permission), Scope: "self"}}}}
		ctx, err := guard.Prepare(context.Background(), authorized, nil)
		if err != nil {
			t.Fatal(err)
		}
		scope, ok := FromContext(ctx)
		if !ok || scope.All || !scope.Self {
			t.Fatalf("unrelated read grant widened %s: %+v", operation, scope)
		}
		authorized.Policy.Operation = "unknown.operation"
		if _, err = guard.Prepare(context.Background(), authorized, nil); err == nil {
			t.Fatal("unknown operation accepted")
		}
	}
}
