package featurecatalog

import (
	"context"
	"embed"
	"errors"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store is the single persistence adapter for CommercialFeature lifecycle
// facts. Plan, add-on, subscription and entitlement references remain owned by
// their existing repositories and are supplied as ReferenceImpact at Retire.
type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("commercial feature: database required")
	}
	return &Store{db: db}, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		data, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if err := s.db.WithContext(ctx).Exec(string(data)).Error; err != nil {
			return err
		}
	}
	return nil
}
