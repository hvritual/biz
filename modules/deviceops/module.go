package deviceops

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
	deviceapp "github.com/hvritual/biz/internal/deviceops/application"
	"github.com/hvritual/biz/internal/deviceops/domain"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	devicepolicy "github.com/hvritual/biz/internal/deviceops/policy"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	devicerest "github.com/hvritual/biz/internal/deviceops/transport/rest"
	devicerpc "github.com/hvritual/biz/internal/deviceops/transport/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/execution/idempotencygorm"
	"yunka.io/framework/operation"
	"yunka.io/framework/requestscope"
	"yunka.io/gateway/authz"
)

const ModuleName = "deviceops"

type Module struct {
	dependencies Dependencies
	mu           sync.RWMutex
	httpServer   *http.Server
	grpcServer   *grpc.Server
	httpListener net.Listener
	grpcListener net.Listener
	serveErr     error
}

func NewModule(dependencies Dependencies) (*Module, error) {
	if dependencies.Logger == nil {
		return nil, errors.New("deviceops: logger is required")
	}
	if dependencies.PrimaryDatabase == nil {
		return nil, errors.New("deviceops: primary database is required")
	}
	if err := dependencies.Config.Validate(); err != nil {
		return nil, err
	}
	return &Module{dependencies: dependencies}, nil
}
func (*Module) Name() string { return ModuleName }

func (module *Module) Start(ctx context.Context) error {
	accessStore, err := accesspersistence.New(module.dependencies.PrimaryDatabase)
	if err != nil {
		return err
	}
	if module.dependencies.Config.AutoMigrate {
		if err := accessStore.AutoMigrate(ctx); err != nil {
			return fmt.Errorf("deviceops: access migrate: %w", err)
		}
		if err := devicepersistence.AutoMigrate(ctx, module.dependencies.PrimaryDatabase); err != nil {
			return fmt.Errorf("deviceops: domain migrate: %w", err)
		}
		if err := devicepersistence.EnsureIndexes(module.dependencies.PrimaryDatabase); err != nil {
			return fmt.Errorf("deviceops: indexes: %w", err)
		}
	}
	if config := module.dependencies.Config.Bootstrap; config.Token != "" {
		if err := accessStore.Bootstrap(ctx, accesspersistence.Bootstrap{TenantID: config.TenantID, TenantName: config.TenantName, UserID: config.UserID, Email: config.Email, Token: config.Token}, devicepolicy.Permissions()); err != nil {
			return fmt.Errorf("deviceops: bootstrap identity: %w", err)
		}
		principal, err := accessStore.Authenticate(ctx, config.Token)
		if err != nil {
			return fmt.Errorf("deviceops: bootstrap authenticate: %w", err)
		}
		siteRepository, err := devicepersistence.NewSiteRepository(module.dependencies.PrimaryDatabase)
		if err != nil {
			return err
		}
		trusted := identity.WithPrincipal(ctx, principal)
		if _, err := siteRepository.Get(trusted, config.SiteID); err != nil {
			site := domain.Site{ID: config.SiteID, Name: config.SiteName}
			if err := siteRepository.Create(trusted, &site); err != nil {
				return fmt.Errorf("deviceops: bootstrap site: %w", err)
			}
		}
	}
	grantAuthorizer, err := authz.NewGrantAuthorizer(accessStore)
	if err != nil {
		return err
	}
	guard, err := devicesecurity.NewGuard(accessStore)
	if err != nil {
		return err
	}
	guards := authz.NewStaticGuardResolver(map[authz.OperationID]authz.OperationGuard{
		authz.OperationID("device.list"):                    guard,
		authz.OperationID("device.get"):                     guard,
		authz.OperationID("device.create"):                  guard,
		authz.OperationID("device.update"):                  guard,
		authz.OperationID("device.delete"):                  guard,
		authz.OperationID("site.validate_transfer_target"): guard,
		authz.OperationID("device.transfer"):                guard,
	})
	security, err := authz.NewExecutionSecurity(grantAuthorizer, guards)
	if err != nil {
		return err
	}
	transactions, err := requestscope.NewGORMExecutionFactory(module.dependencies.PrimaryDatabase)
	if err != nil {
		return err
	}
	idempotencyStore, err := idempotencygorm.NewStore(module.dependencies.PrimaryDatabase, idempotencygorm.Options{})
	if err != nil {
		return err
	}
	if module.dependencies.Config.AutoMigrate {
		if err := idempotencyStore.EnsureSchema(ctx); err != nil {
			return fmt.Errorf("deviceops: idempotency migrate: %w", err)
		}
	}
	idempotency, err := execution.NewIdempotencyCoordinator(idempotencyStore)
	if err != nil {
		return err
	}
	executor := operation.NewExecutorWithOptions(security, operation.ExecutorOptions{Transactions: transactions, Idempotency: idempotency})
	repositories, err := devicepersistence.NewScopedRepositoryFactory(module.dependencies.PrimaryDatabase)
	if err != nil {
		return err
	}
	deviceService, err := deviceapp.NewService(repositories)
	if err != nil {
		return err
	}
	siteService, err := deviceapp.NewSiteManagementService(repositories)
	if err != nil {
		return err
	}
	siteCapability, err := deviceapp.NewDeviceopsSiteManagementChildCapability(siteService, executor)
	if err != nil {
		return err
	}
	deviceCapability, err := deviceapp.NewDeviceopsDeviceManagementChildCapability(deviceService, executor)
	if err != nil {
		return err
	}
	transferService, err := deviceapp.NewCrossApplicationTransferService(siteCapability, deviceCapability)
	if err != nil {
		return err
	}

	apiMux := http.NewServeMux()
	if err := devicerest.RegisterDeviceManagementOperationExecutor(apiMux, deviceService, executor); err != nil {
		return err
	}
	if err := devicerest.RegisterDeviceTransferOperationExecutor(apiMux, transferService, executor); err != nil {
		return err
	}
	rootMux := http.NewServeMux()
	rootMux.HandleFunc("GET /healthz", module.healthHTTP)
	rootMux.Handle("/v1/", httpAuthentication(accessStore, apiMux))
	httpListener, err := net.Listen("tcp", module.dependencies.Config.HTTPListenAddress)
	if err != nil {
		return fmt.Errorf("deviceops: HTTP listen: %w", err)
	}
	httpServer := &http.Server{Handler: rootMux, ReadHeaderTimeout: 5 * time.Second}

	grpcListener, err := net.Listen("tcp", module.dependencies.Config.GRPCListenAddress)
	if err != nil {
		_ = httpListener.Close()
		return fmt.Errorf("deviceops: gRPC listen: %w", err)
	}
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcAuthentication(accessStore)))
	if err := devicerpc.RegisterDeviceManagementOperationExecutor(grpcServer, deviceService, executor); err != nil {
		_ = httpListener.Close()
		_ = grpcListener.Close()
		return err
	}
	if err := devicerpc.RegisterSiteManagementOperationExecutor(grpcServer, siteService, executor); err != nil {
		_ = httpListener.Close()
		_ = grpcListener.Close()
		return err
	}
	if err := devicerpc.RegisterDeviceTransferOperationExecutor(grpcServer, transferService, executor); err != nil {
		_ = httpListener.Close()
		_ = grpcListener.Close()
		return err
	}

	module.mu.Lock()
	module.httpServer = httpServer
	module.grpcServer = grpcServer
	module.httpListener = httpListener
	module.grpcListener = grpcListener
	module.serveErr = nil
	module.mu.Unlock()
	go module.serveHTTP()
	go module.serveGRPC()
	return nil
}

