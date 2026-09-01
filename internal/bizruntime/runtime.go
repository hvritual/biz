package bizruntime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	generatedassembly "github.com/hvritual/biz/internal/assembly"
	deviceapp "github.com/hvritual/biz/internal/deviceops/application"
	"github.com/hvritual/biz/internal/deviceops/domain"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	devicepolicy "github.com/hvritual/biz/internal/deviceops/policy"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	accessmodule "github.com/hvritual/biz/modules/access"
	"github.com/hvritual/biz/modules/deviceops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"yunka.io/framework/core"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/execution/idempotencygorm"
	"yunka.io/framework/operation"
	"yunka.io/framework/platform"
	"yunka.io/framework/requestscope"
	"yunka.io/framework/runtimecomponent"
	"yunka.io/gateway/authz"
)

type Started struct {
	App          *core.App
	Applications generatedassembly.Applications
	httpAddress  string
	grpcAddress  string
}

func (started *Started) HTTPAddress() string {
	if started == nil { return "" }
	return started.httpAddress
}

func (started *Started) GRPCAddress() string {
	if started == nil { return "" }
	return started.grpcAddress
}

func Bootstrap(ctx context.Context, provider *platform.Provider, config deviceops.Config) (*Started, error) {
	return BootstrapWithOptions(ctx, provider, Options{DeviceOps: config})
}

func BootstrapWithOptions(ctx context.Context, provider *platform.Provider, options Options) (*Started, error) {
	if provider == nil { return nil, errors.New("biz runtime: platform provider is required") }
	if err := options.Validate(); err != nil { return nil, err }
	config := options.DeviceOps
	if ctx == nil { ctx = context.Background() }

	httpListener, err := net.Listen("tcp", config.HTTPListenAddress)
	if err != nil { return nil, fmt.Errorf("biz runtime: HTTP listen: %w", err) }
	grpcListener, err := net.Listen("tcp", config.GRPCListenAddress)
	if err != nil { _ = httpListener.Close(); return nil, fmt.Errorf("biz runtime: gRPC listen: %w", err) }

	authenticator := &runtimeAuthenticator{}
	health := &runtimeHealth{}
	diagnosticsEndpoint := &runtimeDiagnostics{}
	apiMux := http.NewServeMux()
	rootMux := http.NewServeMux()
	rootMux.HandleFunc("GET /healthz", health.handle)
	rootMux.Handle("GET "+diagnosticsPath, diagnosticsEndpoint)
	rootMux.Handle("/v1/", httpAuthentication(authenticator, apiMux))
	httpServer := &http.Server{Handler: rootMux, ReadHeaderTimeout: 5 * time.Second}
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcAuthentication(authenticator)))

	httpComponent, err := runtimecomponent.HTTP(runtimecomponent.HTTPOptions{Name: "http-server", Server: httpServer, Listener: httpListener})
	if err != nil { _ = httpListener.Close(); _ = grpcListener.Close(); return nil, err }
	grpcComponent, err := runtimecomponent.GRPC(runtimecomponent.GRPCOptions{Name: "grpc-server", Server: grpcServer, Listener: grpcListener})
	if err != nil { _ = httpListener.Close(); _ = grpcListener.Close(); return nil, err }

	result, err := generatedassembly.Bootstrap(ctx, generatedassembly.BootstrapOptions{
		Platform: provider,
		BindRuntime: func(bindCtx context.Context, prepared *platform.Provider) (generatedassembly.RuntimeBindings, error) {
			return bindRuntime(bindCtx, prepared, options, authenticator)
		},
		Transports: generatedassembly.TransportBindings{HTTP: apiMux, RPC: grpcServer},
		RuntimeComponents: []core.RuntimeComponent{httpComponent, grpcComponent},
	})
	if err != nil {
		_ = httpServer.Close(); grpcServer.Stop(); _ = httpListener.Close(); _ = grpcListener.Close()
		return nil, err
	}
	health.set(result.App)
	if err := diagnosticsEndpoint.set(result.App); err != nil {
		_ = result.App.Shutdown(ctx)
		return nil, fmt.Errorf("biz runtime: diagnostics: %w", err)
	}
	return &Started{App: result.App, Applications: result.Applications, httpAddress: httpListener.Addr().String(), grpcAddress: grpcListener.Addr().String()}, nil
}

