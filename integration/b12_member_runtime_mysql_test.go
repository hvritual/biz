//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/modules/deviceops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
	"yunka.io/framework/platform"
	"yunka.io/pkg/logExt"
)

func startB123Runtime(t *testing.T, db *gorm.DB) *bizruntime.Started {
	t.Helper()
	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	provider, err := platform.New(platform.Options{
		Config: bizruntime.ConfigProvider{DeviceOps: config},
		Logger: logExt.NewBaseLogger(),
		Databases: map[string]platform.DatabaseFactory{
			"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
				return platform.BorrowedDatabase(db), nil
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started, err := bizruntime.Bootstrap(ctx, provider, config)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = started.App.Shutdown(shutdown)
	})
	return started
}

func seedB123TenantAdmin(t *testing.T, db *gorm.DB, tenantID, userID, email, rawToken string) {
	defer ce05LegacyFixtureGrants(t, db, tenantID, []string{"access-management"})
	t.Helper()
	roleID := tenantID + ":member-admin"
	exec := func(query string, args ...any) {
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO biz_tenants (id,name,status,version,created_at,updated_at) VALUES (?,?,?,?,NOW(3),NOW(3))", tenantID, tenantID, "active", 1)
	exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(3))", userID, email, "active")
	exec("INSERT INTO biz_memberships (tenant_id,user_id,status,version,created_at,updated_at) VALUES (?,?,?,?,NOW(3),NOW(3))", tenantID, userID, "active", 1)
	exec("INSERT INTO biz_roles (id,tenant_id,name,status) VALUES (?,?,?,?)", roleID, tenantID, "member-admin", "active")
	exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantID, userID, roleID)
	exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, roleID, "tenant.member.read", "all")
	exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, roleID, "tenant.member.manage", "all")
	exec("INSERT INTO biz_api_tokens (token_hash,tenant_id,user_id,disabled,created_at) VALUES (?,?,?,?,NOW(3))", accesspersistence.TokenHash(rawToken), tenantID, userID, false)
}

func inviteB123HTTP(t *testing.T, base, token, email, key string) (*accessv1.TenantMemberDTO, int, []byte) {
	t.Helper()
	payload, err := protojson.Marshal(&accessv1.InviteTenantMemberRequest{Email: email})
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodPost, base+"/v1/tenant/members", bytes.NewReader(payload))
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

func getB123HTTP(t *testing.T, base, token, userID string) (*accessv1.TenantMemberDTO, int, []byte) {
	t.Helper()
	request, _ := http.NewRequest(http.MethodGet, base+"/v1/tenant/members/"+userID, nil)
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
	var member accessv1.TenantMemberDTO
	if err := protojson.Unmarshal(body, &member); err != nil {
		t.Fatal(err)
	}
	return &member, response.StatusCode, body
}

func listB123HTTP(t *testing.T, base, token string, values url.Values) (*accessv1.ListTenantMembersResponse, int, []byte) {
	t.Helper()
	endpoint := base + "/v1/tenant/members"
	if encoded := values.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	request, _ := http.NewRequest(http.MethodGet, endpoint, nil)
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
	var result accessv1.ListTenantMembersResponse
	if err := protojson.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return &result, response.StatusCode, body
}


