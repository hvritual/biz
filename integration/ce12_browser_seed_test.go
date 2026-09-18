//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	devicepolicy "github.com/hvritual/biz/internal/deviceops/policy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"yunka.io/gateway/authz"
)

type ce12BrowserFixture struct {
	BaseURL                 string `json:"base_url"`
	UIBaseURL               string `json:"ui_base_url"`
	DiscoveryURL            string `json:"discovery_url"`
	Email                   string `json:"email"`
	Password                string `json:"password"`
	Username                string `json:"username"`
	Phone                   string `json:"phone"`
	SingleEmail             string `json:"single_email"`
	SinglePassword          string `json:"single_password"`
	SingleUsername          string `json:"single_username"`
	SinglePhone             string `json:"single_phone"`
	EmptyEmail              string `json:"empty_email"`
	EmptyPassword           string `json:"empty_password"`
	EmptyUsername           string `json:"empty_username"`
	PrivacyEmail            string `json:"privacy_email"`
	PrivacyPassword         string `json:"privacy_password"`
	AllowedTenant           string `json:"allowed_tenant"`
	IAMDeniedTenant         string `json:"iam_denied_tenant"`
	EntitlementDeniedTenant string `json:"entitlement_denied_tenant"`
	BrandTenantA            string `json:"brand_tenant_a"`
	BrandTenantB            string `json:"brand_tenant_b"`
	BrandTenantAName        string `json:"brand_tenant_a_name"`
	BrandTenantBName        string `json:"brand_tenant_b_name"`
}

