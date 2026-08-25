package deviceops

import (
	"testing"

	"yunka.io/framework/core/identity"
)

func TestPermissionSpecificDataScope(t *testing.T) {
	plan := AccessPlan{
		Principal: identity.Principal{TenantID: "tenant-a", UserID: "user-a", Authenticated: true},
		Permissions: map[string]permissionScope{
			PermissionDeviceRead:   {Allowed: true, Self: true},
			PermissionDeviceUpdate: {Allowed: true, Sites: true},
		},
		SiteIDs: []string{"site-a"},
	}
	if plan.canTargetSite(PermissionDeviceRead, "site-a") {
		t.Fatal("self-only read permission must not inherit the update permission site scope")
	}
	if !plan.canTargetSite(PermissionDeviceUpdate, "site-a") {
		t.Fatal("update permission must retain its own site scope")
	}
	if plan.canTargetSite(PermissionDeviceUpdate, "site-b") {
		t.Fatal("site scope must not allow an undeclared site")
	}
}

func TestAllScopeTargetsAnySite(t *testing.T) {
	plan := AccessPlan{
		Principal: identity.Principal{TenantID: "tenant-a", UserID: "user-a", Authenticated: true},
		Permissions: map[string]permissionScope{
			PermissionDeviceCreate: {Allowed: true, All: true},
		},
	}
	if !plan.canTargetSite(PermissionDeviceCreate, "any-site") {
		t.Fatal("all scope must allow any site inside the trusted tenant")
	}
}
