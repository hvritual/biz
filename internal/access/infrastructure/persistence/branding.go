package persistence

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
)

func (repository *TenantRepository) GetBranding(ctx context.Context, tenantID string) (domain.TenantBranding, error) {
	var row tenantRecord
	if err := repository.database.WithContext(ctx).Select("id", "brand_preset", "brand_primary", "version", "updated_at").Where("id = ?", tenantID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.TenantBranding{}, ports.ErrTenantProfileNotFound
		}
		return domain.TenantBranding{}, err
	}
	// Rows predating the migration inherit the documented default; malformed saved values do not.
	if row.BrandPreset == "" && row.BrandPrimary == "" {
		row.BrandPreset = "blue"
	}
	preset, primary, err := domain.NormalizeTenantBranding(row.BrandPreset, row.BrandPrimary)
	if err != nil {
		return domain.TenantBranding{}, err
	}
	return domain.TenantBranding{TenantID: row.ID, Preset: preset, Primary: primary, Version: row.Version, UpdatedAt: row.UpdatedAt}, nil
}

func (repository *TenantRepository) CanManageBranding(ctx context.Context, tenantID, userID string) (bool, error) {
	var count int64
	err := repository.database.WithContext(ctx).Table("biz_memberships AS m").
		Joins("JOIN biz_member_roles AS mr ON mr.tenant_id = m.tenant_id AND mr.user_id = m.user_id").
		Joins("JOIN biz_roles AS r ON r.tenant_id = mr.tenant_id AND r.id = mr.role_id AND r.status = ?", domain.TenantRoleStatusActive).
		Joins("JOIN biz_permission_grants AS pg ON pg.tenant_id = r.tenant_id AND pg.role_id = r.id").
		Where("m.tenant_id = ? AND m.user_id = ? AND m.status = ? AND pg.permission = ? AND pg.scope = ?", tenantID, userID, domain.TenantMemberStatusActive, "tenant.organization.manage", "all").Count(&count).Error
	return count > 0, err
}

func (repository *TenantRepository) UpdateBranding(ctx context.Context, value *domain.TenantBranding, expectedVersion uint64) error {
	if value == nil || expectedVersion == 0 {
		return domain.ErrInvalidTenantBranding
	}
	result := repository.database.WithContext(ctx).Model(&tenantRecord{}).
		Where("id = ? AND version = ?", value.TenantID, expectedVersion).
		Updates(map[string]any{"brand_preset": value.Preset, "brand_primary": value.Primary, "version": gorm.Expr("version + 1"), "updated_at": value.UpdatedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ports.ErrTenantProfileConflict
	}
	value.Version = expectedVersion + 1
	return nil
}

var _ ports.TenantBrandingRepository = (*TenantRepository)(nil)
