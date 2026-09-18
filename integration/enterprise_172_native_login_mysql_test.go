//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

func enterprise172Protection(t *testing.T) *accesspersistence.ContactProtection {
	t.Helper()
	protection, err := accesspersistence.NewContactProtection(accesspersistence.ContactProtectionConfig{
		ActiveVersion: "v1",
		Keys:          map[string][]byte{"v1": []byte(strings.Repeat("K", 32))},
		LookupKey:     []byte(strings.Repeat("H", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return protection
}

func TestEnterprise172ProtectedIdentifierResolutionAndAmbiguity(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx := context.Background()
	protection := enterprise172Protection(t)
	store, err := accesspersistence.NewWithContactProtection(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSecuritySchema(ctx); err != nil {
		t.Fatal(err)
	}

	const (
		multiUser = "enterprise172-multi-user"
		multiMail = "enterprise172.multi@example.invalid"
		multiPass = "Enterprise172-Multi-Password!"
		multiName = "multiuser"
		multiPhone = "+491701720001"
	)
	for _, tenant := range []string{"enterprise172-a", "enterprise172-b"} {
		if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
			TenantID: tenant, TenantName: tenant, UserID: multiUser, Email: multiMail, Token: tenant + "-token",
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetUserPassword(ctx, multiUser, multiPass); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserUsername(ctx, multiUser, multiName); err != nil {
		t.Fatal(err)
	}
	memberRepo, err := accesspersistence.NewTenantMemberRepositoryWithContactProtection(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	member, err := memberRepo.Get(ctx, "enterprise172-a", multiUser)
	if err != nil {
		t.Fatal(err)
	}
	member.Phone = multiPhone
	if err := memberRepo.Update(ctx, &member, member.Version); err != nil {
		t.Fatal(err)
	}

	for _, identifier := range []string{multiMail, multiName, multiPhone} {
		resolved, err := store.ResolveLoginIdentifier(ctx, identifier)
		if err != nil {
			t.Fatalf("resolve %q: %v", identifier, err)
		}
		if resolved.Identity.UserID != multiUser {
			t.Fatalf("resolve %q user=%q", identifier, resolved.Identity.UserID)
		}
		identity, err := store.AuthenticateUserPassword(ctx, identifier, multiPass)
		if err != nil || identity.UserID != multiUser {
			t.Fatalf("password auth %q => %+v %v", identifier, identity, err)
		}
	}

	const duplicateUser = "enterprise172-duplicate-user"
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: "enterprise172-c", TenantName: "enterprise172-c", UserID: duplicateUser,
		Email: "enterprise172.duplicate@example.invalid", Token: "enterprise172-c-token",
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, duplicateUser, "Enterprise172-Duplicate-Password!"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserUsername(ctx, duplicateUser, multiName); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveLoginIdentifier(ctx, multiName); !errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {
		t.Fatalf("duplicate username was not rejected generically: %v", err)
	}

	duplicateMember, err := memberRepo.Get(ctx, "enterprise172-c", duplicateUser)
	if err != nil {
		t.Fatal(err)
	}
	duplicateMember.Phone = multiPhone
	if err := memberRepo.Update(ctx, &duplicateMember, duplicateMember.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveLoginIdentifier(ctx, multiPhone); !errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {
		t.Fatalf("phone shared by multiple accounts was not rejected generically: %v", err)
	}

	if _, _, err := accesspersistence.NormalizeLoginIdentifier("12345"); !errors.Is(err, accesspersistence.ErrInvalidLoginIdentifier) {
		t.Fatalf("pure numeric short account identifier accepted: %v", err)
	}
}

func TestEnterprise172PasswordLockPersistsAcrossStoreInstances(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx := context.Background()
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSecuritySchema(ctx); err != nil {
		t.Fatal(err)
	}

	const (
		userID = "enterprise172-lock-user"
		email = "enterprise172.lock@example.invalid"
		password = "Enterprise172-Correct-Password!"
	)
	if err := store.BootstrapGlobalUser(ctx, accesspersistence.GlobalUserBootstrap{ID: userID, Email: email}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, userID, password); err != nil {
		t.Fatal(err)
	}
	policy := accesspersistence.DefaultFirstPartyLoginPolicy()
	for attempt := 0; attempt < policy.MaxFailures; attempt++ {
		if _, _, err := store.AuthenticateFirstPartyLoginWithAudit(ctx, email, "wrong-password", "127.0.0.1:12345", policy); !errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {
			t.Fatalf("wrong-password attempt %d returned %v", attempt+1, err)
		}
	}
	blocked, until, err := store.FirstPartyLoginThrottleState(ctx, email)
	if err != nil || !blocked || until == nil || !until.After(time.Now().UTC()) {
		t.Fatalf("lock state missing: blocked=%v until=%v err=%v", blocked, until, err)
	}

	second, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := second.AuthenticateFirstPartyLoginWithAudit(ctx, email, password, "127.0.0.1:54321", policy); !errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {
		t.Fatalf("second store bypassed persistent lock: %v", err)
	}

	if err := db.Exec("UPDATE biz_idp_login_throttles SET blocked_until=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 SECOND), window_started_at=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 HOUR) WHERE identity_hash=?", accesspersistence.LoginIdentifierThrottleHash(email)).Error; err != nil {
		t.Fatal(err)
	}
	identity, _, err := second.AuthenticateFirstPartyLoginWithAudit(ctx, email, password, "127.0.0.1:54321", policy)
	if err != nil || identity.UserID != userID {
		t.Fatalf("expired lock did not recover: %+v %v", identity, err)
	}
}

func TestEnterprise172NoActiveTenantStillAuthenticatesGlobalAccount(t *testing.T) {
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
	const (
		userID = "enterprise172-empty-user"
		email = "enterprise172.empty@example.invalid"
	)
	if err := store.BootstrapGlobalUser(ctx, accesspersistence.GlobalUserBootstrap{ID: userID, Email: email}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, userID, "Enterprise172-Empty-Password!"); err != nil {
		t.Fatal(err)
	}
	identity, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.enterprise172.invalid", "subject-empty", email, true)
	if err != nil {
		t.Fatal(err)
	}
	raw, authentication, err := store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || !authentication.Principal.Authenticated && authentication.Session.UserID != userID {
		t.Fatalf("unexpected tenantless session: raw=%v auth=%+v", raw != "", authentication)
	}
	if authentication.Session.ActiveTenantID != "" || len(authentication.Session.Tenants) != 0 {
		t.Fatalf("global account without active memberships guessed tenant: %+v", authentication.Session)
	}
}
