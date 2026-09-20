//go:build integration

package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestEnterprise178RoleMetadataQueryProtectionAndDelete(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantID := "enterprise-178-" + stamp
	adminID := "enterprise-178-admin-" + stamp
	token := "enterprise-178-token-" + stamp
	seedB123TenantAdmin(t, db, tenantID, adminID, adminID+"@example.invalid", token)
	seedB124RoleAdminPermissions(t, db, tenantID)

	created, status, body := roleHTTPB124(t, http.MethodPost, base+"/v1/tenant/roles", token, "enterprise-178-create:"+stamp, &accessv1.CreateTenantRoleRequest{
		Name:        "Regional Operator",
		Description: "Owns the regional operating workflow",
	})
	if status != http.StatusOK {
		t.Fatalf("create role status=%d body=%s", status, body)
	}
	if created.GetDescription() != "Owns the regional operating workflow" || created.GetSystemRole() || created.GetRoleCode() != "" || created.GetMemberCount() != 0 {
		t.Fatalf("created role metadata=%+v", created)
	}

	_, status, body = roleHTTPB124(t, http.MethodPost, base+"/v1/tenant/roles", token, "enterprise-178-long-description:"+stamp, &accessv1.CreateTenantRoleRequest{
		Name:        "Too Long",
		Description: strings.Repeat("x", 121),
	})
	if status == http.StatusOK {
		t.Fatalf("overlong role description unexpectedly succeeded body=%s", body)
	}

	request, err := http.NewRequest(http.MethodGet, base+"/v1/tenant/roles?query="+url.QueryEscape("regional")+"&status=TENANT_ROLE_STATUS_ACTIVE", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("filtered role list status=%d", response.StatusCode)
	}
	var listed accessv1.ListTenantRolesResponse
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(payload, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.GetRoles()) != 1 || listed.GetRoles()[0].GetId() != created.GetId() {
		t.Fatalf("filtered roles=%+v", listed.GetRoles())
	}

	memberID := "enterprise-178-member-" + stamp
	seedB124PlainMember(t, db, tenantID, memberID, memberID+"@example.invalid", "enterprise-178-member-token-"+stamp)
	assigned, status, body := roleHTTPB124(t, http.MethodPost, base+"/v1/tenant/roles/"+created.GetId()+"/members", token, "enterprise-178-assign:"+stamp, &accessv1.AssignTenantRoleMemberRequest{
		RoleId: created.GetId(),
		UserId: memberID,
	})
	if status != http.StatusOK {
		t.Fatalf("assign member status=%d body=%s", status, body)
	}
	if assigned.GetMemberCount() != 1 {
		t.Fatalf("assigned member_count=%d want=1", assigned.GetMemberCount())
	}

	_, status, body = roleHTTPB124(t, http.MethodPost, base+"/v1/tenant/roles/"+created.GetId()+"/disable", token, "enterprise-178-disable-in-use:"+stamp, &accessv1.DisableTenantRoleRequest{
		RoleId:  created.GetId(),
		Version: assigned.GetVersion(),
	})
	if status == http.StatusOK {
		t.Fatalf("member-bound role disable unexpectedly succeeded body=%s", body)
	}
	_, status, body = roleHTTPB124(t, http.MethodDelete, base+"/v1/tenant/roles/"+created.GetId()+"?version="+fmt.Sprint(assigned.GetVersion()), token, "enterprise-178-delete-in-use:"+stamp, nil)
	if status == http.StatusOK {
		t.Fatalf("member-bound role delete unexpectedly succeeded body=%s", body)
	}

	unassigned, status, body := roleHTTPB124(t, http.MethodPost, base+"/v1/tenant/roles/"+created.GetId()+"/members/"+memberID+"/revoke", token, "enterprise-178-revoke:"+stamp, &accessv1.RevokeTenantRoleMemberRequest{
		RoleId: created.GetId(),
		UserId: memberID,
	})
	if status != http.StatusOK || unassigned.GetMemberCount() != 0 {
		t.Fatalf("revoke member status=%d role=%+v body=%s", status, unassigned, body)
	}

	_, status, body = roleHTTPB124(t, http.MethodDelete, base+"/v1/tenant/roles/"+created.GetId()+"?version="+fmt.Sprint(unassigned.GetVersion()), token, "enterprise-178-delete:"+stamp, nil)
	if status != http.StatusOK {
		t.Fatalf("delete unbound role status=%d body=%s", status, body)
	}
	_, status, _ = roleHTTPB124(t, http.MethodGet, base+"/v1/tenant/roles/"+created.GetId(), token, "", nil)
	if status == http.StatusOK {
		t.Fatal("deleted role remained readable")
	}
	var grants int64
	if err := db.Table("biz_permission_grants").Where("tenant_id = ? AND role_id = ?", tenantID, created.GetId()).Count(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if grants != 0 {
		t.Fatalf("deleted role grants=%d want=0", grants)
	}

	for _, reserved := range []struct {
		id   string
		name string
		code string
	}{
		{tenantID + ":owner", "owner", "tenant_owner"},
		{tenantID + ":admin", "tenant_admin", "tenant_admin"},
	} {
		if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,description,role_code,system_role,status,version) VALUES (?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE role_code=VALUES(role_code),system_role=VALUES(system_role),status=VALUES(status)", reserved.id, tenantID, reserved.name, "", reserved.code, true, "active", 1).Error; err != nil {
			t.Fatal(err)
		}
		role, status, body := roleHTTPB124(t, http.MethodGet, base+"/v1/tenant/roles/"+reserved.id, token, "", nil)
		if status != http.StatusOK || !role.GetSystemRole() || role.GetRoleCode() != reserved.code {
			t.Fatalf("reserved role read status=%d role=%+v body=%s", status, role, body)
		}
		_, status, body = roleHTTPB124(t, http.MethodPatch, base+"/v1/tenant/roles/"+reserved.id, token, "enterprise-178-update-system:"+reserved.code+":"+stamp, &accessv1.UpdateTenantRoleRequest{
			RoleId:      reserved.id,
			Name:        "renamed",
			Description: "changed",
			Version:     role.GetVersion(),
		})
		if status == http.StatusOK {
			t.Fatalf("system role update unexpectedly succeeded role=%s body=%s", reserved.code, body)
		}
		_, status, body = roleHTTPB124(t, http.MethodDelete, base+"/v1/tenant/roles/"+reserved.id+"?version="+fmt.Sprint(role.GetVersion()), token, "enterprise-178-delete-system:"+reserved.code+":"+stamp, nil)
		if status == http.StatusOK {
			t.Fatalf("system role delete unexpectedly succeeded role=%s body=%s", reserved.code, body)
		}
	}
}
