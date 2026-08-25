package deviceops

import (
	"errors"

	"gorm.io/gorm"
)

type Repositories struct {
	database *gorm.DB
}

func newRepositories(database *gorm.DB) (Repositories, error) {
	if database == nil {
		return Repositories{}, errors.New("deviceops: repository database is required")
	}
	return Repositories{database: database}, nil
}

func (repos Repositories) applyDeviceScope(plan AccessPlan, permission string) *gorm.DB {
	query := repos.database.Where("tenant_id = ?", plan.Principal.TenantID)
	scope, ok := plan.Permissions[permission]
	if !ok || !scope.Allowed {
		return query.Where("1 = 0")
	}
	if scope.All {
		return query
	}
	if scope.Sites && scope.Self {
		if len(plan.SiteIDs) == 0 {
			return query.Where("created_by = ?", plan.Principal.UserID)
		}
		return query.Where("(site_id IN ? OR created_by = ?)", plan.SiteIDs, plan.Principal.UserID)
	}
	if scope.Sites {
		if len(plan.SiteIDs) == 0 {
			return query.Where("1 = 0")
		}
		return query.Where("site_id IN ?", plan.SiteIDs)
	}
	if scope.Self {
		return query.Where("created_by = ?", plan.Principal.UserID)
	}
	return query.Where("1 = 0")
}

func (repos Repositories) ListDevices(plan AccessPlan) ([]Device, error) {
	var devices []Device
	if err := repos.applyDeviceScope(plan, PermissionDeviceRead).Order("created_at DESC").Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

func (repos Repositories) FindDevice(plan AccessPlan, permission, id string) (Device, error) {
	var device Device
	err := repos.applyDeviceScope(plan, permission).Where("id = ?", id).First(&device).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Device{}, ErrNotFound
	}
	return device, err
}

func (repos Repositories) SiteExists(tenantID, siteID string) (bool, error) {
	var count int64
	if err := repos.database.Model(&Site{}).Where("tenant_id = ? AND id = ?", tenantID, siteID).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 1, nil
}

func (repos Repositories) CreateDevice(device *Device) error {
	return repos.database.Create(device).Error
}

func (repos Repositories) UpdateDevice(plan AccessPlan, current Device, name, siteID string, expectedVersion uint64) (Device, error) {
	updates := map[string]any{"name": name, "site_id": siteID, "version": expectedVersion + 1}
	result := repos.applyDeviceScope(plan, PermissionDeviceUpdate).
		Model(&Device{}).
		Where("id = ? AND version = ?", current.ID, expectedVersion).
		Updates(updates)
	if result.Error != nil {
		return Device{}, result.Error
	}
	if result.RowsAffected != 1 {
		return Device{}, ErrConflict
	}
	return repos.FindDevice(plan, PermissionDeviceUpdate, current.ID)
}

func (repos Repositories) DeleteDevice(plan AccessPlan, current Device, expectedVersion uint64) error {
	result := repos.applyDeviceScope(plan, PermissionDeviceDelete).
		Where("id = ? AND version = ?", current.ID, expectedVersion).
		Delete(&Device{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
