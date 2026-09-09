//go:build integration

package integration
import("testing";commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1")
// The qualification script restarts its own MySQL container between these
// separate processes. Running the pair alone does not prove DB restart.
func TestCE06PersistenceBeforeRestart(t *testing.T){
 e:=ce04NewEnvironment(t,"ce06-restart")
 e.mustCreate(ce04Request(e.tenantA,"ce06-persist-grant",0,commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE,"device-operations",commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT))
 view:=e.explain(e.tenantA);if view.EntitlementVersion!=1||view.SourceVersion!=1{t.Fatal(view)}
 t.Logf("CE06_SNAPSHOT_WRITTEN version=%d source=%d catalog=%d",view.EntitlementVersion,view.SourceVersion,view.CatalogRevision)
}
func TestCE06PersistenceAfterRestart(t *testing.T){
 e:=ce04NewEnvironment(t,"ce06-restart");view:=e.explain(e.tenantA)
 if view.EntitlementVersion!=1||view.SourceVersion!=1||!ce04Decision(t,view,"capability","device.lifecycle","").Allowed{t.Fatal("immutable snapshot did not survive",view)}
 t.Logf("CE06_SNAPSHOT_READ version=%d source=%d catalog=%d",view.EntitlementVersion,view.SourceVersion,view.CatalogRevision)
}
