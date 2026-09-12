//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"yunka.io/gateway/authz"
)

type ce13PlatformBrowserFixture struct {
	BaseURL            string `json:"base_url"`
	WebBaseURL         string `json:"web_base_url"`
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
	started := startB122Runtime(t, db, "ce13-seed-bootstrap-token")

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

	baseURL := valueOrDefault(os.Getenv("CE13_BIZ_BASE_URL"), "http://127.0.0.1:18080")
	webBaseURL := valueOrDefault(os.Getenv("CE13_WEB_BASE_URL"), baseURL)
	issuer := valueOrDefault(os.Getenv("CE13_IDP_ISSUER"), "http://127.0.0.1:18081/idp")
	const (
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
	)

	seedCE13WebUser(t, store, "ce13-platform-allow-home", allowedUserID, allowedEmail, allowedPassword)
	seedCE13WebUser(t, store, "ce13-platform-denied-home", deniedUserID, deniedEmail, deniedPassword)

	if err := store.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{
		Subject: allowedSubject,
		Token:   allowedAPIKey,
		Permissions: []authz.PermissionKey{
			"platform.module.read",
			"platform.module.manage",
			"platform.module.technical.manage",
			"platform.plan.read",
			"platform.plan.manage",
			"platform.plan.publish",
			"platform.tenant.create",
			"platform.tenant.read",
			"platform.tenant.manage",
			"platform.subscription.manage",
			"platform.subscription.read",
			"platform.subscription.confirm",
			"platform.entitlement.manage",
			"platform.entitlement.read",
			"commercial.catalog.read",
		},
	}); err != nil {
		t.Fatal(err)
	}
	tenantID := seedCE13TenantSubscription(t, started.GRPCAddress(), allowedAPIKey)
	seedCE13WebUser(t, store, tenantID, tenantUserID, tenantEmail, tenantPassword)
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
		BaseURL:            baseURL,
		WebBaseURL:         webBaseURL,
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

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// The browser qualification uses this pre-existing base subscription only as
// the target of CE-13's platform-owned preview/confirm flow. Tenant creation
// and default-rule bootstrap remain API-key-only operations and are therefore
// deliberately not performed by the browser session.
func seedCE13TenantSubscription(t *testing.T, address, token string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := accessv1.NewTenantLifecycleApplicationClient(conn)
	requestID := "ce13-browser-subscription-" + ce04Random(t)
	requestContext := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token, "idempotency-key", requestID)
	created, err := client.CreateTenant(requestContext, &accessv1.CreateTenantRequest{
		RequestId:   requestID,
		Name:        "CE-13 browser acceptance tenant",
		OwnerUserId: "ce13-browser-owner-" + ce04Random(t),
		OwnerEmail:  "ce13-browser-owner-" + ce04Random(t) + "@example.invalid",
		SalesScope:  "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	activateID := "ce13-browser-activate-" + ce04Random(t)
	activateContext := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token, "idempotency-key", activateID)
	if _, err := client.ActivateTenant(activateContext, &accessv1.ActivateTenantRequest{Id: created.GetId(), Version: created.GetVersion()}); err != nil {
		t.Fatal(err)
	}
	return created.GetId()
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
