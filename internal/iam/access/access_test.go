package access

import (
	"testing"

	"yunka.io/framework/core/identity"
)

func TestPermissionSpecificDataScope(t *testing.T) {
	plan := Plan{
		Principal: identity.Principal{TenantID: "tenant-a", UserID: "user-a", Authenticated: true},
		Permissions: map[string]PermissionScope{
			PermissionDeviceRead:   {Allowed: true, Self: true},
			PermissionDeviceUpdate: {Allowed: true, Sites: true},
		},
		SiteIDs: []string{"site-a"},
	}
	if plan.CanTargetSite(PermissionDeviceRead, "site-a") {
		t.Fatal("self-only read permission must not inherit update site scope")
	}
	if !plan.CanTargetSite(PermissionDeviceUpdate, "site-a") {
		t.Fatal("update permission must retain its own site scope")
	}
	if plan.CanTargetSite(PermissionDeviceUpdate, "site-b") {
		t.Fatal("site scope must not allow undeclared site")
	}
}

func TestAllScopeTargetsAnySite(t *testing.T) {
	plan := Plan{Permissions: map[string]PermissionScope{PermissionDeviceCreate: {Allowed: true, All: true}}}
	if !plan.CanTargetSite(PermissionDeviceCreate, "any-site") {
		t.Fatal("all scope must allow any site inside trusted tenant")
	}
}
