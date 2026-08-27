package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/deviceops/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

var ErrUnauthorized = errors.New("access: unauthorized")

type DataScope string

const (
	DataScopeNone  DataScope = "none"
	DataScopeSelf  DataScope = "self"
	DataScopeSites DataScope = "sites"
	DataScopeAll   DataScope = "all"
)

type Tenant struct {
	ID        string    `gorm:"column:id;primaryKey;size:64"`
	Name      string    `gorm:"column:name;size:200;not null"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (Tenant) TableName() string { return "biz_tenants" }

type User struct {
	ID        string    `gorm:"column:id;primaryKey;size:64"`
	Email     string    `gorm:"column:email;size:320;not null;uniqueIndex"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (User) TableName() string { return "biz_users" }

type Membership struct {
	TenantID  string    `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID    string    `gorm:"column:user_id;primaryKey;size:64"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (Membership) TableName() string { return "biz_memberships" }

type Role struct {
	ID       string `gorm:"column:id;primaryKey;size:160"`
	TenantID string `gorm:"column:tenant_id;size:64;not null;index:idx_role_tenant;uniqueIndex:uniq_role_name,priority:1"`
	Name     string `gorm:"column:name;size:100;not null;uniqueIndex:uniq_role_name,priority:2"`
	Status   string `gorm:"column:status;size:32;not null"`
}

func (Role) TableName() string { return "biz_roles" }

type MemberRole struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID   string `gorm:"column:user_id;primaryKey;size:64"`
	RoleID   string `gorm:"column:role_id;primaryKey;size:160"`
}

func (MemberRole) TableName() string { return "biz_member_roles" }

type RolePermission struct {
	TenantID   string `gorm:"column:tenant_id;primaryKey;size:64"`
	RoleID     string `gorm:"column:role_id;primaryKey;size:160"`
	Permission string `gorm:"column:permission;primaryKey;size:120"`
}

func (RolePermission) TableName() string { return "biz_role_permissions" }

type RoleDataScope struct {
	TenantID   string    `gorm:"column:tenant_id;primaryKey;size:64"`
	RoleID     string    `gorm:"column:role_id;primaryKey;size:160"`
	Permission string    `gorm:"column:permission;primaryKey;size:120"`
	Scope      DataScope `gorm:"column:scope;size:16;not null"`
}

func (RoleDataScope) TableName() string { return "biz_role_data_scopes" }

type MemberSite struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID   string `gorm:"column:user_id;primaryKey;size:64"`
	SiteID   string `gorm:"column:site_id;primaryKey;size:64"`
}

func (MemberSite) TableName() string { return "biz_member_sites" }

