package modulecatalog

import (
	"context"
	"embed"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

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
		if err = s.db.WithContext(ctx).Exec(string(data)).Error; err != nil {
			return err
		}
	}
	return nil
}
