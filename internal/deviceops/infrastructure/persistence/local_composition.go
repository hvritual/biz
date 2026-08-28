package persistence

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/deviceops/ports"
	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
)

type LocalTransferRepositories = requestscope.Pair[ports.ScopedDeviceRepository, ports.SiteRepository]

// NewLocalTransferRepositoryFactory composes heterogeneous repository ports over
// the UnitOfWork already owned by the root C9.7 ExecutionScope.
func NewLocalTransferRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[LocalTransferRepositories], error) {
	if database == nil {
		return nil, errors.New("deviceops persistence: database is required")
	}
	devices := requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.ScopedDeviceRepository, error) {
		return NewDeviceRepository(transaction)
	})
	sites := requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.SiteRepository, error) {
		return NewSiteRepository(transaction)
	})
	return requestscope.Compose2(devices, sites), nil
}

// NewLocalTransferScopeFactory remains for pre-C9.7 compatibility tests.
func NewLocalTransferScopeFactory(database *gorm.DB) (requestscope.ScopeFactory[LocalTransferRepositories], error) {
	unit, err := requestscope.NewGORMFactory(database, nil)
	if err != nil {
		return nil, err
	}
	repositories, err := NewLocalTransferRepositoryFactory(database)
	if err != nil {
		return nil, err
	}
	return requestscope.NewFactory(requestscope.FactoryOptions[LocalTransferRepositories]{UnitOfWork: unit, Repositories: repositories})
}
