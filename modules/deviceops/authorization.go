package deviceops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
)

var (
	ErrUnauthorized = errors.New("deviceops: unauthorized")
	ErrForbidden    = errors.New("deviceops: forbidden")
	ErrNotFound     = errors.New("deviceops: not found")
	ErrConflict     = errors.New("deviceops: conflict")
	ErrInvalid      = errors.New("deviceops: invalid request")
)

type permissionScope struct {
	Allowed bool
	All     bool
	Self    bool
	Sites   bool
}

type AccessPlan struct {
	Principal  identity.Principal
	Permissions map[string]permissionScope
	SiteIDs     []string
}

func (plan AccessPlan) Can(permission string) bool {
	scope, ok := plan.Permissions[permission]
	return ok && scope.Allowed
}

func (plan AccessPlan) canTargetSite(permission, siteID string) bool {
	scope, ok := plan.Permissions[permission]
	if !ok || !scope.Allowed {
		return false
	}
	if scope.All {
		return true
	}
	if !scope.Sites {
		return false
	}
	for _, allowed := range plan.SiteIDs {
		if allowed == siteID {
			return true
		}
	}
	return false
}

type Authenticator struct {
	database *gorm.DB
}

func NewAuthenticator(database *gorm.DB) (*Authenticator, error) {
	if database == nil {
		return nil, errors.New("deviceops: authentication database is required")
	}
	return &Authenticator{database: database}, nil
}

func (auth *Authenticator) Authenticate(ctx context.Context, rawToken string) (AccessPlan, error) {
	if auth == nil || auth.database == nil || strings.TrimSpace(rawToken) == "" {
		return AccessPlan{}, ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])
	var token APIToken
	if err := auth.database.WithContext(ctx).Where("token_hash = ? AND disabled = ?", tokenHash, false).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AccessPlan{}, ErrUnauthorized
		}
		return AccessPlan{}, err
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now()) {
		return AccessPlan{}, ErrUnauthorized
	}
	var membership Membership
	if err := auth.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND status = ?", token.TenantID, token.UserID, "active").First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return AccessPlan{}, ErrUnauthorized
		}
		return AccessPlan{}, err
	}

	type grantRow struct {
		RoleName   string
		Permission string
		DataScope  DataScope
	}
	var rows []grantRow
	if err := auth.database.WithContext(ctx).
		Table("biz_member_roles mr").
		Select("r.name AS role_name, rp.permission, rp.data_scope").
		Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ?", "active").
		Joins("JOIN biz_role_permissions rp ON rp.role_id = mr.role_id AND rp.tenant_id = mr.tenant_id").
		Where("mr.tenant_id = ? AND mr.user_id = ?", token.TenantID, token.UserID).
		Scan(&rows).Error; err != nil {
		return AccessPlan{}, err
	}

	roleSet := map[string]struct{}{}
	permissions := make(map[string]permissionScope)
	for _, row := range rows {
		roleSet[row.RoleName] = struct{}{}
		scope := permissions[row.Permission]
		scope.Allowed = true
		switch row.DataScope {
		case DataScopeAll:
			scope.All = true
		case DataScopeSites:
			scope.Sites = true
		case DataScopeSelf:
			scope.Self = true
		}
		permissions[row.Permission] = scope
	}
	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	var siteRows []MemberSite
	if err := auth.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", token.TenantID, token.UserID).Find(&siteRows).Error; err != nil {
		return AccessPlan{}, err
	}
	sites := make([]string, 0, len(siteRows))
	for _, row := range siteRows {
		sites = append(sites, row.SiteID)
	}
	principal := identity.Principal{
		Subject:       "user:" + token.UserID,
		TenantID:      token.TenantID,
		UserID:        token.UserID,
		Roles:         roles,
		AuthMethod:    "bearer-token",
		Authenticated: true,
	}
	return AccessPlan{Principal: principal, Permissions: permissions, SiteIDs: sites}, nil
}
