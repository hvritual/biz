package persistence

import (
	"context"
	"embed"
	"errors"
	"gorm.io/gorm"
)

//go:embed planmigrations/*.sql
var planMigrations embed.FS

// MigratePlans is composition-time schema setup, never a business use case.
func MigratePlans(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("plan migration: database required")
	}
	entries, err := planMigrations.ReadDir("planmigrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		b, err := planMigrations.ReadFile("planmigrations/" + entry.Name())
		if err != nil {
			return err
		}
		if err := db.WithContext(ctx).Exec(string(b)).Error; err != nil {
			return err
		}
	}
	return nil
}
