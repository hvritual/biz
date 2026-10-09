//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	mysql "github.com/go-sql-driver/mysql"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	persistence "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
	"google.golang.org/protobuf/proto"
)

func ce340RequireSQLRejection(t *testing.T, err error, number uint16) {
	t.Helper()
	var failure *mysql.MySQLError
	if !errors.As(err, &failure) || failure.Number != number {
		t.Fatalf("SQL rejection=%v want MySQL error %d", err, number)
	}
}

func TestCE340MySQLPreviewMigrationPreservesHistoryAndConstraints(t *testing.T) {
	e := ce09New(t)
	preview := e.preview(e.plan(ce09Terms(25, 30)))
	receipt, err := e.confirm(e.confirmation(preview))
	if err != nil {
		t.Fatal(err)
	}
	before := ce09State(t, e)

	// Reconstruct the released CE09 preview schema with real legacy history.
	// This isolated fixture uses exactly the old constraints and nullability;
	// no production data, FK disabling, payload rewriting or fake subscription.
	legacySchema := `ALTER TABLE biz_commercial_change_previews
 DROP CHECK ce340_preview_source_shape,
 DROP FOREIGN KEY ce340_preview_tenant,
 DROP FOREIGN KEY ce340_preview_source_plan,
 MODIFY tenant_id VARCHAR(64) NOT NULL,
 MODIFY source_plan_code VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 MODIFY source_plan_version BIGINT UNSIGNED NOT NULL,
 ADD CONSTRAINT ce09_preview_tenant FOREIGN KEY(tenant_id)
   REFERENCES biz_commercial_subscriptions(tenant_id) ON DELETE RESTRICT ON UPDATE RESTRICT,
 ADD CONSTRAINT ce09_preview_source_plan FOREIGN KEY(source_plan_code,source_plan_version)
   REFERENCES biz_commercial_plan_versions(plan_code,version) ON DELETE RESTRICT ON UPDATE RESTRICT`
	if err := e.db.Exec(legacySchema).Error; err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := persistence.MigrateSubscriptions(context.Background(), e.db); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt, err)
		}
	}
	ce09EqualState(t, before, ce09State(t, e))
	gotPreview, err := e.changes.GetSubscriptionChangePreview(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: preview.ChangeId})
	if err != nil || !proto.Equal(preview, gotPreview) {
		t.Fatalf("legacy preview changed: %v %v", gotPreview, err)
	}
	gotReceipt, err := e.changes.GetSubscriptionChangeReceipt(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: preview.ChangeId})
	if err != nil || !proto.Equal(receipt, gotReceipt) {
		t.Fatalf("legacy receipt changed: %v %v", gotReceipt, err)
	}

	// Existing changes still require a real, exact source PlanVersion. The
	// target FK and commercial anchor remain restrictive after the migration.
	ce340RequireSQLRejection(t, e.db.Exec("UPDATE biz_commercial_change_previews SET source_plan_code=NULL, source_plan_version=NULL WHERE change_id=?", preview.ChangeId).Error, 3819)
	ce340RequireSQLRejection(t, e.db.Exec("UPDATE biz_commercial_change_previews SET source_plan_version=18446744073709551615 WHERE change_id=?", preview.ChangeId).Error, 1452)
	ce340RequireSQLRejection(t, e.db.Exec("UPDATE biz_commercial_change_previews SET target_plan_version=18446744073709551615 WHERE change_id=?", preview.ChangeId).Error, 1452)
	ce340RequireSQLRejection(t, e.db.Exec("UPDATE biz_commercial_change_previews SET tenant_id=? WHERE change_id=?", "ce340-missing-anchor-"+ce04Random(t), preview.ChangeId).Error, 1452)
	ce09EqualState(t, before, ce09State(t, e))
}

func TestCE340MySQLInitialPreviewHasNoSourcePlanAndDoesNotActivate(t *testing.T) {
	e := ce09OnDB(t, ce08FreshFixtureDB(t), "ce340-platform-"+ce04Random(t))
	tenantID := ce340NoSubscriptionTenant(t, e)
	target := e.plan(ce09Terms(40, 30))
	preview := ce340InitialPreview(t, e, target)
	var count int64
	if err := e.db.Table("biz_commercial_change_previews").Where("change_id=? AND source_plan_code IS NULL AND source_plan_version IS NULL", preview.ChangeId).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("initial source reference count=%d err=%v", count, err)
	}
	for _, table := range []string{
		"biz_commercial_subscriptions", "biz_commercial_entitlement_sources",
		"biz_commercial_change_receipts", "biz_commercial_provisioning_tasks",
		"biz_memberships", "biz_roles", "biz_member_roles",
	} {
		if err := e.db.Table(table).Where("tenant_id=?", tenantID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("preview prematurely created state table=%s rows=%d err=%v", table, count, err)
		}
	}
	for _, decision := range e.view().Decisions {
		if decision.Allowed {
			t.Fatalf("preview unlocked entitlement: %+v", decision)
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := persistence.MigrateSubscriptions(context.Background(), e.db); err != nil {
			t.Fatalf("repeat migration with initial preview: %v", err)
		}
	}
	got, err := e.changes.GetSubscriptionChangePreview(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: tenantID, ChangeId: preview.ChangeId})
	if err != nil || !proto.Equal(preview, got) {
		t.Fatalf("initial preview did not round trip: %v %v", got, err)
	}
	ce340RequireSQLRejection(t, e.db.Exec("UPDATE biz_commercial_change_previews SET source_plan_code=?, source_plan_version=? WHERE change_id=?", target.PlanCode, target.Version, preview.ChangeId).Error, 3819)
	ce340RequireSQLRejection(t, e.db.Exec("UPDATE biz_commercial_change_previews SET source_plan_version=? WHERE change_id=?", target.Version, preview.ChangeId).Error, 3819)
}
