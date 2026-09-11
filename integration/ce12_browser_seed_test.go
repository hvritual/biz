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
	DiscoveryURL            string `json:"discovery_url"`
	Email                   string `json:"email"`
	Password                string `json:"password"`
	AllowedTenant           string `json:"allowed_tenant"`
	IAMDeniedTenant         string `json:"iam_denied_tenant"`
	EntitlementDeniedTenant string `json:"entitlement_denied_tenant"`
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

	userID := "ce12-browser-user"
	email := "ce12.browser@example.invalid"
	password := "Correct-Horse-Battery-Staple-2026!"
	allowed := ce12CreateActiveTenant(t, tenants, platformToken, "CE12 Allowed", userID, email)
	iamDenied := ce12CreateActiveTenant(t, tenants, platformToken, "CE12 IAM Denied", "ce12-iam-owner", "ce12.iam.owner@example.invalid")
	entitlementDenied := ce12CreateActiveTenant(t, tenants, platformToken, "CE12 Entitlement Denied", "ce12-entitlement-owner", "ce12.entitlement.owner@example.invalid")

	browserReadPermissions := append([]authz.PermissionKey{}, devicepolicy.Permissions()...)
	browserReadPermissions = append(browserReadPermissions, "tenant.entitlement.read", "commercial.catalog.read")
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: allowed, TenantName: "CE12 Allowed", UserID: userID, Email: email, Token: "ce12-setup-allowed",
	}, browserReadPermissions); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: iamDenied, TenantName: "CE12 IAM Denied", UserID: userID, Email: email, Token: "ce12-setup-iam-denied",
	}, []authz.PermissionKey{"tenant.entitlement.read", "commercial.catalog.read"}); err != nil {
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
		DiscoveryURL:            "http://127.0.0.1:18081/idp/.well-known/openid-configuration",
		Email:                   email,
		Password:                password,
		AllowedTenant:           allowed,
		IAMDeniedTenant:         iamDenied,
		EntitlementDeniedTenant: entitlementDenied,
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