type applicationFactories struct {
	device             *deviceapp.Service
	site               *deviceapp.SiteManagementService
	tenantRepositories requestscope.RepositoryFactory[accessports.TenantRepositories]
}

var _ generatedassembly.ApplicationFactories = applicationFactories{}

func (factory applicationFactories) BuildDeviceopsDeviceManagement(generatedassembly.DeviceopsDeviceManagementDependencies) (deviceapp.DeviceManagementApplication, error) {
	if factory.device == nil { return nil, errors.New("biz runtime: device management application is required") }
	return factory.device, nil
}

func (factory applicationFactories) BuildDeviceopsSiteManagement(generatedassembly.DeviceopsSiteManagementDependencies) (deviceapp.SiteManagementApplication, error) {
	if factory.site == nil { return nil, errors.New("biz runtime: site management application is required") }
	return factory.site, nil
}

func (factory applicationFactories) BuildDeviceopsDeviceTransfer(dependencies generatedassembly.DeviceopsDeviceTransferDependencies) (deviceapp.DeviceTransferApplication, error) {
	return deviceapp.NewCrossApplicationTransferService(dependencies.DeviceopsSiteManagement, dependencies.DeviceopsDeviceManagement)
}

func bindRuntime(ctx context.Context, provider *platform.Provider, options Options, authenticator *runtimeAuthenticator) (generatedassembly.RuntimeBindings, error) {
	config := options.DeviceOps
	deviceContext, err := provider.ForModule(deviceops.GeneratedDescriptor())
	if err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: deviceops capabilities: %w", err) }
	deviceDatabase, err := deviceContext.Databases().GORM("primary")
	if err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: deviceops primary database: %w", err) }

	accessContext, err := provider.ForModule(accessmodule.GeneratedDescriptor())
	if err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: access capabilities: %w", err) }
	accessDatabase, err := accessContext.Databases().GORM("primary")
	if err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: access primary database: %w", err) }

	accessStore, err := accesspersistence.New(accessDatabase)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	if config.AutoMigrate {
		if err := accessStore.AutoMigrate(ctx); err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: access migrate: %w", err) }
		if err := accessStore.EnsurePlatformSchema(ctx); err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: platform IAM migrate: %w", err) }
		if err := devicepersistence.AutoMigrate(ctx, deviceDatabase); err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: domain migrate: %w", err) }
		if err := devicepersistence.EnsureIndexes(deviceDatabase); err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: indexes: %w", err) }
	}
	if bootstrap := config.Bootstrap; bootstrap.Token != "" {
		if err := accessStore.Bootstrap(ctx, accesspersistence.Bootstrap{
			TenantID: bootstrap.TenantID, TenantName: bootstrap.TenantName,
			UserID: bootstrap.UserID, Email: bootstrap.Email, Token: bootstrap.Token,
		}, devicepolicy.Permissions()); err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: bootstrap identity: %w", err) }
		principal, err := accessStore.Authenticate(ctx, bootstrap.Token)
		if err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: bootstrap authenticate: %w", err) }
		sites, err := devicepersistence.NewSiteRepository(deviceDatabase)
		if err != nil { return generatedassembly.RuntimeBindings{}, err }
		trusted := identity.WithPrincipal(ctx, principal)
		if _, err := sites.Get(trusted, bootstrap.SiteID); err != nil {
			value := domain.Site{ID: bootstrap.SiteID, Name: bootstrap.SiteName}
			if err := sites.Create(trusted, &value); err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: bootstrap site: %w", err) }
		}
	}
	if options.PlatformBootstrap.Enabled() {
		if err := accessStore.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{
			Subject: options.PlatformBootstrap.Subject,
			Token: options.PlatformBootstrap.Token,
			Permissions: options.PlatformBootstrap.Permissions,
		}); err != nil {
			return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: platform bootstrap: %w", err)
		}
	}

	grantResolver, err := accesspersistence.NewPrincipalGrantResolver(accessStore)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	grantAuthorizer, err := authz.NewGrantAuthorizerWithResolver(grantResolver)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	guard, err := devicesecurity.NewGuard(accessStore)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	guards := authz.NewStaticGuardResolver(map[authz.OperationID]authz.OperationGuard{
		"device.list": guard, "device.get": guard, "device.create": guard, "device.update": guard,
		"device.delete": guard, "site.validate_transfer_target": guard, "device.transfer": guard,
	})
	security, err := authz.NewExecutionSecurity(grantAuthorizer, guards)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	transactions, err := requestscope.NewGORMExecutionFactory(accessDatabase)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	idempotencyStore, err := idempotencygorm.NewStore(accessDatabase, idempotencygorm.Options{})
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	if config.AutoMigrate {
		if err := idempotencyStore.EnsureSchema(ctx); err != nil { return generatedassembly.RuntimeBindings{}, fmt.Errorf("biz runtime: idempotency migrate: %w", err) }
	}
	idempotency, err := execution.NewIdempotencyCoordinator(idempotencyStore)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	executor := operation.NewExecutorWithOptions(security, operation.ExecutorOptions{Transactions: transactions, Idempotency: idempotency})

	deviceRepositories, err := devicepersistence.NewScopedRepositoryFactory(deviceDatabase)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	tenantRepositories, err := accesspersistence.NewTenantRepositoryFactory(accessDatabase)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	deviceService, err := deviceapp.NewService(deviceRepositories)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	siteService, err := deviceapp.NewSiteManagementService(deviceRepositories)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	authenticator.set(accessStore)
	return generatedassembly.RuntimeBindings{
		Factories: applicationFactories{device: deviceService, site: siteService, tenantRepositories: tenantRepositories},
		Executor: executor,
	}, nil
}

