//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"yunka.io/gateway/authz"
)

func TestEnterprise171TenantSelectionContextVersionAndScopedRevocation(t *testing.T) {
	db := ce08FreshFixtureDB(t)
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
	if err := store.EnsureFirstPartyIDPSchema(ctx); err != nil {
		t.Fatal(err)
	}

	const (
		userID = "enterprise-171-user"
		email  = "enterprise171@example.invalid"
	)
	for _, tenant := range []struct{ id, name, token string }{
		{"enterprise-171-a", "Enterprise 171 A", "enterprise-171-a-token"},
		{"enterprise-171-b", "Enterprise 171 B", "enterprise-171-b-token"},
	} {
		if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
			TenantID: tenant.id, TenantName: tenant.name, UserID: userID, Email: email, Token: tenant.token,
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	identity, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.enterprise171.invalid", "enterprise-171-sub", email, true)
	if err != nil {
		t.Fatal(err)
	}

	rawA, sessionA, err := store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if sessionA.Session.ActiveTenantID != "" || sessionA.Session.ContextVersion != 1 || len(sessionA.Session.Tenants) != 2 {
		t.Fatalf("multi-tenant initial session=%+v", sessionA.Session)
	}
	initialCSRFA := sessionA.Session.CSRFToken
	sessionA, err = store.SwitchWebSessionTenant(ctx, rawA, "enterprise-171-a")
	if err != nil {
		t.Fatal(err)
	}
	if sessionA.Session.ActiveTenantID != "enterprise-171-a" || sessionA.Session.ContextVersion != 2 {
		t.Fatalf("tenant A switch did not advance context: %+v", sessionA.Session)
	}
	if sessionA.Session.CSRFToken == initialCSRFA {
		t.Fatal("tenant switch did not rotate CSRF")
	}

	rawB, sessionB, err := store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sessionB, err = store.SwitchWebSessionTenant(ctx, rawB, "enterprise-171-b")
	if err != nil {
		t.Fatal(err)
	}
	if sessionB.Session.ActiveTenantID != "enterprise-171-b" || sessionB.Session.ContextVersion != 2 {
		t.Fatalf("tenant B switch failed: %+v", sessionB.Session)
	}
	if _, err := store.SwitchWebSessionTenant(ctx, rawB, "enterprise-171-not-member"); !errors.Is(err, accesspersistence.ErrWebTenantDenied) {
		t.Fatalf("unauthorized tenant switch accepted: %v", err)
	}

	memberRepository, err := accesspersistence.NewTenantMemberRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	memberA, err := memberRepository.Get(ctx, "enterprise-171-a", userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := memberA.Suspend(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := memberRepository.Update(ctx, &memberA, memberA.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawA); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("tenant A active session survived member suspension: %v", err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawB); err != nil {
		t.Fatalf("tenant B session was incorrectly revoked by tenant A event: %v", err)
	}
	var revokedA struct{ Reason, Scope string }
	if err := db.Table("biz_web_sessions").Select("revoked_reason AS reason, revoked_scope AS scope").
		Where("token_hash = ?", accesspersistence.TokenHash(rawA)).Scan(&revokedA).Error; err != nil {
		t.Fatal(err)
	}
	if revokedA.Reason != "membership_suspended" || revokedA.Scope != "tenant:enterprise-171-a" {
		t.Fatalf("tenant revocation evidence=%+v", revokedA)
	}

	memberA, err = memberRepository.Get(ctx, "enterprise-171-a", userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := memberA.Activate(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := memberRepository.Update(ctx, &memberA, memberA.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawA); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("restored membership revived revoked tenant session: %v", err)
	}

	rawFreshA, freshA, err := store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	freshA, err = store.SwitchWebSessionTenant(ctx, rawFreshA, "enterprise-171-a")
	if err != nil || freshA.Session.ActiveTenantID != "enterprise-171-a" {
		t.Fatalf("fresh login after restore cannot enter tenant A: %+v %v", freshA.Session, err)
	}

	if err := store.SetUserPassword(ctx, userID, "Enterprise171-Password-A1"); err != nil {
		t.Fatal(err)
	}
	if err := store.RotateUserPassword(ctx, userID, "Enterprise171-Password-B2"); err != nil {
		t.Fatal(err)
	}
	for label, raw := range map[string]string{"fresh-a": rawFreshA, "tenant-b": rawB} {
		if _, err := store.AuthenticateWebSession(ctx, raw); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
			t.Fatalf("%s survived global account security event: %v", label, err)
		}
		var evidence struct{ Reason, Scope string }
		if err := db.Table("biz_web_sessions").Select("revoked_reason AS reason, revoked_scope AS scope").
			Where("token_hash = ?", accesspersistence.TokenHash(raw)).Scan(&evidence).Error; err != nil {
			t.Fatal(err)
		}
		if evidence.Reason != "account_security" || evidence.Scope != "account" {
			t.Fatalf("%s global revocation evidence=%+v", label, evidence)
		}
	}

	rawRemoved, removedSession, err := store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	removedSession, err = store.SwitchWebSessionTenant(ctx, rawRemoved, "enterprise-171-a")
	if err != nil || removedSession.Session.ActiveTenantID != "enterprise-171-a" {
		t.Fatalf("remove fixture cannot enter tenant A: %+v %v", removedSession.Session, err)
	}
	memberA, err = memberRepository.Get(ctx, "enterprise-171-a", userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := memberA.Remove(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := memberRepository.Update(ctx, &memberA, memberA.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawRemoved); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("removed member session remained reusable: %v", err)
	}
	var removedEvidence struct{ Reason, Scope string }
	if err := db.Table("biz_web_sessions").Select("revoked_reason AS reason, revoked_scope AS scope").
		Where("token_hash = ?", accesspersistence.TokenHash(rawRemoved)).Scan(&removedEvidence).Error; err != nil {
		t.Fatal(err)
	}
	if removedEvidence.Reason != "membership_removed" || removedEvidence.Scope != "tenant:enterprise-171-a" {
		t.Fatalf("remove revocation evidence=%+v", removedEvidence)
	}

	rawLogout, _, err := store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeWebSession(ctx, rawLogout); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawLogout); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("logout session replay accepted: %v", err)
	}
}

func TestEnterprise171ZeroSingleTenantAndRefreshBoundaries(t *testing.T) {
	db := ce08FreshFixtureDB(t)
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
		TenantID: "enterprise-171-single", TenantName: "Single Tenant", UserID: "enterprise-171-single-user",
		Email: "single171@example.invalid", Token: "single171-token",
	}, nil); err != nil {
		t.Fatal(err)
	}
	singleIdentity, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.enterprise171.invalid", "single-sub", "single171@example.invalid", true)
	if err != nil {
		t.Fatal(err)
	}
	rawSingle, single, err := store.CreateWebSession(ctx, singleIdentity, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if single.Session.ActiveTenantID != "enterprise-171-single" || single.Session.ContextVersion != 1 {
		t.Fatalf("single tenant did not enter directly: %+v", single.Session)
	}

	if err := db.Exec("UPDATE biz_web_sessions SET expires_at=DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 10 SECOND) WHERE token_hash=?", accesspersistence.TokenHash(rawSingle)).Error; err != nil {
		t.Fatal(err)
	}
	refreshed, changed, err := store.RefreshWebSession(ctx, rawSingle, 30*time.Second, 2*time.Minute)
	if err != nil || !changed {
		t.Fatalf("near-expiry session did not refresh: changed=%v err=%v", changed, err)
	}
	if time.Until(refreshed.Session.ExpiresAt) < time.Minute {
		t.Fatalf("refreshed expiry was not extended: %s", refreshed.Session.ExpiresAt)
	}
	if refreshed.Session.ContextVersion != 1 {
		t.Fatalf("TTL refresh changed tenant context version: %d", refreshed.Session.ContextVersion)
	}

	rawExpired, _, err := store.CreateWebSession(ctx, singleIdentity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE biz_web_sessions SET expires_at=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 SECOND) WHERE token_hash=?", accesspersistence.TokenHash(rawExpired)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawExpired); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("expired web session was accepted: %v", err)
	}
	if _, _, err := store.RefreshWebSession(ctx, rawExpired, 30*time.Second, time.Hour); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("expired web session was refreshed: %v", err)
	}

	if err := db.Table("biz_memberships").
		Where("tenant_id = ? AND user_id = ?", "enterprise-171-single", "enterprise-171-single-user").
		Update("status", accessdomain.TenantMemberStatusSuspended).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawSingle); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("suspended active member did not produce invalid session: %v", err)
	}
	if err := db.Table("biz_memberships").
		Where("tenant_id = ? AND user_id = ?", "enterprise-171-single", "enterprise-171-single-user").
		Update("status", accessdomain.TenantMemberStatusActive).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateWebSession(ctx, rawSingle); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("automatic authority invalidation was reversible after membership restore: %v", err)
	}

	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: "enterprise-171-zero", TenantName: "Zero Tenant", UserID: "enterprise-171-zero-user",
		Email: "zero171@example.invalid", Token: "zero171-token",
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", "enterprise-171-zero", "enterprise-171-zero-user").
		Update("status", accessdomain.TenantMemberStatusSuspended).Error; err != nil {
		t.Fatal(err)
	}
	zeroIdentity, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.enterprise171.invalid", "zero-sub", "zero171@example.invalid", true)
	if err != nil {
		t.Fatal(err)
	}
	rawZero, zero, err := store.CreateWebSession(ctx, zeroIdentity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if zero.Session.ActiveTenantID != "" || len(zero.Session.Tenants) != 0 || zero.Principal.Authenticated {
		t.Fatalf("zero-valid-tenant session gained tenant authority: %+v", zero)
	}
	if _, err := store.SwitchWebSessionTenant(ctx, rawZero, "enterprise-171-zero"); !errors.Is(err, accesspersistence.ErrWebTenantDenied) {
		t.Fatalf("suspended membership tenant switch accepted: %v", err)
	}
}

