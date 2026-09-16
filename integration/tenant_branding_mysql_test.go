//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/protobuf/encoding/protojson"
)

func brandingHTTP(t *testing.T, method, endpoint, token, key string, input any) (*accessv1.TenantBrandingDTO, int, string) {
	t.Helper()
	var payload []byte
	var err error
	if input != nil {
		payload, err = json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, endpoint, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("Content-Type", "application/json")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, string(body)
	}
	var result accessv1.TenantBrandingDTO
	if err := protojson.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return &result, response.StatusCode, string(body)
}

func TestTenantBrandingPersistsWithCASIdempotencyAndTrustedTenant(t *testing.T) {
	db := openDB(t)
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress() + "/v1/tenant/branding"
	stamp := fmt.Sprint(time.Now().UnixNano())
	a, b := "brand-a-"+stamp, "brand-b-"+stamp
	ownerA, ownerB := "brand-owner-a-"+stamp, "brand-owner-b-"+stamp
	tokenA, tokenB := "brand-token-a-"+stamp, "brand-token-b-"+stamp
	seedB123TenantAdmin(t, db, a, ownerA, ownerA+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, b, ownerB, ownerB+"@example.invalid", tokenB)
	seedECIR05TenantProfilePermissions(t, db, a)
	seedECIR05TenantProfilePermissions(t, db, b)

	initial, code, body := brandingHTTP(t, "GET", base, tokenA, "", nil)
	if code != 200 || initial.GetTenantId() != a || initial.GetPreset() != "blue" || initial.GetVersion() != 1 || !initial.GetCanManage() {
		t.Fatalf("initial: %d %s", code, body)
	}
	draft := map[string]any{"preset": "custom", "primary": "#125a75", "version": "1"}
	_, code, body = brandingHTTP(t, "PATCH", base, tokenA, "", draft)
	if code != 400 {
		t.Fatalf("missing idempotency: %d %s", code, body)
	}
	saved, code, body := brandingHTTP(t, "PATCH", base, tokenA, "branding-save-"+stamp, draft)
	if code != 200 || saved.GetVersion() != 2 || saved.GetPrimary() != "#125a75" {
		t.Fatalf("save: %d %s", code, body)
	}
	// The framework treats replay of an already-completed idempotency key as an explicit
	// conflict rather than replaying the cached response. The invariant is that the
	// mutation is not executed twice; authoritative readback below must remain version 2.
	_, code, body = brandingHTTP(t, "PATCH", base, tokenA, "branding-save-"+stamp, draft)
	if code != 409 {
		t.Fatalf("completed idempotency replay: %d %s", code, body)
	}
	_, code, body = brandingHTTP(t, "PATCH", base, tokenA, "branding-stale-"+stamp, draft)
	if code != 409 {
		t.Fatalf("stale version: %d %s", code, body)
	}
	readback, code, body := brandingHTTP(t, "GET", base, tokenA, "", nil)
	if code != 200 || readback.GetVersion() != 2 || readback.GetPreset() != "custom" || readback.GetPrimary() != "#125a75" {
		t.Fatalf("readback after completed-key replay: %d %s", code, body)
	}
	other, code, body := brandingHTTP(t, "GET", base, tokenB, "", nil)
	if code != 200 || other.GetTenantId() != b || other.GetPreset() != "blue" || other.GetVersion() != 1 {
		t.Fatalf("tenant B isolation: %d %s", code, body)
	}

	var row struct {
		BrandPreset, BrandPrimary string
		Version                   uint64
	}
	if err := db.Table("biz_tenants").Select("brand_preset,brand_primary,version").Where("id = ?", a).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.BrandPreset != "custom" || row.BrandPrimary != "#125a75" || row.Version != 2 {
		t.Fatalf("database persistence: %+v", row)
	}
	for index, primary := range []string{"#ffffff", "url(https://example.invalid/x)", "#000000;display:none"} {
		_, code, body = brandingHTTP(t, "PATCH", base, tokenA, fmt.Sprintf("bad-brand-%s-%d", stamp, index), map[string]any{"preset": "custom", "primary": primary, "version": "2"})
		if code != 400 {
			t.Fatalf("invalid custom primary: %d %s", code, body)
		}
	}
	reset, code, body := brandingHTTP(t, "PATCH", base, tokenA, "reset-brand-"+stamp, map[string]any{"preset": "blue", "primary": "", "version": "2"})
	if code != 200 || reset.GetVersion() != 3 || reset.GetPreset() != "blue" || reset.GetPrimary() != "" {
		t.Fatalf("reset: %d %s", code, body)
	}
}

func TestTenantBrandingOrdinaryMemberReadsButCannotWrite(t *testing.T) {
	db := openDB(t)
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress() + "/v1/tenant/branding"
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, user, token := "brand-read-"+stamp, "brand-reader-"+stamp, "brand-read-token-"+stamp
	// Existing fixture has member permissions but deliberately no organization.manage grant.
	seedB123TenantAdmin(t, db, tenant, user, user+"@example.invalid", token)
	value, code, body := brandingHTTP(t, "GET", base, token, "", nil)
	if code != 200 || value.GetCanManage() || value.GetTenantId() != tenant {
		t.Fatalf("ordinary read: %d %s", code, body)
	}
	_, code, body = brandingHTTP(t, "PATCH", base, token, "forbidden-brand-"+stamp, map[string]any{"preset": "violet", "version": "1"})
	if code != 403 {
		t.Fatalf("ordinary write: %d %s", code, body)
	}
	_, code, body = brandingHTTP(t, "GET", base, "", "", nil)
	if code != 401 {
		t.Fatalf("anonymous read: %d %s", code, body)
	}
	if err := db.Exec("UPDATE biz_api_tokens SET disabled = ? WHERE token_hash = ?", true, accesspersistence.TokenHash(token)).Error; err != nil {
		t.Fatal(err)
	}
	_, code, body = brandingHTTP(t, "GET", base, token, "", nil)
	if code != 401 {
		t.Fatalf("revoked identity: %d %s", code, body)
	}
}
