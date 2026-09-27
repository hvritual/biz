package persistence

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
)

type NotificationRouteRecipientDirectory struct{ db *gorm.DB }

var _ ports.NotificationRouteRecipients = (*NotificationRouteRecipientDirectory)(nil)

func NewNotificationRouteRecipientDirectory(db *gorm.DB) (*NotificationRouteRecipientDirectory, error) {
	if db == nil {
		return nil, errors.New("access: notification route directory unavailable")
	}
	return &NotificationRouteRecipientDirectory{db: db}, nil
}

func routeRecipientID(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

func (d *NotificationRouteRecipientDirectory) ResolveNotificationRouteRecipients(ctx context.Context, tenant string, ids []string) ([]ports.NotificationRouteRecipient, error) {
	if d == nil || d.db == nil || !routeRecipientID(tenant) || len(ids) < 1 || len(ids) > 102 {
		return nil, ErrUnauthorized
	}
	normalized := append([]string{}, ids...)
	sort.Strings(normalized)
	for i, id := range normalized {
		if !routeRecipientID(id) || (i > 0 && id == normalized[i-1]) {
			return nil, ErrUnauthorized
		}
	}
	var tenantRow tenantRecord
	if err := d.db.WithContext(ctx).Select("id").Where("BINARY id=? AND status=?", tenant, domain.TenantStatusActive).Take(&tenantRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	var rows []struct{ UserID string }
	if err := d.db.WithContext(ctx).Table("biz_memberships m").
		Select("m.user_id").Joins("JOIN biz_users u ON BINARY u.id=BINARY m.user_id AND u.status=?", "active").
		Where("BINARY m.tenant_id=? AND m.status=? AND m.self_deleted_at IS NULL AND BINARY m.user_id IN ?", tenant, domain.TenantMemberStatusActive, normalized).
		Order("m.user_id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	active := map[string]bool{}
	for _, row := range rows {
		active[row.UserID] = true
	}
	out := make([]ports.NotificationRouteRecipient, 0, len(normalized))
	for _, id := range normalized {
		out = append(out, ports.NotificationRouteRecipient{UserID: id, Active: active[id]})
	}
	return out, nil
}
