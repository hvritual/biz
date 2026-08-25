package deviceops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"gorm.io/gorm/clause"
)

const ModuleName = "deviceops"

type Module struct {
	dependencies Dependencies
	service      *Service
	auth         *Authenticator

	mu       sync.RWMutex
	server   *http.Server
	listener net.Listener
	serveErr error
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
	service, err := NewService(dependencies.PrimaryDatabase)
	if err != nil {
		return nil, err
	}
	auth, err := NewAuthenticator(dependencies.PrimaryDatabase)
	if err != nil {
		return nil, err
	}
	return &Module{dependencies: dependencies, service: service, auth: auth}, nil
}

func (*Module) Name() string { return ModuleName }

func (module *Module) Start(ctx context.Context) error {
	if module == nil {
		return errors.New("deviceops: module is nil")
	}
	if module.dependencies.Config.AutoMigrate {
		if err := module.dependencies.PrimaryDatabase.WithContext(ctx).AutoMigrate(
			&Tenant{}, &User{}, &Membership{}, &Role{}, &MemberRole{}, &RolePermission{}, &MemberSite{}, &APIToken{}, &Site{}, &Device{},
		); err != nil {
			return fmt.Errorf("deviceops: migrate: %w", err)
		}
	}
	if module.dependencies.Config.Bootstrap.Token != "" {
		if err := module.bootstrap(ctx); err != nil {
			return fmt.Errorf("deviceops: bootstrap: %w", err)
		}
	}
	listener, err := net.Listen("tcp", module.dependencies.Config.ListenAddress)
	if err != nil {
		return fmt.Errorf("deviceops: listen: %w", err)
	}
	server := &http.Server{
		Handler:           NewHTTPHandler(module.service, module.auth, module.dependencies.PrimaryDatabase),
		ReadHeaderTimeout: 5 * time.Second,
	}
	module.mu.Lock()
	module.listener = listener
	module.server = server
	module.serveErr = nil
	module.mu.Unlock()
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			module.mu.Lock()
			module.serveErr = err
			module.mu.Unlock()
			module.dependencies.Logger.Errorf("deviceops HTTP server: %v", err)
		}
	}()
	return nil
}

func (module *Module) Health(ctx context.Context) error {
	if module == nil || module.dependencies.PrimaryDatabase == nil {
		return errors.New("deviceops: module database unavailable")
	}
	sqlDB, err := module.dependencies.PrimaryDatabase.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.server == nil || module.listener == nil {
		return errors.New("deviceops: HTTP server is not started")
	}
	return module.serveErr
}

func (module *Module) Shutdown(ctx context.Context) error {
	if module == nil {
		return nil
	}
	module.mu.RLock()
	server := module.server
	module.mu.RUnlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

func (module *Module) bootstrap(ctx context.Context) error {
	config := module.dependencies.Config.Bootstrap
	database := module.dependencies.PrimaryDatabase.WithContext(ctx)
	tenant := Tenant{ID: config.TenantID, Name: config.TenantName, Status: "active"}
	user := User{ID: config.UserID, Email: config.Email, Status: "active"}
	membership := Membership{TenantID: config.TenantID, UserID: config.UserID, Status: "active"}
	roleID := config.TenantID + ":owner"
	role := Role{ID: roleID, TenantID: config.TenantID, Name: "owner", Status: "active"}
	memberRole := MemberRole{TenantID: config.TenantID, UserID: config.UserID, RoleID: roleID}
	permissions := []RolePermission{
		{TenantID: config.TenantID, RoleID: roleID, Permission: PermissionDeviceRead, DataScope: DataScopeAll},
		{TenantID: config.TenantID, RoleID: roleID, Permission: PermissionDeviceCreate, DataScope: DataScopeAll},
		{TenantID: config.TenantID, RoleID: roleID, Permission: PermissionDeviceUpdate, DataScope: DataScopeAll},
		{TenantID: config.TenantID, RoleID: roleID, Permission: PermissionDeviceDelete, DataScope: DataScopeAll},
	}
	hash := sha256.Sum256([]byte(config.Token))
	token := APIToken{TokenHash: hex.EncodeToString(hash[:]), TenantID: config.TenantID, UserID: config.UserID}
	for _, value := range []any{&tenant, &user, &membership, &role, &memberRole, &token} {
		if err := database.Clauses(clause.OnConflict{DoNothing: true}).Create(value).Error; err != nil {
			return err
		}
	}
	for index := range permissions {
		if err := database.Clauses(clause.OnConflict{DoNothing: true}).Create(&permissions[index]).Error; err != nil {
			return err
		}
	}
	return nil
}
