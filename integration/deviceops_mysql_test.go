//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/modules/deviceops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunka.io/pkg/logExt"
)

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("YUNKA_TEST_MYSQL_DSN is required")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func startModule(t *testing.T, db *gorm.DB, tenant, ownerToken, site string) *deviceops.Module {
	t.Helper()
	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	config.Bootstrap = deviceops.BootstrapConfig{
		TenantID: tenant, TenantName: tenant, UserID: tenant + "-owner",
		Email: tenant + "-owner@example.invalid", Token: ownerToken, SiteID: site, SiteName: site,
	}
	module, err := deviceops.NewModule(deviceops.Dependencies{Config: config, Logger: logExt.NewBaseLogger(), PrimaryDatabase: db})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := module.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = module.Shutdown(shutdown)
	})
	return module
}

func postDevice(t *testing.T, base, token, site, name, serial string) map[string]any {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"siteId": site, "name": name, "serial": serial})
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/devices", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "create:"+serial)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func seedReader(t *testing.T, db *gorm.DB, tenant, user, token, site, roleID, roleName, permission, scope string) {
	t.Helper()
	exec := func(query string, args ...any) {
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT IGNORE INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(3))", user, user+"@example.invalid", "active")
	exec("INSERT IGNORE INTO biz_memberships (tenant_id,user_id,status,created_at) VALUES (?,?,?,NOW(3))", tenant, user, "active")
	exec("INSERT IGNORE INTO biz_roles (id,tenant_id,name,status) VALUES (?,?,?,?)", roleID, tenant, roleName, "active")
	exec("INSERT IGNORE INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenant, user, roleID)
	exec("INSERT IGNORE INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenant, roleID, permission, scope)
	if site != "" {
		exec("INSERT IGNORE INTO biz_member_sites (tenant_id,user_id,site_id) VALUES (?,?,?)", tenant, user, site)
	}
	exec("INSERT IGNORE INTO biz_api_tokens (token_hash,tenant_id,user_id,disabled,created_at) VALUES (?,?,?,?,NOW(3))", accesspersistence.TokenHash(token), tenant, user, false)
}

func listHTTP(t *testing.T, base, token string) []map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+"/v1/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d body=%s", resp.StatusCode, body)
	}
	var out struct {
		Devices []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out.Devices
}

func TestRESTAndGRPCShareOperationSecurityRuntime(t *testing.T) {
	db := openDB(t)
	tenant, ownerToken, site := "tenant-parity", "owner-parity-token", "site-parity"
	module := startModule(t, db, tenant, ownerToken, site)
	base := "http://" + module.HTTPAddress()
	postDevice(t, base, ownerToken, site, "A", "SN-PARITY-A")

	readerToken := "reader-parity-token"
	seedReader(t, db, tenant, "reader-parity", readerToken, site, tenant+":reader", "reader", "device.read", "sites")
	if devices := listHTTP(t, base, readerToken); len(devices) != 1 || devices[0]["serial"] != "SN-PARITY-A" {
		t.Fatalf("REST reader devices=%v", devices)
	}

	payload, _ := json.Marshal(map[string]any{"siteId": site, "name": "Denied", "serial": "SN-PARITY-DENIED"})
	denied, _ := http.NewRequest(http.MethodPost, base+"/v1/devices", bytes.NewReader(payload))
	denied.Header.Set("Authorization", "Bearer "+readerToken)
	denied.Header.Set("Content-Type", "application/json")
	deniedResp, err := (&http.Client{Timeout: 5 * time.Second}).Do(denied)
	if err != nil {
		t.Fatal(err)
	}
	_ = deniedResp.Body.Close()
	if deniedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("REST create deny status=%d", deniedResp.StatusCode)
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, module.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := deviceopsv1.NewDeviceApplicationClient(conn)
	grpcCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+readerToken)
	grpcList, err := client.ListDevices(grpcCtx, &deviceopsv1.ListDevicesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(grpcList.GetDevices()) != 1 || grpcList.GetDevices()[0].GetSerial() != "SN-PARITY-A" {
		t.Fatalf("gRPC reader devices=%v", grpcList.GetDevices())
	}
	_, err = client.CreateDevice(grpcCtx, &deviceopsv1.CreateDeviceRequest{SiteId: site, Name: "Denied", Serial: "SN-GRPC-DENIED"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("gRPC create deny err=%v code=%s", err, status.Code(err))
	}
}

func TestCrossRoleLegacyScopeCannotEscalateGrant(t *testing.T) {
	db := openDB(t)
	tenant, ownerToken, site := "tenant-scope", "owner-scope-token", "site-scope"
	module := startModule(t, db, tenant, ownerToken, site)
	base := "http://" + module.HTTPAddress()
	postDevice(t, base, ownerToken, site, "Owner", "SN-SCOPE-OWNER")
	postDevice(t, base, ownerToken, site, "Reader", "SN-SCOPE-READER")
	if err := db.Exec("UPDATE biz_deviceops_device SET created_by = ? WHERE tenant_id = ? AND serial = ?", "reader-scope", tenant, "SN-SCOPE-READER").Error; err != nil {
		t.Fatal(err)
	}

	readerToken := "reader-scope-token"
	seedReader(t, db, tenant, "reader-scope", readerToken, "", tenant+":reader", "reader", "device.read", "self")
	seedReader(t, db, tenant, "reader-scope", readerToken, "", tenant+":other", "other", "device.create", "all")

	// Simulate a stale pre-C8.5 scope row on a role that does NOT grant device.read.
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS biz_role_data_scopes (
		tenant_id varchar(64) NOT NULL,
		role_id varchar(160) NOT NULL,
		permission varchar(120) NOT NULL,
		scope varchar(16) NOT NULL,
		PRIMARY KEY (tenant_id, role_id, permission)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT IGNORE INTO biz_role_data_scopes (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenant, tenant+":other", "device.read", "all").Error; err != nil {
		t.Fatal(err)
	}

	devices := listHTTP(t, base, readerToken)
	if len(devices) != 1 || devices[0]["serial"] != "SN-SCOPE-READER" {
		t.Fatalf("cross-role scope escalation detected: devices=%v", devices)
	}
}
