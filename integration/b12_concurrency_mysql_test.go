//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type b126HTTPResult struct {
	status int
	body   []byte
	err    error
}

func b126PostProto(base, path, token, idempotencyKey string, message proto.Message) b126HTTPResult {
	payload, err := protojson.Marshal(message)
	if err != nil {
		return b126HTTPResult{err: err}
	}
	request, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(payload))
	if err != nil {
		return b126HTTPResult{err: err}
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return b126HTTPResult{err: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	return b126HTTPResult{status: response.StatusCode, body: body, err: err}
}

func TestB126ConcurrentSameEmailInviteConvergesToOneGlobalUser(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "b126-email-a-"+stamp, "b126-email-b-"+stamp
	tokenA, tokenB := "b126-email-token-a-"+stamp, "b126-email-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, "b126-email-admin-a-"+stamp, "b126-email-admin-a-"+stamp+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, "b126-email-admin-b-"+stamp, "b126-email-admin-b-"+stamp+"@example.invalid", tokenB)
	sharedEmail := "b126-shared-" + stamp + "@example.invalid"

	start := make(chan struct{})
	results := make(chan b126HTTPResult, 2)
	var wg sync.WaitGroup
	for _, call := range []struct{ token, key string }{{tokenA, "b126-invite-a:" + stamp}, {tokenB, "b126-invite-b:" + stamp}} {
		wg.Add(1)
		go func(token, key string) {
			defer wg.Done()
			<-start
			results <- b126PostProto(base, "/v1/tenant/members", token, key, &accessv1.InviteTenantMemberRequest{Email: sharedEmail})
		}(call.token, call.key)
	}
	close(start)
	wg.Wait()
	close(results)

	userIDs := make([]string, 0, 2)
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.status != http.StatusOK {
			t.Fatalf("concurrent invite status=%d body=%s", result.status, result.body)
		}
		member := &accessv1.TenantMemberDTO{}
		if err := protojson.Unmarshal(result.body, member); err != nil {
			t.Fatal(err)
		}
		userIDs = append(userIDs, member.GetUserId())
	}
	if len(userIDs) != 2 || userIDs[0] == "" || userIDs[0] != userIDs[1] {
		t.Fatalf("global identity did not converge userIDs=%v", userIDs)
	}
	var users, memberships int64
	if err := db.Table("biz_users").Where("email = ?", sharedEmail).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id IN ? AND user_id = ?", []string{tenantA, tenantB}, userIDs[0]).Count(&memberships).Error; err != nil {
		t.Fatal(err)
	}
	if users != 1 || memberships != 2 {
		t.Fatalf("concurrent identity rows users=%d memberships=%d", users, memberships)
	}
}