func TestEnterprise171PlatformSessionCannotSelectTenant(t *testing.T) {
	db := ce08FreshFixtureDB(t)
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
	if err := store.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{
		Subject: "enterprise-171-platform", Token: "enterprise-171-platform-token",
		Permissions: []authz.PermissionKey{authz.PermissionKey("platform.tenant.read")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.BindOIDCPlatformIdentity(ctx, "https://issuer.enterprise171.invalid", "platform-171-sub", "enterprise-171-platform", ""); err != nil {
		t.Fatal(err)
	}
	identity, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.enterprise171.invalid", "platform-171-sub", "", false)
	if err != nil {
		t.Fatal(err)
	}
	raw, session, err := store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if session.Session.ActorKind != accesspersistence.WebActorPlatform || session.Session.ActiveTenantID != "" {
		t.Fatalf("platform session crossed into tenant context: %+v", session.Session)
	}
	if _, err := store.SwitchWebSessionTenant(ctx, raw, "any-tenant"); !errors.Is(err, accesspersistence.ErrWebTenantDenied) {
		t.Fatalf("platform identity selected tenant: %v", err)
	}
}

func TestEnterprise171SessionIsolationMigration(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	for _, name := range []string{"0002_ce12_first_party_identity.sql", "0010_enterprise_session_isolation.sql"} {
		path := filepath.Join("..", "internal", "access", "infrastructure", "persistence", "migrations", name)
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(payload)).Error; err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	for _, column := range []string{"context_version", "revoked_reason", "revoked_scope"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_web_sessions' AND COLUMN_NAME=?", column).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("migration missing web-session column %s", column)
		}
	}
}
