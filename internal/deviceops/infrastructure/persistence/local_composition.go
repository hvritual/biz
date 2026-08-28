package persistence

import (
	"context"

	"github.com/hvritual/biz/internal/deviceops/ports"
	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
)

type LocalTransferRepositories = requestscope.Pair[ports.ScopedDeviceRepository, ports.SiteRepository]

// NewLocalTransferScopeFactory proves the C8.7 local-composition invariant:
// heterogeneous repository ports are constructed over exactly one request-owned
// UnitOfWork. The Application receives typed ports and never a second DB handle.
func NewLocalTransferScopeFactory(database *gorm.DB) (requestscope.ScopeFactory[LocalTransferRepositories], error) {
	unit, err := requestscope.NewGORMFactory(database, nil)
	if err != nil {
		return nil, err
	}
	devices := requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.ScopedDeviceRepository, error) {
		return NewDeviceRepository(transaction)
	})
	sites := requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.SiteRepository, error) {
		return NewSiteRepository(transaction)
	})
	return requestscope.NewFactory(requestscope.FactoryOptions[LocalTransferRepositories]{
		UnitOfWork:   unit,
		Repositories: requestscope.Compose2(devices, sites),
	})
}
