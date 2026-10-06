//go:build integration

package integration

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
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
		multiUser  = "enterprise172-multi-user"
		multiMail  = "enterprise172.multi@example.invalid"
		multiPass  = "Enterprise172-Multi-Password!"
		multiName  = "multiuser"
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
	if err := store.SetUserUsername(ctx, duplicateUser, multiName); !errors.Is(err, accesspersistence.ErrLoginIdentifierConflict) {
		t.Fatalf("duplicate global username err=%v want=%v", err, accesspersistence.ErrLoginIdentifierConflict)
	}
	resolvedUsername, err := store.ResolveLoginIdentifier(ctx, multiName)
	if err != nil || resolvedUsername.Identity.UserID != multiUser {
		t.Fatalf("global username owner changed after rejected duplicate: %+v %v", resolvedUsername, err)
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
		userID   = "enterprise172-lock-user"
		email    = "enterprise172.lock@example.invalid"
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
	if err := store.EnsureFirstPartyIDPSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureWebSessionSchema(ctx); err != nil {
		t.Fatal(err)
	}
	const (
		userID = "enterprise172-empty-user"
		email  = "enterprise172.empty@example.invalid"
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

func TestEnterprise191LegalLoginSuccessRateAndP95(t *testing.T) {
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
		sampleCount = 100
		userID      = "enterprise191-login-user"
		email       = "enterprise191.login@example.invalid"
		password    = "Enterprise191-Login9A"
	)
	if err := store.BootstrapGlobalUser(ctx, accesspersistence.GlobalUserBootstrap{ID: userID, Email: email}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, userID, password); err != nil {
		t.Fatal(err)
	}

	policy := accesspersistence.DefaultFirstPartyLoginPolicy()
	const workerCount = 8
	type loginResult struct {
		sample   int
		latency  time.Duration
		identity string
		err      error
	}
	jobs := make(chan int)
	results := make(chan loginResult, sampleCount)
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer wg.Done()
			for sample := range jobs {
				started := time.Now()
				identity, _, err := store.AuthenticateFirstPartyLoginWithAudit(
					ctx, email, password, "127.0.0.1:19191", policy,
				)
				results <- loginResult{
					sample:   sample,
					latency:  time.Since(started),
					identity: identity.UserID,
					err:      err,
				}
			}
		}()
	}
	for sample := 0; sample < sampleCount; sample++ {
		jobs <- sample
	}
	close(jobs)
	wg.Wait()
	close(results)

	latencies := make([]time.Duration, 0, sampleCount)
	successes := 0
	for result := range results {
		latencies = append(latencies, result.latency)
		if result.err == nil && result.identity == userID {
			successes++
			continue
		}
		t.Logf("ENTERPRISE191_LOGIN_FAILURE sample=%d identity=%q err=%v", result.sample+1, result.identity, result.err)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95 := latencies[(95*len(latencies)+99)/100-1]
	rate := float64(successes) / float64(sampleCount)
	t.Logf(
		"ENTERPRISE191_LOGIN_METRIC samples=%d successes=%d success_rate=%.5f p95_ms=%.3f max_ms=%.3f environment=ci_mysql",
		sampleCount, successes, rate, float64(p95)/float64(time.Millisecond),
		float64(latencies[len(latencies)-1])/float64(time.Millisecond),
	)
	if rate < 0.995 {
		t.Fatalf("legal login success rate %.5f below 0.995", rate)
	}
	if p95 > 2*time.Second {
		t.Fatalf("legal login p95=%s exceeds 2s", p95)
	}
}
