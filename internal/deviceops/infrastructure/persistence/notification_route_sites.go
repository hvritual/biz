package persistence

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/deviceops/ports"
	"gorm.io/gorm"
)

type NotificationRouteGroupDirectory struct{ db *gorm.DB }

var _ ports.NotificationRouteGroups = (*NotificationRouteGroupDirectory)(nil)

func NewNotificationRouteGroupDirectory(db *gorm.DB) (*NotificationRouteGroupDirectory, error) {
	if db == nil {
		return nil, ports.ErrNotificationSiteDirectoryUnavailable
	}
	return &NotificationRouteGroupDirectory{db: db}, nil
}

func (d *NotificationRouteGroupDirectory) NotificationRouteGroupActive(ctx context.Context, tenant, group string) (bool, error) {
	if d == nil || d.db == nil || !notificationSiteID(tenant) || !notificationSiteID(group) {
		return false, ports.ErrNotificationSiteQueryInvalid
	}
	var row struct{ ID string }
	err := d.db.WithContext(ctx).Table("biz_deviceops_site").Select("id").
		Where("BINARY tenant_id=? AND BINARY id=? AND deleted_at IS NULL", tenant, group).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.ID == group, nil
}
