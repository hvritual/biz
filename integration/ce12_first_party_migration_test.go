//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestCE12FirstPartyIdentityMigration(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("YUNKA_TEST_MYSQL_DSN is required")
	}
	database, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrationPath := filepath.Join("..", "internal", "access", "infrastructure", "persistence", "migrations", "0002_ce12_first_party_identity.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(string(migration)).Error; err != nil {
		t.Fatalf("apply CE-12 identity migration: %v", err)
	}
	for _, table := range []string{
		"biz_user_password_credentials",
		"biz_idp_authorization_requests",
		"biz_idp_authorization_codes",
		"biz_idp_login_throttles",
		"biz_idp_login_audit",
		"biz_web_identities",
		"biz_web_sessions",
		"biz_web_login_flows",
	} {
		if !database.Migrator().HasTable(table) {
			t.Fatalf("CE-12 migration did not create %s", table)
		}
	}
}
