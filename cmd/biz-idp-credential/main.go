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
	action := flag.String("action", "rotate", "credential action: rotate or disable")
	flag.Parse()
	user := strings.TrimSpace(*userID)
	if user == "" {
		return errors.New("-user-id is required")
	}
	dsn := strings.TrimSpace(os.Getenv("YUNKA_BIZ_MYSQL_DSN"))
	if dsn == "" {
		return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
	}
	database, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open credential database: %w", err)
	}
	store, err := accesspersistence.New(database)
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(*action)) {
	case "disable":
		if err := store.DisableUserPassword(context.Background(), user); err != nil {
			return err
		}
		fmt.Printf("credential disabled and web sessions revoked for user %s\n", user)
		return nil
	case "rotate":
		password, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && len(password) == 0 {
			return fmt.Errorf("read password from stdin: %w", err)
		}
		password = strings.TrimRight(password, "\r\n")
		if err := store.RotateUserPassword(context.Background(), user, password); err != nil {
			return err
		}
		fmt.Printf("credential rotated and web sessions revoked for user %s\n", user)
		return nil
	default:
		return errors.New("-action must be rotate or disable")
	}
}
