package persistence

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"unicode"

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
	var primaryCount int64
	if err := db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'
		AND INDEX_NAME='PRIMARY' AND COLUMN_NAME='receipt_key'`).Scan(&primaryCount).Error; err != nil {
		return fmt.Errorf("access: role receipt primary key check: %w", err)
	}
	if primaryCount != 1 {
		return errors.New("access: role creation receipt schema missing PRIMARY key; fix migration 0022 before starting biz")
	}
	// A named CHECK is insufficient: MySQL 8.4 supports NOT ENFORCED and a
	// different check body under the same name. Validate both metadata facts
	// without executing any DDL or generating side effects on startup.
	var check struct {
		Enforced    string `gorm:"column:enforced"`
		CheckClause string `gorm:"column:check_clause"`
	}
	result := db.WithContext(ctx).Raw(`SELECT tc.ENFORCED AS enforced, cc.CHECK_CLAUSE AS check_clause
		FROM information_schema.TABLE_CONSTRAINTS AS tc
		JOIN information_schema.CHECK_CONSTRAINTS AS cc
		  ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA
		 AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
		WHERE tc.TABLE_SCHEMA=DATABASE()
		  AND tc.CONSTRAINT_SCHEMA=DATABASE()
		  AND tc.TABLE_NAME='biz_role_creation_receipts'
		  AND tc.CONSTRAINT_NAME='access_role_receipt_json'
		  AND tc.CONSTRAINT_TYPE='CHECK'`).Scan(&check)
	if result.Error != nil {
		return fmt.Errorf("access: role receipt JSON check metadata: %w", result.Error)
	}
	if result.RowsAffected != 1 || check.Enforced != "YES" || !validRoleReceiptCheckClause(check.CheckClause) {
		return errors.New("access: role creation receipt JSON check absent, disabled or changed; fix schema 0022 before starting biz")
	}
	return nil
}

// MySQL SHOW CREATE TABLE/CHECK_CLAUSE adds quoting and redundant grouping.
// We accept only the exact known predicate's tokens, not a merely named or
// weaker CHECK (such as OR TRUE) or a different JSON field.
func validRoleReceiptCheckClause(clause string) bool {
	var normalized strings.Builder
	for _, character := range clause {
		switch {
		case unicode.IsSpace(character), character == '`', character == '(', character == ')':
			continue
		case character >= 'A' && character <= 'Z':
			normalized.WriteRune(character + ('a' - 'A'))
		default:
			normalized.WriteRune(character)
		}
	}
	return normalized.String() == "payloadisnullorjson_validpayload"
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
