//go:build integration

package integration

import (
	"testing"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
)

func TestCE295MySQLPublishedVersionDoesNotAutoMigrateExistingTenant(t *testing.T) {
	e := ce09New(t)
	secondID := ce04Random(t)
	second := ce08Tenant(t, e.createTenant(secondID, secondID, "owner-"+secondID, "ce09"))
	quota := func(tenant string) uint64 {
		t.Helper()
		view, err := e.entitlements.ExplainEntitlements(e.ctx(), &v1.ExplainEntitlementsRequest{TenantId: tenant})
		if err != nil {
			t.Fatal(err)
		}
		return ce09Quota(t, view)
	}
	before := e.getSubscription(e.tenant)
	if before.PlanCode != e.old.PlanCode || before.PlanVersion != e.old.Version || quota(e.tenant) != 10 || quota(second.Id) != 10 {
		t.Fatalf("unexpected v1 baseline: %+v", before)
	}

	v2, err := e.plans.CreatePlanVersion(e.ctx(), &v1.CreatePlanVersionRequest{
		RequestId: "ce295-clone-" + ce04Random(t), PlanCode: e.old.PlanCode,
		FromVersion: e.old.Version, ExpectedPlanRevision: e.old.PlanRevision, Reason: "create V2 without moving tenants",
	})
	if err != nil {
		t.Fatal(err)
	}
	v2, err = e.plans.UpdatePlanDraft(e.ctx(), &v1.UpdatePlanDraftRequest{
		RequestId: "ce295-update-" + ce04Random(t), PlanCode: v2.PlanCode, Version: v2.Version,
		ExpectedRevision: v2.Revision, Name: "CE295 V2", Terms: ce09Terms(25, 30), Reason: "add V2 capacity",
	})
	if err != nil {
		t.Fatal(err)
	}
	v2, err = e.plans.PublishPlanVersion(e.ctx(), ce07State(v2, ce04Random(t)))
	if err != nil {
		t.Fatal(err)
	}
	v3, err := e.plans.CreatePlanVersion(e.ctx(), &v1.CreatePlanVersionRequest{RequestId: "ce295-clone-v3-" + ce04Random(t), PlanCode: v2.PlanCode, FromVersion: v2.Version, ExpectedPlanRevision: v2.PlanRevision, Reason: "create V3 without moving tenants"})
	if err != nil {
		t.Fatal(err)
	}
	v3, err = e.plans.UpdatePlanDraft(e.ctx(), &v1.UpdatePlanDraftRequest{RequestId: "ce295-update-v3-" + ce04Random(t), PlanCode: v3.PlanCode, Version: v3.Version, ExpectedRevision: v3.Revision, Name: "CE295 V3", Terms: ce09Terms(30, 30), Reason: "add V3 capacity"})
	if err != nil {
		t.Fatal(err)
	}
	v3, err = e.plans.PublishPlanVersion(e.ctx(), ce07State(v3, ce04Random(t)))
	if err != nil {
		t.Fatal(err)
	}
	v4, err := e.plans.CreatePlanVersion(e.ctx(), &v1.CreatePlanVersionRequest{RequestId: "ce295-clone-v4-" + ce04Random(t), PlanCode: v3.PlanCode, FromVersion: v3.Version, ExpectedPlanRevision: v3.PlanRevision, Reason: "create V4 without moving tenants"})
	if err != nil {
		t.Fatal(err)
	}
	v4, err = e.plans.UpdatePlanDraft(e.ctx(), &v1.UpdatePlanDraftRequest{RequestId: "ce295-update-v4-" + ce04Random(t), PlanCode: v4.PlanCode, Version: v4.Version, ExpectedRevision: v4.Revision, Name: "CE295 V4", Terms: ce09Terms(35, 30), Reason: "add V4 capacity"})
	if err != nil {
		t.Fatal(err)
	}
	v4, err = e.plans.PublishPlanVersion(e.ctx(), ce07State(v4, ce04Random(t)))
	if err != nil {
		t.Fatal(err)
	}

	stillOld := e.getSubscription(e.tenant)
	secondStillOld := e.getSubscription(second.Id)
	if v4.Version != 4 || stillOld.PlanVersion != e.old.Version || secondStillOld.PlanVersion != e.old.Version || stillOld.PlanCode != e.old.PlanCode || quota(e.tenant) != 10 || quota(second.Id) != 10 {
		t.Fatalf("publishing V2 changed existing tenant: %+v", stillOld)
	}

	preview := e.preview(v4)
	if preview.Target.Version != v4.Version || ce09Quota(t, preview.ProjectedEntitlements) != 35 || quota(e.tenant) != 10 {
		t.Fatalf("preview should project V4 without changing current entitlement: %+v", preview)
	}
	receipt, err := e.confirm(e.confirmation(preview))
	if err != nil || receipt.Status != "APPLIED" {
		t.Fatalf("confirm=%+v err=%v", receipt, err)
	}
	after := e.getSubscription(e.tenant)
	if after.PlanVersion != v4.Version || after.PlanCode != v4.PlanCode || quota(e.tenant) != 35 || quota(second.Id) != 10 {
		t.Fatalf("confirmed V4 did not become entitlement authority while old tenant stayed unchanged: %+v", after)
	}
}
