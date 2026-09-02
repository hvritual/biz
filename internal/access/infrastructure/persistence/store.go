package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/gateway/authz"
)

var ErrUnauthorized = errors.New("access: unauthorized")

type tenantRecord struct {
	ID        string    `gorm:"column:id;primaryKey;size:64"`
	Name      string    `gorm:"column:name;size:200;not null"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	Version   uint64    `gorm:"column:version;not null;default:1"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (tenantRecord) TableName() string { return "biz_tenants" }

type userRecord struct {
	ID        string    `gorm:"column:id;primaryKey;size:64"`
	Email     string    `gorm:"column:email;size:320;not null;uniqueIndex"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (userRecord) TableName() string { return "biz_users" }

type membershipRecord struct {
	TenantID  string    `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID    string    `gorm:"column:user_id;primaryKey;size:64"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	Version   uint64    `gorm:"column:version;not null;default:1"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (membershipRecord) TableName() string { return "biz_memberships" }

type roleRecord struct {
	ID       string `gorm:"column:id;primaryKey;size:160"`
	TenantID string `gorm:"column:tenant_id;size:64;not null;index:idx_role_tenant;uniqueIndex:uniq_role_name,priority:1"`
	Name     string `gorm:"column:name;size:100;not null;uniqueIndex:uniq_role_name,priority:2"`
	Status   string `gorm:"column:status;size:32;not null"`
	Version  uint64 `gorm:"column:version;not null;default:1"`
}

func (roleRecord) TableName() string { return "biz_roles" }

type memberRoleRecord struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID   string `gorm:"column:user_id;primaryKey;size:64"`
	RoleID   string `gorm:"column:role_id;primaryKey;size:160"`
}

func (memberRoleRecord) TableName() string { return "biz_member_roles" }

type permissionGrantRecord struct {
	TenantID   string                 `gorm:"column:tenant_id;primaryKey;size:64"`
	RoleID     string                 `gorm:"column:role_id;primaryKey;size:160"`
	Permission string                 `gorm:"column:permission;primaryKey;size:120"`
	Scope      accessdomain.DataScope `gorm:"column:scope;size:16;not null"`
}

func (permissionGrantRecord) TableName() string { return "biz_permission_grants" }

type memberSiteRecord struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID   string `gorm:"column:user_id;primaryKey;size:64"`
	SiteID   string `gorm:"column:site_id;primaryKey;size:64"`
}

func (memberSiteRecord) TableName() string { return "biz_member_sites" }

