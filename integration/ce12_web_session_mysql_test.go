//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	devicepolicy "github.com/hvritual/biz/internal/deviceops/policy"
	"yunka.io/gateway/authz"
)

func TestCE12WebSessionTenantSelectionAndLiveAuthority(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureWebSessionSchema(ctx); err != nil {
		t.Fatal(err)
	}

	const userID = "ce12-user"
	const email = "ce12-user@example.test"
	for _, tenant := range []struct {
		id, name, token string
	}{
		{id: "ce12-tenant-a", name: "CE12 Tenant A", token: "ce12-token-a"},
		{id: "ce12-tenant-b", name: "CE12 Tenant B", token: "ce12-token-b"},
	} {
		if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
			TenantID: tenant.id, TenantName: tenant.name, UserID: userID, Email: email, Token: tenant.token,
		}, devicepolicy.Permissions()); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.example.test", "subject-unverified", email, false); !errors.Is(err, accesspersistence.ErrWebIdentityUnbound) {
		t.Fatalf("unverified email binding error = %v, want ErrWebIdentityUnbound", err)
	}
	webIdentity, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.example.test", "subject-1", email, true)
	if err != nil {
		t.Fatal(err)
	}
	if webIdentity.ActorKind != accesspersistence.WebActorUser || webIdentity.ActorID != userID {
		t.Fatalf("unexpected identity binding: %+v", webIdentity)
	}

	rawSession, authentication, err := store.CreateWebSession(ctx, webIdentity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if authentication.Session.ActiveTenantID != "" || authentication.Principal.Authenticated {
		t.Fatalf("multi-tenant session must start without active tenant: %+v", authentication)
	}
	if len(authentication.Session.Tenants) != 2 {
		t.Fatalf("tenant count = %d, want 2", len(authentication.Session.Tenants))
	}
	if !store.ValidateWebSessionCSRF(authentication, authentication.Session.CSRFToken) {
		t.Fatal("issued CSRF token must validate")
	}
	if store.ValidateWebSessionCSRF(authentication, "wrong-token") {
		t.Fatal("wrong CSRF token must not validate")
	}

	authentication, err = store.SwitchWebSessionTenant(ctx, rawSession, "ce12-tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if got := authentication.Principal.TenantID; got != "ce12-tenant-a" {
		t.Fatalf("principal tenant = %q, want ce12-tenant-a", got)
	}
	if authentication.Principal.UserID != userID || !authentication.Principal.Authenticated {
		t.Fatalf("unexpected tenant principal: %+v", authentication.Principal)
	}
	if authentication.Principal.AuthMethod != accesspersistence.AuthMethodWeb {
		t.Fatalf("auth method = %q, want %q", authentication.Principal.AuthMethod, accesspersistence.AuthMethodWeb)
	}

	if _, err := store.SwitchWebSessionTenant(ctx, rawSession, "ce12-not-a-member"); !errors.Is(err, accesspersistence.ErrWebTenantDenied) {
		t.Fatalf("non-member switch error = %v, want ErrWebTenantDenied", err)
	}

	if err := db.Table("biz_memberships").
		Where("tenant_id = ? AND user_id = ?", "ce12-tenant-a", userID).
		Update("status", accessdomain.TenantMemberStatusSuspended).Error; err != nil {
		t.Fatal(err)
	}
	authentication, err = store.AuthenticateWebSession(ctx, rawSession)
	if err != nil {
		t.Fatalf("identity session should survive loss of only the active tenant: %v", err)
	}
	if authentication.Session.ActiveTenantID != "" || authentication.Principal.Authenticated {
		t.Fatalf("invalid active tenant must be removed from effective session context: %+v", authentication)
	}
	if len(authentication.Session.Tenants) != 1 || authentication.Session.Tenants[0].ID != "ce12-tenant-b" {
		t.Fatalf("remaining valid tenant projection = %+v, want only tenant-b", authentication.Session.Tenants)
	}

	authentication, err = store.SwitchWebSessionTenant(ctx, rawSession, "ce12-tenant-b")
	if err != nil {
		t.Fatalf("switch from invalidated tenant A to valid tenant B: %v", err)
	}
	if authentication.Principal.TenantID != "ce12-tenant-b" {
		t.Fatalf("principal tenant after recovery switch = %q, want tenant-b", authentication.Principal.TenantID)
	}
	if err := store.RevokeWebSession(ctx, rawSession); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawSession); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("revoked session error = %v, want ErrWebSessionInvalid", err)
	}
}

func TestCE12OIDCBindingIsStableAfterVerifiedFirstBind(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureWebSessionSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: "ce12-bind-tenant", TenantName: "CE12 Bind Tenant", UserID: "ce12-bind-user",
		Email: "stable@example.test", Token: "ce12-bind-token",
	}, devicepolicy.Permissions()); err != nil {
		t.Fatal(err)
	}

	first, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.example.test", "stable-sub", "stable@example.test", true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.example.test", "stable-sub", "changed@example.test", false)
	if err != nil {
		t.Fatal(err)
	}
	if first.ActorKind != accesspersistence.WebActorUser || first.ActorID != "ce12-bind-user" || second.ActorID != first.ActorID {
		t.Fatalf("OIDC issuer+sub binding drifted: first=%+v second=%+v", first, second)
	}
}

func TestCE12PlatformWebAuthorityStaysTenantlessAndRevocable(t *testing.T) {
	db := openDB(t)
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
	const platformSubject = "ce12-platform-admin"
	if err := store.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{
		Subject: platformSubject,
		Token:   "ce12-platform-token",
		Permissions: []authz.PermissionKey{
			authz.PermissionKey("platform.tenant.read"),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.BindOIDCPlatformIdentity(ctx, "https://issuer.example.test", "platform-sub", platformSubject, "platform@example.test"); err != nil {
		t.Fatal(err)
	}
	identityLink, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.example.test", "platform-sub", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if identityLink.ActorKind != accesspersistence.WebActorPlatform || identityLink.ActorID != platformSubject {
		t.Fatalf("unexpected platform identity: %+v", identityLink)
	}
	rawSession, session, err := store.CreateWebSession(ctx, identityLink, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if session.Principal.TenantID != "" || session.Principal.Subject != platformSubject {
		t.Fatalf("platform web principal must remain tenantless: %+v", session.Principal)
	}
	if _, err := store.AuthenticatePlatformSubject(ctx, platformSubject, accesspersistence.AuthMethodWeb); err != nil {
		t.Fatalf("enabled platform authority rejected: %v", err)
	}
	if err := db.Table("biz_platform_credentials").Where("subject = ?", platformSubject).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticatePlatformSubject(ctx, platformSubject, accesspersistence.AuthMethodWeb); !errors.Is(err, accesspersistence.ErrUnauthorized) {
		t.Fatalf("disabled platform authority error = %v, want ErrUnauthorized", err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawSession); !errors.Is(err, accesspersistence.ErrUnauthorized) {
		t.Fatalf("disabled platform authority must invalidate web session authority, got %v", err)
	}
}
