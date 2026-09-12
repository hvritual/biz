package persistence

import (
	"context"
	"embed"
	"errors"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var entitlementMigrations embed.FS

// MigrateEntitlements is explicit infrastructure setup; it never runs from a use case.
func MigrateEntitlements(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("commercial migration: database required")
	}
	entries, err := entitlementMigrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		data, err := entitlementMigrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if err = db.WithContext(ctx).Exec(string(data)).Error; err != nil {
			return err
		}
	}
	return nil
}
