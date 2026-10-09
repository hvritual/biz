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
	// Use database/sql scalar scans throughout. GORM's struct-field mapping of
	// INFORMATION_SCHEMA result labels can differ by case and reject a valid
	// migrated database. Require exactly the four released 0022 columns.
	var columnCount, matchingColumns int64
	err := db.WithContext(ctx).Raw(`SELECT COUNT(*), COALESCE(SUM(
		CASE WHEN
		  (COLUMN_NAME='receipt_key' AND DATA_TYPE='varchar'
		    AND CHARACTER_MAXIMUM_LENGTH=64 AND IS_NULLABLE='NO')
		  OR (COLUMN_NAME='tenant_id' AND DATA_TYPE='varchar'
		    AND CHARACTER_MAXIMUM_LENGTH=64 AND IS_NULLABLE='NO')
		  OR (COLUMN_NAME='fingerprint' AND DATA_TYPE='varchar'
		    AND CHARACTER_MAXIMUM_LENGTH=64 AND IS_NULLABLE='NO')
		  OR (COLUMN_NAME='payload' AND DATA_TYPE='mediumtext'
		    AND IS_NULLABLE='YES')
		THEN 1 ELSE 0 END),0)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'`).Row().Scan(&columnCount, &matchingColumns)
	if err != nil {
		return fmt.Errorf("access: role receipt columns preflight: %w", err)
	}
	if columnCount != 4 || matchingColumns != 4 {
		return errors.New("access: role creation receipt columns/type/length/nullability mismatch; fix migration 0022 before starting biz")
	}
	// A (receipt_key, fingerprint) composite PK allows two different payloads
	// for one transport key. Only the single receipt_key key is admissible.
	var primaryCount, receiptKeyCount int64
	err = db.WithContext(ctx).Raw(`SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN COLUMN_NAME='receipt_key' THEN 1 ELSE 0 END),0)
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'
		  AND INDEX_NAME='PRIMARY'`).Row().Scan(&primaryCount, &receiptKeyCount)
	if err != nil {
		return fmt.Errorf("access: role receipt primary key preflight: %w", err)
	}
	if primaryCount != 1 || receiptKeyCount != 1 {
		return errors.New("access: role creation receipt requires receipt_key as sole PRIMARY key; fix migration 0022 before starting biz")
	}
	// The role and its receipt commit atomically in a root database transaction.
	var transactionalEngineCount int64
	err = db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.TABLES
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'
		  AND ENGINE='InnoDB'`).Row().Scan(&transactionalEngineCount)
	if err != nil {
		return fmt.Errorf("access: role receipt storage engine preflight: %w", err)
	}
	if transactionalEngineCount != 1 {
		return errors.New("access: role creation receipt table must use InnoDB; fix migration 0022 before starting biz")
	}
	// A CHECK with the right name can still be NOT ENFORCED or accept invalid
	// JSON. Require the enforced canonical predicate, not merely its name.
	var enforced, checkClause string
	err = db.WithContext(ctx).Raw(`SELECT tc.ENFORCED, cc.CHECK_CLAUSE
		FROM information_schema.TABLE_CONSTRAINTS AS tc
		JOIN information_schema.CHECK_CONSTRAINTS AS cc
		  ON cc.CONSTRAINT_SCHEMA=tc.CONSTRAINT_SCHEMA
		 AND cc.CONSTRAINT_NAME=tc.CONSTRAINT_NAME
		WHERE tc.TABLE_SCHEMA=DATABASE()
		  AND tc.CONSTRAINT_SCHEMA=DATABASE()
		  AND tc.TABLE_NAME='biz_role_creation_receipts'
		  AND tc.CONSTRAINT_NAME='access_role_receipt_json'
		  AND tc.CONSTRAINT_TYPE='CHECK'`).Row().Scan(&enforced, &checkClause)
	if err != nil {
		return fmt.Errorf("access: role receipt JSON check metadata: %w", err)
	}
	if enforced != "YES" || !validRoleReceiptCheckClause(checkClause) {
		return errors.New("access: role creation receipt JSON check disabled or changed; fix schema 0022 before starting biz")
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
