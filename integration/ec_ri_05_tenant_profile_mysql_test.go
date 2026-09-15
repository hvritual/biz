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

func seedECIR05TenantProfilePermissions(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	roleID := tenantID + ":member-admin"
	for _, permission := range []string{"tenant.organization.read", "tenant.organization.manage"} {
		if err := db.Exec("INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)", tenantID, roleID, permission, "all").Error; err != nil {
			t.Fatal(err)
		}
	}
}

func tenantProfileHTTP(t *testing.T, method, endpoint, token, key string, input proto.Message) (*accessv1.TenantProfileDTO, int, []byte) {
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
	var profile accessv1.TenantProfileDTO
	if err := protojson.Unmarshal(body, &profile); err != nil {
		t.Fatal(err)
	}
	return &profile, response.StatusCode, body
}

func TestECIR05TenantProfileRESTIsAuthoritativeCASAndTenantScoped(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "ec-ri-05-a-"+stamp, "ec-ri-05-b-"+stamp
	adminA, adminB := "ec-ri-05-admin-a-"+stamp, "ec-ri-05-admin-b-"+stamp
	tokenA, tokenB := "ec-ri-05-token-a-"+stamp, "ec-ri-05-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, adminA, adminA+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, adminB, adminB+"@example.invalid", tokenB)
	seedECIR05TenantProfilePermissions(t, db, tenantA)
	seedECIR05TenantProfilePermissions(t, db, tenantB)

	initialA, statusCode, body := tenantProfileHTTP(t, http.MethodGet, base+"/v1/tenant/profile", tokenA, "", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("initial tenant A profile status=%d body=%s", statusCode, body)
	}
	if initialA.GetTenantId() != tenantA || initialA.GetName() != tenantA || initialA.GetShortName() != tenantA || initialA.GetTimezone() != "Asia/Shanghai" || initialA.GetVersion() != 1 {
		t.Fatalf("unexpected initial tenant A profile=%+v", initialA)
	}

	_, statusCode, body = tenantProfileHTTP(t, http.MethodPatch, base+"/v1/tenant/profile", tokenA, "", &accessv1.UpdateTenantProfileRequest{
		Name: "CoffeeLink A", ShortName: "CLA", Timezone: "Asia/Taipei", Version: initialA.GetVersion(),
	})
	if statusCode != http.StatusBadRequest {
		t.Fatalf("profile update without idempotency status=%d want=%d body=%s", statusCode, http.StatusBadRequest, body)
	}

	updatedA, statusCode, body := tenantProfileHTTP(t, http.MethodPatch, base+"/v1/tenant/profile", tokenA, "ec-ri-05-update:"+stamp, &accessv1.UpdateTenantProfileRequest{
		Name:         "CoffeeLink A",
		ShortName:    "CLA",
		Industry:     "IoT SaaS",
		CompanySize:  "50–200 人",
		Timezone:     "Asia/Taipei",
		ContactName:  "Alice Chen",
		Phone:        "+886-2-1234-5678",
		Email:        "OWNER@COFFEELINK.TEST",
		Address:      "Taipei",
		Description:  "Authoritative tenant profile",
		LogoAssetRef: "asset://tenant/logo/a",
		Version:      initialA.GetVersion(),
	})
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A profile update status=%d body=%s", statusCode, body)
	}
	if updatedA.GetTenantId() != tenantA || updatedA.GetVersion() != initialA.GetVersion()+1 || updatedA.GetName() != "CoffeeLink A" || updatedA.GetEmail() != "owner@coffeelink.test" || updatedA.GetLogoAssetRef() != "asset://tenant/logo/a" {
		t.Fatalf("unexpected updated tenant A profile=%+v", updatedA)
	}

	var persisted struct {
		Name         string
		ShortName    string
		Industry     string
		CompanySize  string
		Timezone     string
		ContactName  string
		Phone        string
		Email        string
		Address      string
		Description  string
		LogoAssetRef string
		Version      uint64
	}
	if err := db.Table("biz_tenants").Select("name, short_name, industry, company_size, timezone, contact_name, phone, email, address, description, logo_asset_ref, version").Where("id = ?", tenantA).Take(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Name != updatedA.GetName() || persisted.ShortName != updatedA.GetShortName() || persisted.Timezone != updatedA.GetTimezone() || persisted.Email != updatedA.GetEmail() || persisted.LogoAssetRef != updatedA.GetLogoAssetRef() || persisted.Version != updatedA.GetVersion() {
		t.Fatalf("profile not persisted authoritatively: %+v", persisted)
	}

	_, statusCode, body = tenantProfileHTTP(t, http.MethodPatch, base+"/v1/tenant/profile", tokenA, "ec-ri-05-stale:"+stamp, &accessv1.UpdateTenantProfileRequest{
		Name: "Stale Name", ShortName: "STALE", Timezone: "UTC", Version: initialA.GetVersion(),
	})
	if statusCode != http.StatusConflict {
		t.Fatalf("stale profile update status=%d want=%d body=%s", statusCode, http.StatusConflict, body)
	}
	readbackA, statusCode, body := tenantProfileHTTP(t, http.MethodGet, base+"/v1/tenant/profile", tokenA, "", nil)
	if statusCode != http.StatusOK || readbackA.GetName() != updatedA.GetName() || readbackA.GetVersion() != updatedA.GetVersion() {
		t.Fatalf("stale write changed tenant A profile: status=%d body=%s readback=%+v", statusCode, body, readbackA)
	}

	_, statusCode, body = tenantProfileHTTP(t, http.MethodPatch, base+"/v1/tenant/profile", tokenA, "ec-ri-05-data-url:"+stamp, &accessv1.UpdateTenantProfileRequest{
		Name: readbackA.GetName(), ShortName: readbackA.GetShortName(), Timezone: readbackA.GetTimezone(), LogoAssetRef: "data:image/png;base64,AAAA", Version: readbackA.GetVersion(),
	})
	if statusCode != http.StatusBadRequest {
		t.Fatalf("DataURL logo status=%d want=%d body=%s", statusCode, http.StatusBadRequest, body)
	}
	readbackA2, statusCode, body := tenantProfileHTTP(t, http.MethodGet, base+"/v1/tenant/profile", tokenA, "", nil)
	if statusCode != http.StatusOK || readbackA2.GetLogoAssetRef() != updatedA.GetLogoAssetRef() || readbackA2.GetVersion() != updatedA.GetVersion() {
		t.Fatalf("invalid logo mutated profile: status=%d body=%s readback=%+v", statusCode, body, readbackA2)
	}

	profileB, statusCode, body := tenantProfileHTTP(t, http.MethodGet, base+"/v1/tenant/profile", tokenB, "", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant B profile status=%d body=%s", statusCode, body)
	}
	if profileB.GetTenantId() != tenantB || profileB.GetName() != tenantB || profileB.GetVersion() != 1 {
		t.Fatalf("tenant B observed tenant A profile: %+v", profileB)
	}
	if profileB.GetName() == updatedA.GetName() || profileB.GetLogoAssetRef() == updatedA.GetLogoAssetRef() {
		t.Fatalf("cross-tenant profile leak: A=%+v B=%+v", updatedA, profileB)
	}
}
