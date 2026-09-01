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
	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/modules/deviceops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
	"yunka.io/framework/platform"
	"yunka.io/gateway/authz"
	"yunka.io/pkg/logExt"
)

func startB122Runtime(t *testing.T, db *gorm.DB, token string) *bizruntime.Started {
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
	started, err := bizruntime.BootstrapWithOptions(ctx, provider, bizruntime.Options{
		DeviceOps: config,
		PlatformBootstrap: bizruntime.PlatformBootstrap{
			Subject: "platform-admin:b12",
			Token:   token,
			Permissions: []authz.PermissionKey{
				"platform.tenant.create", "platform.tenant.read", "platform.tenant.manage",
			},
		},
	})
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

func TestB122TenantLifecycleRESTAndGRPCUseUnifiedExecutor(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	token := "platform-b12-" + stamp
	started := startB122Runtime(t, db, token)
	base := "http://" + started.HTTPAddress()

	payload, err := protojson.Marshal(&accessv1.CreateTenantRequest{
		Name:        "Runtime Tenant " + stamp,
		OwnerUserId: "owner-" + stamp,
		OwnerEmail:  "owner-" + stamp + "@example.invalid",
	})
	if err != nil {
		t.Fatal(err)
	}

	// The PB-declared idempotency policy is enforced by the real REST adapter
	// and Executor before Application code runs.
	missingKey, _ := http.NewRequest(http.MethodPost, base+"/v1/tenants", bytes.NewReader(payload))
	missingKey.Header.Set("Authorization", "Bearer "+token)
	missingKey.Header.Set("Content-Type", "application/json")
	missingResp, err := (&http.Client{Timeout: 5 * time.Second}).Do(missingKey)
	if err != nil {
		t.Fatal(err)
	}
	missingBody, _ := io.ReadAll(missingResp.Body)
	_ = missingResp.Body.Close()
	if missingResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing idempotency status=%d body=%s", missingResp.StatusCode, missingBody)
	}

	create, _ := http.NewRequest(http.MethodPost, base+"/v1/tenants", bytes.NewReader(payload))
	create.Header.Set("Authorization", "Bearer "+token)
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set("Idempotency-Key", "tenant-create:"+stamp)
	createResp, err := (&http.Client{Timeout: 5 * time.Second}).Do(create)
	if err != nil {
		t.Fatal(err)
	}
	createBody, _ := io.ReadAll(createResp.Body)
	_ = createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create status=%d body=%s", createResp.StatusCode, createBody)
	}
	var created accessv1.TenantDTO
	if err := protojson.Unmarshal(createBody, &created); err != nil {
		t.Fatal(err)
	}
	if created.GetId() == "" || created.GetStatus() != accessv1.TenantStatus_TENANT_STATUS_PENDING || created.GetVersion() != 1 {
		t.Fatalf("created=%+v", created)
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := accessv1.NewTenantLifecycleApplicationClient(conn)
	grpcCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token, "idempotency-key", "tenant-activate:"+stamp)
	active, err := client.ActivateTenant(grpcCtx, &accessv1.ActivateTenantRequest{Id: created.GetId(), Version: created.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	if active.GetStatus() != accessv1.TenantStatus_TENANT_STATUS_ACTIVE || active.GetVersion() != 2 {
		t.Fatalf("active=%+v", active)
	}

	get, _ := http.NewRequest(http.MethodGet, base+"/v1/tenants/"+created.GetId(), nil)
	get.Header.Set("Authorization", "Bearer "+token)
	getResp, err := (&http.Client{Timeout: 5 * time.Second}).Do(get)
	if err != nil {
		t.Fatal(err)
	}
	getBody, _ := io.ReadAll(getResp.Body)
	_ = getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getResp.StatusCode, getBody)
	}
	var observed accessv1.TenantDTO
	if err := protojson.Unmarshal(getBody, &observed); err != nil {
		t.Fatal(err)
	}
	if observed.GetStatus() != accessv1.TenantStatus_TENANT_STATUS_ACTIVE || observed.GetVersion() != 2 {
		t.Fatalf("observed=%+v", observed)
	}

	// A reused completed idempotency key cannot execute the mutation twice.
	_, err = client.ActivateTenant(grpcCtx, &accessv1.ActivateTenantRequest{Id: created.GetId(), Version: created.GetVersion()})
	if err == nil {
		t.Fatal("reused completed idempotency key unexpectedly executed")
	}
}
