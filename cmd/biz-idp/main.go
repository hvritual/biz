package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
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
	dsn := strings.TrimSpace(os.Getenv("YUNKA_BIZ_MYSQL_DSN"))
	if dsn == "" {
		return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
	}
	publicURL := strings.TrimRight(strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_PUBLIC_URL")), "/")
	if publicURL == "" {
		return errors.New("YUNKA_BIZ_IDP_PUBLIC_URL is required")
	}
	redirectURL := strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_REDIRECT_URL"))
	if redirectURL == "" {
		return errors.New("YUNKA_BIZ_IDP_REDIRECT_URL is required")
	}
	signingKeyFile := strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_SIGNING_KEY_FILE"))
	if signingKeyFile == "" {
		return errors.New("YUNKA_BIZ_IDP_SIGNING_KEY_FILE is required")
	}
	signingKey, err := os.ReadFile(signingKeyFile)
	if err != nil {
		return fmt.Errorf("read IdP signing key: %w", err)
	}
	config := bizruntime.FirstPartyIdPConfig{
		PublicURL:     publicURL,
		ClientID:      envOr("YUNKA_BIZ_IDP_CLIENT_ID", "biz-web"),
		RedirectURL:   redirectURL,
		SigningKeyPEM: string(signingKey),
		SigningKeyID:  strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_SIGNING_KEY_ID")),
		LoginTTL:      envDuration("YUNKA_BIZ_IDP_LOGIN_TTL", 5*time.Minute),
		CodeTTL:       envDuration("YUNKA_BIZ_IDP_CODE_TTL", 90*time.Second),
		TokenTTL:      envDuration("YUNKA_BIZ_IDP_TOKEN_TTL", 5*time.Minute),
		CookieSecure:  envBool("YUNKA_BIZ_IDP_COOKIE_SECURE", true),
	}
	if err := config.Validate(); err != nil {
		return err
	}
	database, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open IdP database: %w", err)
	}
	store, err := accesspersistence.New(database)
	if err != nil {
		return err
	}
	if envBool("YUNKA_BIZ_IDP_AUTO_MIGRATE", false) {
		if err := store.EnsureFirstPartyIDPSchema(context.Background()); err != nil {
			return fmt.Errorf("migrate IdP schema: %w", err)
		}
	}
	handler, err := bizruntime.NewFirstPartyIdPHandler(config, store)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              envOr("YUNKA_BIZ_IDP_LISTEN", "127.0.0.1:8081"),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		return server.Shutdown(shutdown)
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
