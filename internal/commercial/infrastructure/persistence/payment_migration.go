package persistence

import (
	"context"
	"embed"
	"errors"

	"gorm.io/gorm"
)

//go:embed paymentmigrations/*.sql
var paymentMigrations embed.FS

// MigratePayments installs append-only payment order and callback-deduplication
// tables. It is composition-time setup, never a payment application concern.
func MigratePayments(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("payment migration: database required")
	}
	entries, err := paymentMigrations.ReadDir("paymentmigrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		body, err := paymentMigrations.ReadFile("paymentmigrations/" + entry.Name())
		if err != nil {
			return err
		}
		if err = db.WithContext(ctx).Exec(string(body)).Error; err != nil {
			return err
		}
	}
	return nil
}
