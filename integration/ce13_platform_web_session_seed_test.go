//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"yunka.io/gateway/authz"
)

type ce13PlatformBrowserFixture struct {
	BaseURL            string `json:"base_url"`
	DiscoveryURL       string `json:"discovery_url"`
	AllowedEmail       string `json:"allowed_email"`
	AllowedPassword    string `json:"allowed_password"`
	DeniedEmail        string `json:"denied_email"`
	DeniedPassword     string `json:"denied_password"`
	TenantEmail        string `json:"tenant_email"`
	TenantPassword     string `json:"tenant_password"`
	TenantID           string `json:"tenant_id"`
	AllowedAPIKey      string `json:"allowed_api_key"`
	AllowedSubject     string `json:"allowed_subject"`
	DeniedSubject      string `json:"denied_subject"`
	PlatformOIDCIssuer string `json:"platform_oidc_issuer"`
}

func TestCE13PlatformWebSessionSeed(t *testing.T) {
	path := os.Getenv("CE13_PLATFORM_E2E_ENV_FILE")
	if path == "" {
		t.Skip("CE13_PLATFORM_E2E_ENV_FILE is not configured")
	}

	db := openDB(t)
	// Reuse the qualified runtime bootstrap to materialize the commercial module
	// catalog and all current schemas before the standalone browser servers start.
	startB122Runtime(t, db, "ce13-seed-bootstrap-token")

	ctx := context.Background()
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsurePlatformSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureWebSessionSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSchema(ctx); err != nil {
		t.Fatal(err)
	}

	const (
		issuer          = "http://127.0.0.1:18081/idp"
		allowedUserID   = "ce13-platform-allow-user"
		allowedEmail    = "ce13.platform.allow@example.invalid"
		allowedPassword = "CE13-Allow-Correct-Horse-2026!"
		allowedSubject  = "ce13-platform-allow"
		allowedAPIKey   = "ce13-platform-api-key-allow"
		deniedUserID    = "ce13-platform-denied-user"
		deniedEmail     = "ce13.platform.denied@example.invalid"
		deniedPassword  = "CE13-Denied-Correct-Horse-2026!"
		deniedSubject   = "ce13-platform-denied"
		tenantUserID    = "ce13-tenant-only-user"
		tenantEmail     = "ce13.tenant.only@example.invalid"
		tenantPassword  = "CE13-Tenant-Correct-Horse-2026!"
		tenantID        = "ce13-tenant-only"
	)

	seedCE13WebUser(t, store, "ce13-platform-allow-home", allowedUserID, allowedEmail, allowedPassword)
	seedCE13WebUser(t, store, "ce13-platform-denied-home", deniedUserID, deniedEmail, deniedPassword)
	seedCE13WebUser(t, store, tenantID, tenantUserID, tenantEmail, tenantPassword)

	if err := store.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{
		Subject: allowedSubject,
		Token:   allowedAPIKey,
		Permissions: []authz.PermissionKey{
			"platform.module.read",
			"platform.module.manage",
			"platform.plan.read",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{
		Subject: deniedSubject,
		Token:   "ce13-platform-api-key-denied",
		Permissions: []authz.PermissionKey{
			"platform.tenant.read",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.BindOIDCPlatformIdentity(ctx, issuer, "biz-user:"+allowedUserID, allowedSubject, allowedEmail); err != nil {
		t.Fatal(err)
	}
	if err := store.BindOIDCPlatformIdentity(ctx, issuer, "biz-user:"+deniedUserID, deniedSubject, deniedEmail); err != nil {
		t.Fatal(err)
	}

	fixture := ce13PlatformBrowserFixture{
		BaseURL:            "http://127.0.0.1:18080",
		DiscoveryURL:       issuer + "/.well-known/openid-configuration",
		AllowedEmail:       allowedEmail,
		AllowedPassword:    allowedPassword,
		DeniedEmail:        deniedEmail,
		DeniedPassword:     deniedPassword,
		TenantEmail:        tenantEmail,
		TenantPassword:     tenantPassword,
		TenantID:           tenantID,
		AllowedAPIKey:      allowedAPIKey,
		AllowedSubject:     allowedSubject,
		DeniedSubject:      deniedSubject,
		PlatformOIDCIssuer: issuer,
	}
	payload, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}

func seedCE13WebUser(t *testing.T, store *accesspersistence.Store, tenantID, userID, email, password string) {
	t.Helper()
	ctx := context.Background()
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID:   tenantID,
		TenantName: tenantID,
		UserID:     userID,
		Email:      email,
		Token:      "setup-" + userID,
	}, []authz.PermissionKey{"tenant.entitlement.read"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, userID, password); err != nil {
		t.Fatal(err)
	}
}
