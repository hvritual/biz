package modulecatalog

import (
	"context"
	"embed"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func (s *Store) Migrate(ctx context.Context) error {
	sql, err := migrationFS.ReadFile("migrations/0001_module_catalog.sql")
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Exec(string(sql)).Error
}
