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
	// Match the released 0022 schema, not just a compatible-looking DATA_TYPE.
	// A shortened key can fail writes; a nullable tenant/fingerprint or an
	// extra required column can break the receipt authority after startup.
	type receiptColumn struct {
		ColumnName string `gorm:"column:column_name"`
		DataType   string `gorm:"column:data_type"`
		IsNullable string `gorm:"column:is_nullable"`
		MaxLength  *int64 `gorm:"column:character_maximum_length"`
	}
	var columns []receiptColumn
	result := db.WithContext(ctx).Raw(`SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, CHARACTER_MAXIMUM_LENGTH
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'`).Scan(&columns)
	if result.Error != nil {
		return fmt.Errorf("access: role creation receipt column metadata: %w", result.Error)
	}
	type requirement struct {
		dataType string
		nullable string
		length   int64
	}
	required := map[string]requirement{
		"receipt_key": {"varchar", "NO", 64},
		"tenant_id":   {"varchar", "NO", 64},
		"fingerprint": {"varchar", "NO", 64},
		"payload":     {"mediumtext", "YES", 0},
	}
	// Fail closed for extra NOT NULL columns without defaults as well.
	if len(columns) != len(required) {
		return errors.New("access: role creation receipt schema has incompatible columns; fix migration 0022 before starting biz")
	}
	seen := make(map[string]bool, len(columns))
	for _, col := range columns {
		want, ok := required[col.ColumnName]
		if !ok || seen[col.ColumnName] ||
			!strings.EqualFold(col.DataType, want.dataType) ||
			col.IsNullable != want.nullable ||
			(want.length != 0 && (col.MaxLength == nil || *col.MaxLength != want.length)) {
			return fmt.Errorf("access: role creation receipt schema incompatible column %s; fix migration 0022 before starting biz", col.ColumnName)
		}
		seen[col.ColumnName] = true
	}

	// This receipt is written alongside the role inside its root transaction.
	// A pre-existing nontransactional table would violate that invariant.
	var engine string
	engineResult := db.WithContext(ctx).Raw(`SELECT ENGINE FROM information_schema.TABLES
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'`).Scan(&engine)
	if engineResult.Error != nil {
		return fmt.Errorf("access: role receipt storage engine check: %w", engineResult.Error)
	}
	if engineResult.RowsAffected != 1 || !strings.EqualFold(engine, "InnoDB") {
		return errors.New("access: role creation receipt table must use InnoDB; fix migration 0022 before starting biz")
	}

	// A PRIMARY KEY starting with receipt_key is not enough. The composite
	// (receipt_key,fingerprint) can store two rows for a single transport key.
	var primary []struct {
		ColumnName string `gorm:"column:column_name"`
	}
	primaryResult := db.WithContext(ctx).Raw(`SELECT COLUMN_NAME FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='biz_role_creation_receipts'
		  AND INDEX_NAME='PRIMARY' ORDER BY SEQ_IN_INDEX`).Scan(&primary)
	if primaryResult.Error != nil {
		return fmt.Errorf("access: role receipt primary key metadata: %w", primaryResult.Error)
	}
	if len(primary) != 1 || primary[0].ColumnName != "receipt_key" {
		return errors.New("access: role creation receipt requires receipt_key as sole PRIMARY key; fix migration 0022 before starting biz")
	}
	// A named CHECK is insufficient: MySQL 8.4 supports NOT ENFORCED and a
	// different check body under the same name. Validate both metadata facts
	// without executing any DDL or generating side effects on startup.
	var check struct {
		Enforced    string `gorm:"column:enforced"`
		CheckClause string `gorm:"column:check_clause"`
	}
	result = db.WithContext(ctx).Raw(`SELECT tc.ENFORCED AS enforced, cc.CHECK_CLAUSE AS check_clause
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