func (module *Module) serveHTTP() {
	if err := module.httpServer.Serve(module.httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		module.recordServeError(err)
	}
}
func (module *Module) serveGRPC() {
	if err := module.grpcServer.Serve(module.grpcListener); err != nil {
		module.recordServeError(err)
	}
}
func (module *Module) recordServeError(err error) {
	module.mu.Lock()
	if module.serveErr == nil {
		module.serveErr = err
	}
	module.mu.Unlock()
	module.dependencies.Logger.Errorf("deviceops server: %v", err)
}

func (module *Module) healthHTTP(writer http.ResponseWriter, request *http.Request) {
	if err := module.Health(request.Context()); err != nil {
		http.Error(writer, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(`{"status":"ok"}`))
}

func (module *Module) Health(ctx context.Context) error {
	sqlDB, err := module.dependencies.PrimaryDatabase.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.httpListener == nil || module.grpcListener == nil {
		return errors.New("deviceops: servers not started")
	}
	return module.serveErr
}

func (module *Module) HTTPAddress() string {
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.httpListener == nil {
		return ""
	}
	return module.httpListener.Addr().String()
}
func (module *Module) GRPCAddress() string {
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.grpcListener == nil {
		return ""
	}
	return module.grpcListener.Addr().String()
}

func (module *Module) Shutdown(ctx context.Context) error {
	module.mu.RLock()
	httpServer, grpcServer := module.httpServer, module.grpcServer
	module.mu.RUnlock()
	var httpErr error
	if httpServer != nil {
		httpErr = httpServer.Shutdown(ctx)
	}
	if grpcServer != nil {
		done := make(chan struct{})
		go func() { grpcServer.GracefulStop(); close(done) }()
		select {
		case <-done:
		case <-ctx.Done():
			grpcServer.Stop()
		}
	}
	return httpErr
}

func parseBearer(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 7 || !strings.EqualFold(value[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}
func httpAuthentication(accessStore *accesspersistence.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := accessStore.Authenticate(r.Context(), parseBearer(r.Header.Get("Authorization")))
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(identity.WithPrincipal(r.Context(), principal)))
	})
}
func grpcAuthentication(accessStore *accesspersistence.Store) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		raw := ""
		if values := md.Get("authorization"); len(values) > 0 {
			raw = parseBearer(values[0])
		}
		principal, err := accessStore.Authenticate(ctx, raw)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		return handler(identity.WithPrincipal(ctx, principal), request)
	}
}