type apiTokenRecord struct {
	TokenHash string     `gorm:"column:token_hash;primaryKey;size:64"`
	TenantID  string     `gorm:"column:tenant_id;size:64;not null;index"`
	UserID    string     `gorm:"column:user_id;size:64;not null;index"`
	ExpiresAt *time.Time `gorm:"column:expires_at"`
	Disabled  bool       `gorm:"column:disabled;not null;default:false"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
}

func (apiTokenRecord) TableName() string { return "biz_api_tokens" }

type Store struct{ database *gorm.DB }

func New(database *gorm.DB) (*Store, error) {
	if database == nil {
		return nil, errors.New("access: database is required")
	}
	return &Store{database: database}, nil
}

func (store *Store) AutoMigrate(ctx context.Context) error {
	return store.database.WithContext(ctx).AutoMigrate(
		&tenantRecord{}, &userRecord{}, &membershipRecord{}, &roleRecord{},
		&memberRoleRecord{}, &permissionGrantRecord{}, &memberSiteRecord{}, &apiTokenRecord{},
	)
}

func TokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (store *Store) Authenticate(ctx context.Context, rawToken string) (identity.Principal, error) {
	if store == nil || store.database == nil || strings.TrimSpace(rawToken) == "" {
		return identity.Principal{}, ErrUnauthorized
	}
	var token apiTokenRecord
	if err := store.database.WithContext(ctx).Where("token_hash = ? AND disabled = ?", TokenHash(rawToken), false).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now()) {
		return identity.Principal{}, ErrUnauthorized
	}
	var tenant tenantRecord
	if err := store.database.WithContext(ctx).Where("id = ? AND status = ?", token.TenantID, accessdomain.TenantStatusActive).First(&tenant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	var membership membershipRecord
	if err := store.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND status = ?", token.TenantID, token.UserID, accessdomain.TenantMemberStatusActive).First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	var roles []string
	if err := store.database.WithContext(ctx).Table("biz_member_roles mr").
		Select("r.name").
		Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ?", accessdomain.TenantRoleStatusActive).
		Where("mr.tenant_id = ? AND mr.user_id = ?", token.TenantID, token.UserID).
		Scan(&roles).Error; err != nil {
		return identity.Principal{}, err
	}
	sort.Strings(roles)
	return identity.Principal{
		Subject: "user:" + token.UserID, TenantID: token.TenantID, UserID: token.UserID,
		Roles: roles, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true,
	}, nil
}

func (store *Store) ResolveGrants(ctx context.Context, tenantID string, roles []string, permissions []authz.PermissionKey) ([]authz.Grant, error) {
	if strings.TrimSpace(tenantID) == "" || len(roles) == 0 || len(permissions) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if value := strings.TrimSpace(string(permission)); value != "" {
			keys = append(keys, value)
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	type grantRow struct {
		RoleID     string
		Permission string
		Scope      accessdomain.DataScope
	}
	var rows []grantRow
	if err := store.database.WithContext(ctx).Table("biz_roles r").
		Select("r.id AS role_id, pg.permission, pg.scope").
		Joins("JOIN biz_permission_grants pg ON pg.role_id = r.id AND pg.tenant_id = r.tenant_id").
		Where("r.tenant_id = ? AND r.status = ? AND r.name IN ? AND pg.permission IN ?", tenantID, accessdomain.TenantRoleStatusActive, roles, keys).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]authz.Grant, 0, len(rows))
	for _, row := range rows {
		result = append(result, authz.Grant{Permission: authz.PermissionKey(row.Permission), RoleID: row.RoleID, Scope: string(row.Scope)})
	}
	return result, nil
}

func (store *Store) ResolveMemberSites(ctx context.Context, tenantID, userID string) ([]string, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(userID) == "" {
		return nil, nil
	}
	var rows []memberSiteRecord
	if err := store.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.SiteID)
	}
	sort.Strings(result)
	return result, nil
}

type Bootstrap struct{ TenantID, TenantName, UserID, Email, Token string }

func (store *Store) Bootstrap(ctx context.Context, config Bootstrap, permissions []authz.PermissionKey) error {
	if strings.TrimSpace(config.Token) == "" {
		return nil
	}
	roleID := config.TenantID + ":owner"
	now := time.Now().UTC()
	values := []any{
		&tenantRecord{ID: config.TenantID, Name: config.TenantName, Status: accessdomain.TenantStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now},
		&userRecord{ID: config.UserID, Email: config.Email, Status: "active", CreatedAt: now},
		&membershipRecord{TenantID: config.TenantID, UserID: config.UserID, Status: accessdomain.TenantMemberStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now},
		&roleRecord{ID: roleID, TenantID: config.TenantID, Name: accessdomain.TenantOwnerRoleName, Status: accessdomain.TenantRoleStatusActive, Version: 1},
		&memberRoleRecord{TenantID: config.TenantID, UserID: config.UserID, RoleID: roleID},
		&apiTokenRecord{TokenHash: TokenHash(config.Token), TenantID: config.TenantID, UserID: config.UserID, CreatedAt: now},
	}
	db := store.database.WithContext(ctx)
	for _, value := range values {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(value).Error; err != nil {
			return err
		}
	}
	for _, permission := range permissions {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&permissionGrantRecord{
			TenantID: config.TenantID, RoleID: roleID, Permission: string(permission), Scope: accessdomain.DataScopeAll,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
