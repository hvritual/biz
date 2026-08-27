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

	"github.com/hvritual/biz/internal/access/store"
	"github.com/hvritual/biz/internal/deviceops/application"
	"github.com/hvritual/biz/modules/deviceops"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/pkg/logExt"
)

func TestMultiTenantRBACAndDataScope(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("YUNKA_TEST_MYSQL_DSN is required")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	token := "owner-token"
	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	config.Bootstrap = deviceops.BootstrapConfig{TenantID: "tenant-a", TenantName: "Tenant A", UserID: "owner", Email: "owner@example.invalid", Token: token, SiteID: "site-1", SiteName: "Site 1"}
	module, err := deviceops.NewModule(deviceops.Dependencies{Config: config, Logger: logExt.NewBaseLogger(), PrimaryDatabase: db})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := module.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = module.Shutdown(shutdown)
	})
	base := "http://" + module.HTTPAddress()
	client := &http.Client{Timeout: 5 * time.Second}
	post := func(site, name, serial string) map[string]any {
		payload, _ := json.Marshal(map[string]any{"siteId": site, "name": name, "serial": serial})
		req, _ := http.NewRequest(http.MethodPost, base+"/v1/devices", bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("POST status=%d body=%s", resp.StatusCode, body)
		}
		var out map[string]any
		_ = json.Unmarshal(body, &out)
		return out
	}
	first := post("site-1", "A", "SN-A")
	if first["serial"] != "SN-A" {
		t.Fatalf("first=%v", first)
	}

	roleID := "tenant-a:reader"
	readerToken := "reader-token"
	records := []any{&store.User{ID: "reader", Email: "reader@example.invalid", Status: "active"}, &store.Membership{TenantID: "tenant-a", UserID: "reader", Status: "active"}, &store.Role{ID: roleID, TenantID: "tenant-a", Name: "reader", Status: "active"}, &store.MemberRole{TenantID: "tenant-a", UserID: "reader", RoleID: roleID}, &store.RolePermission{TenantID: "tenant-a", RoleID: roleID, Permission: application.PermissionDeviceRead}, &store.RoleDataScope{TenantID: "tenant-a", RoleID: roleID, Permission: application.PermissionDeviceRead, Scope: store.DataScopeSites}, &store.MemberSite{TenantID: "tenant-a", UserID: "reader", SiteID: "site-1"}, &store.APIToken{TokenHash: store.TokenHash(readerToken), TenantID: "tenant-a", UserID: "reader"}}
	for _, record := range records {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, base+"/v1/devices", nil)
	req.Header.Set("Authorization", "Bearer "+readerToken)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reader list status=%d body=%s", resp.StatusCode, body)
	}
	var list struct {
		Devices []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Devices) != 1 || list.Devices[0]["serial"] != "SN-A" {
		t.Fatalf("reader list=%s", body)
	}
	payload, _ := json.Marshal(map[string]any{"siteId": "site-1", "name": "Denied", "serial": "SN-D"})
	denied, _ := http.NewRequest(http.MethodPost, base+"/v1/devices", bytes.NewReader(payload))
	denied.Header.Set("Authorization", "Bearer "+readerToken)
	denied.Header.Set("Content-Type", "application/json")
	deniedResp, err := client.Do(denied)
	if err != nil {
		t.Fatal(err)
	}
	deniedBody, _ := io.ReadAll(deniedResp.Body)
	_ = deniedResp.Body.Close()
	if deniedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("reader create status=%d body=%s", deniedResp.StatusCode, deniedBody)
	}
}
