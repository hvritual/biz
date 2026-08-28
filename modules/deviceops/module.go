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
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	devicerest "github.com/hvritual/biz/internal/deviceops/transport/rest"
	devicerpc "github.com/hvritual/biz/internal/deviceops/transport/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"yunka.io/framework/core"
	"yunka.io/framework/execution"
	"yunka.io/framework/execution/idempotencygorm"
	"yunka.io/framework/modules"
	"yunka.io/framework/operation"
	"yunka.io/framework/requestscope"
	"yunka.io/framework/workflow/saga"
	"yunka.io/gateway/authz"
)

const Name = "deviceops"

type Config struct {
	HTTPListenAddress string
	GRPCListenAddress string
}

type Dependencies struct {
	Database *gorm.DB
	Config   Config
}

type Module struct {
	dependencies Dependencies

	mu           sync.Mutex
	httpServer   *http.Server
	grpcServer   *grpc.Server
	httpListener net.Listener
	grpcListener net.Listener
	serveErr     error
}

func New(dependencies Dependencies) (*Module, error) {
	if dependencies.Database == nil {
		return nil, errors.New("deviceops: database is required")
	}
	if strings.TrimSpace(dependencies.Config.HTTPListenAddress) == "" {
		dependencies.Config.HTTPListenAddress = "127.0.0.1:0"
	}
	if strings.TrimSpace(dependencies.Config.GRPCListenAddress) == "" {
		dependencies.Config.GRPCListenAddress = "127.0.0.1:0"
	}
	return &Module{dependencies: dependencies}, nil
}

func (module *Module) Descriptor() modules.Descriptor {
	return modules.Descriptor{Name: Name}
}

func (module *Module) Start(_ core.Context) error {
	if module == nil {
		return errors.New("deviceops: module is nil")
	}
	database := module.dependencies.Database
	accessStore, err := accesspersistence.New(database)
	if err != nil {
		return err
	}
	if err := accessStore.EnsureSchema(context.Background()); err != nil {
		return err
	}
	if err := devicepersistence.EnsureSchema(database); err != nil {
		return err
	}
	repositories, err := devicepersistence.NewScopedRepositoryFactory(database)
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
	authorizer, err := authz.NewGrantAuthorizer(accessStore)
	if err != nil {
		return err
	}
	guard, err := devicesecurity.NewGuard(accessStore)
	if err != nil {
		return err
	}
	guards := authz.NewStaticGuardResolver(map[authz.OperationID]authz.OperationGuard{
		authz.OperationID("device.list"):   guard,
		authz.OperationID("device.get"):    guard,
		authz.OperationID("device.create"): guard,
		authz.OperationID("device.update"): guard,
		authz.OperationID("device.delete"): guard,
		authz.OperationID("device.transfer"): guard,
	})
	security, err := authz.NewExecutionSecurity(authorizer, guards)
	if err != nil {
		return err
	}
	transactions, err := requestscope.NewGORMExecutionFactory(database)
	if err != nil {
		return err
	}
	idempotencyStore, err := idempotencygorm.NewStore(database, idempotencygorm.Options{})
	if err != nil {
		return err
	}
	if err := idempotencyStore.EnsureSchema(context.Background()); err != nil {
		return err
	}
	idempotency, err := execution.NewIdempotencyCoordinator(idempotencyStore)
	if err != nil {
		return err
	}
	executor := operation.NewExecutorWithOptions(security, operation.ExecutorOptions{
		Transactions: transactions,
		Idempotency:  idempotency,
	})
	outboxStore, err := saga.NewGORMOutboxStore(database)
	if err != nil {
		return err
	}
	provisioningService, err := deviceapp.NewProvisioningService(repositories, saga.NewStager(outboxStore))
	if err != nil {
		return err
	}
	_ = provisioningService // pressure-only Application; exercised directly by integration tests.
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
	// site_management is an internal-only Application. Its canonical Operation
	// is reachable only through generated child capabilities, not gRPC.
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
		module.mu.Lock()
		module.serveErr = err
		module.mu.Unlock()
	}
}

func (module *Module) serveGRPC() {
	if err := module.grpcServer.Serve(module.grpcListener); err != nil {
		module.mu.Lock()
		module.serveErr = err
		module.mu.Unlock()
	}
}

func (module *Module) Stop(_ core.Context) error {
	if module == nil {
		return nil
	}
	module.mu.Lock()
	httpServer, grpcServer := module.httpServer, module.grpcServer
	module.mu.Unlock()
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	if httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (module *Module) HTTPAddress() string {
	module.mu.Lock()
	defer module.mu.Unlock()
	if module.httpListener == nil {
		return ""
	}
	return module.httpListener.Addr().String()
}

func (module *Module) GRPCAddress() string {
	module.mu.Lock()
	defer module.mu.Unlock()
	if module.grpcListener == nil {
		return ""
	}
	return module.grpcListener.Addr().String()
}

func (module *Module) ServeError() error {
	module.mu.Lock()
	defer module.mu.Unlock()
	return module.serveErr
}

func (module *Module) healthHTTP(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("ok"))
}

func httpAuthentication(store *accesspersistence.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		token := strings.TrimSpace(strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "))
		principal, ok := store.ResolveAPIKey(request.Context(), token)
		if !ok {
			http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request.WithContext(authz.WithPrincipal(request.Context(), principal)))
	})
}

func grpcAuthentication(store *accesspersistence.Store) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		incoming, _ := metadata.FromIncomingContext(ctx)
		values := incoming.Get("authorization")
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization")
		}
		token := strings.TrimSpace(strings.TrimPrefix(values[0], "Bearer "))
		principal, ok := store.ResolveAPIKey(ctx, token)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization")
		}
		return handler(authz.WithPrincipal(ctx, principal), req)
	}
}