type runtimeAuthenticator struct { mu sync.RWMutex; store *accesspersistence.Store }

func (authenticator *runtimeAuthenticator) set(store *accesspersistence.Store) {
	authenticator.mu.Lock(); authenticator.store = store; authenticator.mu.Unlock()
}

func (authenticator *runtimeAuthenticator) authenticate(ctx context.Context, raw string) (identity.Principal, error) {
	authenticator.mu.RLock(); store := authenticator.store; authenticator.mu.RUnlock()
	if store == nil { return identity.Principal{}, accesspersistence.ErrUnauthorized }
	principal, err := store.AuthenticatePlatform(ctx, raw)
	if err == nil { return principal, nil }
	if !errors.Is(err, accesspersistence.ErrUnauthorized) { return identity.Principal{}, err }
	return store.Authenticate(ctx, raw)
}

type runtimeHealth struct { mu sync.RWMutex; app *core.App }

func (health *runtimeHealth) set(app *core.App) { health.mu.Lock(); health.app = app; health.mu.Unlock() }

func (health *runtimeHealth) handle(writer http.ResponseWriter, request *http.Request) {
	health.mu.RLock(); app := health.app; health.mu.RUnlock()
	if app == nil || !app.Health(request.Context()).Ready { http.Error(writer, "unhealthy", http.StatusServiceUnavailable); return }
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(`{"status":"ok"}`))
}

func parseBearer(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 7 || !strings.EqualFold(value[:7], "Bearer ") { return "" }
	return strings.TrimSpace(value[7:])
}

func httpAuthentication(authenticator *runtimeAuthenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, err := authenticator.authenticate(request.Context(), parseBearer(request.Header.Get("Authorization")))
		if err != nil { http.Error(writer, "Unauthorized", http.StatusUnauthorized); return }
		next.ServeHTTP(writer, request.WithContext(identity.WithPrincipal(request.Context(), principal)))
	})
}

func grpcAuthentication(authenticator *runtimeAuthenticator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		raw := ""
		if values := md.Get("authorization"); len(values) > 0 { raw = parseBearer(values[0]) }
		principal, err := authenticator.authenticate(ctx, raw)
		if err != nil { return nil, status.Error(codes.Unauthenticated, "unauthenticated") }
		return handler(identity.WithPrincipal(ctx, principal), request)
	}
}
