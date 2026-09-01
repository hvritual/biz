package persistence

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
)

type TenantRepository struct {
	database *gorm.DB
}

func NewTenantRepository(database *gorm.DB) (*TenantRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: tenant database is required")
	}
	return &TenantRepository{database: database}, nil
}

func (repository *TenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	if repository == nil || repository.database == nil || tenant == nil {
		return errors.New("access persistence: tenant repository unavailable")
	}
	row := tenantRecord{
		ID: tenant.ID, Name: tenant.Name, Status: tenant.Status, Version: tenant.Version,
		CreatedAt: tenant.CreatedAt, UpdatedAt: tenant.UpdatedAt,
	}
	return repository.database.WithContext(ctx).Create(&row).Error
}

func (repository *TenantRepository) Get(ctx context.Context, id string) (domain.Tenant, error) {
	if repository == nil || repository.database == nil {
		return domain.Tenant{}, errors.New("access persistence: tenant repository unavailable")
	}
	var row tenantRecord
	if err := repository.database.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Tenant{}, ports.ErrTenantNotFound
		}
		return domain.Tenant{}, err
	}
	return row.domain(), nil
}

func (repository *TenantRepository) List(ctx context.Context) ([]domain.Tenant, error) {
	if repository == nil || repository.database == nil {
		return nil, errors.New("access persistence: tenant repository unavailable")
	}
	var rows []tenantRecord
	if err := repository.database.WithContext(ctx).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Tenant, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.domain())
	}
	return result, nil
}

func (repository *TenantRepository) Update(ctx context.Context, tenant *domain.Tenant, expectedVersion uint64) error {
	if repository == nil || repository.database == nil || tenant == nil || expectedVersion == 0 {
		return errors.New("access persistence: tenant update requires repository, value and version")
	}
	result := repository.database.WithContext(ctx).Model(&tenantRecord{}).
		Where("id = ? AND version = ?", tenant.ID, expectedVersion).
		Updates(map[string]any{
			"name": tenant.Name,
			"status": tenant.Status,
			"updated_at": tenant.UpdatedAt,
			"version": gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := repository.database.WithContext(ctx).Model(&tenantRecord{}).Where("id = ?", tenant.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ports.ErrTenantNotFound
		}
		return ports.ErrTenantConflict
	}
	tenant.Version = expectedVersion + 1
	return nil
}

func (row tenantRecord) domain() domain.Tenant {
	return domain.Tenant{
		ID: row.ID,
		Name: row.Name,
		Status: row.Status,
		Version: row.Version,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func NewTenantRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[ports.TenantRepositories], error) {
	if database == nil {
		return nil, errors.New("access persistence: database is required")
	}
	return requestscope.GORMRepositories(func(_ context.Context, transaction *gorm.DB) (ports.TenantRepositories, error) {
		tenant, err := NewTenantRepository(transaction)
		if err != nil {
			return ports.TenantRepositories{}, err
		}
		return ports.TenantRepositories{Tenant: tenant}, nil
	}), nil
}
