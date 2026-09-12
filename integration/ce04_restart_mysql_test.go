//go:build integration

package integration

import (
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"strings"
	"testing"
)

// Qualification executes these in separate processes, with a real MySQL service
// restart between them. A normal all-tests run proves application reconnection,
// not a database restart; only the workflow's restart.log proves that boundary.
func TestCE04PersistenceBeforeRestart(t *testing.T) {
	e := ce04NewEnvironment(t, "ce04-restart")
	e.mustCreate(ce04Request(e.tenantA, "restart-grant", 0, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT))
	add := ce04Request(e.tenantA, "restart-add", 1, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_QUOTA, "tenant.devices", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_QUOTA_ADD)
	add.Limit = &commercialv1.EntitlementLimit{Value: 3}
	e.mustCreate(add)
	t.Log("CE04_RESTART_CHECKPOINT_WRITTEN source_version=2 quota=3")
}
func TestCE04PersistenceAfterRestart(t *testing.T) {
	e := ce04NewEnvironment(t, "ce04-restart")
	r := e.explain(e.tenantA)
	if r.SourceVersion != 2 || !ce04Decision(t, r, "capability", "device.lifecycle", "").Allowed || ce04Decision(t, r, "quota", "tenant.devices", "").Limit.Value != 3 {
		t.Fatal("persistent sources not restored", r)
	}
	sources, err := e.client.ListEntitlementOverrides(ce04Context(e.token, ""), &commercialv1.ListEntitlementOverridesRequest{TenantId: e.tenantA})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources.Sources) != 2 {
		t.Fatal(sources)
	}
	for _, s := range sources.Sources {
		if !strings.HasPrefix(s.ActorId, "ce04-restart") || s.Version != 1 {
			t.Fatal(s)
		}
	}
	t.Log("CE04_RESTART_CHECKPOINT_READ source_version=2 quota=3")
}
