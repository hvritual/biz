//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestEnterprise177RemoveRestoreIsTenantScopedAtomicAndKeepsOldSessionRevoked(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started, verificationProtection := startB123Enterprise176Runtime(t, db)
	base := "http://" + started.HTTPAddress()
	ctx := context.Background()

	tenantA := "e177-a-" + stamp
	adminA := "e177-admin-a-" + stamp
	adminToken := "e177-admin-token-" + stamp
	seedB123TenantAdmin(t, db, tenantA, adminA, adminA+"@example.invalid", adminToken)
	adminRoleID := tenantA + ":member-admin"
	if err := db.Exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantA, adminRoleID, "tenant.role.manage", "all").Error; err != nil {
		t.Fatal(err)
	}

	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureWebSessionSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSchema(ctx); err != nil {
		t.Fatal(err)
	}

	targetUser := "e177-target-" + stamp
	targetEmail := targetUser + "@example.invalid"
	tenantB := "e177-b-" + stamp
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantB, TenantName: "Tenant B", UserID: targetUser, Email: targetEmail, Token: "e177-b-token-" + stamp,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_memberships (tenant_id,user_id,status,name,email,version,created_at,updated_at) VALUES (?,?,?,?,?,?,NOW(6),NOW(6))",
		tenantA, targetUser, accessdomain.TenantMemberStatusActive, "Target Member", targetEmail, 1).Error; err != nil {
		t.Fatal(err)
	}
	activeRole := tenantA + ":operator"
	disabledRole := tenantA + ":legacy-disabled"
	for _, role := range []struct{ id, name, status string }{
		{activeRole, "operator", accessdomain.TenantRoleStatusActive},
		{disabledRole, "legacy-disabled", accessdomain.TenantRoleStatusDisabled},
	} {
		if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,1)", role.id, tenantA, role.name, role.status).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantA, targetUser, role.id).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO biz_member_sites (tenant_id,user_id,site_id) VALUES (?,?,?)", tenantA, targetUser, "site-e177").Error; err != nil {
		t.Fatal(err)
	}

	identity, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.enterprise177.invalid", "target-sub-"+stamp, targetEmail, true)
	if err != nil {
		t.Fatal(err)
	}
	rawSession, session, err := store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if session.Session.ActiveTenantID != "" {
		t.Fatalf("multi-tenant target unexpectedly auto-selected tenant: %+v", session.Session)
	}
	session, err = store.SwitchWebSessionTenant(ctx, rawSession, tenantA)
	if err != nil || session.Session.ActiveTenantID != tenantA {
		t.Fatalf("cannot establish pre-removal tenant session: %+v %v", session.Session, err)
	}

	memberRepo, err := accesspersistence.NewTenantMemberRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	beforeQuota, err := memberRepo.CountQuotaMembers(ctx, tenantA)
	if err != nil {
		t.Fatal(err)
	}

	statusCode, body := enterprise177Post(t, base, adminToken, "e177-remove:"+stamp,
		"/v1/tenant/members/"+url.PathEscape(targetUser)+"/remove",
		&accessv1.RemoveTenantMemberRequest{UserId: targetUser, Version: 1, Reason: "employee left tenant A"},
	)
	if statusCode != http.StatusOK {
		t.Fatalf("remove status=%d body=%s", statusCode, body)
	}
	removed := enterprise177DecodeMember(t, body)
	if removed.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_REMOVED || removed.GetVersion() != 2 {
		t.Fatalf("unexpected removed readback: %+v", removed)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawSession); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("old tenant session survived removal: %v", err)
	}
	var roleCount, siteCount, roleSnapshotCount, siteSnapshotCount int64
	for table, out := range map[string]*int64{
		"biz_member_roles":                  &roleCount,
		"biz_member_sites":                  &siteCount,
		"biz_member_removed_role_snapshots": &roleSnapshotCount,
		"biz_member_removed_site_snapshots": &siteSnapshotCount,
	} {
		if err := db.Table(table).Where("tenant_id = ? AND user_id = ?", tenantA, targetUser).Count(out).Error; err != nil {
			t.Fatal(err)
		}
	}
	if roleCount != 0 || siteCount != 0 || roleSnapshotCount != 2 || siteSnapshotCount != 1 {
		t.Fatalf("remove relation evidence roles=%d sites=%d roleSnapshots=%d siteSnapshots=%d", roleCount, siteCount, roleSnapshotCount, siteSnapshotCount)
	}
	afterRemoveQuota, err := memberRepo.CountQuotaMembers(ctx, tenantA)
	if err != nil {
		t.Fatal(err)
	}
	if afterRemoveQuota+1 != beforeQuota {
		t.Fatalf("removed member did not release usage: before=%d after=%d", beforeQuota, afterRemoveQuota)
	}
	var tenantBStatus string
	if err := db.Table("biz_memberships").Select("status").Where("tenant_id = ? AND user_id = ?", tenantB, targetUser).Scan(&tenantBStatus).Error; err != nil {
		t.Fatal(err)
	}
	if tenantBStatus != accessdomain.TenantMemberStatusActive {
		t.Fatalf("tenant B membership changed after tenant A removal: %s", tenantBStatus)
	}

	removedPage, code, body := enterprise177ListRemoved(t, base, adminToken, 1, 20)
	if code != http.StatusOK || removedPage.GetTotal() != 1 || len(removedPage.GetMembers()) != 1 || removedPage.GetMembers()[0].GetUserId() != targetUser {
		t.Fatalf("removed list status=%d body=%s page=%+v", code, body, removedPage)
	}

	statusCode, body = enterprise177Post(t, base, adminToken, "e177-restore:"+stamp,
		"/v1/tenant/members/"+url.PathEscape(targetUser)+"/restore",
		&accessv1.RestoreTenantMemberRequest{UserId: targetUser, Version: 2, Reason: "approved return"},
	)
	if statusCode != http.StatusOK {
		t.Fatalf("restore status=%d body=%s", statusCode, body)
	}
	restored := enterprise177DecodeMember(t, body)
	if restored.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE || restored.GetVersion() != 3 {
		t.Fatalf("unexpected restored member: %+v", restored)
	}
	var activeRoleCount, disabledRoleCount int64
	if err := db.Table("biz_member_roles").Where("tenant_id = ? AND user_id = ? AND role_id = ?", tenantA, targetUser, activeRole).Count(&activeRoleCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_member_roles").Where("tenant_id = ? AND user_id = ? AND role_id = ?", tenantA, targetUser, disabledRole).Count(&disabledRoleCount).Error; err != nil {
		t.Fatal(err)
	}
	if activeRoleCount != 1 || disabledRoleCount != 0 {
		t.Fatalf("restore resurrected invalid roles: active=%d disabled=%d", activeRoleCount, disabledRoleCount)
	}
	if err := db.Table("biz_member_sites").Where("tenant_id = ? AND user_id = ?", tenantA, targetUser).Count(&siteCount).Error; err != nil {
		t.Fatal(err)
	}
	if siteCount != 0 {
		t.Fatalf("restore silently resurrected unresolved data scope: site_count=%d", siteCount)
	}
	afterRestoreQuota, err := memberRepo.CountQuotaMembers(ctx, tenantA)
	if err != nil {
		t.Fatal(err)
	}
	if afterRestoreQuota != beforeQuota {
		t.Fatalf("restored member usage mismatch: before=%d restored=%d", beforeQuota, afterRestoreQuota)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawSession); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("restore revived pre-removal web session: %v", err)
	}
	freshRaw, freshSession, err := store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	freshSession, err = store.SwitchWebSessionTenant(ctx, freshRaw, tenantA)
	if err != nil || freshSession.Session.ActiveTenantID != tenantA {
		t.Fatalf("fresh login cannot enter restored tenant: %+v %v", freshSession.Session, err)
	}

	var lifecycleEvents int64
	if err := db.Table("biz_security_notification_outbox").
		Where("tenant_id = ? AND user_id = ? AND kind = ?", tenantA, targetUser, string(accessdomain.SecurityNotificationMemberLifecycle)).
		Count(&lifecycleEvents).Error; err != nil {
		t.Fatal(err)
	}
	if lifecycleEvents != 2 {
		t.Fatalf("remove/restore notification events=%d want=2", lifecycleEvents)
	}
	verification, err := accesspersistence.NewVerificationRepository(db, verificationProtection)
	if err != nil {
		t.Fatal(err)
	}
	var removeEventID string
	if err := db.Table("biz_security_notification_outbox").Select("event_id").
		Where("tenant_id = ? AND user_id = ? AND business_event_id LIKE ?", tenantA, targetUser, "member-lifecycle/%/removed/%").
		Limit(1).Scan(&removeEventID).Error; err != nil {
		t.Fatal(err)
	}
	claim, _, err := verification.ClaimSecurityNotification(ctx, removeEventID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(claim.Secret, "employee left tenant A") {
		t.Fatalf("lifecycle notification lost server-side reason: %q", claim.Secret)
	}
}

