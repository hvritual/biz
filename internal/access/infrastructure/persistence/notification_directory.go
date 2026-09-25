package persistence

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/core/identity"
)

// NotificationRecipientDirectory borrows the current root transaction. All SQL
// remains in Access; Notification receives only IDs and display names.
type NotificationRecipientDirectory struct{ db *gorm.DB }

var _ ports.NotificationRecipients = (*NotificationRecipientDirectory)(nil)

func NewNotificationRecipientDirectory(db *gorm.DB) (*NotificationRecipientDirectory, error) {
	if db == nil {
		return nil, errors.New("access: notification directory unavailable")
	}
	return &NotificationRecipientDirectory{db: db}, nil
}
func notificationDirectoryCaller(ctx context.Context, tenant string) (identity.Principal, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.TenantID == "" || p.TenantID != tenant || p.UserID == "" {
		return identity.Principal{}, ErrUnauthorized
	}
	return p, nil
}

// LockCaller keeps tenant/user/membership eligibility valid until a notification
// configuration transaction completes. Call before resource/contact row locks.
func (d *NotificationRecipientDirectory) LockCaller(ctx context.Context, tenant string, lock bool) error {
	p, err := notificationDirectoryCaller(ctx, tenant)
	if err != nil {
		return err
	}
	query := func() *gorm.DB {
		tx := d.db.WithContext(ctx)
		if lock {
			tx = tx.Clauses(clause.Locking{Strength: "SHARE"})
		}
		return tx
	}
	if lock {
		if _, ok := d.db.Statement.ConnPool.(gorm.TxCommitter); !ok {
			return errors.New("access: root transaction required for notification locks")
		}
	}
	var t tenantRecord
	if err := query().Select("id", "status").Where("BINARY id = ? AND status = ?", tenant, domain.TenantStatusActive).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUnauthorized
		}
		return err
	}
	var u userRecord
	if err := query().Select("id", "status").Where("BINARY id = ? AND status = ?", p.UserID, "active").First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUnauthorized
		}
		return err
	}
	var m membershipRecord
	if err := query().Select("tenant_id", "user_id", "status").Where("BINARY tenant_id = ? AND BINARY user_id = ? AND status = ? AND self_deleted_at IS NULL", tenant, p.UserID, domain.TenantMemberStatusActive).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUnauthorized
		}
		return err
	}
	return nil
}

func (d *NotificationRecipientDirectory) query(ctx context.Context, tenant string) *gorm.DB {
	return d.db.WithContext(ctx).Table("biz_memberships m").Joins("JOIN biz_users u ON u.id = m.user_id AND u.status = ?", "active").Where("BINARY m.tenant_id = ? AND m.status = ? AND m.self_deleted_at IS NULL", tenant, domain.TenantMemberStatusActive)
}
func (d *NotificationRecipientDirectory) List(ctx context.Context, tenant, query string, page, size int) ([]ports.NotificationRecipient, uint64, error) {
	if _, err := notificationDirectoryCaller(ctx, tenant); err != nil {
		return nil, 0, err
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return nil, 0, errors.New("access: invalid directory pagination")
	}
	build := func() *gorm.DB {
		q := d.query(ctx, tenant)
		if query != "" {
			q = q.Where("LOCATE(?, m.name) > 0 OR LOCATE(?, m.user_id) > 0", query, query)
		}
		return q
	}
	var count int64
	if err := build().Count(&count).Error; err != nil {
		return nil, 0, err
	}
	var rows []struct {
		UserID string
		Name   string
	}
	if err := build().Select("m.user_id, m.name").Order("m.name ASC, m.user_id ASC").Limit(size).Offset((page - 1) * size).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]ports.NotificationRecipient, 0, len(rows))
	for _, row := range rows {
		name := row.Name
		if name == "" {
			name = row.UserID
		}
		out = append(out, ports.NotificationRecipient{UserID: row.UserID, Name: name})
	}
	return out, uint64(count), nil
}
func (d *NotificationRecipientDirectory) Validate(ctx context.Context, tenant string, ids []string, lock bool) error {
	if _, err := notificationDirectoryCaller(ctx, tenant); err != nil {
		return err
	}
	if lock {
		if _, ok := d.db.Statement.ConnPool.(gorm.TxCommitter); !ok {
			return errors.New("access: root transaction required for notification locks")
		}
	}
	if len(ids) < 1 || len(ids) > 102 {
		return ErrUnauthorized
	}
	sorted := append([]string{}, ids...)
	sort.Strings(sorted)
	for i, id := range sorted {
		if id == "" || (i > 0 && id == sorted[i-1]) {
			return ErrUnauthorized
		}
		q := d.query(ctx, tenant).Select("m.user_id").Where("BINARY m.user_id = ?", id)
		if lock {
			q = q.Clauses(clause.Locking{Strength: "SHARE"})
		}
		var row struct{ UserID string }
		if err := q.Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUnauthorized
			}
			return err
		}
		if row.UserID != id {
			return ErrUnauthorized
		}
	}
	return nil
}