func TestCE12BrowserSeed(t *testing.T) {
	path := os.Getenv("CE12_E2E_ENV_FILE")
	if path == "" {
		t.Skip("CE12_E2E_ENV_FILE is not configured")
	}
	db := openDB(t)
	platformToken := "ce12-browser-platform-token"
	started := startB122Runtime(t, db, platformToken)
	ctx := context.Background()
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{
		Subject: "platform-admin:b12",
		Token:   platformToken,
		Permissions: []authz.PermissionKey{
			"platform.entitlement.manage", "platform.entitlement.read", "platform.tenant.read", "commercial.catalog.read",
			"platform.module.manage", "platform.module.read", "platform.module.technical.manage",
		},
	}); err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.DialContext(ctx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	tenants := accessv1.NewTenantLifecycleApplicationClient(conn)
	entitlements := commercialv1.NewEntitlementManagementApplicationClient(conn)
	catalog := commercialv1.NewModuleCatalogApplicationClient(conn)

	// Reuse the CE-04/CE-02 platform Module Catalog authority. Branding is mapped
	// to access-management/tenant.lifecycle, so the module must exist in the live
	// commercial catalog before the real entitlement override can be accepted.
	accessModule, err := catalog.GetModule(ce04Context(platformToken, ""), &commercialv1.GetModuleRequest{ModuleCode: "access-management"})
	if err != nil {
		key := "ce12-access-module-" + ce04Random(t)
		accessModule, err = catalog.CreateModule(ce04Context(platformToken, key), &commercialv1.CreateModuleRequest{
			RequestId:  key,
			ModuleCode: "access-management",
			Name:       "access-management",
			Reason:     "CE12 real branding qualification catalog",
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if accessModule.TechnicalStatus != commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY {
		key := "ce12-access-ready-" + ce04Random(t)
		accessModule, err = catalog.SetModuleTechnicalStatus(ce04Context(platformToken, key), &commercialv1.SetModuleTechnicalStatusRequest{
			RequestId:       key,
			ModuleCode:      "access-management",
			TechnicalStatus: commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY,
			Version:         accessModule.Version,
			Reason:          "CE12 restore real access-management qualification fixture",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	userID := "ce12-browser-user"
	email := "ce12.browser@example.invalid"
	password := "Correct-Horse-Battery-Staple-2026!"
	username := "ce12multi"
	phone := "+491701234567"
	singleUserID := "ce12-native-single"
	singleEmail := "ce12.native.single@example.invalid"
	singlePassword := "CE12-Native-Single-2026!"
	singleUsername := "ce12single"
	singlePhone := "+491701234568"
	emptyUserID := "ce12-native-empty"
	emptyEmail := "ce12.native.empty@example.invalid"
	emptyPassword := "CE12-Native-Empty-2026!"
	emptyUsername := "ce12empty"
	privacyUserID := "ce12-privacy-user"
	privacyEmail := "ce12.privacy@example.invalid"
	privacyPassword := "CE12-Privacy-Consent-2026!"
	allowed := ce12CreateActiveTenant(t, tenants, platformToken, "CE12 Allowed", userID, email)
	iamDenied := ce12CreateActiveTenant(t, tenants, platformToken, "CE12 IAM Denied", "ce12-iam-owner", "ce12.iam.owner@example.invalid")
	entitlementDenied := ce12CreateActiveTenant(t, tenants, platformToken, "CE12 Entitlement Denied", "ce12-entitlement-owner", "ce12.entitlement.owner@example.invalid")

	browserReadPermissions := append([]authz.PermissionKey{}, devicepolicy.Permissions()...)
	browserReadPermissions = append(browserReadPermissions, "tenant.entitlement.read", "commercial.catalog.read", "tenant.branding.read")
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: allowed, TenantName: "CE12 Allowed", UserID: userID, Email: email, Token: "ce12-setup-allowed",
	}, browserReadPermissions); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: iamDenied, TenantName: "CE12 IAM Denied", UserID: userID, Email: email, Token: "ce12-setup-iam-denied",
	}, []authz.PermissionKey{"tenant.entitlement.read", "commercial.catalog.read", "tenant.branding.read"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: entitlementDenied, TenantName: "CE12 Entitlement Denied", UserID: userID, Email: email, Token: "ce12-setup-entitlement-denied",
	}, browserReadPermissions); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, userID, password); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserUsername(ctx, userID, username); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", allowed, userID).Update("phone", phone).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: allowed, TenantName: "CE12 Allowed", UserID: singleUserID, Email: singleEmail, Token: "ce12-native-single-bootstrap",
	}, browserReadPermissions); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, singleUserID, singlePassword); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserUsername(ctx, singleUserID, singleUsername); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", allowed, singleUserID).Update("phone", singlePhone).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.BootstrapGlobalUser(ctx, accesspersistence.GlobalUserBootstrap{ID: emptyUserID, Email: emptyEmail}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, emptyUserID, emptyPassword); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserUsername(ctx, emptyUserID, emptyUsername); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: allowed, TenantName: "CE12 Allowed", UserID: privacyUserID, Email: privacyEmail, Token: "ce12-privacy-bootstrap",
	}, browserReadPermissions); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, privacyUserID, privacyPassword); err != nil {
		t.Fatal(err)
	}

	// #147 reuses the real CE-12 tenant/session authority to prove that browser
	// appearance preferences remain orthogonal to #107 server-authoritative branding.
	if err := db.Table("biz_tenants").Where("id = ?", allowed).Updates(map[string]any{
		"brand_preset":  "violet",
		"brand_primary": "",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_tenants").Where("id = ?", iamDenied).Updates(map[string]any{
		"brand_preset":  "emerald",
		"brand_primary": "",
	}).Error; err != nil {
		t.Fatal(err)
	}

	// Use the real CE-04 entitlement authority. The fixture grants the capability
	// required by the live catalog rather than bypassing Commercial Guard.
	for _, grant := range []struct {
		tenant string
		id     string
	}{{allowed, "ce12-brand-a-lifecycle"}, {iamDenied, "ce12-brand-b-lifecycle"}} {
		var version uint64
		if err := db.Table("biz_commercial_entitlement_state").Select("version").Where("tenant_id = ?", grant.tenant).Scan(&version).Error; err != nil {
			t.Fatal(err)
		}
		if version == 0 {
			t.Fatalf("branding tenant %s has no subscription-derived source version", grant.tenant)
		}
		_, err = entitlements.CreateEntitlementOverride(ce04Context(platformToken, grant.id), &commercialv1.CreateEntitlementOverrideRequest{
			RequestId:       grant.id,
			TenantId:        grant.tenant,
			ExpectedVersion: version,
			ModuleCode:      "access-management",
			Target:          commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY,
			Key:             "tenant.lifecycle",
			Effect:          commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT,
			EffectiveAt:     time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
			Reason:          "CE12 browser E2E grants real access-management capability for branding proof",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	var sourceVersion uint64
	if err := db.Table("biz_commercial_entitlement_state").Select("version").Where("tenant_id = ?", entitlementDenied).Scan(&sourceVersion).Error; err != nil {
		t.Fatal(err)
	}
	if sourceVersion == 0 {
		t.Fatal("entitlement denied tenant has no subscription-derived source version")
	}
	_, err = entitlements.CreateEntitlementOverride(ce04Context(platformToken, "ce12-browser-deny-override"), &commercialv1.CreateEntitlementOverrideRequest{
		RequestId:       "ce12-browser-deny-override",
		TenantId:        entitlementDenied,
		ExpectedVersion: sourceVersion,
		ModuleCode:      "device-operations",
		Target:          commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY,
		Key:             "device.lifecycle",
		Effect:          commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_DENY,
		EffectiveAt:     time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		Reason:          "CE12 browser E2E proves commercial denial after IAM allow",
	})
	if err != nil {
		t.Fatal(err)
	}

	fixture := ce12BrowserFixture{
		BaseURL:                 "http://127.0.0.1:18080",
		UIBaseURL:               "http://127.0.0.1:15173",
		DiscoveryURL:            "http://127.0.0.1:18081/idp/.well-known/openid-configuration",
		Email:                   email,
		Password:                password,
		Username:                username,
		Phone:                   phone,
		SingleEmail:             singleEmail,
		SinglePassword:          singlePassword,
		SingleUsername:          singleUsername,
		SinglePhone:             singlePhone,
		EmptyEmail:              emptyEmail,
		EmptyPassword:           emptyPassword,
		EmptyUsername:           emptyUsername,
		PrivacyEmail:            privacyEmail,
		PrivacyPassword:         privacyPassword,
		AllowedTenant:           allowed,
		IAMDeniedTenant:         iamDenied,
		EntitlementDeniedTenant: entitlementDenied,
		BrandTenantA:            allowed,
		BrandTenantB:            iamDenied,
		BrandTenantAName:        "CE12 Allowed",
		BrandTenantBName:        "CE12 IAM Denied",
	}
	payload, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}

func ce12CreateActiveTenant(t *testing.T, client accessv1.TenantLifecycleApplicationClient, token, name, ownerID, ownerEmail string) string {
	t.Helper()
	stamp := ce04Random(t)
	created, err := client.CreateTenant(ce04Context(token, "ce12-create-"+stamp), &accessv1.CreateTenantRequest{
		Name:        name,
		OwnerUserId: ownerID,
		OwnerEmail:  ownerEmail,
		RequestId:   "ce12-create-" + stamp,
		SalesScope:  "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	active, err := client.ActivateTenant(ce04Context(token, "ce12-activate-"+stamp), &accessv1.ActivateTenantRequest{Id: created.GetId(), Version: created.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	return active.GetId()
}
