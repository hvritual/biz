//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

func TestEnterprise168MigrationThenBackfillProtectsNullLegacyColumns(t *testing.T) {
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
	if err := db.Exec(`INSERT INTO biz_users (id,email,status,created_at) VALUES ('legacy-null-user','legacy-null@example.invalid','active',NOW(6))`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO biz_memberships (tenant_id,user_id,status,name,phone,employee_id,position,department_id,version,created_at,updated_at) VALUES ('legacy-null-tenant','legacy-null-user','active','Legacy Null','+491701234567','','','',1,NOW(6),NOW(6))`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO biz_web_identities (issuer,subject,actor_kind,actor_id,email,created_at,updated_at) VALUES ('issuer-null','subject-null','user','legacy-null-user','legacy-null@example.invalid',NOW(6),NOW(6))`).Error; err != nil {
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

	var nullCounts struct {
		UserCipherNull   int64
		MemberEmailNull  int64
		MemberPhoneNull  int64
	}
	if err := db.Raw(`SELECT
		(SELECT COUNT(*) FROM biz_users WHERE id='legacy-null-user' AND email_ciphertext IS NULL) AS user_cipher_null,
		(SELECT COUNT(*) FROM biz_memberships WHERE tenant_id='legacy-null-tenant' AND user_id='legacy-null-user' AND email_ciphertext IS NULL) AS member_email_null,
		(SELECT COUNT(*) FROM biz_memberships WHERE tenant_id='legacy-null-tenant' AND user_id='legacy-null-user' AND phone_ciphertext IS NULL) AS member_phone_null`).Scan(&nullCounts).Error; err != nil {
		t.Fatal(err)
	}
	if nullCounts.UserCipherNull != 1 || nullCounts.MemberEmailNull != 1 || nullCounts.MemberPhoneNull != 1 {
		t.Fatalf("fixture did not reproduce migration NULL semantics: %+v", nullCounts)
	}

	protection := enterprise168Protection(t, "v1", map[string][]byte{"v1": []byte(strings.Repeat("1", 32))})
	store, err := accesspersistence.NewWithContactProtection(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.BackfillLegacyContacts(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Users != 1 || result.Memberships != 1 || result.Phones != 1 {
		t.Fatalf("migration NULL rows were skipped by backfill: %+v", result)
	}

	var user struct {
		Email, EmailCiphertext string
	}
	if err := db.Table("biz_users").Select("email,email_ciphertext").Where("id='legacy-null-user'").Scan(&user).Error; err != nil {
		t.Fatal(err)
	}
	var member struct {
		Email, EmailCiphertext, Phone, PhoneCiphertext string
	}
	if err := db.Table("biz_memberships").Select("email,email_ciphertext,phone,phone_ciphertext").Where("tenant_id='legacy-null-tenant' AND user_id='legacy-null-user'").Scan(&member).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(user.Email, "protected:") || user.EmailCiphertext == "" || member.Email != "" || member.EmailCiphertext == "" || member.Phone != "" || member.PhoneCiphertext == "" {
		t.Fatalf("migration/backfill did not remove plaintext facts: user=%+v member=%+v", user, member)
	}
	if strings.Contains(user.EmailCiphertext, "legacy-null@example.invalid") || strings.Contains(member.EmailCiphertext, "legacy-null@example.invalid") || strings.Contains(member.PhoneCiphertext, "1701234567") {
		t.Fatal("ciphertext leaked legacy plaintext")
	}

	var webEmail string
	if err := db.Table("biz_web_identities").Select("COALESCE(email,'')").Where("issuer='issuer-null' AND subject='subject-null'").Scan(&webEmail).Error; err != nil {
		t.Fatal(err)
	}
	if webEmail != "" {
		t.Fatalf("migration retained OIDC plaintext email: %q", webEmail)
	}
}
