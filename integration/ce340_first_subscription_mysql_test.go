//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ce340NoSubscriptionTenant(t *testing.T, e *ce09Environment) string {
	t.Helper()
	repository, err := accesspersistence.NewTenantRepository(e.db)
	if err != nil {
		t.Fatal(err)
	}
	id := "ce340-" + ce04Random(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	tenant := accessdomain.NewTenant(id, "CE340 no-subscription "+id, now)
	if err := tenant.Activate(now); err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(context.Background(), &tenant); err != nil {
		t.Fatal(err)
	}
	e.tenant = id
	return id
}

func ce340InitialPreview(t *testing.T, e *ce09Environment, target *commercialv1.PlanVersionDTO) *commercialv1.SubscriptionChangePreviewDTO {
	t.Helper()
	requestID := "ce340-preview-" + ce04Random(t)
	preview, err := e.changes.PreviewSubscriptionChange(
		ce04Context(e.token, requestID),
		&commercialv1.PreviewSubscriptionChangeRequest{
			TenantId:          e.tenant,
			RequestId:         requestID,
			Action:            "INITIAL",
			TargetPlanCode:    target.PlanCode,
			TargetPlanVersion: target.Version,
			SalesScope:        "ce09",
			Reason:            "CE340 platform first activation",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Before != nil || preview.Action != "INITIAL" || preview.Classification != "INITIAL" || preview.Target.GetPlanCode() != target.GetPlanCode() || preview.Target.GetVersion() != target.GetVersion() {
		t.Fatalf("initial preview did not bind exact target: %+v", preview)
	}
	return preview
}

func ce340InitialConfirm(preview *commercialv1.SubscriptionChangePreviewDTO, requestID string) *commercialv1.ConfirmSubscriptionChangeRequest {
	return &commercialv1.ConfirmSubscriptionChangeRequest{
		TenantId:    preview.TenantId,
		ChangeId:    preview.ChangeId,
		RequestId:   requestID,
		PreviewHash: preview.PreviewHash,
		Reason:      "CE340 confirmed exact first activation",
	}
}

func TestCE340MySQLFirstActivationRequiresExistingTenant(t *testing.T) {
	e := ce09OnDB(t, ce08FreshFixtureDB(t), "ce340-platform-"+ce04Random(t))
	target := e.plan(ce09Terms(20, 30))
	missing := "ce340-missing-" + ce04Random(t)
	requestID := "ce340-missing-preview-" + ce04Random(t)

	_, err := e.changes.PreviewSubscriptionChange(
		ce04Context(e.token, requestID),
		&commercialv1.PreviewSubscriptionChangeRequest{
			TenantId:          missing,
			RequestId:         requestID,
			Action:            "INITIAL",
			TargetPlanCode:    target.PlanCode,
			TargetPlanVersion: target.Version,
			SalesScope:        "ce09",
			Reason:            "must reject missing tenant",
		},
	)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("missing tenant error=%v want NotFound", err)
	}

	var subscriptions, sources int64
	if err := e.db.Table("biz_commercial_subscriptions").Where("tenant_id=?", missing).Count(&subscriptions).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.db.Table("biz_commercial_entitlement_sources").Where("tenant_id=?", missing).Count(&sources).Error; err != nil {
		t.Fatal(err)
	}
	if subscriptions != 0 || sources != 0 {
		t.Fatalf("missing tenant leaked commercial state subscriptions=%d sources=%d", subscriptions, sources)
	}
}

func TestCE340MySQLConcurrentFirstActivationCreatesOneSubscriptionAndNoIAM(t *testing.T) {
	e := ce09OnDB(t, ce08FreshFixtureDB(t), "ce340-platform-"+ce04Random(t))
	tenantID := ce340NoSubscriptionTenant(t, e)
	target := e.plan(ce09Terms(40, 30))

	first := ce340InitialPreview(t, e, target)
	second := ce340InitialPreview(t, e, target)
	if first.ChangeId == second.ChangeId {
		t.Fatal("distinct first-activation requests unexpectedly shared a change id")
	}

	requests := []*commercialv1.ConfirmSubscriptionChangeRequest{
		ce340InitialConfirm(first, "ce340-confirm-"+ce04Random(t)),
		ce340InitialConfirm(second, "ce340-confirm-"+ce04Random(t)),
	}
	type result struct {
		receipt *commercialv1.SubscriptionChangeReceiptDTO
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, len(requests))
	var wg sync.WaitGroup
	for _, request := range requests {
		request := request
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			receipt, err := e.changes.ConfirmSubscriptionChange(ce04Context(e.token, request.RequestId), request)
			results <- result{receipt: receipt, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var applied int
	var conflicts int
	for outcome := range results {
		switch {
		case outcome.err == nil:
			applied++
			if outcome.receipt == nil || outcome.receipt.Status != "APPLIED" || outcome.receipt.Action != "INITIAL" || outcome.receipt.After == nil {
				t.Fatalf("unexpected successful receipt: %+v", outcome.receipt)
			}
		case status.Code(outcome.err) == codes.Aborted:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent first-activation error: %v", outcome.err)
		}
	}
	if applied != 1 || conflicts != 1 {
		t.Fatalf("concurrent first activation applied=%d conflicts=%d want 1/1", applied, conflicts)
	}

	var subscriptionCount, membershipCount, roleCount, memberRoleCount int64
	for _, check := range []struct {
		table string
		out   *int64
	}{
		{"biz_commercial_subscriptions", &subscriptionCount},
		{"biz_memberships", &membershipCount},
		{"biz_roles", &roleCount},
		{"biz_member_roles", &memberRoleCount},
	} {
		if err := e.db.Table(check.table).Where("tenant_id=?", tenantID).Count(check.out).Error; err != nil {
			t.Fatal(err)
		}
	}
	if subscriptionCount != 1 {
		t.Fatalf("first activation created %d base subscriptions", subscriptionCount)
	}
	if membershipCount != 0 || roleCount != 0 || memberRoleCount != 0 {
		t.Fatalf("commercial activation mutated IAM membership=%d roles=%d assignments=%d", membershipCount, roleCount, memberRoleCount)
	}

	subscription := e.getSubscription(tenantID)
	if subscription.PlanCode != target.PlanCode || subscription.PlanVersion != target.Version || subscription.State != "ACTIVE" || subscription.SalesScope != "ce09" {
		t.Fatalf("final subscription does not bind exact target: %+v", subscription)
	}
	if ce09Quota(t, e.view()) != 40 {
		t.Fatal("final entitlement does not contain the activated plan quota")
	}

	var origin string
	if err := e.db.Table("biz_commercial_subscriptions").
		Select("JSON_UNQUOTE(JSON_EXTRACT(payload,'$.origin'))").
		Where("tenant_id=?", tenantID).
		Scan(&origin).Error; err != nil {
		t.Fatal(err)
	}
	if origin != "INITIAL_ACTIVATION" {
		t.Fatalf("subscription origin=%q want INITIAL_ACTIVATION", origin)
	}

	var receipts int64
	if err := e.db.Table("biz_commercial_change_receipts").Where("tenant_id=?", tenantID).Count(&receipts).Error; err != nil {
		t.Fatal(err)
	}
	if receipts != 1 {
		t.Fatalf("concurrent first activation wrote %d receipts want 1", receipts)
	}
}