// ManagementScope consumes the existing Access scope algebra, using locking
// reads for mutations. Grant, role, policy and membership changes therefore
// cannot race a successfully authorized configuration transaction. The caller
// supplies the generated action's permission, never a browser-supplied scope.
func (d *NotificationRecipientDirectory) ManagementScope(ctx context.Context, tenant, user, permission string, lock bool) (domain.EffectiveBusinessScope, error) {
	principal, err := notificationDirectoryCaller(ctx, tenant)
	if err != nil || principal.UserID != user || permission == "" {
		return domain.EffectiveBusinessScope{}, ErrUnauthorized
	}
	if err := d.LockCaller(ctx, tenant, lock); err != nil {
		return domain.EffectiveBusinessScope{}, err
	}
	type scopeRow struct {
		Scope          domain.DataScope
		ReferenceID    *string
		PolicyID       *string
		PolicyTenantID string
		PolicyStatus   string
		PolicyVersion  uint64
		NotBefore      *time.Time
		ExpiresAt      *time.Time
	}
	tx := d.db.WithContext(ctx)
	if lock {
		tx = tx.Clauses(clause.Locking{Strength: "SHARE"})
	}
	var rows []scopeRow
	err = tx.Table("biz_memberships m").
		Select("pg.scope, r.data_policy_id AS reference_id, p.id AS policy_id, p.tenant_id AS policy_tenant_id, p.status AS policy_status, p.version AS policy_version, p.not_before, p.expires_at").
		Joins("JOIN biz_member_roles mr ON mr.tenant_id = m.tenant_id AND mr.user_id = m.user_id").
		Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ?", domain.TenantRoleStatusActive).
		Joins("JOIN biz_permission_grants pg ON pg.role_id = r.id AND pg.tenant_id = r.tenant_id").
		Joins("LEFT JOIN biz_data_policies p ON p.tenant_id = r.tenant_id AND p.id = r.data_policy_id").
		Where("BINARY m.tenant_id = ? AND BINARY m.user_id = ? AND m.status = ? AND m.self_deleted_at IS NULL AND pg.permission = ?", tenant, user, domain.TenantMemberStatusActive, permission).
		Order("r.id ASC").Scan(&rows).Error
	if err != nil {
		return domain.EffectiveBusinessScope{}, err
	}
	grants := make([]domain.BusinessScopeGrant, 0, len(rows))
	for _, row := range rows {
		grant := domain.BusinessScopeGrant{Scope: row.Scope}
		if row.ReferenceID != nil {
			grant.PolicyID = *row.ReferenceID
		}
		if row.PolicyID != nil {
			var sites []string
			q := d.db.WithContext(ctx).Model(&dataPolicySiteRecord{}).Where("BINARY tenant_id = ? AND policy_id = ?", tenant, *row.PolicyID).Order("site_id ASC")
			if lock {
				q = q.Clauses(clause.Locking{Strength: "SHARE"})
			}
			if err := q.Pluck("site_id", &sites).Error; err != nil {
				return domain.EffectiveBusinessScope{}, err
			}
			grant.Policy = &domain.DataPolicy{ID: *row.PolicyID, TenantID: row.PolicyTenantID, Status: row.PolicyStatus, Version: row.PolicyVersion, NotBefore: row.NotBefore, ExpiresAt: row.ExpiresAt, SiteIDs: sites}
		}
		grants = append(grants, grant)
	}
	var memberSites []string
	q := d.db.WithContext(ctx).Model(&memberSiteRecord{}).Where("BINARY tenant_id = ? AND BINARY user_id = ?", tenant, user).Order("site_id ASC")
	if lock {
		q = q.Clauses(clause.Locking{Strength: "SHARE"})
	}
	if err := q.Pluck("site_id", &memberSites).Error; err != nil {
		return domain.EffectiveBusinessScope{}, err
	}
	return domain.ResolveEffectiveBusinessScope(tenant, grants, memberSites, time.Now().UTC())
}
