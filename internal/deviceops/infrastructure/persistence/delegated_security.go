package persistence

import (
	"context"
	"errors"
	"strings"

	"github.com/hvritual/biz/internal/deviceops/ports"
	"gorm.io/gorm"
)

type DeviceOwnerResolver struct {
	database *gorm.DB
}

func NewDeviceOwnerResolver(database *gorm.DB) (*DeviceOwnerResolver, error) {
	if database == nil {
		return nil, errors.New("deviceops persistence: database is required")
	}
	return &DeviceOwnerResolver{database: database}, nil
}

func (resolver *DeviceOwnerResolver) ResolveDeviceOwner(ctx context.Context, deviceID string) (string, error) {
	if resolver == nil || resolver.database == nil || strings.TrimSpace(deviceID) == "" {
		return "", ports.ErrNotFound
	}
	var record DevicePORecord
	if err := resolver.database.WithContext(ctx).Where("id = ?", strings.TrimSpace(deviceID)).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ports.ErrNotFound
		}
		return "", err
	}
	if strings.TrimSpace(record.TenantID) == "" {
		return "", ports.ErrNotFound
	}
	return strings.TrimSpace(record.TenantID), nil
}
