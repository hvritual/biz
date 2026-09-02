//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
)

type b125Counts struct {
	tenants     int64
	users       int64
	memberships int64
	roles       int64
	grants      int64
	memberRoles int64
}

func b125Snapshot(t *testing.T, db *gorm.DB) b125Counts {
	t.Helper()
	count := func(table string) int64 {
		var value int64
		if err := db.Table(table).Count(&value).Error; err != nil {
			t.Fatal(err)
		}
		return value
	}
	return b125Counts{
		tenants: count("biz_tenants"), users: count("biz_users"), memberships: count("biz_memberships"),
		roles: count("biz_roles"), grants: count("biz_permission_grants"), memberRoles: count("biz_member_roles"),
	}
}

func b125CreateTenant(t *testing.T, base, token, key, name, ownerUserID, ownerEmail string) (int, []byte) {
	t.Helper()
	payload, err := protojson.Marshal(&accessv1.CreateTenantRequest{Name: name, OwnerUserId: ownerUserID, OwnerEmail: ownerEmail})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, base+"/v1/tenants", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, body
}

func TestB125TenantCreateBootstrapsOwnerInOneRootUoW(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	platformToken := "b125-platform-" + stamp
	started := startB122Runtime(t, db, platformToken)
	base := "http://" + started.HTTPAddress()
	ownerUserID := "b125-owner-" + stamp
	ownerEmail := ownerUserID + "@example.invalid"

	status, body := b125CreateTenant(t, base, platformToken, "b125-success:"+stamp, "B125 Tenant "+stamp, ownerUserID, ownerEmail)
	if status != http.StatusOK {
		t.Fatalf("create status=%d body=%s", status, body)
	}
	created := &accessv1.TenantDTO{}
	if err := protojson.Unmarshal(body, created); err != nil {
		t.Fatal(err)
	}
	if created.GetId() == "" || created.GetStatus() != accessv1.TenantStatus_TENANT_STATUS_PENDING || created.GetVersion() != 1 {
		t.Fatalf("created=%+v", created)
	}

	var tenantCount, userCount, memberCount, ownerRoleCount, assignmentCount int64
	for _, check := range []struct {
		query *gorm.DB
		out   *int64
	}{
		{db.Table("biz_tenants").Where("id = ? AND status = ?", created.GetId(), "pending"), &tenantCount},
		{db.Table("biz_users").Where("id = ? AND email = ?", ownerUserID, ownerEmail), &userCount},
		{db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ? AND status = ?", created.GetId(), ownerUserID, "active"), &memberCount},
		{db.Table("biz_roles").Where("tenant_id = ? AND id = ? AND name = ? AND status = ?", created.GetId(), created.GetId()+":owner", "owner", "active"), &ownerRoleCount},
		{db.Table("biz_member_roles").Where("tenant_id = ? AND user_id = ? AND role_id = ?", created.GetId(), ownerUserID, created.GetId()+":owner"), &assignmentCount},
	} {
		if err := check.query.Count(check.out).Error; err != nil {
			t.Fatal(err)
		}
	}
	if tenantCount != 1 || userCount != 1 || memberCount != 1 || ownerRoleCount != 1 || assignmentCount != 1 {
		t.Fatalf("bootstrap counts tenant=%d user=%d member=%d role=%d assignment=%d", tenantCount, userCount, memberCount, ownerRoleCount, assignmentCount)
	}
	var grantCount int64
	if err := db.Table("biz_permission_grants").Where("tenant_id = ? AND role_id = ?", created.GetId(), created.GetId()+":owner").Count(&grantCount).Error; err != nil {
		t.Fatal(err)
	}
	if grantCount < 4 {
		t.Fatalf("owner grant count=%d want>=4", grantCount)
	}
}

func TestB125ChildFailureRollsBackTenantAndMemberAndAllowsIdempotentRetry(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	platformToken := "b125-platform-rollback-" + stamp
	started := startB122Runtime(t, db, platformToken)
	base := "http://" + started.HTTPAddress()
	name := "B125 Rollback Tenant " + stamp
	ownerUserID := "b125-rollback-owner-" + stamp
	ownerEmail := ownerUserID + "@example.invalid"
	key := "b125-rollback:" + stamp

	if err := db.Exec("DROP TRIGGER IF EXISTS b125_fail_owner_role").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER b125_fail_owner_role BEFORE INSERT ON biz_roles FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'b125 forced owner role failure'").Error; err != nil {
		t.Fatal(err)
	}
	triggerPresent := true
	t.Cleanup(func() {
		if triggerPresent {
			_ = db.Exec("DROP TRIGGER IF EXISTS b125_fail_owner_role").Error
		}
	})

	before := b125Snapshot(t, db)
	status, body := b125CreateTenant(t, base, platformToken, key, name, ownerUserID, ownerEmail)
	if status == http.StatusOK {
		t.Fatalf("forced child failure unexpectedly succeeded body=%s", body)
	}
	after := b125Snapshot(t, db)
	if after != before {
		t.Fatalf("root rollback leaked business rows before=%+v after=%+v", before, after)
	}
	var leakedTenant int64
	if err := db.Table("biz_tenants").Where("name = ?", name).Count(&leakedTenant).Error; err != nil {
		t.Fatal(err)
	}
	if leakedTenant != 0 {
		t.Fatalf("failed bootstrap leaked tenant rows=%d", leakedTenant)
	}

	if err := db.Exec("DROP TRIGGER IF EXISTS b125_fail_owner_role").Error; err != nil {
		t.Fatal(err)
	}
	triggerPresent = false

	// A failed root attempt must not be recorded as completed. Reusing the same
	// idempotency key after the cause is removed must execute and commit once.
	status, body = b125CreateTenant(t, base, platformToken, key, name, ownerUserID, ownerEmail)
	if status != http.StatusOK {
		t.Fatalf("retry after rollback status=%d body=%s", status, body)
	}
	created := &accessv1.TenantDTO{}
	if err := protojson.Unmarshal(body, created); err != nil {
		t.Fatal(err)
	}
	if created.GetId() == "" {
		t.Fatal("retry returned empty tenant id")
	}
	var memberCount, roleCount, assignmentCount int64
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", created.GetId(), ownerUserID).Count(&memberCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_roles").Where("tenant_id = ? AND id = ?", created.GetId(), created.GetId()+":owner").Count(&roleCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_member_roles").Where("tenant_id = ? AND user_id = ? AND role_id = ?", created.GetId(), ownerUserID, created.GetId()+":owner").Count(&assignmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if memberCount != 1 || roleCount != 1 || assignmentCount != 1 {
		t.Fatalf("retry bootstrap incomplete member=%d role=%d assignment=%d", memberCount, roleCount, assignmentCount)
	}
}
