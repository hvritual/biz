// biz-local-ce02-repair finishes the one-off repair after the proven CE-02
// fixture rows have already been archived. It never moves or deletes modules.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const migrationKey = "local-ce02-archived-fixtures-catalog-v1"

type repairMigration struct {
	MigrationKey         string    `gorm:"primaryKey;size:128"`
	CatalogVersionBefore uint64    `gorm:"not null"`
	CatalogVersionAfter  uint64    `gorm:"not null"`
	CompletedAt          time.Time `gorm:"type:datetime(6);not null"`
}

func (repairMigration) TableName() string { return "biz_commercial_local_repair_migrations" }

type auditSnapshot struct {
	MigrationKey  string    `gorm:"primaryKey;size:128"`
	AuditID       uint64    `gorm:"primaryKey"`
	ModuleCode    string    `gorm:"size:96;not null"`
	Actor         string    `gorm:"size:160;not null"`
	Action        string    `gorm:"size:64;not null"`
	BeforeJSON    string    `gorm:"type:text"`
	AfterJSON     string    `gorm:"type:text"`
	Reason        string    `gorm:"size:512;not null"`
	RequestID     string    `gorm:"size:128"`
	CreatedAt     time.Time `gorm:"type:datetime(6);not null"`
	SnapshottedAt time.Time `gorm:"type:datetime(6);not null"`
}

func (auditSnapshot) TableName() string { return "biz_commercial_module_fixture_audit_archive" }

type audit struct {
	ID         uint64
	ModuleCode string
	Actor      string
	Action     string
	BeforeJSON string
	AfterJSON  string
	Reason     string
	RequestID  string
	CreatedAt  time.Time
}

func (audit) TableName() string { return "biz_commercial_module_audit" }

type dependency struct{ ModuleCode, DependsOn string }

