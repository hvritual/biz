//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// This is an empirical CI-environment P95, not a production load-test claim.
// Every sample verifies the same old session's next request, without a cache grace period.
func TestEnterprise179RoleRevocationLatencyP95(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()
	tenant := "role-latency-" + stamp
	adminToken := "role-latency-admin-" + stamp
	memberID, memberToken := "role-latency-member-"+stamp, "role-latency-session-"+stamp
	seedB123TenantAdmin(t, db, tenant, "admin-"+stamp, "admin-"+stamp+"@example.invalid", adminToken)
	seedB124RoleAdminPermissions(t, db, tenant)
	seedB124PlainMember(t, db, tenant, memberID, memberID+"@example.invalid", memberToken)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	connection, err := grpc.DialContext(ctx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	roles := accessv1.NewTenantRolePermissionApplicationClient(connection)
	operationContext := func(key string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+adminToken, "idempotency-key", "latency:"+stamp+":"+key)
	}
	role, err := roles.CreateTenantRole(operationContext("create"), &accessv1.CreateTenantRoleRequest{Name: "Revocation latency reader"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := roles.AssignTenantRoleMember(operationContext("assign"), &accessv1.AssignTenantRoleMemberRequest{RoleId: role.GetId(), UserId: memberID}); err != nil {
		t.Fatal(err)
	}

	const sampleCount = 20
	latencies := make([]time.Duration, 0, sampleCount)
	for sample := 0; sample < sampleCount; sample++ {
		role, err = roles.SetTenantRolePermissions(operationContext(fmt.Sprintf("grant-%d", sample)), &accessv1.SetTenantRolePermissionsRequest{
			RoleId: role.GetId(), Version: role.GetVersion(),
			Permissions: []*accessv1.PermissionGrantInput{
				{Permission: "tenant.member.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL},
				{Permission: "tenant.role.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := memberListStatusB124(t, base, memberToken); got != http.StatusOK {
			t.Fatalf("sample %d before revoke: status=%d want=200", sample, got)
		}
		begin := time.Now()
		role, err = roles.SetTenantRolePermissions(operationContext(fmt.Sprintf("revoke-%d", sample)), &accessv1.SetTenantRolePermissionsRequest{
			RoleId: role.GetId(), Version: role.GetVersion(),
			Permissions: []*accessv1.PermissionGrantInput{
				{Permission: "tenant.role.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := memberListStatusB124(t, base, memberToken); got != http.StatusForbidden {
			t.Fatalf("sample %d next request after revoke: status=%d want=403", sample, got)
		}
		elapsed := time.Since(begin)
		latencies = append(latencies, elapsed)
		if got := roleListStatus179(t, base, memberToken); got != http.StatusOK {
			t.Fatalf("sample %d retained action: status=%d want=200", sample, got)
		}
		t.Logf("REVOCATION_SAMPLE sample=%d latency_ms=%.3f next_request_status=403 retained_action_status=200", sample+1, float64(elapsed)/float64(time.Millisecond))
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	// Nearest-rank P95: ceil(0.95*N)-1 in the zero-based sorted sample.
	p95 := latencies[(95*len(latencies)+99)/100-1]
	t.Logf("REVOCATION_P95 samples=%d p95_ms=%.3f max_ms=%.3f limit_ms=60000 environment=ci_mysql", len(latencies), float64(p95)/float64(time.Millisecond), float64(latencies[len(latencies)-1])/float64(time.Millisecond))
	if p95 > time.Minute {
		t.Fatalf("revocation P95=%s exceeds PRD limit 60s; this is not an authorization grace window", p95)
	}
}
