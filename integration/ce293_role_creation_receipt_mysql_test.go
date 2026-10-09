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

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
)

func ce293VerifyRoleReceiptRollback(t *testing.T, db *gorm.DB, tenant string) {
	t.Helper()
	role := domain.NewRole("rollback-"+ce04Random(t), tenant, "rollback receipt", time.Now().UTC())
	key, fingerprint := strings.Repeat("c", 64), strings.Repeat("d", 64)
	outside, err := persistence.NewTenantRoleRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outside.CreateOnce(context.Background(), &role, key, fingerprint); err == nil {
		t.Fatal("receipt creation accepted a nontransactional connection")
	}
	rollback := errors.New("injected root rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		repository, err := persistence.NewTenantRoleRepository(tx)
		if err != nil {
			return err
		}
		if _, err := repository.CreateOnce(context.Background(), &role, key, fingerprint); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("root rollback was not exercised: %v", err)
	}
	for table, query := range map[string]string{"biz_roles": "id", "biz_role_creation_receipts": "receipt_key"} {
		value := role.ID
		if query == "receipt_key" {
			value = key
		}
		var count int64
		if err := db.Table(table).Where(query+"=?", value).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("rollback left %s row: %d %v", table, count, err)
		}
	}
	// A completed claim marker alone must not authorize recreating a receipt.
	err = db.Transaction(func(tx *gorm.DB) error {
		repository, err := persistence.NewTenantRoleRepository(tx)
		if err != nil {
			return err
		}
		_, err = repository.CreateOnce(ports.WithRoleCreationReplay(context.Background()), &role, key, fingerprint)
		return err
	})
	if !errors.Is(err, ports.ErrTenantRoleConflict) {
		t.Fatalf("missing completed receipt did not fail closed: %v", err)
	}
}

// This covers an upgrade of an existing schema with AUTO_MIGRATE disabled.
// ce08FreshFixtureDB is restricted to the existing backed-up CI database.
func TestCE293RoleCreationVersionedMigrationWithoutAutoMigrate(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	// Preexisting role authority must survive the additive schema migration.
	if err := db.Exec(`CREATE TABLE biz_roles (id VARCHAR(160) NOT NULL PRIMARY KEY, tenant_id VARCHAR(64) NOT NULL) ENGINE=InnoDB`).Error; err != nil {
		t.Fatal(err)
	}
	legacyRole := "preupgrade-" + ce04Random(t)
	if err := db.Exec(`INSERT INTO biz_roles(id, tenant_id) VALUES (?, ?)`, legacyRole, "tenant-legacy").Error; err != nil {
		t.Fatal(err)
	}
	migrationPath := filepath.Join("..", "internal", "access", "infrastructure", "persistence", "migrations", "0022_access_role_creation_receipts.sql")
	sql, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable("biz_role_creation_receipts") {
		t.Fatal("receipt table unexpectedly existed before versioned migration")
	}
	// Create the new table using ONLY the released versioned SQL, not GORM.
	if err := db.Exec(string(sql)).Error; err != nil {
		t.Fatalf("apply 0022 migration: %v", err)
	}
	if !db.Migrator().HasTable("biz_role_creation_receipts") {
		t.Fatal("receipt migration was not applied")
	}
	for name, dataType := range map[string]string{
		"receipt_key": "varchar", "tenant_id": "varchar", "fingerprint": "varchar", "payload": "mediumtext",
	} {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts' AND COLUMN_NAME=? AND DATA_TYPE=?`, name, dataType).Scan(&count).Error; err != nil || count != 1 {
			t.Fatalf("migration column %s %s missing: %d %v", name, dataType, count, err)
		}
	}
	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts' AND INDEX_NAME='PRIMARY' AND COLUMN_NAME='receipt_key'`).Scan(&count).Error; err != nil || count != 1 {
		t.Fatalf("unique receipt key missing: %d %v", count, err)
	}
	if err := db.Table("biz_role_creation_receipts").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("migration forged history: count=%d err=%v", count, err)
	}
	receiptKey, fingerprint := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if err := db.Exec(`INSERT INTO biz_role_creation_receipts(receipt_key, tenant_id, fingerprint, payload) VALUES (?,?,?,?)`, receiptKey, "tenant-legacy", fingerprint, "not-valid-json").Error; err == nil {
		t.Fatal("invalid immutable receipt payload bypassed database check")
	}
	const receipt = `{"ID":"original","TenantID":"tenant-legacy","Version":1}`
	if err := db.Exec(`INSERT INTO biz_role_creation_receipts(receipt_key, tenant_id, fingerprint, payload) VALUES (?,?,?,?)`, receiptKey, "tenant-legacy", fingerprint, receipt).Error; err != nil {
		t.Fatal(err)
	}
	// Reapplying the versioned migration must leave old roles and new receipts alone.
	if err := db.Exec(string(sql)).Error; err != nil {
		t.Fatalf("repeat migration changed schema: %v", err)
	}
	if err := db.Table("biz_roles").Where("id=? AND tenant_id=?", legacyRole, "tenant-legacy").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("legacy role lost: %d %v", count, err)
	}
	var stored string
	if err := db.Table("biz_role_creation_receipts").Select("payload").Where("receipt_key=?", receiptKey).Scan(&stored).Error; err != nil || stored != receipt {
		t.Fatalf("idempotent migration changed receipt: %s %v", stored, err)
	}
}
