package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/modules/deviceops"
	"yunka.io/framework/core/eventBus"
	"yunka.io/framework/platform"
	"yunka.io/pkg/logExt"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dsn := os.Getenv("YUNKA_BIZ_MYSQL_DSN")
	if dsn == "" {
		return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
	}
	config := deviceops.DefaultConfig()
	if value := os.Getenv("YUNKA_BIZ_HTTP_LISTEN"); value != "" {
		config.HTTPListenAddress = value
	}
	if value := os.Getenv("YUNKA_BIZ_LISTEN"); value != "" {
		config.HTTPListenAddress = value
	}
	if value := os.Getenv("YUNKA_BIZ_GRPC_LISTEN"); value != "" {
		config.GRPCListenAddress = value
	}
	config.AutoMigrate = envBool("YUNKA_BIZ_AUTO_MIGRATE", false)
	config.Bootstrap.Token = os.Getenv("YUNKA_BIZ_BOOTSTRAP_TOKEN")
	if config.Bootstrap.Token != "" {
		config.Bootstrap.TenantID = envOr("YUNKA_BIZ_BOOTSTRAP_TENANT_ID", "tenant-demo")
		config.Bootstrap.TenantName = envOr("YUNKA_BIZ_BOOTSTRAP_TENANT_NAME", "Demo Tenant")
		config.Bootstrap.UserID = envOr("YUNKA_BIZ_BOOTSTRAP_USER_ID", "user-owner")
		config.Bootstrap.Email = envOr("YUNKA_BIZ_BOOTSTRAP_EMAIL", "owner@example.invalid")
		config.Bootstrap.SiteID = envOr("YUNKA_BIZ_BOOTSTRAP_SITE_ID", "site-demo")
		config.Bootstrap.SiteName = envOr("YUNKA_BIZ_BOOTSTRAP_SITE_NAME", "Demo Site")
	}
	if err := config.Validate(); err != nil {
		return err
	}
	provider, err := platform.New(platform.Options{
		Config:   bizruntime.ConfigProvider{DeviceOps: config},
		Logger:   logExt.NewBaseLogger(),
		EventBus: eventBus.NewTrieEventBus(),
		Databases: map[string]platform.DatabaseFactory{
			"primary": platform.MySQLFactory{Configurations: map[string]platform.MySQLConfig{
				"primary": {
					DSN: dsn, MaxOpenConns: 32, MaxIdleConns: 8,
					ConnMaxLifetime: 30 * time.Minute, ConnMaxIdleTime: 5 * time.Minute,
				},
			}},
		},
	})
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	started, err := bizruntime.Bootstrap(ctx, provider, config)
	if err != nil {
		return err
	}
	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return started.App.Shutdown(shutdownCtx)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
