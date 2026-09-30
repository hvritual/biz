//go:build integration

package integration

import (
	"fmt"
	"testing"

	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
)

// Branding is the approved Marketing Reference Slice simulation. It exercises
// a real tenant write operation, never a Preview lifecycle page or fake
// marketing data. The test proves the future marketing module admission path
// with existing Brand & Theme authority.
func TestCE296MySQLBrandingBasedMarketingReferenceIAMEntitlementMatrix(t *testing.T) {
	e := ce05New(t)
	endpoint := "http://" + e.runtime.HTTPAddress() + "/v1/tenant/branding"
	stamp := ce04Random(t)

	grantBrandingIAM := func(tenant string) {
		t.Helper()
		if err := e.db.Exec("INSERT IGNORE INTO biz_permission_grants(tenant_id,role_id,permission,scope) SELECT tenant_id,role_id,?,'all' FROM biz_member_roles WHERE tenant_id=? AND user_id=?", "tenant.organization.manage", tenant, tenant+"-user").Error; err != nil {
			t.Fatal(err)
		}
	}
	grantLifecycle := func(tenant, requestID string) {
		t.Helper()
		var version uint64
		if err := e.db.Table("biz_commercial_entitlement_state").Select("version").Where("tenant_id = ?", tenant).Scan(&version).Error; err != nil {
			t.Fatalf("tenant %s entitlement state=%d err=%v", tenant, version, err)
		}
		_, err := e.create(&commercialv1.CreateEntitlementOverrideRequest{RequestId: requestID, TenantId: tenant, ExpectedVersion: version, ModuleCode: "access-management", Target: commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, Key: "tenant.lifecycle", Effect: commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT, Reason: "CE296 branding marketing simulation entitlement"})
		if err != nil {
			t.Fatal(err)
		}
	}
	version := func(tenant string) uint64 {
		t.Helper()
		var value uint64
		if err := e.db.Table("biz_tenants").Select("version").Where("id = ?", tenant).Scan(&value).Error; err != nil {
			t.Fatal(err)
		}
		return value
	}
	update := func(token string, current uint64) int {
		t.Helper()
		_, status, _ := brandingHTTP(t, "PATCH", endpoint, token, "ce296-brand-"+stamp+fmt.Sprint(current), map[string]any{"preset": "violet", "version": fmt.Sprint(current)})
		return status
	}

	// Neither IAM nor entitlement: IAM rejects before commercial facts are disclosed.
	beforeB := version(e.tenantB)
	if status := update(e.tokenB, beforeB); status != 403 {
		t.Fatalf("neither status=%d", status)
	}
	if version(e.tenantB) != beforeB {
		t.Fatal("neither denial wrote branding")
	}

	// IAM without entitlement: Commercial Guard rejects and writes nothing.
	grantBrandingIAM(e.tenantA)
	beforeA := version(e.tenantA)
	if status := update(e.tokenA, beforeA); status != 403 {
		t.Fatalf("missing entitlement status=%d", status)
	}
	if version(e.tenantA) != beforeA {
		t.Fatal("entitlement denial wrote branding")
	}

	// Entitlement without IAM still cannot write.
	grantLifecycle(e.tenantB, "ce296-brand-b-lifecycle")
	if status := update(e.tokenB, beforeB); status != 403 {
		t.Fatalf("missing IAM status=%d", status)
	}
	if version(e.tenantB) != beforeB {
		t.Fatal("IAM denial wrote branding")
	}

	// Both facts present: the existing authoritative branding write succeeds.
	grantLifecycle(e.tenantA, "ce296-brand-a-lifecycle")
	if status := update(e.tokenA, beforeA); status != 200 {
		t.Fatalf("allow status=%d", status)
	}
	if version(e.tenantA) != beforeA+1 {
		t.Fatal("authorized branding update did not persist")
	}
}
