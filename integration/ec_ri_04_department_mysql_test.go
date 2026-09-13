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
	"gorm.io/gorm"
)

func seedECIR04OrganizationPermissions(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	roleID := tenantID + ":member-admin"
	for _, permission := range []string{"tenant.organization.read", "tenant.organization.manage"} {
		if err := db.Exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, roleID, permission, "all").Error; err != nil {
			t.Fatal(err)
		}
	}
}

func departmentHTTP(t *testing.T, method, endpoint, token, key string, input proto.Message) (*accessv1.TenantDepartmentDTO, int, []byte) {
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
	var department accessv1.TenantDepartmentDTO
	if err := protojson.Unmarshal(body, &department); err != nil {
		t.Fatal(err)
	}
	return &department, response.StatusCode, body
}

func departmentListHTTP(t *testing.T, base, token string) (*accessv1.ListTenantDepartmentsResponse, int, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, base+"/v1/tenant/departments", nil)
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
	var result accessv1.ListTenantDepartmentsResponse
	if err := protojson.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return &result, response.StatusCode, body
}

func TestECIR04TenantDepartmentRESTIsAuthoritativeAndTenantScoped(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "ec-ri-04-a-"+stamp, "ec-ri-04-b-"+stamp
	adminA, adminB := "ec-ri-04-admin-a-"+stamp, "ec-ri-04-admin-b-"+stamp
	tokenA, tokenB := "ec-ri-04-token-a-"+stamp, "ec-ri-04-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, adminA, adminA+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, adminB, adminB+"@example.invalid", tokenB)
	seedECIR04OrganizationPermissions(t, db, tenantA)
	seedECIR04OrganizationPermissions(t, db, tenantB)

	root, statusCode, body := departmentHTTP(t, http.MethodPost, base+"/v1/tenant/departments", tokenA, "ec-ri-04-root:"+stamp, &accessv1.CreateTenantDepartmentRequest{
		Name: "Operations", LeaderUserId: adminA, Email: "ops@example.invalid", Sort: 10,
	})
	if statusCode != http.StatusOK {
		t.Fatalf("create root status=%d body=%s", statusCode, body)
	}
	if root.GetDepartmentId() == "" || root.GetVersion() != 1 || root.GetStatus() != accessv1.TenantDepartmentStatus_TENANT_DEPARTMENT_STATUS_ACTIVE {
		t.Fatalf("root department=%+v", root)
	}

	child, statusCode, body := departmentHTTP(t, http.MethodPost, base+"/v1/tenant/departments", tokenA, "ec-ri-04-child:"+stamp, &accessv1.CreateTenantDepartmentRequest{
		Name: "Customer Success", ParentId: root.GetDepartmentId(), LeaderUserId: adminA, Sort: 20,
	})
	if statusCode != http.StatusOK {
		t.Fatalf("create child status=%d body=%s", statusCode, body)
	}
	if child.GetParentId() != root.GetDepartmentId() {
		t.Fatalf("child parent=%q want=%q", child.GetParentId(), root.GetDepartmentId())
	}

	otherTenantDepartment, statusCode, body := departmentHTTP(t, http.MethodPost, base+"/v1/tenant/departments", tokenB, "ec-ri-04-b-root:"+stamp, &accessv1.CreateTenantDepartmentRequest{
		Name: "Tenant B Root", LeaderUserId: adminB,
	})
	if statusCode != http.StatusOK {
		t.Fatalf("create tenant B department status=%d body=%s", statusCode, body)
	}
	if _, statusCode, _ := departmentHTTP(t, http.MethodGet, base+"/v1/tenant/departments/"+root.GetDepartmentId(), tokenB, "", nil); statusCode == http.StatusOK {
		t.Fatal("tenant B read tenant A department")
	}

	listed, statusCode, body := departmentListHTTP(t, base, tokenA)
	if statusCode != http.StatusOK {
		t.Fatalf("list departments status=%d body=%s", statusCode, body)
	}
	seen := map[string]bool{}
	for _, item := range listed.GetDepartments() {
		seen[item.GetDepartmentId()] = true
	}
	if !seen[root.GetDepartmentId()] || !seen[child.GetDepartmentId()] || seen[otherTenantDepartment.GetDepartmentId()] {
		t.Fatalf("tenant A list is not isolated: %+v", listed.GetDepartments())
	}

	_, statusCode, body = departmentHTTP(t, http.MethodPost, base+"/v1/tenant/departments", tokenA, "ec-ri-04-cross-leader:"+stamp, &accessv1.CreateTenantDepartmentRequest{
		Name: "Invalid Leader", LeaderUserId: adminB,
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("cross-tenant leader status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}

	_, statusCode, body = departmentHTTP(t, http.MethodPatch, base+"/v1/tenant/departments/"+root.GetDepartmentId(), tokenA, "ec-ri-04-cycle:"+stamp, &accessv1.UpdateTenantDepartmentRequest{
		DepartmentId: root.GetDepartmentId(), Name: root.GetName(), ParentId: child.GetDepartmentId(), LeaderUserId: adminA, Version: root.GetVersion(),
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("cycle move status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}

	updatedChild, statusCode, body := departmentHTTP(t, http.MethodPatch, base+"/v1/tenant/departments/"+child.GetDepartmentId(), tokenA, "ec-ri-04-update:"+stamp, &accessv1.UpdateTenantDepartmentRequest{
		DepartmentId: child.GetDepartmentId(), Name: "Customer Growth", ParentId: root.GetDepartmentId(), LeaderUserId: adminA, Sort: 21, Version: child.GetVersion(),
	})
	if statusCode != http.StatusOK {
		t.Fatalf("update child status=%d body=%s", statusCode, body)
	}
	if updatedChild.GetVersion() != child.GetVersion()+1 || updatedChild.GetName() != "Customer Growth" {
		t.Fatalf("updated child=%+v", updatedChild)
	}
	_, statusCode, body = departmentHTTP(t, http.MethodPatch, base+"/v1/tenant/departments/"+child.GetDepartmentId(), tokenA, "ec-ri-04-stale:"+stamp, &accessv1.UpdateTenantDepartmentRequest{
		DepartmentId: child.GetDepartmentId(), Name: "Stale Name", ParentId: root.GetDepartmentId(), LeaderUserId: adminA, Version: child.GetVersion(),
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("stale department update status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}
	readback, statusCode, body := departmentHTTP(t, http.MethodGet, base+"/v1/tenant/departments/"+child.GetDepartmentId(), tokenA, "", nil)
	if statusCode != http.StatusOK || readback.GetName() != updatedChild.GetName() || readback.GetVersion() != updatedChild.GetVersion() {
		t.Fatalf("stale write changed department: status=%d body=%s readback=%+v", statusCode, body, readback)
	}

	assignedAdmin, statusCode, body := updateB123ProfileHTTP(t, base, tokenA, "ec-ri-04-assign-admin:"+stamp, &accessv1.UpdateTenantMemberProfileRequest{
		UserId: adminA, Name: "Admin A", Position: "Director", DepartmentId: child.GetDepartmentId(), Version: 1,
	})
	if statusCode != http.StatusOK {
		t.Fatalf("assign existing member status=%d body=%s", statusCode, body)
	}
	if assignedAdmin.GetDepartmentId() != child.GetDepartmentId() {
		t.Fatalf("admin department=%q want=%q", assignedAdmin.GetDepartmentId(), child.GetDepartmentId())
	}

	disabledChild, statusCode, body := departmentHTTP(t, http.MethodPost, base+"/v1/tenant/departments/"+child.GetDepartmentId()+"/disable", tokenA, "ec-ri-04-disable:"+stamp, &accessv1.DisableTenantDepartmentRequest{
		DepartmentId: child.GetDepartmentId(), Version: updatedChild.GetVersion(),
	})
	if statusCode != http.StatusOK || disabledChild.GetStatus() != accessv1.TenantDepartmentStatus_TENANT_DEPARTMENT_STATUS_DISABLED {
		t.Fatalf("disable child status=%d body=%s department=%+v", statusCode, body, disabledChild)
	}

	// Existing historical assignment remains valid for non-organization profile edits.
	retainedAdmin, statusCode, body := updateB123ProfileHTTP(t, base, tokenA, "ec-ri-04-retained-profile:"+stamp, &accessv1.UpdateTenantMemberProfileRequest{
		UserId: adminA, Name: "Admin A Updated", Position: "Director", DepartmentId: child.GetDepartmentId(), Version: assignedAdmin.GetVersion(),
	})
	if statusCode != http.StatusOK {
		t.Fatalf("same disabled department profile edit status=%d body=%s", statusCode, body)
	}
	if retainedAdmin.GetDepartmentId() != child.GetDepartmentId() {
		t.Fatalf("historical assignment changed: %+v", retainedAdmin)
	}

	newUserID := "ec-ri-04-member-" + stamp
	if err := db.Exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(3))", newUserID, newUserID+"@example.invalid", "active").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_memberships (tenant_id,user_id,status,version,created_at,updated_at) VALUES (?,?,?,?,NOW(3),NOW(3))", tenantA, newUserID, "active", 1).Error; err != nil {
		t.Fatal(err)
	}
	_, statusCode, body = updateB123ProfileHTTP(t, base, tokenA, "ec-ri-04-new-disabled-assignment:"+stamp, &accessv1.UpdateTenantMemberProfileRequest{
		UserId: newUserID, Name: "New Member", DepartmentId: child.GetDepartmentId(), Version: 1,
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("new assignment to disabled department status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}
	memberReadback, statusCode, body := getB123HTTP(t, base, tokenA, newUserID)
	if statusCode != http.StatusOK {
		t.Fatalf("new member readback status=%d body=%s", statusCode, body)
	}
	if memberReadback.GetDepartmentId() != "" || memberReadback.GetVersion() != 1 {
		t.Fatalf("rejected assignment mutated member: %+v", memberReadback)
	}

	_, statusCode, body = updateB123ProfileHTTP(t, base, tokenA, "ec-ri-04-cross-tenant-assignment:"+stamp, &accessv1.UpdateTenantMemberProfileRequest{
		UserId: newUserID, Name: "New Member", DepartmentId: otherTenantDepartment.GetDepartmentId(), Version: 1,
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("cross-tenant department assignment status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}

	enabledChild, statusCode, body := departmentHTTP(t, http.MethodPost, base+"/v1/tenant/departments/"+child.GetDepartmentId()+"/enable", tokenA, "ec-ri-04-enable:"+stamp, &accessv1.EnableTenantDepartmentRequest{
		DepartmentId: child.GetDepartmentId(), Version: disabledChild.GetVersion(),
	})
	if statusCode != http.StatusOK || enabledChild.GetStatus() != accessv1.TenantDepartmentStatus_TENANT_DEPARTMENT_STATUS_ACTIVE {
		t.Fatalf("enable child status=%d body=%s department=%+v", statusCode, body, enabledChild)
	}
}
