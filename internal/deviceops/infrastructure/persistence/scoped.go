package persistence

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
)

func applyDeviceDataScope(query *gorm.DB, scope ports.DeviceScope) *gorm.DB {
	if scope.All {
		return query
	}
	if scope.Sites && scope.Self {
		if len(scope.SiteIDs) == 0 {
			return query.Where("created_by = ?", scope.UserID)
		}
		return query.Where("(site_id IN ? OR created_by = ?)", scope.SiteIDs, scope.UserID)
	}
	if scope.Sites {
		if len(scope.SiteIDs) == 0 {
			return query.Where("1 = 0")
		}
		return query.Where("site_id IN ?", scope.SiteIDs)
	}
	if scope.Self {
		return query.Where("created_by = ?", scope.UserID)
	}
	return query.Where("1 = 0")
}

func (repository *DeviceRepository) ListScoped(ctx context.Context, scope ports.DeviceScope) ([]domain.Device, error) {
	query, err := repository.scoped(ctx)
	if err != nil {
		return nil, err
	}
	var rows []DevicePORecord
	if err := applyDeviceDataScope(query, scope).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Device, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.Domain())
	}
	return result, nil
}

func (repository *DeviceRepository) GetScoped(ctx context.Context, scope ports.DeviceScope, id string) (domain.Device, error) {
	query, err := repository.scoped(ctx)
	if err != nil {
		return domain.Device{}, err
	}
	var row DevicePORecord
	if err := applyDeviceDataScope(query, scope).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Device{}, ports.ErrNotFound
		}
		return domain.Device{}, err
	}
	return row.Domain(), nil
}

func NewScopedScopeFactory(database *gorm.DB) (requestscope.ScopeFactory[ports.ScopedRepositories], error) {
	unit, err := requestscope.NewGORMFactory(database, nil)
	if err != nil {
		return nil, err
	}
	return requestscope.NewFactory(requestscope.FactoryOptions[ports.ScopedRepositories]{
		UnitOfWork: unit,
		Repositories: requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.ScopedRepositories, error) {
			devices, err := NewDeviceRepository(transaction)
			if err != nil {
				return ports.ScopedRepositories{}, err
			}
			sites, err := NewSiteRepository(transaction)
			if err != nil {
				return ports.ScopedRepositories{}, err
			}
			return ports.ScopedRepositories{Device: devices, Site: sites}, nil
		}),
	})
}

func EnsureIndexes(database *gorm.DB) error {
	if database == nil {
		return errors.New("deviceops persistence: database is required")
	}
	if !database.Migrator().HasIndex(&DevicePORecord{}, "uniq_deviceops_tenant_serial") {
		if err := database.Exec("CREATE UNIQUE INDEX uniq_deviceops_tenant_serial ON biz_deviceops_device (tenant_id, serial)").Error; err != nil {
			return err
		}
	}
	return nil
}
