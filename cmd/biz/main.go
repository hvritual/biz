package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
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

	webAuth := bizruntime.WebAuthConfig{}
	if issuer := strings.TrimSpace(os.Getenv("YUNKA_BIZ_OIDC_ISSUER")); issuer != "" {
		webAuth = bizruntime.WebAuthConfig{
			IssuerURL:               issuer,
			ClientID:                strings.TrimSpace(os.Getenv("YUNKA_BIZ_OIDC_CLIENT_ID")),
			ClientSecret:            os.Getenv("YUNKA_BIZ_OIDC_CLIENT_SECRET"),
			RedirectURL:             strings.TrimSpace(os.Getenv("YUNKA_BIZ_OIDC_REDIRECT_URL")),
			PostLogoutRedirectURL:   strings.TrimSpace(os.Getenv("YUNKA_BIZ_OIDC_POST_LOGOUT_REDIRECT_URL")),
			Scopes:                  strings.Fields(envOr("YUNKA_BIZ_OIDC_SCOPES", "openid profile email")),
			SessionTTL:              envDuration("YUNKA_BIZ_OIDC_SESSION_TTL", 8*time.Hour),
			FlowTTL:                 envDuration("YUNKA_BIZ_OIDC_FLOW_TTL", 5*time.Minute),
			CookieSecure:            envBool("YUNKA_BIZ_OIDC_COOKIE_SECURE", true),
			PlatformExternalSubject: strings.TrimSpace(os.Getenv("YUNKA_BIZ_OIDC_PLATFORM_EXTERNAL_SUBJECT")),
			PlatformSubject:         strings.TrimSpace(os.Getenv("YUNKA_BIZ_OIDC_PLATFORM_SUBJECT")),
			PlatformEmail:           strings.TrimSpace(os.Getenv("YUNKA_BIZ_OIDC_PLATFORM_EMAIL")),
		}
		if err := webAuth.Validate(); err != nil {
			return err
		}
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
	workerToken := os.Getenv("YUNKA_BIZ_PROVISIONING_WORKER_TOKEN")
	lifecycle, err := lifecycleConfiguration()
	if err != nil {
		return err
	}
	started, err := bizruntime.BootstrapWithOptions(ctx, provider, bizruntime.Options{
		DeviceOps:           config,
		CommercialLifecycle: lifecycle,
		ProvisioningWorker: bizruntime.ProvisioningWorkerOptions{
			Token: workerToken, Automatic: workerToken != "",
		},
		WebAuth: webAuth,
	})
	if err != nil {
		return err
	}
	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return started.App.Shutdown(shutdownCtx)
}

// YUNKA_BIZ_COMMERCIAL_TRIAL_PLANS is a comma-separated list of immutable
// plan references (for example "starter@2,trial@1"). An unset value never
// invents a trial. Grace is a Go duration and defaults to zero, which retains
// the fail-closed restricted-at-boundary behavior.
func lifecycleConfiguration() (subscription.LifecyclePolicy, error) {
	grace, err := envGraceDuration("YUNKA_BIZ_COMMERCIAL_GRACE_DURATION")
	if err != nil {
		return subscription.LifecyclePolicy{}, err
	}
	policy := subscription.LifecyclePolicy{
		GraceDuration:    grace,
		BusinessTimezone: strings.TrimSpace(envOr("YUNKA_BIZ_COMMERCIAL_TIMEZONE", "UTC")),
	}
	for _, raw := range strings.Split(strings.TrimSpace(os.Getenv("YUNKA_BIZ_COMMERCIAL_TRIAL_PLANS")), ",") {
		if raw == "" {
			continue
		}
		code, version, ok := strings.Cut(strings.TrimSpace(raw), "@")
		parsed, err := strconv.ParseUint(version, 10, 64)
		if !ok || err != nil || parsed == 0 {
			return subscription.LifecyclePolicy{}, fmt.Errorf("invalid YUNKA_BIZ_COMMERCIAL_TRIAL_PLANS entry %q", raw)
		}
		policy.TrialPlans = append(policy.TrialPlans, subscription.PlanReference{PlanCode: code, Version: parsed})
	}
	if err := policy.Validate(); err != nil {
		return subscription.LifecyclePolicy{}, err
	}
	return policy.Canonical(), nil
}

func envGraceDuration(name string) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return parsed, nil
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

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}
