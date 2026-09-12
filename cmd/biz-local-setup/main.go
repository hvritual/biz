// biz-local-setup creates the small, real account set used by the local
// console. It is intentionally an operator command: it starts no long-lived
// process and never resets shared local data.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/modules/deviceops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunka.io/framework/platform"
	"yunka.io/gateway/authz"
	"yunka.io/pkg/logExt"
)

const (
	platformUserID = "local-platform-admin"
	tenantUserID   = "local-tenant-admin"
	platformEmail  = "platform-admin@local.biz.invalid"
	tenantEmail    = "tenant-admin@local.biz.invalid"
	issuer         = "http://127.0.0.1:18381/idp"
)

type credentials struct {
	PlatformEmail    string `json:"platform_email"`
	PlatformPassword string `json:"platform_password"`
	TenantEmail      string `json:"tenant_email"`
	TenantPassword   string `json:"tenant_password"`
	PlatformToken    string `json:"platform_token"`
	WorkerToken      string `json:"worker_token"`
	TenantID         string `json:"tenant_id"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	path := flag.String("credentials-file", ".local/biz-local-credentials.json", "mode-0600 local credential file")
	validateConfig := flag.Bool("validate-config", false, "validate the designated local DSN without connecting or writing")
	flag.Parse()
	dsn := strings.TrimSpace(os.Getenv("YUNKA_BIZ_MYSQL_DSN"))
	if dsn == "" {
		return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
	}
	if err := validateLocalDSN(dsn); err != nil {
		return err
	}
	if *validateConfig {
		return nil
	}
	creds, exists, err := loadCredentials(*path)
	if err != nil {
		return err
	}
	if !exists {
		creds = credentials{PlatformEmail: platformEmail, TenantEmail: tenantEmail}
		if creds.PlatformPassword, err = secret(); err != nil {
			return err
		}
		if creds.TenantPassword, err = secret(); err != nil {
			return err
		}
		if creds.PlatformToken, err = secret(); err != nil {
			return err
		}
		if creds.WorkerToken, err = secret(); err != nil {
			return err
		}
		// Persist the recovery intent before the first database connection. A
		// later failure can therefore resume with the same identities and keys.
		if err := saveCredentials(*path, creds); err != nil {
			return fmt.Errorf("persist local credential intent: %w", err)
		}
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open local database: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	started, err := startTemporaryRuntime(ctx, db, creds.PlatformToken)
	if err != nil {
		return err
	}
	defer func() {
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		_ = started.App.Shutdown(shutdown)
	}()
	store, err := accesspersistence.New(db)
	if err != nil {
		return err
	}
	if err := store.EnsureFirstPartyIDPSchema(ctx); err != nil {
		return err
	}
	if err := store.EnsureWebSessionSchema(ctx); err != nil {
		return err
	}
	if err := store.BootstrapGlobalUser(ctx, accesspersistence.GlobalUserBootstrap{ID: platformUserID, Email: creds.PlatformEmail}); err != nil {
		return fmt.Errorf("platform account: %w", err)
	}
	if err := store.BootstrapGlobalUser(ctx, accesspersistence.GlobalUserBootstrap{ID: tenantUserID, Email: creds.TenantEmail}); err != nil {
		return fmt.Errorf("tenant account: %w", err)
	}
	if err := store.SetUserPassword(ctx, platformUserID, creds.PlatformPassword); err != nil {
		return fmt.Errorf("platform password: %w", err)
	}
	if err := store.SetUserPassword(ctx, tenantUserID, creds.TenantPassword); err != nil {
		return fmt.Errorf("tenant password: %w", err)
	}
	if err := store.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{Subject: "local-provisioning-worker", Token: creds.WorkerToken, Permissions: []authz.PermissionKey{"commercial.catalog.read", "platform.plan.read", "platform.provisioning.execute"}}); err != nil {
		return err
	}
	if err := store.BindOIDCPlatformIdentity(ctx, issuer, "biz-user:"+platformUserID, "local-platform-admin", creds.PlatformEmail); err != nil {
		return fmt.Errorf("platform OIDC identity: %w", err)
	}
	creds.TenantID, err = seedCommercialAndTenant(ctx, started.GRPCAddress(), creds.PlatformToken, creds.TenantID)
	if err != nil {
		return err
	}
	if err := saveCredentials(*path, creds); err != nil {
		return err
	}
	fmt.Printf("local accounts prepared; credentials are in %s\n", *path)
	return nil
}

func startTemporaryRuntime(ctx context.Context, db *gorm.DB, token string) (*bizruntime.Started, error) {
	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	provider, err := platform.New(platform.Options{Config: bizruntime.ConfigProvider{DeviceOps: config}, Logger: logExt.NewBaseLogger(), Databases: map[string]platform.DatabaseFactory{"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
		return platform.BorrowedDatabase(db), nil
	})}})
	if err != nil {
		return nil, err
	}
	return bizruntime.BootstrapWithOptions(ctx, provider, bizruntime.Options{DeviceOps: config, PlatformBootstrap: bizruntime.PlatformBootstrap{Subject: "local-platform-admin", Token: token, Permissions: []authz.PermissionKey{"platform.tenant.create", "platform.tenant.read", "platform.tenant.manage", "platform.plan.read", "platform.plan.manage", "platform.plan.publish", "commercial.catalog.read", "platform.subscription.manage", "platform.subscription.read", "platform.subscription.confirm", "platform.module.manage", "platform.module.read", "platform.module.technical.manage", "platform.entitlement.read", "platform.entitlement.manage"}}})
}

func seedCommercialAndTenant(ctx context.Context, address, token, savedTenantID string) (string, error) {
	conn, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return "", err
	}
	defer conn.Close()
	call := func(key string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token, "idempotency-key", key)
	}
	catalog := commercialv1.NewModuleCatalogApplicationClient(conn)
	plans := commercialv1.NewPlanManagementApplicationClient(conn)
	subscriptions := commercialv1.NewSubscriptionManagementApplicationClient(conn)
	if err := ensureModule(ctx, catalog, call, "access-management", "Access management"); err != nil {
		return "", err
	}
	if err := ensureModule(ctx, catalog, call, "device-operations", "Device operations"); err != nil {
		return "", err
	}
	published, err := plans.GetPlanVersion(call("local-plan-get"), &commercialv1.GetPlanVersionRequest{PlanCode: "local-free", Version: 1})
	if status.Code(err) == codes.NotFound {
		draft, createErr := plans.CreatePlanDraft(call("local-plan-create"), &commercialv1.CreatePlanDraftRequest{RequestId: "local-plan-create", PlanCode: "local-free", Name: "Local free", Terms: localPlanTerms(), Reason: "local console setup"})
		if createErr != nil {
			return "", createErr
		}
		published, err = plans.PublishPlanVersion(call("local-plan-publish"), &commercialv1.ChangePlanVersionStateRequest{RequestId: "local-plan-publish", PlanCode: draft.GetPlanCode(), Version: draft.GetVersion(), ExpectedRevision: draft.GetRevision(), Reason: "local console setup"})
	} else if err != nil {
		return "", err
	}
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(published.GetState(), "published") {
		return "", errors.New("local-free plan exists but is not published")
	}
	if err = ensureDefaultRule(subscriptions, call, published); err != nil {
		return "", err
	}
	tenants := accessv1.NewTenantLifecycleApplicationClient(conn)
	tenant, err := ensureTenant(tenants, call, savedTenantID)
	if err != nil {
		return "", err
	}
	if tenant.GetStatus() != accessv1.TenantStatus_TENANT_STATUS_ACTIVE {
		return "", errors.New("local tenant did not activate")
	}
	subscription, err := subscriptions.GetTenantSubscription(call("local-tenant-subscription"), &commercialv1.GetTenantSubscriptionRequest{TenantId: tenant.GetId()})
	if err != nil {
		return "", err
	}
	if subscription.GetState() != "ACTIVE" || subscription.GetPlanCode() != published.GetPlanCode() || subscription.GetPlanVersion() != published.GetVersion() {
		return "", errors.New("local tenant subscription is not the local-free active subscription")
	}
	return tenant.GetId(), nil
}

func ensureDefaultRule(subscriptions commercialv1.SubscriptionManagementApplicationClient, call func(string) context.Context, plan *commercialv1.PlanVersionDTO) error {
	rules, err := subscriptions.ListDefaultSubscriptionRules(call("local-rule-list"), &commercialv1.ListDefaultSubscriptionRulesRequest{})
	if err != nil {
		return err
	}
	for _, rule := range rules.GetRules() {
		if rule.GetRuleId() != "local-free-default" {
			continue
		}
		if rule.GetPriority() != -1000 || rule.GetSalesScope() != "*" || rule.GetPlanCode() != plan.GetPlanCode() || rule.GetPlanVersion() != plan.GetVersion() || !rule.GetEnabled() {
			return errors.New("local default subscription rule conflicts with existing state")
		}
		return nil
	}
	_, err = subscriptions.PutDefaultSubscriptionRule(call("local-rule-put"), &commercialv1.PutDefaultSubscriptionRuleRequest{RequestId: "local-rule-put", RuleId: "local-free-default", Priority: -1000, SalesScope: "*", PlanCode: plan.GetPlanCode(), PlanVersion: plan.GetVersion(), Enabled: true, Reason: "local console setup"})
	return err
}

func ensureTenant(tenants accessv1.TenantLifecycleApplicationClient, call func(string) context.Context, savedTenantID string) (*accessv1.TenantDTO, error) {
	var tenant *accessv1.TenantDTO
	var err error
	if savedTenantID != "" {
		tenant, err = tenants.GetTenant(call("local-tenant-get"), &accessv1.GetTenantRequest{Id: savedTenantID})
		if status.Code(err) == codes.NotFound {
			return nil, errors.New("saved local tenant ID no longer exists")
		}
		if err != nil {
			return nil, err
		}
		if tenant.GetName() != "Local tenant" {
			return nil, errors.New("saved local tenant ID belongs to a different tenant")
		}
	} else {
		listed, listErr := tenants.ListTenants(call("local-tenant-list"), &accessv1.ListTenantsRequest{})
		if listErr != nil {
			return nil, listErr
		}
		for _, candidate := range listed.GetTenants() {
			if candidate.GetName() != "Local tenant" {
				continue
			}
			if tenant != nil {
				return nil, errors.New("multiple existing tenants match the local setup name")
			}
			tenant = candidate
		}
		if tenant == nil {
			tenant, err = tenants.CreateTenant(call("local-tenant-create"), &accessv1.CreateTenantRequest{Name: "Local tenant", OwnerUserId: tenantUserID, OwnerEmail: tenantEmail, RequestId: "local-tenant-create", SalesScope: "default"})
			if err != nil {
				return nil, err
			}
		}
	}
	if tenant.GetStatus() == accessv1.TenantStatus_TENANT_STATUS_ACTIVE {
		return tenant, nil
	}
	if tenant.GetStatus() != accessv1.TenantStatus_TENANT_STATUS_PENDING {
		return nil, errors.New("local tenant exists in an unsupported state")
	}
	return tenants.ActivateTenant(call("local-tenant-activate"), &accessv1.ActivateTenantRequest{Id: tenant.GetId(), Version: tenant.GetVersion()})
}

func ensureModule(ctx context.Context, catalog commercialv1.ModuleCatalogApplicationClient, call func(string) context.Context, code, name string) error {
	module, err := catalog.GetModule(call("local-module-get-"+code), &commercialv1.GetModuleRequest{ModuleCode: code})
	if status.Code(err) == codes.NotFound {
		module, err = catalog.CreateModule(call("local-module-create-"+code), &commercialv1.CreateModuleRequest{RequestId: "local-module-create-" + code, ModuleCode: code, Name: name, Reason: "local console setup"})
	}
	if err != nil {
		return err
	}
	if module.GetTechnicalStatus() != commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY {
		module, err = catalog.SetModuleTechnicalStatus(call("local-module-ready-"+code), &commercialv1.SetModuleTechnicalStatusRequest{RequestId: "local-module-ready-" + code, ModuleCode: code, Version: module.GetVersion(), TechnicalStatus: commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY, Reason: "local console setup"})
		if err != nil {
			return err
		}
	}
	if module.GetSalesStatus() != commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE {
		_, err = catalog.SetModuleSalesStatus(call("local-module-sellable-"+code), &commercialv1.SetModuleSalesStatusRequest{RequestId: "local-module-sellable-" + code, ModuleCode: code, Version: module.GetVersion(), SalesStatus: commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE, Reason: "local console setup"})
	}
	return err
}

func localPlanTerms() *commercialv1.PlanTerms {
	return &commercialv1.PlanTerms{Modules: []*commercialv1.PlanModule{{ModuleCode: "access-management", CapabilityCodes: []string{"tenant.lifecycle", "tenant.member.lifecycle", "tenant.role.permission"}, Quotas: []*commercialv1.PlanQuota{{Key: "tenant.members", Unlimited: true}}, Fields: []*commercialv1.PlanField{{Key: "member.profile", Action: "read", Mode: "masked"}, {Key: "member.profile", Action: "write", Mode: "allow"}, {Key: "member.profile", Action: "export", Mode: "deny"}}}, {ModuleCode: "device-operations", CapabilityCodes: []string{"device.lifecycle", "device.transfer"}, Quotas: []*commercialv1.PlanQuota{{Key: "tenant.devices", Unlimited: true}}, Fields: []*commercialv1.PlanField{{Key: "device.identity", Action: "read", Mode: "allow"}, {Key: "device.identity", Action: "write", Mode: "allow"}, {Key: "device.identity", Action: "export", Mode: "allow"}}}}, SalesScope: []string{"*"}, ValidityMode: "unlimited"}
}

func validateLocalDSN(dsn string) error {
	config, err := mysql.ParseDSN(dsn)
	if err != nil || config.Net != "tcp" || config.Addr != "127.0.0.1:13316" || config.DBName != "biz_evolution" {
		return errors.New("local setup requires the designated biz_evolution DSN at 127.0.0.1:13316")
	}
	return nil
}

func secret() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func loadCredentials(path string) (credentials, bool, error) {
	var value credentials
	if info, err := os.Stat(path); err == nil && info.Mode().Perm() != 0600 {
		return value, false, errors.New("local credential file must have mode 0600")
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return value, false, nil
	}
	if err != nil {
		return value, false, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, false, fmt.Errorf("read local credentials: %w", err)
	}
	if value.PlatformPassword == "" || value.TenantPassword == "" || value.PlatformToken == "" || value.WorkerToken == "" {
		return value, false, errors.New("local credential file is incomplete")
	}
	return value, true, nil
}
func saveCredentials(path string, value credentials) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0600); err != nil {
		return err
	}
	if err := os.Chmod(temporary, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
