//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// Fresh fixture tables reuse the configured database. Tests never create databases.
// The suite runner backs up/restores this local development database around a run.
func ce08FreshFixtureDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("YUNKA_TEST_RESET_FIXTURES") != "1" {
		t.Fatal("fixture reset requires YUNKA_TEST_RESET_FIXTURES=1; use the backed-up shared-database runner")
	}
	db := openDB(t)
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	rows, err := conn.QueryContext(context.Background(), "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'")
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	valid := regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(name, "biz_") && valid.MatchString(name) {
			tables = append(tables, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if _, err := conn.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS=1")
	for _, name := range tables {
		if _, err := conn.ExecContext(context.Background(), fmt.Sprintf("DROP TABLE `%s`", name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = pool.Close() })
	return db
}
