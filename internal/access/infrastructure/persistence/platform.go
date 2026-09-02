package persistence

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/gateway/authz"
)

type platformCredentialRecord struct {
	TokenHash string    `gorm:"column:token_hash;primaryKey;size:64"`
	Subject   string    `gorm:"column:subject;size:200;not null;index"`
	Disabled  bool      `gorm:"column:disabled;not null;default:false"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (platformCredentialRecord) TableName() string { return "biz_platform_credentials" }

type platformPermissionGrantRecord struct {
	Subject    string `gorm:"column:subject;primaryKey;size:200"`
	Permission string `gorm:"column:permission;primaryKey;size:120"`
}

func (platformPermissionGrantRecord) TableName() string { return "biz_platform_permission_grants" }

type PlatformBootstrap struct {
	Subject     string
	Token       string
	Permissions []authz.PermissionKey
}

func (store *Store) EnsurePlatformSchema(ctx context.Context) error {
	if store == nil || store.database == nil {
		return errors.New("access: platform schema store unavailable")
	}
	return store.database.WithContext(ctx).AutoMigrate(&platformCredentialRecord{}, &platformPermissionGrantRecord{})
}

func (store *Store) BootstrapPlatform(ctx context.Context, bootstrap PlatformBootstrap) error {
	if store == nil || store.database == nil {
		return errors.New("access: platform bootstrap store unavailable")
	}
	subject := strings.TrimSpace(bootstrap.Subject)
	token := strings.TrimSpace(bootstrap.Token)
	if subject == "" || token == "" || len(bootstrap.Permissions) == 0 {
		return errors.New("access: platform bootstrap requires subject, token and permissions")
	}
	now := time.Now().UTC()
	db := store.database.WithContext(ctx)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&platformCredentialRecord{
		TokenHash: TokenHash(token), Subject: subject, CreatedAt: now,
	}).Error; err != nil {
		return err
	}
	for _, permission := range bootstrap.Permissions {
		value := strings.TrimSpace(string(permission))
		if value == "" {
			continue
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&platformPermissionGrantRecord{
			Subject: subject, Permission: value,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) AuthenticatePlatform(ctx context.Context, rawToken string) (identity.Principal, error) {
	if store == nil || store.database == nil || strings.TrimSpace(rawToken) == "" {
		return identity.Principal{}, ErrUnauthorized
	}
	var row platformCredentialRecord
	if err := store.database.WithContext(ctx).
		Where("token_hash = ? AND disabled = ?", TokenHash(rawToken), false).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	return identity.Principal{
		Subject: row.Subject, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true,
	}, nil
}

// PrincipalGrantResolver is the Biz IAM projection consumed by Yunka's
// principal-aware GrantResolver seam. Tenant-bound and tenantless authority
// remain physically separate and cannot satisfy each other accidentally.
type PrincipalGrantResolver struct{ store *Store }

func NewPrincipalGrantResolver(store *Store) (*PrincipalGrantResolver, error) {
	if store == nil || store.database == nil {
		return nil, errors.New("access: principal grant store is required")
	}
	return &PrincipalGrantResolver{store: store}, nil
}

func (resolver *PrincipalGrantResolver) ResolveGrants(ctx context.Context, request authz.GrantRequest) ([]authz.Grant, error) {
	if resolver == nil || resolver.store == nil {
		return nil, errors.New("access: principal grant resolver unavailable")
	}
	if request.TenantBound {
		return resolver.store.ResolveGrants(ctx, request.Principal.TenantID, request.Principal.Roles, request.Permissions)
	}
	subject := strings.TrimSpace(request.Principal.Subject)
	if subject == "" || len(request.Permissions) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(request.Permissions))
	for _, permission := range request.Permissions {
		if value := strings.TrimSpace(string(permission)); value != "" {
			keys = append(keys, value)
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	var rows []platformPermissionGrantRecord
	if err := resolver.store.database.WithContext(ctx).
		Where("subject = ? AND permission IN ?", subject, keys).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]authz.Grant, 0, len(rows))
	for _, row := range rows {
		result = append(result, authz.Grant{
			Permission: authz.PermissionKey(row.Permission), RoleID: "direct:" + subject,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Permission < result[j].Permission })
	return result, nil
}