func TestEnterprise177RejectsSelfDeactivationAndOrdinaryAdminAgainstOwner(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started, _ := startB123Enterprise176Runtime(t, db)
	base := "http://" + started.HTTPAddress()
	tenant := "e177-guard-" + stamp
	admin := "e177-admin-" + stamp
	token := "e177-admin-token-" + stamp
	seedB123TenantAdmin(t, db, tenant, admin, admin+"@example.invalid", token)

	status, _ := enterprise177Post(t, base, token, "e177-self:"+stamp,
		"/v1/tenant/members/"+url.PathEscape(admin)+"/suspend",
		&accessv1.SuspendTenantMemberRequest{UserId: admin, Version: 1, Reason: "self suspend should fail"},
	)
	if status == http.StatusOK {
		t.Fatal("administrator suspended own membership")
	}

	ownerRole := tenant + ":owner"
	if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,1)", ownerRole, tenant, accessdomain.TenantOwnerRoleName, accessdomain.TenantRoleStatusActive).Error; err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		userID := fmt.Sprintf("e177-owner-%d-%s", index, stamp)
		email := userID + "@example.invalid"
		if err := db.Exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(6))", userID, email, "active").Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO biz_memberships (tenant_id,user_id,status,email,version,created_at,updated_at) VALUES (?,?,?,?,1,NOW(6),NOW(6))", tenant, userID, accessdomain.TenantMemberStatusActive, email).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenant, userID, ownerRole).Error; err != nil {
			t.Fatal(err)
		}
	}
	targetOwner := "e177-owner-0-" + stamp
	status, _ = enterprise177Post(t, base, token, "e177-owner-deny:"+stamp,
		"/v1/tenant/members/"+url.PathEscape(targetOwner)+"/suspend",
		&accessv1.SuspendTenantMemberRequest{UserId: targetOwner, Version: 1, Reason: "ordinary admin must not suspend owner"},
	)
	if status == http.StatusOK {
		t.Fatal("ordinary administrator suspended protected owner")
	}
	var targetStatus string
	if err := db.Table("biz_memberships").Select("status").Where("tenant_id = ? AND user_id = ?", tenant, targetOwner).Scan(&targetStatus).Error; err != nil {
		t.Fatal(err)
	}
	if targetStatus != accessdomain.TenantMemberStatusActive {
		t.Fatalf("rejected owner suspension left partial state: %s", targetStatus)
	}
}