func (dependency) TableName() string { return "biz_commercial_module_dependencies" }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := validateRecoveryContext(); err != nil {
		return err
	}
	dsn := strings.TrimSpace(os.Getenv("YUNKA_BIZ_MYSQL_DSN"))
	if dsn == "" {
		return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
	}
	if err := validateDSN(dsn); err != nil {
		return err
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open designated local database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	// Keeping one connection makes PROCESSLIST and all row locks meaningful.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var schema string
	if err := db.WithContext(ctx).Raw("SELECT DATABASE()").Scan(&schema).Error; err != nil {
		return fmt.Errorf("designated database verification failed: %w", err)
	}
	if schema != "biz_evolution" {
		return fmt.Errorf("designated database verification failed: schema=%q", schema)
	}
	var clients int64
	if err := db.WithContext(ctx).Raw("SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE DB=DATABASE() AND ID<>CONNECTION_ID()").Scan(&clients).Error; err != nil {
		return err
	}
	if clients != 0 {
		return fmt.Errorf("refusing repair while %d other database client(s) are connected", clients)
	}

	// MySQL DDL commits implicitly, so it is deliberately complete before the
	// business transaction below. These tables retain evidence; no source table
	// is changed by this phase.
	if err := createEvidenceSchema(db.WithContext(ctx)); err != nil {
		return fmt.Errorf("create repair evidence schema: %w", err)
	}

	var before, after uint64
	var already bool
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		version, err := consistency.LockCatalog(tx, true)
		if err != nil {
			return fmt.Errorf("lock catalog state: %w", err)
		}
		before = version
		// Lock both the active key range and archived rows. The repair deliberately
		// has no INSERT/DELETE against either module table.
		var active []string
		if err := tx.Raw("SELECT module_code FROM biz_commercial_modules WHERE module_code IN ('core','future') ORDER BY module_code FOR UPDATE").Scan(&active).Error; err != nil {
			return err
		}
		if len(active) != 0 {
			return fmt.Errorf("refusing repair: active fixture modules remain: %s", strings.Join(active, ","))
		}
		var archived []string
		if err := tx.Raw("SELECT module_code FROM biz_commercial_module_fixture_archive WHERE module_code IN ('core','future') ORDER BY module_code FOR UPDATE").Scan(&archived).Error; err != nil {
			return fmt.Errorf("lock archived fixture modules: %w", err)
		}
		if strings.Join(archived, ",") != "core,future" {
			return fmt.Errorf("refusing repair: archived modules=%q, want core,future", strings.Join(archived, ","))
		}
		var dependencies []dependency
		if err := tx.Raw("SELECT module_code, depends_on FROM biz_commercial_module_dependencies WHERE module_code IN ('core','future') OR depends_on IN ('core','future') ORDER BY module_code, depends_on FOR UPDATE").Scan(&dependencies).Error; err != nil {
			return fmt.Errorf("lock fixture dependencies: %w", err)
		}
		if len(dependencies) != 0 {
			return fmt.Errorf("refusing repair: fixture dependency references=%d, want 0", len(dependencies))
		}
		var audits []audit
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("module_code IN ?", []string{"core", "future"}).Order("module_code, id").Find(&audits).Error; err != nil {
			return fmt.Errorf("lock CE02 audit: %w", err)
		}
		if err := validateCE02Audits(audits); err != nil {
			return err
		}
		var receipt repairMigration
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("migration_key = ?", migrationKey).First(&receipt).Error
		if err == nil {
			already = true
			before = receipt.CatalogVersionBefore
			after = receipt.CatalogVersionAfter
			if version < after || receipt.CatalogVersionAfter != receipt.CatalogVersionBefore+1 {
				return errors.New("existing repair receipt does not match catalog state")
			}
			var snapshots int64
			if err := tx.Model(&auditSnapshot{}).Where("migration_key = ?", migrationKey).Count(&snapshots).Error; err != nil {
				return err
			}
			if snapshots != 2 {
				return fmt.Errorf("existing repair receipt has %d audit snapshots, want 2", snapshots)
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now, err := consistency.Now(tx)
		if err != nil {
			return err
		}
		for _, row := range audits {
			if err := tx.Create(&auditSnapshot{MigrationKey: migrationKey, AuditID: row.ID, ModuleCode: row.ModuleCode, Actor: row.Actor, Action: row.Action, BeforeJSON: row.BeforeJSON, AfterJSON: row.AfterJSON, Reason: row.Reason, RequestID: row.RequestID, CreatedAt: row.CreatedAt, SnapshottedAt: now}).Error; err != nil {
				return fmt.Errorf("save immutable audit snapshot: %w", err)
			}
		}
		if err := consistency.AdvanceCatalog(tx); err != nil {
			return fmt.Errorf("advance catalog exactly once: %w", err)
		}
		after = before + 1
		if err := tx.Create(&repairMigration{MigrationKey: migrationKey, CatalogVersionBefore: before, CatalogVersionAfter: after, CompletedAt: now}).Error; err != nil {
			return fmt.Errorf("save one-time repair receipt: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("CE02 archived-fixture repair rolled back: %w", err)
	}
	if err := verify(db.WithContext(ctx), before, after); err != nil {
		return err
	}
	if already {
		fmt.Printf("CE02_ARCHIVED_FIXTURE_REPAIR=ALREADY_APPLIED catalog_version=%d\n", after)
	} else {
		fmt.Printf("CE02_ARCHIVED_FIXTURE_REPAIR=PASS catalog_version_before=%d catalog_version_after=%d\n", before, after)
	}
	return nil
}

func validateDSN(dsn string) error {
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("invalid local MySQL DSN: %w", err)
	}
	if cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:13316" || cfg.DBName != "biz_evolution" {
		return errors.New("repair must use tcp(127.0.0.1:13316)/biz_evolution")
	}
	return nil
}

func validateCE02Audits(rows []audit) error {
	if len(rows) != 2 {
		return fmt.Errorf("refusing repair: CE02 audit rows=%d, want exactly 2", len(rows))
	}
	want := map[string]struct{ requestID, reason string }{
		"core":   {requestID: "core", reason: "create"},
		"future": {requestID: "future", reason: "registered but unavailable"},
	}
	for _, row := range rows {
		expected, ok := want[row.ModuleCode]
		if !ok || row.Actor != "platform-admin:ce02" || row.Action != "create" || row.Reason != expected.reason || row.RequestID != expected.requestID {
			return fmt.Errorf("refusing repair: audit id=%d is not the proven CE02 creation record", row.ID)
		}
		delete(want, row.ModuleCode)
	}
	if len(want) != 0 {
		return errors.New("refusing repair: CE02 audit does not cover core and future")
	}
	return nil
}

func verify(db *gorm.DB, before, after uint64) error {
	var active, archived, dependencies int64
	if err := db.Raw("SELECT COUNT(*) FROM biz_commercial_modules WHERE module_code IN ('core','future')").Scan(&active).Error; err != nil {
		return err
	}
	if err := db.Raw("SELECT COUNT(*) FROM biz_commercial_module_fixture_archive WHERE module_code IN ('core','future')").Scan(&archived).Error; err != nil {
		return err
	}
	if err := db.Raw("SELECT COUNT(*) FROM biz_commercial_module_dependencies WHERE module_code IN ('core','future') OR depends_on IN ('core','future')").Scan(&dependencies).Error; err != nil {
		return err
	}
	var audits []audit
	if err := db.Where("module_code IN ?", []string{"core", "future"}).Order("module_code, id").Find(&audits).Error; err != nil {
		return err
	}
	if err := validateCE02Audits(audits); err != nil {
		return err
	}
	var snapshots []auditSnapshot
	if err := db.Where("migration_key = ?", migrationKey).Order("module_code, audit_id").Find(&snapshots).Error; err != nil {
		return err
	}
	if len(snapshots) != len(audits) {
		return fmt.Errorf("post-repair snapshot rows=%d, want %d", len(snapshots), len(audits))
	}
	for i, snapshot := range snapshots {
		row := audits[i]
		if snapshot.AuditID != row.ID || snapshot.ModuleCode != row.ModuleCode || snapshot.Actor != row.Actor || snapshot.Action != row.Action || snapshot.BeforeJSON != row.BeforeJSON || snapshot.AfterJSON != row.AfterJSON || snapshot.Reason != row.Reason || snapshot.RequestID != row.RequestID || !snapshot.CreatedAt.Equal(row.CreatedAt) {
			return fmt.Errorf("post-repair audit snapshot does not reproduce audit id=%d", row.ID)
		}
	}
	if len(audits) != 2 {
		return errors.New("post-repair CE02 source audit count changed")
	}
	if len(snapshots) != 2 {
		return errors.New("post-repair audit snapshot count changed")
	}
	var receipt repairMigration
	if err := db.Where("migration_key = ?", migrationKey).First(&receipt).Error; err != nil {
		return err
	}
	var state consistency.CatalogState
	if err := db.Where("id = ?", 1).First(&state).Error; err != nil {
		return err
	}
	if active != 0 || archived != 2 || dependencies != 0 || receipt.CatalogVersionBefore != before || receipt.CatalogVersionAfter != after || state.Version < after {
		return fmt.Errorf("post-repair verification failed: active=%d archive=%d dependencies=%d audits=%d snapshots=%d receipt=%d->%d state=%d", active, archived, dependencies, len(audits), len(snapshots), receipt.CatalogVersionBefore, receipt.CatalogVersionAfter, state.Version)
	}
	return nil
}

func validateRecoveryContext() error {
	marker := strings.TrimSpace(os.Getenv("CE02_REPAIR_RECOVERY_MARKER"))
	backup := strings.TrimSpace(os.Getenv("CE02_REPAIR_BACKUP"))
	if marker == "" || backup == "" {
		return errors.New("run through scripts/repair-ce02-archived-fixtures.py so backup, marker, and lock are established")
	}
	markerPath, err := filepath.Abs(marker)
	if err != nil {
		return err
	}
	backupPath, err := filepath.Abs(backup)
	if err != nil {
		return err
	}
	info, err := os.Stat(markerPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		return errors.New("recovery marker is missing or not mode 0600")
	}
	backupInfo, err := os.Stat(backupPath)
	if err != nil || backupInfo.Mode().Perm() != 0o600 {
		return errors.New("recovery backup is missing or not mode 0600")
	}
	var receipt struct{ Container, Database, Backup, Operation string }
	data, err := os.ReadFile(markerPath)
	if err != nil || json.Unmarshal(data, &receipt) != nil {
		return errors.New("recovery marker is unreadable")
	}
	if receipt.Container != "biz-evolution-mysql-20260912" || receipt.Database != "biz_evolution" || receipt.Operation != "ce02-archived-fixture-repair" || receipt.Backup != backupPath {
		return errors.New("recovery marker does not describe this repair backup")
	}
	return nil
}

func createEvidenceSchema(db *gorm.DB) error {
	// GORM's MySQL time mapping defaults to DATETIME(3), which would lose
	// microseconds from the source audit and defeat the byte-for-byte snapshot
	// check. Keep this DDL explicit and outside the business transaction.
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS biz_commercial_local_repair_migrations (
 migration_key VARCHAR(128) NOT NULL PRIMARY KEY,
 catalog_version_before BIGINT UNSIGNED NOT NULL,
 catalog_version_after BIGINT UNSIGNED NOT NULL,
 completed_at DATETIME(6) NOT NULL
) ENGINE=InnoDB`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS biz_commercial_module_fixture_audit_archive (
 migration_key VARCHAR(128) NOT NULL,
 audit_id BIGINT UNSIGNED NOT NULL,
 module_code VARCHAR(96) NOT NULL,
 actor VARCHAR(160) NOT NULL,
 action VARCHAR(64) NOT NULL,
 before_json TEXT,
 after_json TEXT,
 reason VARCHAR(512) NOT NULL,
 request_id VARCHAR(128),
 created_at DATETIME(6) NOT NULL,
 snapshotted_at DATETIME(6) NOT NULL,
 PRIMARY KEY(migration_key, audit_id)
) ENGINE=InnoDB`).Error; err != nil {
		return err
	}
	if err := db.Exec("ALTER TABLE biz_commercial_local_repair_migrations MODIFY completed_at DATETIME(6) NOT NULL").Error; err != nil {
		return err
	}
	return db.Exec("ALTER TABLE biz_commercial_module_fixture_audit_archive MODIFY created_at DATETIME(6) NOT NULL, MODIFY snapshotted_at DATETIME(6) NOT NULL").Error
}