func TestB123TenantMemberLifecycleIsTenantScopedAcrossRESTAndGRPC(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "b123-a-"+stamp, "b123-b-"+stamp
	tokenA, tokenB := "b123-token-a-"+stamp, "b123-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, "admin-a-"+stamp, "admin-a-"+stamp+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, "admin-b-"+stamp, "admin-b-"+stamp+"@example.invalid", tokenB)

	sharedEmail := "shared-" + stamp + "@example.invalid"

	// PB idempotency is enforced before member Application execution.
	_, statusCode, _ := inviteB123HTTP(t, base, tokenA, sharedEmail, "")
	if statusCode != http.StatusBadRequest {
		t.Fatalf("invite without idempotency status=%d want=%d", statusCode, http.StatusBadRequest)
	}

	memberA, statusCode, body := inviteB123HTTP(t, base, tokenA, sharedEmail, "invite-a:"+stamp)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A invite status=%d body=%s", statusCode, body)
	}
	memberB, statusCode, body := inviteB123HTTP(t, base, tokenB, sharedEmail, "invite-b:"+stamp)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant B invite status=%d body=%s", statusCode, body)
	}
	if memberA.GetUserId() == "" || memberA.GetUserId() != memberB.GetUserId() {
		t.Fatalf("global identity was not reused: A=%q B=%q", memberA.GetUserId(), memberB.GetUserId())
	}
	if memberA.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_INVITED || memberB.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_INVITED {
		t.Fatalf("unexpected invited statuses: A=%s B=%s", memberA.GetStatus(), memberB.GetStatus())
	}

	// An A-only member cannot be discovered from B even when B knows the user id.
	aOnly, statusCode, body := inviteB123HTTP(t, base, tokenA, "a-only-"+stamp+"@example.invalid", "invite-a-only:"+stamp)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A-only invite status=%d body=%s", statusCode, body)
	}
	if _, statusCode, _ := getB123HTTP(t, base, tokenB, aOnly.GetUserId()); statusCode == http.StatusOK {
		t.Fatalf("tenant B read tenant A-only member user=%s", aOnly.GetUserId())
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(dialCtx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := accessv1.NewTenantMemberLifecycleApplicationClient(connection)

	ctxA := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tokenA, "idempotency-key", "activate-a:"+stamp)
	activeA, err := client.ActivateTenantMember(ctxA, &accessv1.ActivateTenantMemberRequest{UserId: memberA.GetUserId(), Version: memberA.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	if activeA.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE || activeA.GetVersion() != 2 {
		t.Fatalf("active A=%+v", activeA)
	}

	// B has the same global User but an independent Membership row/version.
	observedB, statusCode, body := getB123HTTP(t, base, tokenB, memberB.GetUserId())
	if statusCode != http.StatusOK {
		t.Fatalf("tenant B get status=%d body=%s", statusCode, body)
	}
	if observedB.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_INVITED || observedB.GetVersion() != 1 {
		t.Fatalf("tenant B membership mutated by tenant A: %+v", observedB)
	}

	// Stale optimistic version cannot suspend A.
	staleCtxA := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tokenA, "idempotency-key", "suspend-a-stale:"+stamp)
	if _, err := client.SuspendTenantMember(staleCtxA, &accessv1.SuspendTenantMemberRequest{UserId: memberA.GetUserId(), Version: 1}); err == nil {
		t.Fatal("stale member version unexpectedly succeeded")
	}

	validCtxA := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+tokenA, "idempotency-key", "suspend-a:"+stamp)
	suspendedA, err := client.SuspendTenantMember(validCtxA, &accessv1.SuspendTenantMemberRequest{UserId: memberA.GetUserId(), Version: activeA.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	if suspendedA.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_SUSPENDED || suspendedA.GetVersion() != 3 {
		t.Fatalf("suspended A=%+v", suspendedA)
	}

	// A credential tied to the suspended membership becomes invalid immediately.
	memberToken := "b123-member-token-" + stamp
	if err := db.Exec("INSERT INTO biz_api_tokens (token_hash,tenant_id,user_id,disabled,created_at) VALUES (?,?,?,?,NOW(3))", accesspersistence.TokenHash(memberToken), tenantA, memberA.GetUserId(), false).Error; err != nil {
		t.Fatal(err)
	}
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(context.Background(), memberToken); err == nil {
		t.Fatal("suspended tenant member credential unexpectedly authenticated")
	}

	observedBAfter, statusCode, body := getB123HTTP(t, base, tokenB, memberB.GetUserId())
	if statusCode != http.StatusOK {
		t.Fatalf("tenant B get after A suspension status=%d body=%s", statusCode, body)
	}
	if observedBAfter.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_INVITED || observedBAfter.GetVersion() != 1 {
		t.Fatalf("tenant B membership changed after A suspension: %+v", observedBAfter)
	}
}


func TestB123Enterprise176MemberListFiltersPaginationAndTenantIsolation(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "e176-a-"+stamp, "e176-b-"+stamp
	tokenA, tokenB := "e176-token-a-"+stamp, "e176-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, "e176-admin-a-"+stamp, "admin-a-"+stamp+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, "e176-admin-b-"+stamp, "admin-b-"+stamp+"@example.invalid", tokenB)

	roleID := tenantA + ":query-role"
	if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", roleID, tenantA, "query-role", "active", 1).Error; err != nil {
		t.Fatal(err)
	}

	memberIDs := make([]string, 0, 12)
	emails := make([]string, 0, 12)
	for index := 0; index < 12; index++ {
		email := fmt.Sprintf("e176-member-%02d-%s@example.invalid", index, stamp)
		member, statusCode, body := inviteB123HTTP(t, base, tokenA, email, fmt.Sprintf("e176-invite-%02d:%s", index, stamp))
		if statusCode != http.StatusOK {
			t.Fatalf("invite %d status=%d body=%s", index, statusCode, body)
		}
		memberIDs = append(memberIDs, member.GetUserId())
		emails = append(emails, email)
		name := fmt.Sprintf("Member %02d", index)
		if index == 0 {
			name = "Alpha Operator"
		}
		phone := fmt.Sprintf("+4917012345%02d", index)
		departmentID := "dept-other"
		if index < 4 {
			departmentID = "dept-a"
		}
		if err := db.Table("biz_memberships").
			Where("tenant_id = ? AND user_id = ?", tenantA, member.GetUserId()).
			Updates(map[string]any{
				"name": name, "phone": phone, "employee_id": fmt.Sprintf("EMP-%04d", index), "department_id": departmentID,
			}).Error; err != nil {
			t.Fatal(err)
		}
		if index%2 == 0 {
			if err := db.Exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantA, member.GetUserId(), roleID).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := db.Table("biz_users").Where("id = ?", memberIDs[1]).Update("username", "account-176-"+stamp).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", tenantA, memberIDs[11]).Update("status", "removed").Error; err != nil {
		t.Fatal(err)
	}
	sharedB, statusCode, body := inviteB123HTTP(t, base, tokenB, "e176-b-only-"+stamp+"@example.invalid", "e176-b-invite:"+stamp)
	if statusCode != http.StatusOK || sharedB.GetUserId() == "" {
		t.Fatalf("tenant B fixture status=%d body=%s", statusCode, body)
	}

	first, statusCode, body := listB123HTTP(t, base, tokenA, url.Values{"page": {"1"}, "page_size": {"5"}})
	if statusCode != http.StatusOK {
		t.Fatalf("page 1 status=%d body=%s", statusCode, body)
	}
	if first.GetTotal() != 12 || len(first.GetMembers()) != 5 {
		t.Fatalf("page 1 total=%d members=%d want total=12 members=5", first.GetTotal(), len(first.GetMembers()))
	}
	third, statusCode, body := listB123HTTP(t, base, tokenA, url.Values{"page": {"3"}, "page_size": {"5"}})
	if statusCode != http.StatusOK || third.GetTotal() != 12 || len(third.GetMembers()) != 2 {
		t.Fatalf("page 3 status=%d total=%d members=%d body=%s", statusCode, third.GetTotal(), len(third.GetMembers()), body)
	}
	repeated, statusCode, body := listB123HTTP(t, base, tokenA, url.Values{"page": {"1"}, "page_size": {"5"}})
	if statusCode != http.StatusOK {
		t.Fatalf("repeat page status=%d body=%s", statusCode, body)
	}
	for index := range first.GetMembers() {
		if first.GetMembers()[index].GetUserId() != repeated.GetMembers()[index].GetUserId() {
			t.Fatalf("member list order is not stable: first=%v repeat=%v", first.GetMembers(), repeated.GetMembers())
		}
	}

	cases := []struct {
		name   string
		values url.Values
		total  uint64
	}{
		{name: "name", values: url.Values{"query": {"Alpha"}, "page": {"1"}, "page_size": {"10"}}, total: 1},
		{name: "account", values: url.Values{"query": {"account-176-" + stamp}, "page": {"1"}, "page_size": {"10"}}, total: 1},
		{name: "email", values: url.Values{"query": {emails[2]}, "page": {"1"}, "page_size": {"10"}}, total: 1},
		{name: "phone", values: url.Values{"query": {"+49 170 1234503"}, "page": {"1"}, "page_size": {"10"}}, total: 1},
		{name: "department", values: url.Values{"department_id": {"dept-a"}, "page": {"1"}, "page_size": {"10"}}, total: 4},
		{name: "role and status", values: url.Values{"role_id": {roleID}, "status": {"TENANT_MEMBER_STATUS_INVITED"}, "page": {"1"}, "page_size": {"10"}}, total: 6},
		{name: "removed stays excluded", values: url.Values{"status": {"TENANT_MEMBER_STATUS_REMOVED"}, "page": {"1"}, "page_size": {"10"}}, total: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, gotStatus, gotBody := listB123HTTP(t, base, tokenA, tc.values)
			if gotStatus != http.StatusOK {
				t.Fatalf("status=%d body=%s", gotStatus, gotBody)
			}
			if result.GetTotal() != tc.total || uint64(len(result.GetMembers())) != tc.total {
				t.Fatalf("total=%d members=%d want=%d", result.GetTotal(), len(result.GetMembers()), tc.total)
			}
		})
	}

	crossTenant, statusCode, body := listB123HTTP(t, base, tokenB, url.Values{"query": {emails[2]}, "page": {"1"}, "page_size": {"10"}})
	if statusCode != http.StatusOK {
		t.Fatalf("cross tenant query status=%d body=%s", statusCode, body)
	}
	if crossTenant.GetTotal() != 0 || len(crossTenant.GetMembers()) != 0 {
		t.Fatalf("tenant B observed tenant A member: %+v", crossTenant)
	}
}
