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
	"google.golang.org/protobuf/proto"
)

func roleHTTPB124(t *testing.T, method, endpoint, token, key string, input proto.Message) (*accessv1.TenantRoleDTO, int, []byte) {
	t.Helper()
	var payload []byte
	var err error
	if input != nil {
		payload, err = protojson.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, endpoint, bytes.NewReader(payload))
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
	var role accessv1.TenantRoleDTO
	if err := protojson.Unmarshal(body, &role); err != nil {
		t.Fatal(err)
	}
	return &role, response.StatusCode, body
}

func TestB124ECIR03RoleRESTConflictsAre409AndFactsRemainAuthoritative(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "ec-ri-03-a-"+stamp, "ec-ri-03-b-"+stamp
	tokenA, tokenB := "ec-ri-03-token-a-"+stamp, "ec-ri-03-token-b-"+stamp
	adminA, adminB := "ec-ri-03-admin-a-"+stamp, "ec-ri-03-admin-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, adminA, adminA+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, adminB, adminB+"@example.invalid", tokenB)
	seedB124RoleAdminPermissions(t, db, tenantA)
	seedB124RoleAdminPermissions(t, db, tenantB)

	created, statusCode, body := roleHTTPB124(t, http.MethodPost, base+"/v1/tenant/roles", tokenA, "ec-ri-03-create:"+stamp, &accessv1.CreateTenantRoleRequest{Name: "site-reader"})
	if statusCode != http.StatusOK {
		t.Fatalf("create role status=%d body=%s", statusCode, body)
	}
	if created.GetVersion() != 1 {
		t.Fatalf("created role version=%d want=1", created.GetVersion())
	}

	_, statusCode, _ = roleHTTPB124(t, http.MethodGet, base+"/v1/tenant/roles/"+created.GetId(), tokenB, "", nil)
	if statusCode == http.StatusOK {
		t.Fatal("tenant B read tenant A role through REST")
	}

	updated, statusCode, body := roleHTTPB124(t, http.MethodPut, base+"/v1/tenant/roles/"+created.GetId()+"/permissions", tokenA, "ec-ri-03-permissions:"+stamp, &accessv1.SetTenantRolePermissionsRequest{
		RoleId: created.GetId(),
		Version: created.GetVersion(),
		Permissions: []*accessv1.PermissionGrantInput{{Permission: "tenant.member.read", Scope: accessv1.DataScope_DATA_SCOPE_SITES}},
	})
	if statusCode != http.StatusOK {
		t.Fatalf("set role permissions status=%d body=%s", statusCode, body)
	}
	if updated.GetVersion() != 2 || len(updated.GetPermissions()) != 1 || updated.GetPermissions()[0].GetScope() != accessv1.DataScope_DATA_SCOPE_SITES {
		t.Fatalf("updated role=%+v", updated)
	}

	_, statusCode, body = roleHTTPB124(t, http.MethodPut, base+"/v1/tenant/roles/"+created.GetId()+"/permissions", tokenA, "ec-ri-03-stale:"+stamp, &accessv1.SetTenantRolePermissionsRequest{
		RoleId: created.GetId(),
		Version: created.GetVersion(),
		Permissions: []*accessv1.PermissionGrantInput{{Permission: "tenant.member.manage", Scope: accessv1.DataScope_DATA_SCOPE_ALL}},
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("stale role permissions status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}

	readback, statusCode, body := roleHTTPB124(t, http.MethodGet, base+"/v1/tenant/roles/"+created.GetId(), tokenA, "", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("role readback status=%d body=%s", statusCode, body)
	}
	if readback.GetVersion() != updated.GetVersion() || len(readback.GetPermissions()) != 1 || readback.GetPermissions()[0].GetPermission() != "tenant.member.read" {
		t.Fatalf("stale write changed authoritative role: %+v", readback)
	}

	ownerRoleID := tenantA + ":owner"
	if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", ownerRoleID, tenantA, "owner", "active", 1).Error; err != nil {
		t.Fatal(err)
	}
	for _, permission := range []string{"tenant.member.manage", "tenant.member.read", "tenant.role.manage", "tenant.role.read"} {
		if err := db.Exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantA, ownerRoleID, permission, "all").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantA, adminA, ownerRoleID).Error; err != nil {
		t.Fatal(err)
	}

	_, statusCode, body = roleHTTPB124(t, http.MethodPost, base+"/v1/tenant/roles/"+ownerRoleID+"/disable", tokenA, "ec-ri-03-owner-disable:"+stamp, &accessv1.DisableTenantRoleRequest{RoleId: ownerRoleID, Version: 1})
	if statusCode != http.StatusConflict {
		t.Fatalf("owner disable status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}
	_, statusCode, body = roleHTTPB124(t, http.MethodPost, base+"/v1/tenant/roles/"+ownerRoleID+"/members/"+adminA+"/revoke", tokenA, "ec-ri-03-owner-revoke:"+stamp, &accessv1.RevokeTenantRoleMemberRequest{RoleId: ownerRoleID, UserId: adminA})
	if statusCode != http.StatusConflict {
		t.Fatalf("last owner revoke status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}

	ownerReadback, statusCode, body := roleHTTPB124(t, http.MethodGet, base+"/v1/tenant/roles/"+ownerRoleID, tokenA, "", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("owner readback status=%d body=%s", statusCode, body)
	}
	if ownerReadback.GetStatus() != accessv1.TenantRoleStatus_TENANT_ROLE_STATUS_ACTIVE {
		t.Fatalf("owner status=%v want active", ownerReadback.GetStatus())
	}
	var ownerAssignments int64
	if err := db.Table("biz_member_roles").Where("tenant_id = ? AND role_id = ?", tenantA, ownerRoleID).Count(&ownerAssignments).Error; err != nil {
		t.Fatal(err)
	}
	if ownerAssignments != 1 {
		t.Fatalf("owner assignments=%d want=1", ownerAssignments)
	}
}
