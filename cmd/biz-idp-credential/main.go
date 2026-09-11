package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

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
	userID := flag.String("user-id", "", "existing Biz user id")
	flag.Parse()
	if strings.TrimSpace(*userID) == "" {
		return errors.New("-user-id is required")
	}
	dsn := strings.TrimSpace(os.Getenv("YUNKA_BIZ_MYSQL_DSN"))
	if dsn == "" {
		return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
	}
	password, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(password) == 0 {
		return fmt.Errorf("read password from stdin: %w", err)
	}
	password = strings.TrimRight(password, "\r\n")
	database, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open credential database: %w", err)
	}
	store, err := accesspersistence.New(database)
	if err != nil {
		return err
	}
	if err := store.SetUserPassword(context.Background(), *userID, password); err != nil {
		return err
	}
	fmt.Printf("credential updated for user %s\n", strings.TrimSpace(*userID))
	return nil
}
