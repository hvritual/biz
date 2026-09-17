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

func enterprise168Protection(t *testing.T, active string, keys map[string][]byte) *accesspersistence.ContactProtection {
	t.Helper()
	protection, err := accesspersistence.NewContactProtection(accesspersistence.ContactProtectionConfig{
		ActiveVersion: active,
		Keys:          keys,
		LookupKey:     []byte(strings.Repeat("L", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return protection
}

func TestEnterprise168ProtectedContactsAreEncryptedMaskedAndTenantScoped(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx := context.Background()
	protection := enterprise168Protection(t, "v1", map[string][]byte{"v1": []byte(strings.Repeat("1", 32))})
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
	if err := store.EnsureWebSessionSchema(ctx); err != nil {
		t.Fatal(err)
	}
	repository, err := accesspersistence.NewTenantMemberRepositoryWithContactProtection(db, protection)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	loginEmail := " Shared.User@Example.invalid "
	memberA, err := repository.Invite(ctx, "tenant-a", "user-a", loginEmail, now)
	if err != nil {
		t.Fatal(err)
	}
	memberB, err := repository.Invite(ctx, "tenant-b", "user-b-proposed", loginEmail, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if memberA.UserID != memberB.UserID {
		t.Fatalf("same global Account was not reused: A=%q B=%q", memberA.UserID, memberB.UserID)
	}
	if !strings.Contains(memberA.Email, "***") || !strings.Contains(memberB.Email, "***") {
		t.Fatalf("member contact email was not masked: A=%q B=%q", memberA.Email, memberB.Email)
	}

	password := "Enterprise168PasswordA1"
	if err := store.SetUserPassword(ctx, memberA.UserID, password); err != nil {
		t.Fatal(err)
	}
	identity, err := store.AuthenticateUserPassword(ctx, "shared.user@example.invalid", password)
	if err != nil || identity.UserID != memberA.UserID || identity.Email != "shared.user@example.invalid" {
		t.Fatalf("protected Account password login failed: %+v %v", identity, err)
	}
	webIdentity, err := store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.example.invalid", "subject-168", "shared.user@example.invalid", true)
	if err != nil || webIdentity.ActorID != memberA.UserID {
		t.Fatalf("protected Account OIDC binding failed: %+v %v", webIdentity, err)
	}

	memberA, err = repository.Get(ctx, "tenant-a", memberA.UserID)
	if err != nil {
		t.Fatal(err)
	}
	memberA.Phone = "+49 (170) 123-4567"
	if err := repository.Update(ctx, &memberA, memberA.Version); err != nil {
		t.Fatal(err)
	}

	var account struct {
		Email           string
		EmailCiphertext string
		EmailLookupHash *string
		EmailKeyVersion string
	}
	if err := db.Table("biz_users").Select("email,email_ciphertext,email_lookup_hash,email_key_version").Where("id = ?", memberA.UserID).Scan(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.Email == "shared.user@example.invalid" || !strings.HasPrefix(account.Email, "protected:") || account.EmailCiphertext == "" || account.EmailLookupHash == nil || account.EmailKeyVersion != "v1" {
		t.Fatalf("global Account email is not protected at rest: %+v", account)
	}
	if strings.Contains(account.EmailCiphertext, "shared.user@example.invalid") {
		t.Fatal("global Account ciphertext contains plaintext email")
	}

	type membershipStorage struct {
		Email           string
		EmailCiphertext string
		EmailLookupHash *string
		EmailKeyVersion string
		Phone           string
		PhoneCiphertext string
		PhoneLookupHash *string
		PhoneKeyVersion string
	}
	var storedA, storedB membershipStorage
	selectStorage := "email,email_ciphertext,email_lookup_hash,email_key_version,phone,phone_ciphertext,phone_lookup_hash,phone_key_version"
	if err := db.Table("biz_memberships").Select(selectStorage).Where("tenant_id = ? AND user_id = ?", "tenant-a", memberA.UserID).Scan(&storedA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Select(selectStorage).Where("tenant_id = ? AND user_id = ?", "tenant-b", memberA.UserID).Scan(&storedB).Error; err != nil {
		t.Fatal(err)
	}
	if storedA.Email != "" || storedA.EmailCiphertext == "" || storedA.EmailLookupHash == nil || storedA.Phone != "" || storedA.PhoneCiphertext == "" || storedA.PhoneLookupHash == nil {
		t.Fatalf("tenant A contact fields are not encrypted at rest: %+v", storedA)
	}
	if storedB.Email != "" || storedB.EmailCiphertext == "" || storedB.EmailLookupHash == nil || storedB.PhoneCiphertext != "" {
		t.Fatalf("tenant B contact storage unexpected: %+v", storedB)
	}

	contactEmail := "tenant-a-contact@example.invalid"
	ciphertext, lookup, version, err := protection.ProtectEmail(contactEmail)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", "tenant-a", memberA.UserID).Updates(map[string]any{
		"email": "", "email_ciphertext": ciphertext, "email_lookup_hash": lookup, "email_key_version": version,
	}).Error; err != nil {
		t.Fatal(err)
	}
	readA, err := repository.Get(ctx, "tenant-a", memberA.UserID)
	if err != nil {
		t.Fatal(err)
	}
	readB, err := repository.Get(ctx, "tenant-b", memberA.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(readA.Email, "t***") || readA.Email == readB.Email || !strings.HasPrefix(readB.Email, "s***") {
		t.Fatalf("tenant Profile contact isolation failed: A=%q B=%q", readA.Email, readB.Email)
	}
	if _, err := store.AuthenticateUserPassword(ctx, "shared.user@example.invalid", password); err != nil {
		t.Fatalf("tenant Profile contact change altered global Account login: %v", err)
	}

	var boundEmail string
	if err := db.Table("biz_web_identities").Select("COALESCE(email,'')").Where("issuer = ? AND subject = ?", "https://issuer.example.invalid", "subject-168").Scan(&boundEmail).Error; err != nil {
		t.Fatal(err)
	}
	if boundEmail != "" {
		t.Fatalf("OIDC binding persisted duplicate plaintext email: %q", boundEmail)
	}

	unprotectedRepository, err := accesspersistence.NewTenantMemberRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unprotectedRepository.Get(ctx, "tenant-a", memberA.UserID); !errors.Is(err, accesspersistence.ErrSensitiveDataKeyUnavailable) {
		t.Fatalf("protected data without keys did not fail closed: %v", err)
	}
}

func TestEnterprise168LegacyBackfillIsBoundedIdempotentAndKeepsLogin(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx := context.Background()
	legacyStore, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyStore.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := legacyStore.EnsureFirstPartyIDPSchema(ctx); err != nil {
		t.Fatal(err)
	}
	legacyEmail := "legacy-168@example.invalid"
	if err := legacyStore.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: "legacy-tenant", TenantName: "Legacy Tenant", UserID: "legacy-user", Email: legacyEmail, Token: "legacy-token",
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", "legacy-tenant", "legacy-user").Update("phone", "+8613800138000").Error; err != nil {
		t.Fatal(err)
	}
	password := "Enterprise168LegacyA1"
	if err := legacyStore.SetUserPassword(ctx, "legacy-user", password); err != nil {
		t.Fatal(err)
	}

	protection := enterprise168Protection(t, "v1", map[string][]byte{"v1": []byte(strings.Repeat("1", 32))})
	protectedStore, err := accesspersistence.NewWithContactProtection(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	result, err := protectedStore.BackfillLegacyContacts(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Users != 1 || result.Memberships != 1 || result.Phones != 1 {
		t.Fatalf("unexpected backfill counts: %+v", result)
	}
	second, err := protectedStore.BackfillLegacyContacts(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if second.Users != 0 || second.Memberships != 0 || second.Phones != 0 {
		t.Fatalf("backfill was not idempotent: %+v", second)
	}
	if identity, err := protectedStore.AuthenticateUserPassword(ctx, legacyEmail, password); err != nil || identity.UserID != "legacy-user" {
		t.Fatalf("protected login after backfill failed: %+v %v", identity, err)
	}

	var row struct {
		Email, EmailCiphertext, Phone, PhoneCiphertext string
	}
	if err := db.Table("biz_memberships").Select("email,email_ciphertext,phone,phone_ciphertext").Where("tenant_id = ? AND user_id = ?", "legacy-tenant", "legacy-user").Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Email != "" || row.EmailCiphertext == "" || row.Phone != "" || row.PhoneCiphertext == "" || strings.Contains(row.EmailCiphertext, legacyEmail) || strings.Contains(row.PhoneCiphertext, "13800138000") {
		t.Fatalf("legacy membership was not protected: %+v", row)
	}
}

func TestEnterprise168VersionedMigrationAddsProtectedColumnsAndClearsOIDCEmail(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	legacy := []string{
		`CREATE TABLE biz_users (id VARCHAR(64) NOT NULL PRIMARY KEY, email VARCHAR(320) NOT NULL UNIQUE, status VARCHAR(32) NOT NULL, created_at DATETIME(6) NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE biz_memberships (tenant_id VARCHAR(64) NOT NULL, user_id VARCHAR(64) NOT NULL, status VARCHAR(32) NOT NULL, name VARCHAR(100) NOT NULL DEFAULT '', phone VARCHAR(40) NOT NULL DEFAULT '', employee_id VARCHAR(64) NOT NULL DEFAULT '', position VARCHAR(100) NOT NULL DEFAULT '', department_id VARCHAR(64) NOT NULL DEFAULT '', version BIGINT UNSIGNED NOT NULL DEFAULT 1, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL, PRIMARY KEY (tenant_id,user_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE biz_web_identities (issuer VARCHAR(512) NOT NULL, subject VARCHAR(255) NOT NULL, actor_kind VARCHAR(32) NOT NULL, actor_id VARCHAR(200) NOT NULL, email VARCHAR(320) NULL, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL, PRIMARY KEY (issuer,subject)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range legacy {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO biz_web_identities (issuer,subject,actor_kind,actor_id,email,created_at,updated_at) VALUES ('issuer','subject','user','user-1','plain@example.invalid',NOW(6),NOW(6))`).Error; err != nil {
		t.Fatal(err)
	}
	migrationPath := filepath.Join("..", "internal", "access", "infrastructure", "persistence", "migrations", "0007_enterprise_sensitive_contacts.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err != nil {
		t.Fatalf("apply #168 migration: %v", err)
	}
	for table, columns := range map[string][]string{
		"biz_users":       {"username", "email_ciphertext", "email_lookup_hash", "email_key_version"},
		"biz_memberships": {"email", "email_ciphertext", "email_lookup_hash", "email_key_version", "phone_ciphertext", "phone_lookup_hash", "phone_key_version"},
	} {
		for _, column := range columns {
			var count int64
			if err := db.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, table, column).Scan(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("migration did not add %s.%s", table, column)
			}
		}
	}
	for index, table := range map[string]string{
		"uniq_biz_users_email_lookup_hash":  "biz_users",
		"uniq_biz_memberships_email_lookup": "biz_memberships",
		"uniq_biz_memberships_phone_lookup": "biz_memberships",
	} {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?`, table, index).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Fatalf("migration did not create %s", index)
		}
	}
	var email string
	if err := db.Table("biz_web_identities").Select("COALESCE(email,'')").Where("issuer='issuer' AND subject='subject'").Scan(&email).Error; err != nil {
		t.Fatal(err)
	}
	if email != "" {
		t.Fatalf("migration retained duplicate OIDC plaintext email: %q", email)
	}
}
