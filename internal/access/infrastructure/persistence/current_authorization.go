package persistence

import (
	"context"
	"errors"
	"sort"
	"strings"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

type CurrentGrantFact struct {
	Permission authz.PermissionKey `json:"permission"`
	RoleID     string              `json:"role_id"`
	RoleName   string              `json:"role_name"`
	Scope      string              `json:"scope"`
}

type CurrentDataPolicy struct {
	Permission authz.PermissionKey `json:"permission"`
	Scope      string              `json:"scope"`
	SiteIDs    []string            `json:"site_ids,omitempty"`
}

type CurrentAuthorizationSnapshot struct {
	UserID            string              `json:"user_id"`
	UserName          string              `json:"user_name"`
	TenantID          string              `json:"tenant_id"`
	TenantName        string              `json:"tenant_name"`
	Timezone          string              `json:"timezone"`
	Roles             []string            `json:"roles"`
	Grants            []CurrentGrantFact  `json:"grants"`
	DataPolicies      []CurrentDataPolicy `json:"data_policies"`
	SiteIDs           []string            `json:"site_ids"`
	PermissionVersion string              `json:"permission_version"`
}

func (store *Store) ResolveCurrentGrants(ctx context.Context, tenantID, userID string, permissions []authz.PermissionKey) ([]authz.Grant, error) {
	if store == nil || store.database == nil {
		return nil, errors.New("access: current grant store unavailable")
	}
	tenantID = strings.TrimSpace(tenantID)
	userID = strings.TrimSpace(userID)
	if tenantID == "" || userID == "" {
		return nil, nil
	}
	type row struct {
		RoleID     string
		Permission string
		Scope      accessdomain.DataScope
	}
	query := store.database.WithContext(ctx).Table("biz_memberships m").
		Select("r.id AS role_id, pg.permission, pg.scope").
		Joins("JOIN biz_tenants t ON t.id = m.tenant_id AND t.status = ?", accessdomain.TenantStatusActive).
		Joins("JOIN biz_users u ON u.id = m.user_id AND u.status = ?", "active").
		Joins("JOIN biz_member_roles mr ON mr.tenant_id = m.tenant_id AND mr.user_id = m.user_id").
		Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ?", accessdomain.TenantRoleStatusActive).
		Joins("JOIN biz_permission_grants pg ON pg.role_id = r.id AND pg.tenant_id = r.tenant_id").
		Where("m.tenant_id = ? AND m.user_id = ? AND m.status = ?", tenantID, userID, accessdomain.TenantMemberStatusActive)
	if len(permissions) > 0 {
		keys := make([]string, 0, len(permissions))
		for _, permission := range permissions {
			if value := strings.TrimSpace(string(permission)); value != "" {
				keys = append(keys, value)
			}
		}
		if len(keys) == 0 {
			return nil, nil
		}
		query = query.Where("pg.permission IN ?", keys)
	}
	var rows []row
	if err := query.Order("pg.permission ASC, r.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]authz.Grant, 0, len(rows)+1)
	for _, value := range rows {
		result = append(result, authz.Grant{
			Permission: authz.PermissionKey(value.Permission),
			RoleID:     value.RoleID,
			Scope:      string(value.Scope),
		})
	}
	if currentGrantRequestsPermission(permissions, "tenant.branding.read") {
		implicit, err := store.resolveBrandingReadGrant(ctx, authz.GrantRequest{
			Principal: identity.Principal{
				Subject:       "user:" + userID,
				TenantID:      tenantID,
				UserID:        userID,
				Authenticated: true,
			},
			TenantBound: true,
			Operation:   "tenant.branding.get",
			Permissions: []authz.PermissionKey{"tenant.branding.read"},
		})
		if err != nil {
			return nil, err
		}
		result = append(result, implicit...)
	}
	if currentGrantRequestsPermission(permissions, "tenant.personal_profile.self") {
		implicit, err := store.resolvePersonalProfileSelfGrant(ctx, tenantID, userID)
		if err != nil {
			return nil, err
		}
		result = append(result, implicit...)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Permission == result[j].Permission {
			return result[i].RoleID < result[j].RoleID
		}
		return result[i].Permission < result[j].Permission
	})
	return result, nil
}

func (store *Store) resolvePersonalProfileSelfGrant(ctx context.Context, tenantID, userID string) ([]authz.Grant, error) {
	tenantID, userID = strings.TrimSpace(tenantID), strings.TrimSpace(userID)
	if tenantID == "" || userID == "" {
		return nil, nil
	}
	var count int64
	err := store.database.WithContext(ctx).Table("biz_memberships AS m").
		Joins("JOIN biz_tenants AS t ON t.id = m.tenant_id AND t.status = ?", accessdomain.TenantStatusActive).
		Joins("JOIN biz_users AS u ON u.id = m.user_id AND u.status = ?", "active").
		Where("m.tenant_id = ? AND m.user_id = ? AND m.status = ?", tenantID, userID, accessdomain.TenantMemberStatusActive).
		Count(&count).Error
	if err != nil || count != 1 {
		return nil, err
	}
	return []authz.Grant{{
		Permission: "tenant.personal_profile.self",
		RoleID:     "membership:" + tenantID + ":" + userID,
		Scope:      "self",
	}}, nil
}

func currentGrantRequestsPermission(permissions []authz.PermissionKey, expected authz.PermissionKey) bool {
	if len(permissions) == 0 {
		return true
	}
	for _, permission := range permissions {
		if permission == expected {
			return true
		}
	}
	return false
}

func (store *Store) CurrentAuthorization(ctx context.Context, principal identity.Principal) (CurrentAuthorizationSnapshot, error) {
	if store == nil || store.database == nil || !principal.Authenticated {
		return CurrentAuthorizationSnapshot{}, ErrUnauthorized
	}
	tenantID := strings.TrimSpace(principal.TenantID)
	userID := strings.TrimSpace(principal.UserID)
	if tenantID == "" || userID == "" {
		return CurrentAuthorizationSnapshot{}, ErrUnauthorized
	}

	type identityRow struct {
		UserName   string
		TenantName string
		Timezone   string
	}
	var identityValue identityRow
	err := store.database.WithContext(ctx).Table("biz_memberships m").
		Select("m.name AS user_name, t.name AS tenant_name, t.timezone").
		Joins("JOIN biz_tenants t ON t.id = m.tenant_id AND t.status = ?", accessdomain.TenantStatusActive).
		Joins("JOIN biz_users u ON u.id = m.user_id AND u.status = ?", "active").
		Where("m.tenant_id = ? AND m.user_id = ? AND m.status = ?", tenantID, userID, accessdomain.TenantMemberStatusActive).
		Take(&identityValue).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return CurrentAuthorizationSnapshot{}, ErrUnauthorized
		}
		return CurrentAuthorizationSnapshot{}, err
	}

	type roleRow struct {
		ID   string
		Name string
	}
	var roleRows []roleRow
	if err := store.database.WithContext(ctx).Table("biz_member_roles mr").
		Select("r.id, r.name").
		Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ?", accessdomain.TenantRoleStatusActive).
		Where("mr.tenant_id = ? AND mr.user_id = ?", tenantID, userID).
		Order("r.name ASC, r.id ASC").Scan(&roleRows).Error; err != nil {
		return CurrentAuthorizationSnapshot{}, err
	}
	roleNames := make([]string, 0, len(roleRows))
	roleNamesByID := make(map[string]string, len(roleRows))
	for _, role := range roleRows {
		roleNames = append(roleNames, role.Name)
		roleNamesByID[role.ID] = role.Name
	}
	grants, err := store.ResolveCurrentGrants(ctx, tenantID, userID, nil)
	if err != nil {
		return CurrentAuthorizationSnapshot{}, err
	}
	grantFacts := make([]CurrentGrantFact, 0, len(grants))
	for _, grant := range grants {
		grantFacts = append(grantFacts, CurrentGrantFact{
			Permission: grant.Permission,
			RoleID:     grant.RoleID,
			RoleName:   roleNamesByID[grant.RoleID],
			Scope:      grant.Scope,
		})
	}
	sites, err := store.ResolveMemberSites(ctx, tenantID, userID)
	if err != nil {
		return CurrentAuthorizationSnapshot{}, err
	}
	policies := effectiveDataPolicies(grants, sites)
	trusted := identity.WithPrincipal(ctx, identity.Principal{
		Subject:       principal.Subject,
		TenantID:      tenantID,
		UserID:        userID,
		Roles:         roleNames,
		AuthMethod:    principal.AuthMethod,
		Authenticated: true,
	})
	version, err := store.PermissionVersion(trusted)
	if err != nil {
		return CurrentAuthorizationSnapshot{}, err
	}
	return CurrentAuthorizationSnapshot{
		UserID:            userID,
		UserName:          identityValue.UserName,
		TenantID:          tenantID,
		TenantName:        identityValue.TenantName,
		Timezone:          identityValue.Timezone,
		Roles:             roleNames,
		Grants:            grantFacts,
		DataPolicies:      policies,
		SiteIDs:           sites,
		PermissionVersion: version,
	}, nil
}

func effectiveDataPolicies(grants []authz.Grant, sites []string) []CurrentDataPolicy {
	scopeRank := map[string]int{"": 0, "none": 0, "self": 1, "sites": 2, "all": 3}
	best := map[authz.PermissionKey]string{}
	for _, grant := range grants {
		scope := strings.TrimSpace(grant.Scope)
		if scopeRank[scope] > scopeRank[best[grant.Permission]] {
			best[grant.Permission] = scope
		}
		if _, ok := best[grant.Permission]; !ok {
			best[grant.Permission] = scope
		}
	}
	keys := make([]string, 0, len(best))
	for permission := range best {
		keys = append(keys, string(permission))
	}
	sort.Strings(keys)
	out := make([]CurrentDataPolicy, 0, len(keys))
	for _, raw := range keys {
		permission := authz.PermissionKey(raw)
		scope := best[permission]
		policy := CurrentDataPolicy{Permission: permission, Scope: scope}
		if scope == "sites" {
			policy.SiteIDs = append([]string(nil), sites...)
		}
		out = append(out, policy)
	}
	return out
}