type APIToken struct {
	TokenHash string     `gorm:"column:token_hash;primaryKey;size:64"`
	TenantID  string     `gorm:"column:tenant_id;size:64;not null;index"`
	UserID    string     `gorm:"column:user_id;size:64;not null;index"`
	ExpiresAt *time.Time `gorm:"column:expires_at"`
	Disabled  bool       `gorm:"column:disabled;not null;default:false"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
}

func (APIToken) TableName() string { return "biz_api_tokens" }

type Store struct{ database *gorm.DB }

func New(database *gorm.DB) (*Store, error) {
	if database == nil {
		return nil, errors.New("access: database is required")
	}
	return &Store{database: database}, nil
}

func (store *Store) AutoMigrate(ctx context.Context) error {
	return store.database.WithContext(ctx).AutoMigrate(&Tenant{}, &User{}, &Membership{}, &Role{}, &MemberRole{}, &RolePermission{}, &RoleDataScope{}, &MemberSite{}, &APIToken{})
}

func TokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (store *Store) Authenticate(ctx context.Context, rawToken string) (identity.Principal, error) {
	if store == nil || store.database == nil || strings.TrimSpace(rawToken) == "" {
		return identity.Principal{}, ErrUnauthorized
	}
	var token APIToken
	if err := store.database.WithContext(ctx).Where("token_hash = ? AND disabled = ?", TokenHash(rawToken), false).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now()) {
		return identity.Principal{}, ErrUnauthorized
	}
	var membership Membership
	if err := store.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND status = ?", token.TenantID, token.UserID, "active").First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	var roles []string
	if err := store.database.WithContext(ctx).Table("biz_member_roles mr").Select("r.name").Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ?", "active").Where("mr.tenant_id = ? AND mr.user_id = ?", token.TenantID, token.UserID).Scan(&roles).Error; err != nil {
		return identity.Principal{}, err
	}
	sort.Strings(roles)
	return identity.Principal{Subject: "user:" + token.UserID, TenantID: token.TenantID, UserID: token.UserID, Roles: roles, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true}, nil
}

func (store *Store) HasPermissions(ctx context.Context, tenantID string, roles []string, permissions []authz.PermissionKey, mode authz.PermissionMode) (bool, error) {
	if len(permissions) == 0 {
		return true, nil
	}
	if strings.TrimSpace(tenantID) == "" || len(roles) == 0 {
		return false, nil
	}
	keys := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		keys = append(keys, string(permission))
	}
	var granted []string
	if err := store.database.WithContext(ctx).Table("biz_roles r").Distinct("rp.permission").Select("rp.permission").Joins("JOIN biz_role_permissions rp ON rp.role_id = r.id AND rp.tenant_id = r.tenant_id").Where("r.tenant_id = ? AND r.status = ? AND r.name IN ? AND rp.permission IN ?", tenantID, "active", roles, keys).Scan(&granted).Error; err != nil {
		return false, err
	}
	set := make(map[string]struct{}, len(granted))
	for _, value := range granted {
		set[value] = struct{}{}
	}
	if mode == authz.PermissionAny {
		for _, key := range keys {
			if _, ok := set[key]; ok {
				return true, nil
			}
		}
		return false, nil
	}
	for _, key := range keys {
		if _, ok := set[key]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func (store *Store) ResolveDeviceScope(ctx context.Context, principal identity.Principal, permission string) (ports.DeviceScope, error) {
	scope := ports.DeviceScope{UserID: principal.UserID}
	if !principal.Authenticated || principal.TenantID == "" || len(principal.Roles) == 0 {
		return scope, nil
	}
	var values []DataScope
	if err := store.database.WithContext(ctx).Table("biz_roles r").Select("rds.scope").Joins("JOIN biz_role_data_scopes rds ON rds.role_id = r.id AND rds.tenant_id = r.tenant_id").Where("r.tenant_id = ? AND r.status = ? AND r.name IN ? AND rds.permission = ?", principal.TenantID, "active", principal.Roles, permission).Scan(&values).Error; err != nil {
		return scope, err
	}
	for _, value := range values {
		switch value {
		case DataScopeAll:
			scope.All = true
		case DataScopeSites:
			scope.Sites = true
		case DataScopeSelf:
			scope.Self = true
		}
	}
	if scope.Sites {
		var rows []MemberSite
		if err := store.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", principal.TenantID, principal.UserID).Find(&rows).Error; err != nil {
			return scope, err
		}
		scope.SiteIDs = make([]string, 0, len(rows))
		for _, row := range rows {
			scope.SiteIDs = append(scope.SiteIDs, row.SiteID)
		}
		sort.Strings(scope.SiteIDs)
	}
	return scope, nil
}

type Bootstrap struct{ TenantID, TenantName, UserID, Email, Token string }

func (store *Store) Bootstrap(ctx context.Context, config Bootstrap, permissions []string) error {
	if strings.TrimSpace(config.Token) == "" {
		return nil
	}
	roleID := config.TenantID + ":owner"
	values := []any{
		&Tenant{ID: config.TenantID, Name: config.TenantName, Status: "active"},
		&User{ID: config.UserID, Email: config.Email, Status: "active"},
		&Membership{TenantID: config.TenantID, UserID: config.UserID, Status: "active"},
		&Role{ID: roleID, TenantID: config.TenantID, Name: "owner", Status: "active"},
		&MemberRole{TenantID: config.TenantID, UserID: config.UserID, RoleID: roleID},
		&APIToken{TokenHash: TokenHash(config.Token), TenantID: config.TenantID, UserID: config.UserID},
	}
	db := store.database.WithContext(ctx)
	for _, value := range values {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(value).Error; err != nil {
			return err
		}
	}
	for _, permission := range permissions {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&RolePermission{TenantID: config.TenantID, RoleID: roleID, Permission: permission}).Error; err != nil {
			return err
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&RoleDataScope{TenantID: config.TenantID, RoleID: roleID, Permission: permission, Scope: DataScopeAll}).Error; err != nil {
			return err
		}
	}
	return nil
}
