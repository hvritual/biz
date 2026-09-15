//go:build integration

package integration

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
)

func seedECIR06UsagePermission(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	roleID := tenantID + ":member-admin"
	if err := db.Exec("INSERT IGNORE INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, roleID, "tenant.entitlement.read", "all").Error; err != nil {
		t.Fatal(err)
	}
}

func seedECIR06Membership(t *testing.T, db *gorm.DB, tenantID, suffix, status string) string {
	t.Helper()
	userID := tenantID + ":usage:" + suffix
	email := userID + "@example.invalid"
	now := time.Now().UTC()
	if err := db.Exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,?)", userID, email, "active", now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_memberships (tenant_id,user_id,status,version,created_at,updated_at) VALUES (?,?,?,?,?,?)", tenantID, userID, status, 1, now, now).Error; err != nil {
		t.Fatal(err)
	}
	return userID
}

func tenantUsageHTTP(t *testing.T, endpoint, token string) (*commercialv1.GetMyTenantUsageResponse, int, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, body
	}
	var usage commercialv1.GetMyTenantUsageResponse
	if err := protojson.Unmarshal(body, &usage); err != nil {
		t.Fatal(err)
	}
	return &usage, response.StatusCode, body
}

func memberUsageValue(t *testing.T, usage *commercialv1.GetMyTenantUsageResponse) *commercialv1.TenantQuotaUsageDTO {
	t.Helper()
	for _, item := range usage.GetUsages() {
		if item.GetModuleCode() == "access-management" && item.GetKey() == "tenant.members" {
			return item
		}
	}
	t.Fatalf("tenant.members authoritative usage missing: %+v", usage)
	return nil
}

func TestECIR06TenantUsageCountsOnlyNonRemovedMembersAndKeepsTenantScope(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB, tenantNoPermission := "ec-ri-06-usage-a-"+stamp, "ec-ri-06-usage-b-"+stamp, "ec-ri-06-usage-no-perm-"+stamp
	adminA, adminB, adminNoPermission := tenantA+":admin", tenantB+":admin", tenantNoPermission+":admin"
	tokenA, tokenB, tokenNoPermission := tenantA+":token", tenantB+":token", tenantNoPermission+":token"
	seedB123TenantAdmin(t, db, tenantA, adminA, adminA+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, adminB, adminB+"@example.invalid", tokenB)
	seedB123TenantAdmin(t, db, tenantNoPermission, adminNoPermission, adminNoPermission+"@example.invalid", tokenNoPermission)
	seedECIR06UsagePermission(t, db, tenantA)
	seedECIR06UsagePermission(t, db, tenantB)

	invitedA := seedECIR06Membership(t, db, tenantA, "invited", "invited")
	seedECIR06Membership(t, db, tenantA, "suspended", "suspended")
	seedECIR06Membership(t, db, tenantA, "removed", "removed")
	seedECIR06Membership(t, db, tenantB, "invited", "invited")

	usageA, statusCode, body := tenantUsageHTTP(t, base+"/v1/tenant/usage", tokenA)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A usage status=%d body=%s", statusCode, body)
	}
	memberA := memberUsageValue(t, usageA)
	if !memberA.GetKnown() || memberA.GetUsed() != 3 {
		t.Fatalf("tenant A member usage=%+v want known=true used=3", memberA)
	}
	if memberA.GetEvidence() == "" {
		t.Fatal("authoritative member usage must include evidence")
	}

	// A caller cannot override trusted scope with a query-string tenant id.
	spoofed, statusCode, body := tenantUsageHTTP(t, base+"/v1/tenant/usage?tenant_id="+tenantB, tokenA)
	if statusCode != http.StatusOK {
		t.Fatalf("spoofed usage status=%d body=%s", statusCode, body)
	}
	if got := memberUsageValue(t, spoofed).GetUsed(); got != 3 {
		t.Fatalf("tenant query override leaked scope: got=%d want=3", got)
	}

	usageB, statusCode, body := tenantUsageHTTP(t, base+"/v1/tenant/usage", tokenB)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant B usage status=%d body=%s", statusCode, body)
	}
	if got := memberUsageValue(t, usageB).GetUsed(); got != 2 {
		t.Fatalf("tenant B member usage=%d want=2", got)
	}

	if err := db.Exec("UPDATE biz_memberships SET status=?, version=version+1, updated_at=? WHERE tenant_id=? AND user_id=?", "removed", time.Now().UTC(), tenantA, invitedA).Error; err != nil {
		t.Fatal(err)
	}
	usageAfterRemoval, statusCode, body := tenantUsageHTTP(t, base+"/v1/tenant/usage", tokenA)
	if statusCode != http.StatusOK {
		t.Fatalf("usage after removal status=%d body=%s", statusCode, body)
	}
	if got := memberUsageValue(t, usageAfterRemoval).GetUsed(); got != 2 {
		t.Fatalf("removed membership did not release quota: got=%d want=2", got)
	}

	_, statusCode, _ = tenantUsageHTTP(t, base+"/v1/tenant/usage", tokenNoPermission)
	if statusCode != http.StatusForbidden {
		t.Fatalf("usage without tenant.entitlement.read status=%d want=%d", statusCode, http.StatusForbidden)
	}
}
