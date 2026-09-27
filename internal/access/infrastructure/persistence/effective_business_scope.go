package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"yunka.io/gateway/authz"
)

// ResolveBusinessScope reads one current Access snapshot per sensitive request.
// Only roles currently granting the requested action participate. This repository
// never reads DeviceOps tables; current site availability remains resource-owned.
func (store *Store) ResolveBusinessScope(ctx context.Context, tenantID, userID string, permission authz.PermissionKey) (domain.EffectiveBusinessScope, error) {
	if store == nil || store.database == nil {
		return domain.EffectiveBusinessScope{}, errors.New("access: business scope store unavailable")
	}
	tenantID, userID = strings.TrimSpace(tenantID), strings.TrimSpace(userID)
	if tenantID == "" || userID == "" || strings.TrimSpace(string(permission)) == "" {
		return domain.EffectiveBusinessScope{}, ErrUnauthorized
	}
	var result domain.EffectiveBusinessScope
	err := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		type grantRow struct {
			Scope          domain.DataScope
			ReferenceID    *string
			PolicyID       *string
			PolicyTenantID string
			PolicyStatus   string
			PolicyVersion  uint64
			NotBefore      *time.Time
			ExpiresAt      *time.Time
		}
		var rows []grantRow
		if err := tx.Table("biz_memberships m").
			Select("pg.scope, r.data_policy_id AS reference_id, p.id AS policy_id, p.tenant_id AS policy_tenant_id, p.status AS policy_status, p.version AS policy_version, p.not_before, p.expires_at").
			Joins("JOIN biz_tenants t ON t.id = m.tenant_id AND t.status = ?", domain.TenantStatusActive).
			Joins("JOIN biz_users u ON u.id = m.user_id AND u.status = ?", "active").
			Joins("JOIN biz_member_roles mr ON mr.tenant_id = m.tenant_id AND mr.user_id = m.user_id").
			Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ?", domain.TenantRoleStatusActive).
			Joins("JOIN biz_permission_grants pg ON pg.role_id = r.id AND pg.tenant_id = r.tenant_id").
			Joins("LEFT JOIN biz_data_policies p ON p.tenant_id = r.tenant_id AND p.id = r.data_policy_id").
			Where("m.tenant_id = ? AND m.user_id = ? AND m.status = ? AND pg.permission = ?", tenantID, userID, domain.TenantMemberStatusActive, string(permission)).
			Order("r.id ASC").Scan(&rows).Error; err != nil {
			return err
		}
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			if row.PolicyID != nil {
				ids = append(ids, *row.PolicyID)
			}
		}
		policySites := map[string][]string{}
		if len(ids) > 0 {
			var links []dataPolicySiteRecord
			if err := tx.Where("tenant_id = ? AND policy_id IN ?", tenantID, ids).Find(&links).Error; err != nil {
				return err
			}
			for _, link := range links {
				policySites[link.PolicyID] = append(policySites[link.PolicyID], link.SiteID)
			}
		}
		var memberSites []string
		if err := tx.Model(&memberSiteRecord{}).Where("tenant_id = ? AND user_id = ?", tenantID, userID).Order("site_id ASC").Pluck("site_id", &memberSites).Error; err != nil {
			return err
		}
		grants := make([]domain.BusinessScopeGrant, 0, len(rows))
		for _, row := range rows {
			grant := domain.BusinessScopeGrant{Scope: row.Scope}
			if row.ReferenceID != nil {
				grant.PolicyID = *row.ReferenceID
			}
			if row.PolicyID != nil {
				grant.Policy = &domain.DataPolicy{ID: *row.PolicyID, TenantID: row.PolicyTenantID, Status: row.PolicyStatus, Version: row.PolicyVersion, NotBefore: row.NotBefore, ExpiresAt: row.ExpiresAt, SiteIDs: policySites[*row.PolicyID]}
			}
			grants = append(grants, grant)
		}
		var err error
		result, err = domain.ResolveEffectiveBusinessScope(tenantID, grants, memberSites, time.Now().UTC())
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

var _ ports.BusinessScopeResolver = (*Store)(nil)
