package persistence

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

//go:embed migrations/0022_access_role_creation_receipts.sql
var roleCreationMigrationSQL string

// RequireRoleCreationReceiptSchema is a fail-closed production startup check.
// A disabled AutoMigrate must never silently accept a binary which will break
// every tenant.role.create on its first use. This check does not modify schema.
func RequireRoleCreationReceiptSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("access: role receipt database required")
	}
	for column, dataType := range map[string]string{
		"receipt_key": "varchar", "tenant_id": "varchar", "fingerprint": "varchar", "payload": "mediumtext",
	} {
		var count int64
		if err := db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'
			AND COLUMN_NAME=? AND DATA_TYPE=?`, column, dataType).Scan(&count).Error; err != nil {
			return fmt.Errorf("access: role creation receipt schema check: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("access: role creation receipt schema missing/invalid column %s; apply versioned migration 0022 before starting biz", column)
		}
	}
	checks := map[string]string{
		"PRIMARY key": `SELECT COUNT(*) FROM information_schema.STATISTICS
			WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'
			AND INDEX_NAME='PRIMARY' AND COLUMN_NAME='receipt_key'`,
		"JSON check": `SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
			WHERE CONSTRAINT_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'
			AND CONSTRAINT_NAME='access_role_receipt_json' AND CONSTRAINT_TYPE='CHECK'`,
	}
	for requirement, query := range checks {
		var count int64
		if err := db.WithContext(ctx).Raw(query).Scan(&count).Error; err != nil {
			return fmt.Errorf("access: role receipt %s check: %w", requirement, err)
		}
		if count != 1 {
			return fmt.Errorf("access: role creation receipt schema missing %s; apply/fix versioned migration 0022 before starting biz", requirement)
		}
	}
	return nil
}

// ApplyRoleCreationReceiptMigration is for an explicitly invoked pre-binary
// upgrade, not ordinary request handling or automatic production startup.
// This additive migration neither backfills old completed requests nor erases
// existing receipts. An unexpected/pre-existing incompatible table fails closed.
func ApplyRoleCreationReceiptMigration(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("access: role receipt database required")
	}
	if err := db.WithContext(ctx).Exec(roleCreationMigrationSQL).Error; err != nil {
		return fmt.Errorf("access: apply 0022 role creation receipts: %w", err)
	}
	return RequireRoleCreationReceiptSchema(ctx, db)
}