func TestB126ConcurrentPermissionReplacementHasSingleWinnerAndNoMixedSet(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	tenantID := "b126-role-" + stamp
	adminToken := "b126-role-admin-token-" + stamp
	seedB123TenantAdmin(t, db, tenantID, "b126-role-admin-"+stamp, "b126-role-admin-"+stamp+"@example.invalid", adminToken)
	seedB124RoleAdminPermissions(t, db, tenantID)

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(dialCtx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	roles := accessv1.NewTenantRolePermissionApplicationClient(connection)
	createCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+adminToken, "idempotency-key", "b126-role-create:"+stamp)
	role, err := roles.CreateTenantRole(createCtx, &accessv1.CreateTenantRoleRequest{Name: "concurrent-role"})
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		role *accessv1.TenantRoleDTO
		err  error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	sets := [][]*accessv1.PermissionGrantInput{
		{{Permission: "tenant.member.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL}},
		{{Permission: "tenant.member.manage", Scope: accessv1.DataScope_DATA_SCOPE_SELF}},
	}
	for index, set := range sets {
		go func(index int, set []*accessv1.PermissionGrantInput) {
			<-start
			ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+adminToken, "idempotency-key", fmt.Sprintf("b126-role-set-%d:%s", index, stamp))
			updated, err := roles.SetTenantRolePermissions(ctx, &accessv1.SetTenantRolePermissionsRequest{RoleId: role.GetId(), Version: role.GetVersion(), Permissions: set})
			results <- result{role: updated, err: err}
		}(index, set)
	}
	close(start)
	first, second := <-results, <-results
	successes := 0
	for _, value := range []result{first, second} {
		if value.err == nil {
			successes++
			if value.role.GetVersion() != role.GetVersion()+1 {
				t.Fatalf("winner version=%d", value.role.GetVersion())
			}
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent permission replacements successes=%d errors=(%v,%v)", successes, first.err, second.err)
	}

	type grantRow struct{ Permission, Scope string }
	var rows []grantRow
	if err := db.Table("biz_permission_grants").Select("permission, scope").Where("tenant_id = ? AND role_id = ?", tenantID, role.GetId()).Order("permission").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("final grant rows=%v", rows)
	}
	valid := (rows[0].Permission == "tenant.member.read" && rows[0].Scope == "all") || (rows[0].Permission == "tenant.member.manage" && rows[0].Scope == "self")
	if !valid {
		t.Fatalf("final grant set is mixed/unknown: %+v", rows)
	}
}

func TestB126ConcurrentOwnerRevokesCannotRemoveAllOwners(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	tenantID := "b126-owner-" + stamp
	ownerA, ownerB := "b126-owner-a-"+stamp, "b126-owner-b-"+stamp
	adminToken := "b126-owner-token-" + stamp
	seedB123TenantAdmin(t, db, tenantID, ownerA, ownerA+"@example.invalid", adminToken)
	seedB124RoleAdminPermissions(t, db, tenantID)
	seedB124PlainMember(t, db, tenantID, ownerB, ownerB+"@example.invalid", "b126-owner-b-token-"+stamp)
	ownerRoleID := tenantID + ":owner"
	if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", ownerRoleID, tenantID, domain.TenantOwnerRoleName, domain.TenantRoleStatusActive, 1).Error; err != nil {
		t.Fatal(err)
	}
	for _, permission := range domain.OwnerRequiredPermissions {
		if err := db.Exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, ownerRoleID, permission, "all").Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, userID := range []string{ownerA, ownerB} {
		if err := db.Exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantID, userID, ownerRoleID).Error; err != nil {
			t.Fatal(err)
		}
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(dialCtx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	roles := accessv1.NewTenantRolePermissionApplicationClient(connection)

	start := make(chan struct{})
	results := make(chan error, 2)
	for index, userID := range []string{ownerA, ownerB} {
		go func(index int, userID string) {
			<-start
			ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+adminToken, "idempotency-key", fmt.Sprintf("b126-owner-revoke-%d:%s", index, stamp))
			_, err := roles.RevokeTenantRoleMember(ctx, &accessv1.RevokeTenantRoleMemberRequest{RoleId: ownerRoleID, UserId: userID})
			results <- err
		}(index, userID)
	}
	close(start)
	errs := []error{<-results, <-results}
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent owner revokes successes=%d errors=%v", successes, errs)
	}
	var assignments int64
	if err := db.Table("biz_member_roles").Where("tenant_id = ? AND role_id = ?", tenantID, ownerRoleID).Count(&assignments).Error; err != nil {
		t.Fatal(err)
	}
	if assignments != 1 {
		t.Fatalf("owner assignments after race=%d want=1", assignments)
	}
}

func TestB126ConcurrentTenantCreateSameIdempotencyKeyCreatesOneBootstrapTree(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	platformToken := "b126-platform-" + stamp
	started := startB122Runtime(t, db, platformToken)
	base := "http://" + started.HTTPAddress()
	name := "B126 Idempotent Tenant " + stamp
	ownerUserID := "b126-idem-owner-" + stamp
	ownerEmail := ownerUserID + "@example.invalid"
	key := "b126-tenant-create:" + stamp
	message := &accessv1.CreateTenantRequest{Name: name, OwnerUserId: ownerUserID, OwnerEmail: ownerEmail}

	start := make(chan struct{})
	results := make(chan b126HTTPResult, 2)
	for range 2 {
		go func() {
			<-start
			results <- b126PostProto(base, "/v1/tenants", platformToken, key, message)
		}()
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent tenant create transport errors=(%v,%v)", first.err, second.err)
	}
	statuses := []int{first.status, second.status}
	sort.Ints(statuses)
	if statuses[1] != http.StatusOK || (statuses[0] != http.StatusConflict && statuses[0] != http.StatusOK) {
		t.Fatalf("concurrent tenant create statuses=%v bodies=(%s,%s)", statuses, first.body, second.body)
	}

	var tenants int64
	if err := db.Table("biz_tenants").Where("name = ?", name).Count(&tenants).Error; err != nil {
		t.Fatal(err)
	}
	if tenants != 1 {
		t.Fatalf("tenant rows for idempotent race=%d want=1", tenants)
	}
	type tenantRow struct{ ID string }
	var row tenantRow
	if err := db.Table("biz_tenants").Select("id").Where("name = ?", name).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	var members, roles, assignments int64
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", row.ID, ownerUserID).Count(&members).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_roles").Where("tenant_id = ? AND id = ?", row.ID, row.ID+":owner").Count(&roles).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_member_roles").Where("tenant_id = ? AND user_id = ? AND role_id = ?", row.ID, ownerUserID, row.ID+":owner").Count(&assignments).Error; err != nil {
		t.Fatal(err)
	}
	if members != 1 || roles != 1 || assignments != 1 {
		t.Fatalf("idempotent bootstrap tree members=%d roles=%d assignments=%d", members, roles, assignments)
	}
}
