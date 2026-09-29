//go:build integration

package integration

import (
	"testing"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
)

func TestCE295MySQLPublishedVersionDoesNotAutoMigrateExistingTenant(t *testing.T) {
	e := ce09New(t)
	before := e.getSubscription(e.tenant)
	if before.PlanCode != e.old.PlanCode || before.PlanVersion != e.old.Version || ce09Quota(t, e.view()) != 10 {
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

	stillOld := e.getSubscription(e.tenant)
	if stillOld.PlanVersion != e.old.Version || stillOld.PlanCode != e.old.PlanCode || ce09Quota(t, e.view()) != 10 {
		t.Fatalf("publishing V2 changed existing tenant: %+v", stillOld)
	}

	preview := e.preview(v2)
	if preview.Target.Version != v2.Version || ce09Quota(t, preview.ProjectedEntitlements) != 25 || ce09Quota(t, e.view()) != 10 {
		t.Fatalf("preview should project V2 without changing current entitlement: %+v", preview)
	}
	receipt, err := e.confirm(e.confirmation(preview))
	if err != nil || receipt.Status != "APPLIED" {
		t.Fatalf("confirm=%+v err=%v", receipt, err)
	}
	after := e.getSubscription(e.tenant)
	if after.PlanVersion != v2.Version || after.PlanCode != v2.PlanCode || ce09Quota(t, e.view()) != 25 {
		t.Fatalf("confirmed V2 did not become entitlement authority: %+v", after)
	}
}
