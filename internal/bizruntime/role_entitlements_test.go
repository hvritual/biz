package bizruntime

import (
	"context"
	"errors"
	"testing"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"yunka.io/framework/core/identity"
)

type roleEntitlementStub struct {
	result entitlement.Result
	err    error
	seen   []string
}

func (stub *roleEntitlementStub) ReadSnapshot(_ context.Context, tenantID string, _ []string) (entitlement.Result, error) {
	stub.seen = append(stub.seen, tenantID)
	return stub.result, stub.err
}

func tenantRoleContext(tenantID string) context.Context {
	return identity.WithPrincipal(context.Background(), identity.Principal{
		Subject:       "user:test",
		TenantID:      tenantID,
		UserID:        "user-test",
		Authenticated: true,
	})
}

func rolePermissionRequest(permission string) *accessv1.SetTenantRolePermissionsRequest {
	return &accessv1.SetTenantRolePermissionsRequest{
		RoleId:  "role-test",
		Version: 1,
		Permissions: []*accessv1.PermissionGrantInput{{
			Permission: permission,
			Scope:      accessv1.DataScope_DATA_SCOPE_ALL,
		}},
	}
}

func TestEnterprise179RoleGrantValidationUsesCurrentTenantEntitlement(t *testing.T) {
	reader := &roleEntitlementStub{result: entitlement.Result{
		TenantID: "tenant-a",
		Decisions: []entitlement.Decision{{
			Kind:       entitlement.Capability,
			ModuleCode: "access-management",
			Key:        "tenant.role.permission",
			Allowed:    true,
			Reason:     "plan grant",
		}},
	}}
	roles := checkedRoles{entitlements: reader}
	if err := roles.validatePermissionSelection(tenantRoleContext("tenant-a"), rolePermissionRequest("tenant.role.manage")); err != nil {
		t.Fatalf("current entitled role permission rejected: %v", err)
	}
	if len(reader.seen) != 1 || reader.seen[0] != "tenant-a" {
		t.Fatalf("entitlement lookup tenants=%v want tenant-a", reader.seen)
	}
}

func TestEnterprise179RoleGrantValidationRejectsUnknownOrUnentitledPermission(t *testing.T) {
	reader := &roleEntitlementStub{result: entitlement.Result{TenantID: "tenant-a"}}
	roles := checkedRoles{entitlements: reader}
	for _, permission := range []string{"tenant.role.manage", "tenant.unknown.injected"} {
		err := roles.validatePermissionSelection(tenantRoleContext("tenant-a"), rolePermissionRequest(permission))
		if !errors.Is(err, accessports.ErrTenantRoleGrantUnavailable) {
			t.Fatalf("permission %q error=%v want unavailable", permission, err)
		}
	}
}

func TestEnterprise179RoleGrantValidationFailsClosedOnEntitlementReadFailure(t *testing.T) {
	reader := &roleEntitlementStub{err: errors.New("entitlement unavailable")}
	roles := checkedRoles{entitlements: reader}
	if err := roles.validatePermissionSelection(tenantRoleContext("tenant-a"), rolePermissionRequest("tenant.role.manage")); err == nil {
		t.Fatal("entitlement failure allowed role permission write")
	}
}

func TestEnterprise179AvailableActionsExcludeRetiredCommercialCapability(t *testing.T) {
	denied := &roleEntitlementStub{result: entitlement.Result{TenantID: "tenant-a"}}
	actions, _, err := availableTenantRoleActions(context.Background(), denied, "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range actions {
		if action.Code == "tenant.role.set_permissions" {
			t.Fatal("unentitled tenant.role.set_permissions remained in role catalog")
		}
	}

	allowed := &roleEntitlementStub{result: entitlement.Result{
		TenantID: "tenant-a",
		Decisions: []entitlement.Decision{{
			Kind:       entitlement.Capability,
			ModuleCode: "access-management",
			Key:        "tenant.role.permission",
			Allowed:    true,
		}},
	}}
	actions, _, err = availableTenantRoleActions(context.Background(), allowed, "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, action := range actions {
		if action.Code == "tenant.role.set_permissions" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("entitled tenant.role.set_permissions missing from role catalog")
	}
}
