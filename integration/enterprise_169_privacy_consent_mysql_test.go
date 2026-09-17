//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

func TestEnterprise169PrivacyConsentEvidenceAndVersionPolicy(t *testing.T) {
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
		userID   = "enterprise-169-user"
		email    = "enterprise169@example.invalid"
		password = "Enterprise169-Password-A1"
	)
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: "enterprise-169-tenant", TenantName: "Enterprise 169",
		UserID: userID, Email: email, Token: "enterprise-169-bootstrap",
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(ctx, userID, password); err != nil {
		t.Fatal(err)
	}

	requestID, browser, csrf, err := store.CreateFirstPartyAuthorizationRequest(ctx, accesspersistence.FirstPartyAuthorizationRequestInput{
		ClientID: "biz-web", RedirectURI: "http://127.0.0.1:18080/auth/callback",
		State: "state-169", Nonce: "nonce-169", CodeChallenge: strings.Repeat("c", 43), Scope: "openid profile email",
	}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	identity, auditID, err := store.AuthenticateFirstPartyLoginWithAudit(ctx, email, password, "127.0.0.1:12345", accesspersistence.DefaultFirstPartyLoginPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != userID || auditID == 0 {
		t.Fatalf("unexpected authenticated identity/audit: %+v audit=%d", identity, auditID)
	}

	if satisfied, err := store.PrivacyConsentSatisfies(ctx, userID, "v1", true); err != nil || satisfied {
		t.Fatalf("new user unexpectedly satisfied consent: satisfied=%v err=%v", satisfied, err)
	}
	if _, _, _, err := store.AcceptPrivacyConsentAndIssueAuthorizationCode(ctx, accesspersistence.AcceptPrivacyConsentInput{
		RequestID: requestID, BrowserSecret: browser, CSRF: csrf,
		SubmittedVersion: "v1", ExpectedVersion: "v1",
		Source: accesspersistence.PrivacyConsentLoginSource, CodeTTL: time.Minute,
	}); !errors.Is(err, accesspersistence.ErrFirstPartyIDPFlow) {
		t.Fatalf("consent bypass without authenticated flow binding was accepted: %v", err)
	}
	if err := store.BindFirstPartyAuthorizationIdentity(ctx, requestID, browser+"-tampered", csrf, userID, auditID); !errors.Is(err, accesspersistence.ErrFirstPartyIDPFlow) {
		t.Fatalf("tampered browser binding accepted: %v", err)
	}
	if err := store.BindFirstPartyAuthorizationIdentity(ctx, requestID, browser, csrf, "spoof-user", auditID); !errors.Is(err, accesspersistence.ErrFirstPartyIDPFlow) {
		t.Fatalf("spoofed user binding accepted: %v", err)
	}
	if err := store.BindFirstPartyAuthorizationIdentity(ctx, requestID, browser, csrf, userID, auditID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := store.AcceptPrivacyConsentAndIssueAuthorizationCode(ctx, accesspersistence.AcceptPrivacyConsentInput{
		RequestID: requestID, BrowserSecret: browser, CSRF: csrf,
		SubmittedVersion: "v0-tampered", ExpectedVersion: "v1",
		Source: accesspersistence.PrivacyConsentLoginSource, CodeTTL: time.Minute,
	}); !errors.Is(err, accesspersistence.ErrPrivacyConsentVersionMismatch) {
		t.Fatalf("tampered agreement version accepted: %v", err)
	}

	code, authorization, consent, err := store.AcceptPrivacyConsentAndIssueAuthorizationCode(ctx, accesspersistence.AcceptPrivacyConsentInput{
		RequestID: requestID, BrowserSecret: browser, CSRF: csrf,
		SubmittedVersion: "v1", ExpectedVersion: "v1",
		Source: accesspersistence.PrivacyConsentLoginSource, CodeTTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code == "" || authorization.State != "state-169" {
		t.Fatalf("authorization code/result missing: code=%q auth=%+v", code, authorization)
	}
	if consent.UserID != userID || consent.AgreementVersion != "v1" || consent.Source != accesspersistence.PrivacyConsentLoginSource || consent.LoginAuditID != auditID || consent.AuthorizationHash == "" || consent.AcceptedAt.IsZero() || consent.WithdrawnAt != nil {
		t.Fatalf("consent evidence incomplete: %+v", consent)
	}
	readback, err := store.GetPrivacyConsent(ctx, userID, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if readback.LoginAuditID != auditID || readback.AuthorizationHash != consent.AuthorizationHash {
		t.Fatalf("consent/audit linkage changed on readback: %+v", readback)
	}
	if satisfied, err := store.PrivacyConsentSatisfies(ctx, userID, "v1", true); err != nil || !satisfied {
		t.Fatalf("current version consent not satisfied: %v %v", satisfied, err)
	}
	if satisfied, err := store.PrivacyConsentSatisfies(ctx, userID, "v2", true); err != nil || satisfied {
		t.Fatalf("upgrade unexpectedly satisfied exact-current policy: %v %v", satisfied, err)
	}
	if satisfied, err := store.PrivacyConsentSatisfies(ctx, userID, "v2", false); err != nil || !satisfied {
		t.Fatalf("any-active policy did not preserve prior acceptance: %v %v", satisfied, err)
	}
	if err := store.WithdrawPrivacyConsent(ctx, userID, "v1", "qualification"); err != nil {
		t.Fatal(err)
	}
	withdrawn, err := store.GetPrivacyConsent(ctx, userID, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if withdrawn.WithdrawnAt == nil || withdrawn.WithdrawnSource != "qualification" {
		t.Fatalf("withdrawal evidence missing: %+v", withdrawn)
	}
	if satisfied, err := store.PrivacyConsentSatisfies(ctx, userID, "v1", false); err != nil || satisfied {
		t.Fatalf("withdrawn consent remained active: %v %v", satisfied, err)
	}

	expiredID, expiredBrowser, expiredCSRF, err := store.CreateFirstPartyAuthorizationRequest(ctx, accesspersistence.FirstPartyAuthorizationRequestInput{
		ClientID: "biz-web", RedirectURI: "http://127.0.0.1:18080/auth/callback",
		State: "state-expired", Nonce: "nonce-expired", CodeChallenge: strings.Repeat("e", 43), Scope: "openid",
	}, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := store.BindFirstPartyAuthorizationIdentity(ctx, expiredID, expiredBrowser, expiredCSRF, userID, auditID); !errors.Is(err, accesspersistence.ErrFirstPartyIDPFlow) {
		t.Fatalf("expired authorization flow accepted: %v", err)
	}
}

func TestEnterprise169PrivacyConsentMigration(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	for _, name := range []string{"0002_ce12_first_party_identity.sql", "0008_enterprise_privacy_consent.sql"} {
		path := filepath.Join("..", "internal", "access", "infrastructure", "persistence", "migrations", name)
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(payload)).Error; err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	for _, column := range []string{"authenticated_user_id", "login_audit_id", "authenticated_at"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_idp_authorization_requests' AND COLUMN_NAME=?", column).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("migration missing authorization request column %s", column)
		}
	}
	if !db.Migrator().HasTable("biz_privacy_consents") {
		t.Fatal("migration did not create biz_privacy_consents")
	}
	for _, index := range []string{
		"idx_biz_idp_authorization_requests_authenticated_user",
		"idx_biz_idp_authorization_requests_login_audit",
		"idx_biz_privacy_consents_login_audit_id",
	} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND INDEX_NAME=?", index).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Fatalf("migration missing index %s", index)
		}
	}
}
