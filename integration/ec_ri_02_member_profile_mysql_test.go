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
)

func updateB123ProfileHTTP(t *testing.T, base, token, key string, input *accessv1.UpdateTenantMemberProfileRequest) (*accessv1.TenantMemberDTO, int, []byte) {
	t.Helper()
	payload, err := protojson.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPatch, base+"/v1/tenant/members/"+input.GetUserId()+"/profile", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, body
	}
	var member accessv1.TenantMemberDTO
	if err := protojson.Unmarshal(body, &member); err != nil {
		t.Fatal(err)
	}
	return &member, response.StatusCode, body
}

func roleNames(member *accessv1.TenantMemberDTO) map[string]string {
	result := make(map[string]string, len(member.GetRoles()))
	for _, role := range member.GetRoles() {
		result[role.GetRoleName()] = role.GetRoleStatus()
	}
	return result
}

func TestB123ECIR02MemberProfileRoleBindingIsAuthoritativeAndTenantScoped(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "ec-ri-02-a-"+stamp, "ec-ri-02-b-"+stamp
	tokenA, tokenB := "ec-ri-02-token-a-"+stamp, "ec-ri-02-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, "ec-ri-02-admin-a-"+stamp, "admin-a-"+stamp+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, "ec-ri-02-admin-b-"+stamp, "admin-b-"+stamp+"@example.invalid", tokenB)

	sharedEmail := "profile-shared-" + stamp + "@example.invalid"
	memberA, statusCode, body := inviteB123HTTP(t, base, tokenA, sharedEmail, "ec-ri-02-invite-a:"+stamp)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A invite status=%d body=%s", statusCode, body)
	}
	memberB, statusCode, body := inviteB123HTTP(t, base, tokenB, sharedEmail, "ec-ri-02-invite-b:"+stamp)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant B invite status=%d body=%s", statusCode, body)
	}
	if memberA.GetUserId() != memberB.GetUserId() {
		t.Fatalf("expected global user reuse: A=%q B=%q", memberA.GetUserId(), memberB.GetUserId())
	}

	updatedA, statusCode, body := updateB123ProfileHTTP(t, base, tokenA, "ec-ri-02-profile-a:"+stamp, &accessv1.UpdateTenantMemberProfileRequest{
		UserId:       memberA.GetUserId(),
		Name:         " Alice Chen ",
		Phone:        " +886900000001 ",
		EmployeeId:   " EMP-1001 ",
		Position:     " Customer Success Lead ",
		DepartmentId: " dept-success ",
		Version:      memberA.GetVersion(),
	})
	if statusCode != http.StatusOK {
		t.Fatalf("profile update status=%d body=%s", statusCode, body)
	}
	if updatedA.GetVersion() != memberA.GetVersion()+1 {
		t.Fatalf("profile version=%d want=%d", updatedA.GetVersion(), memberA.GetVersion()+1)
	}
	if updatedA.GetName() != "Alice Chen" || updatedA.GetPhone() != "+886900000001" || updatedA.GetEmployeeId() != "EMP-1001" || updatedA.GetPosition() != "Customer Success Lead" || updatedA.GetDepartmentId() != "dept-success" {
		t.Fatalf("profile was not normalized/persisted: %+v", updatedA)
	}

	observedB, statusCode, body := getB123HTTP(t, base, tokenB, memberB.GetUserId())
	if statusCode != http.StatusOK {
		t.Fatalf("tenant B get status=%d body=%s", statusCode, body)
	}
	if observedB.GetName() != "" || observedB.GetPhone() != "" || observedB.GetEmployeeId() != "" || observedB.GetPosition() != "" || observedB.GetDepartmentId() != "" {
		t.Fatalf("tenant A profile leaked into tenant B: %+v", observedB)
	}

	activeRoleID := tenantA + ":site-operator"
	disabledRoleID := tenantA + ":legacy-all"
	exec := func(query string, args ...any) {
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", activeRoleID, tenantA, "site-operator", "active", 1)
	exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", disabledRoleID, tenantA, "legacy-all", "disabled", 1)
	exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantA, memberA.GetUserId(), activeRoleID)
	exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantA, memberA.GetUserId(), disabledRoleID)
	exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantA, activeRoleID, "customer.read", "self")
	exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantA, activeRoleID, "site.read", "sites")
	exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantA, disabledRoleID, "tenant.admin", "all")

	readbackA, statusCode, body := getB123HTTP(t, base, tokenA, memberA.GetUserId())
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A readback status=%d body=%s", statusCode, body)
	}
	roles := roleNames(readbackA)
	if roles["site-operator"] != "active" || roles["legacy-all"] != "disabled" {
		t.Fatalf("role binding summary=%v", roles)
	}
	if readbackA.GetDerivedDataScope() != "sites" {
		t.Fatalf("derived scope=%q want=sites; disabled all-scope role must not elevate", readbackA.GetDerivedDataScope())
	}

	_, statusCode, body = updateB123ProfileHTTP(t, base, tokenA, "ec-ri-02-profile-stale:"+stamp, &accessv1.UpdateTenantMemberProfileRequest{
		UserId:  memberA.GetUserId(),
		Name:    "stale-write",
		Version: memberA.GetVersion(),
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("stale profile update status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}
}