func TestEnterprise177AppealIsSelfOnlyRateLimitedAndNotificationFailureDoesNotGrantAccess(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	protection, err := accesspersistence.NewVerificationProtection(accesspersistence.VerificationProtectionConfig{
		ActiveVersion: "v1",
		Keys:          map[string][]byte{"v1": []byte(strings.Repeat("A", 32))},
		HMACKey:       []byte(strings.Repeat("B", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	verification, err := accesspersistence.NewVerificationRepository(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	if err := verification.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	appeals, err := accesspersistence.NewMemberAppealService(db, nil, protection)
	if err != nil {
		t.Fatal(err)
	}
	if err := appeals.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	stamp := fmt.Sprint(time.Now().UnixNano())
	tenantA := "e177-appeal-a-" + stamp
	owner := "e177-appeal-owner-" + stamp
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantA, TenantName: "Appeal Tenant A", UserID: owner, Email: owner + "@example.invalid", Token: "owner-token-" + stamp,
	}, nil); err != nil {
		t.Fatal(err)
	}
	target := "e177-appeal-target-" + stamp
	targetEmail := target + "@example.invalid"
	if err := db.Exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(6))", target, targetEmail, "active").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_memberships (tenant_id,user_id,status,email,version,created_at,updated_at) VALUES (?,?,?,?,1,NOW(6),NOW(6))", tenantA, target, accessdomain.TenantMemberStatusActive, targetEmail).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := appeals.Submit(ctx, target, tenantA, "active account must not appeal"); !errors.Is(err, accesspersistence.ErrMemberAppealNotEligible) {
		t.Fatalf("active member appeal accepted: %v", err)
	}

	tenantB := "e177-appeal-b-" + stamp
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantB, TenantName: "Appeal Tenant B", UserID: target, Email: targetEmail, Token: "target-b-token-" + stamp,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", tenantA, target).Update("status", accessdomain.TenantMemberStatusSuspended).Error; err != nil {
		t.Fatal(err)
	}
	eligible, err := appeals.ListEligible(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(eligible) != 1 || eligible[0].TenantID != tenantA || eligible[0].Status != accessdomain.TenantMemberStatusSuspended {
		t.Fatalf("appeal eligibility crossed tenant/status boundary: %+v", eligible)
	}
	receipt, err := appeals.Submit(ctx, target, tenantA, "access was suspended by mistake")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.AppealID == "" || receipt.State != accesspersistence.MemberAppealStatePending || len(receipt.NotificationEventIDs) == 0 {
		t.Fatalf("appeal receipt incomplete: %+v", receipt)
	}
	if _, err := appeals.Submit(ctx, target, tenantA, "duplicate within five minutes"); err == nil {
		t.Fatal("duplicate appeal was not rate limited")
	} else {
		var rateLimit accesspersistence.MemberAppealRateLimitError
		if !errors.As(err, &rateLimit) || rateLimit.RetryAfter <= 0 || rateLimit.RetryAfter > 5*time.Minute {
			t.Fatalf("unexpected appeal rate limit: %v", err)
		}
	}

	claim, _, err := verification.ClaimSecurityNotification(ctx, receipt.NotificationEventIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(claim.Secret, target) || !strings.Contains(claim.Secret, "suspended by mistake") {
		t.Fatalf("owner notification lost appeal evidence: %q", claim.Secret)
	}
	if _, err := verification.CompleteSecurityNotification(ctx, claim, "", "PROVIDER_DOWN", false); err != nil {
		t.Fatal(err)
	}
	var appealState string
	if err := db.Table("biz_member_status_appeals").Select("state").Where("tenant_id = ? AND user_id = ?", tenantA, target).Scan(&appealState).Error; err != nil {
		t.Fatal(err)
	}
	if appealState != accesspersistence.MemberAppealStatePending {
		t.Fatalf("notification failure falsely changed appeal state: %s", appealState)
	}
	var membershipStatus string
	if err := db.Table("biz_memberships").Select("status").Where("tenant_id = ? AND user_id = ?", tenantA, target).Scan(&membershipStatus).Error; err != nil {
		t.Fatal(err)
	}
	if membershipStatus != accessdomain.TenantMemberStatusSuspended {
		t.Fatalf("appeal granted tenant access: %s", membershipStatus)
	}
}

func enterprise177Post(t *testing.T, base, token, key, path string, message proto.Message) (int, []byte) {
	t.Helper()
	payload, err := protojson.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, body
}

func enterprise177DecodeMember(t *testing.T, body []byte) *accessv1.TenantMemberDTO {
	t.Helper()
	var result accessv1.TenantMemberDTO
	if err := protojson.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return &result
}

func enterprise177ListRemoved(t *testing.T, base, token string, page, pageSize uint32) (*accessv1.ListTenantMembersResponse, int, []byte) {
	t.Helper()
	endpoint := fmt.Sprintf("%s/v1/tenant/members/removed?page=%d&page_size=%d", base, page, pageSize)
	request, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	var result accessv1.ListTenantMembersResponse
	if response.StatusCode == http.StatusOK {
		if err := protojson.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
	}
	return &result, response.StatusCode, body
}
