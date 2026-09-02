//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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
	"github.com/hvritual/yunka.io/framework/platform"
	"github.com/hvritual/yunka.io/pkg/logExt"
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
