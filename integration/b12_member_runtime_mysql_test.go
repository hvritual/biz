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

func startB123Enterprise176Runtime(t *testing.T, db *gorm.DB) (*bizruntime.Started, *accesspersistence.VerificationProtection) {
	t.Helper()
	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	protection, err := accesspersistence.NewVerificationProtection(accesspersistence.VerificationProtectionConfig{
		ActiveVersion: "v1",
		Keys:          map[string][]byte{"v1": []byte(strings.Repeat("K", 32))},
		HMACKey:       []byte(strings.Repeat("H", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
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
	started, err := bizruntime.BootstrapWithOptionsAndSecurity(ctx, provider, bizruntime.Options{
		DeviceOps: config, MemberActivationTTL: 15 * time.Minute, MemberActivationURL: "http://127.0.0.1:18081/idp/member/activate",
	}, nil, protection)
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
	return started, protection
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

func createB123MemberHTTP(t *testing.T, base, token, key string, input *accessv1.CreateTenantMemberRequest) (*accessv1.TenantMemberCreationReceipt, int, []byte) {
	t.Helper()
	payload, err := protojson.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodPost, base+"/v1/tenant/members/create", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, body
	}
	var receipt accessv1.TenantMemberCreationReceipt
	if err := protojson.Unmarshal(body, &receipt); err != nil {
		t.Fatal(err)
	}
	return &receipt, response.StatusCode, body
}

func activateB123MemberHTTP(t *testing.T, base, token, key, userID string, version uint64) (int, []byte) {
	t.Helper()
	payload, err := protojson.Marshal(&accessv1.ActivateTenantMemberRequest{UserId: userID, Version: version})
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodPost, base+"/v1/tenant/members/"+userID+"/activate", bytes.NewReader(payload))
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

func TestB123Enterprise176AtomicMemberCreateRollsBackAndRequiresActivation(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started, verificationProtection := startB123Enterprise176Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "e176-create-a-"+stamp, "e176-create-b-"+stamp
	tokenA, tokenB := "e176-create-token-a-"+stamp, "e176-create-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, "e176-admin-a-"+stamp, "e176-admin-a-"+stamp+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, "e176-admin-b-"+stamp, "e176-admin-b-"+stamp+"@example.invalid", tokenB)
	for _, tenantID := range []string{tenantA, tenantB} {
		adminRoleID := tenantID + ":member-admin"
		if err := db.Exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, adminRoleID, "tenant.role.manage", "all").Error; err != nil {
			t.Fatal(err)
		}
	}
	roleA, roleB := tenantA+":operator", tenantB+":operator"
	if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", roleA, tenantA, "operator", "active", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", roleB, tenantB, "operator", "active", 1).Error; err != nil {
		t.Fatal(err)
	}

	suffix := stamp
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	username := "user176" + suffix
	email := "member-create-" + stamp + "@example.invalid"
	phone := "+49170176" + suffix
	receipt, statusCode, body := createB123MemberHTTP(t, base, tokenA, "e176-create:"+stamp, &accessv1.CreateTenantMemberRequest{
		Username:       username,
		Email:          email,
		Phone:          phone,
		Name:           "Atomic Member",
		EmployeeId:     "EMP-176",
		Position:       "Operator",
		RoleIds:        []string{roleA},
		ActivationMode: accessv1.TenantMemberActivationMode_TENANT_MEMBER_ACTIVATION_MODE_ACTIVATION_LINK,
	})
	if statusCode != http.StatusOK {
		t.Fatalf("atomic create status=%d body=%s", statusCode, body)
	}
	member := receipt.GetMember()
	if member == nil || member.GetUserId() == "" || member.GetUsername() != username || member.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_INVITED {
		t.Fatalf("unexpected creation receipt: %+v", receipt)
	}
	if receipt.GetNotificationEventId() == "" || receipt.GetDeliveryState() != "PENDING" || receipt.GetMaskedDestination() == "" {
		t.Fatalf("activation receipt does not expose honest queued state: %+v", receipt)
	}
	if strings.Contains(string(body), "password") || strings.Contains(string(body), "token=") {
		t.Fatalf("creation API leaked activation secret: %s", body)
	}
	var roleCount int64
	if err := db.Table("biz_member_roles").Where("tenant_id = ? AND user_id = ? AND role_id = ?", tenantA, member.GetUserId(), roleA).Count(&roleCount).Error; err != nil {
		t.Fatal(err)
	}
	if roleCount != 1 {
		t.Fatalf("initial role was not atomically bound: count=%d", roleCount)
	}
	var activation struct{ State, SecretHash, NotificationEventID string }
	if err := db.Table("biz_member_activations").Select("state, secret_hash, notification_event_id").
		Where("tenant_id = ? AND user_id = ?", tenantA, member.GetUserId()).Scan(&activation).Error; err != nil {
		t.Fatal(err)
	}
	if activation.State != "PENDING" || activation.SecretHash == "" || activation.NotificationEventID != receipt.GetNotificationEventId() {
		t.Fatalf("activation persistence mismatch: %+v", activation)
	}
	var notification struct{ State, DestinationCiphertext, SecretCiphertext string }
	if err := db.Table("biz_security_notification_outbox").Select("state, destination_ciphertext, secret_ciphertext").
		Where("event_id = ?", receipt.GetNotificationEventId()).Scan(&notification).Error; err != nil {
		t.Fatal(err)
	}
	if notification.State != "PENDING" || notification.DestinationCiphertext == "" || notification.SecretCiphertext == "" {
		t.Fatalf("secure activation notification not staged: %+v", notification)
	}

	statusCode, body = activateB123MemberHTTP(t, base, tokenA, "e176-admin-activate:"+stamp, member.GetUserId(), member.GetVersion())
	if statusCode != http.StatusConflict {
		t.Fatalf("admin bypassed pending activation: status=%d body=%s", statusCode, body)
	}

	verificationRepository, err := accesspersistence.NewVerificationRepository(db, verificationProtection)
	if err != nil {
		t.Fatal(err)
	}
	linkClaim, _, err := verificationRepository.ClaimSecurityNotification(context.Background(), receipt.GetNotificationEventId())
	if err != nil {
		t.Fatal(err)
	}
	linkURL, err := url.Parse(linkClaim.Secret)
	if err != nil || linkURL.Query().Get("token") == "" {
		t.Fatalf("activation notification did not contain usable absolute link: %q %v", linkClaim.Secret, err)
	}
	if linkURL.Scheme != "http" || linkURL.Host == "" || linkURL.Path != "/idp/member/activate" {
		t.Fatalf("unexpected activation URL: %s", linkURL)
	}
	if _, err := verificationRepository.CompleteSecurityNotification(context.Background(), linkClaim, "qualification:"+linkClaim.EventID, "", true); err != nil {
		t.Fatal(err)
	}
	memberStore, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	activationView, err := memberStore.CompleteMemberActivationLink(
		context.Background(), linkURL.Query().Get("token"), "MemberA7x", "MemberA7x",
	)
	if err != nil {
		t.Fatal(err)
	}
	if activationView.UserID != member.GetUserId() {
		t.Fatalf("activation completed wrong account: %+v", activationView)
	}
	activeMember, statusCode, body := getB123HTTP(t, base, tokenA, member.GetUserId())
	if statusCode != http.StatusOK || activeMember.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE {
		t.Fatalf("activation link did not activate membership: status=%d body=%s member=%+v", statusCode, body, activeMember)
	}
	if _, err := memberStore.CompleteMemberActivationLink(
		context.Background(), linkURL.Query().Get("token"), "MemberA7x", "MemberA7x",
	); !errors.Is(err, accesspersistence.ErrMemberActivationConsumed) {
		t.Fatalf("activation link replay accepted: %v", err)
	}

	badUsername := "bad176" + suffix
	_, statusCode, _ = createB123MemberHTTP(t, base, tokenA, "e176-bad-role:"+stamp, &accessv1.CreateTenantMemberRequest{
		Username:       badUsername,
		Email:          "bad-role-" + stamp + "@example.invalid",
		Name:           "Rollback Member",
		RoleIds:        []string{tenantA + ":missing-role"},
		ActivationMode: accessv1.TenantMemberActivationMode_TENANT_MEMBER_ACTIVATION_MODE_ACTIVATION_LINK,
	})
	if statusCode == http.StatusOK {
		t.Fatal("create with missing role unexpectedly succeeded")
	}
	var leakedUsers int64
	if err := db.Table("biz_users").Where("username = ?", badUsername).Count(&leakedUsers).Error; err != nil {
		t.Fatal(err)
	}
	if leakedUsers != 0 {
		t.Fatalf("failed atomic create leaked account rows: %d", leakedUsers)
	}

	_, statusCode, _ = createB123MemberHTTP(t, base, tokenA, "e176-duplicate-username:"+stamp, &accessv1.CreateTenantMemberRequest{
		Username:       username,
		Email:          "different-" + stamp + "@example.invalid",
		Name:           "Duplicate Username",
		RoleIds:        []string{roleA},
		ActivationMode: accessv1.TenantMemberActivationMode_TENANT_MEMBER_ACTIVATION_MODE_ACTIVATION_LINK,
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("duplicate global username status=%d want=%d", statusCode, http.StatusConflict)
	}

	_, statusCode, body = createB123MemberHTTP(t, base, tokenB, "e176-existing-account-sms:"+stamp, &accessv1.CreateTenantMemberRequest{
		Username:       username,
		Email:          email,
		Phone:          phone,
		Name:           "Shared Account Tenant B",
		RoleIds:        []string{roleB},
		ActivationMode: accessv1.TenantMemberActivationMode_TENANT_MEMBER_ACTIVATION_MODE_SMS_INITIAL_PASSWORD,
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("existing shared account SMS status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}
	var tenantBMembership int64
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", tenantB, member.GetUserId()).Count(&tenantBMembership).Error; err != nil {
		t.Fatal(err)
	}
	if tenantBMembership != 0 {
		t.Fatalf("rejected existing-account SMS left partial tenant membership: %d", tenantBMembership)
	}

	smsUsername := "sms176" + suffix
	smsPhone := "+49176176" + suffix
	smsReceipt, statusCode, body := createB123MemberHTTP(t, base, tokenB, "e176-sms-new:"+stamp, &accessv1.CreateTenantMemberRequest{
		Username:       smsUsername,
		Phone:          smsPhone,
		Name:           "SMS Member",
		RoleIds:        []string{roleB},
		ActivationMode: accessv1.TenantMemberActivationMode_TENANT_MEMBER_ACTIVATION_MODE_SMS_INITIAL_PASSWORD,
	})
	if statusCode != http.StatusOK {
		t.Fatalf("new-account SMS create status=%d body=%s", statusCode, body)
	}
	smsClaim, _, err := verificationRepository.ClaimSecurityNotification(context.Background(), smsReceipt.GetNotificationEventId())
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(smsClaim.Secret, "\n")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "username=") || !strings.HasPrefix(parts[1], "password=") {
		t.Fatalf("unexpected SMS initial credential payload shape: %q", smsClaim.Secret)
	}
	initialPassword := strings.TrimPrefix(parts[1], "password=")
	if strings.TrimPrefix(parts[0], "username=") != smsUsername || initialPassword == "" {
		t.Fatalf("SMS credential payload does not bind username: %q", smsClaim.Secret)
	}
	if _, err := verificationRepository.CompleteSecurityNotification(context.Background(), smsClaim, "qualification:"+smsClaim.EventID, "", true); err != nil {
		t.Fatal(err)
	}
	identity, err := memberStore.AuthenticateUserPassword(context.Background(), smsUsername, initialPassword)
	if err != nil || !identity.PasswordChangeRequired || identity.UserID != smsReceipt.GetMember().GetUserId() {
		t.Fatalf("initial password did not require forced change: identity=%+v err=%v", identity, err)
	}
	smsBefore, statusCode, body := getB123HTTP(t, base, tokenB, smsReceipt.GetMember().GetUserId())
	if statusCode != http.StatusOK || smsBefore.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_INVITED {
		t.Fatalf("temporary password activated membership early: status=%d body=%s member=%+v", statusCode, body, smsBefore)
	}

	requestID, browserSecret, csrf, err := memberStore.CreateFirstPartyAuthorizationRequest(context.Background(), accesspersistence.FirstPartyAuthorizationRequestInput{
		ClientID:      "biz-web",
		RedirectURI:   "http://127.0.0.1:18080/auth/callback",
		State:         "sms-state-" + stamp,
		Nonce:         "sms-nonce-" + stamp,
		CodeChallenge: "sms-code-challenge-" + stamp,
		Scope:         "openid profile",
	}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	verifiedIdentity, auditID, err := memberStore.AuthenticateFirstPartyLoginWithAudit(
		context.Background(), smsUsername, initialPassword, "127.0.0.1:12345", accesspersistence.DefaultFirstPartyLoginPolicy(),
	)
	if err != nil || !verifiedIdentity.PasswordChangeRequired {
		t.Fatalf("first-party login did not enter forced-change state: identity=%+v audit=%d err=%v", verifiedIdentity, auditID, err)
	}
	if err := memberStore.BindFirstPartyAuthorizationIdentity(
		context.Background(), requestID, browserSecret, csrf, verifiedIdentity.UserID, auditID,
	); err != nil {
		t.Fatal(err)
	}
	completedIdentity, _, err := memberStore.CompleteInitialPasswordActivationForLogin(
		context.Background(), requestID, browserSecret, csrf, "FinalA7x9", "FinalA7x9",
	)
	if err != nil || completedIdentity.UserID != verifiedIdentity.UserID {
		t.Fatalf("forced password change failed: identity=%+v err=%v", completedIdentity, err)
	}
	if _, err := memberStore.AuthenticateUserPassword(context.Background(), smsUsername, initialPassword); !errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {
		t.Fatalf("one-time initial password remained valid: %v", err)
	}
	finalIdentity, err := memberStore.AuthenticateUserPassword(context.Background(), smsUsername, "FinalA7x9")
	if err != nil || finalIdentity.PasswordChangeRequired {
		t.Fatalf("final password not authoritative after forced change: identity=%+v err=%v", finalIdentity, err)
	}
	smsAfter, statusCode, body := getB123HTTP(t, base, tokenB, smsReceipt.GetMember().GetUserId())
	if statusCode != http.StatusOK || smsAfter.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE {
		t.Fatalf("forced password change did not activate membership: status=%d body=%s member=%+v", statusCode, body, smsAfter)
	}
}
