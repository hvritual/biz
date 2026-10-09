// biz-role-receipts-upgrade is an opt-in, pre-binary schema upgrade. It does
// not start the application or run unrelated historical migrations.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || os.Args[1] != "--apply" {
		return errors.New("usage: YUNKA_BIZ_MYSQL_DSN=<configured database> go run ./cmd/biz-role-receipts-upgrade --apply (before deploying new biz binary)")
	}
	dsn := os.Getenv("YUNKA_BIZ_MYSQL_DSN")
	if dsn == "" {
		return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("access role schema upgrade database: %w", err)
	}
	connection, err := db.DB()
	if err != nil {
		return err
	}
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := accesspersistence.ApplyRoleCreationReceiptMigration(ctx, db); err != nil {
		return err
	}
	fmt.Println("ROLE_CREATION_RECEIPT_SCHEMA=VERIFIED version=0022")
	return nil
}
