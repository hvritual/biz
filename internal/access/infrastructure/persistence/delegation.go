package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

type tenantDelegationRecord struct {
	ID              string     `gorm:"column:id;primaryKey;size:64"`
	OwnerTenantID   string     `gorm:"column:owner_tenant_id;size:64;not null;index:idx_delegation_owner"`
	GranteeTenantID string     `gorm:"column:grantee_tenant_id;size:64;not null;index:idx_delegation_grantee"`
	ResourceKind    string     `gorm:"column:resource_kind;size:32;not null"`
	ResourceID      string     `gorm:"column:resource_id;size:64;not null;index:idx_delegation_resource"`
	PermissionsJSON string     `gorm:"column:permissions_json;type:text;not null"`
	Status          string     `gorm:"column:status;size:32;not null;index:idx_delegation_status"`
	Version         uint64     `gorm:"column:version;not null;default:1"`
	ExpiresAt       *time.Time `gorm:"column:expires_at"`
	ActiveKey       *string    `gorm:"column:active_key;size:64;uniqueIndex:uniq_delegation_active"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null"`
}

func (tenantDelegationRecord) TableName() string { return "biz_tenant_delegations" }

type TenantDelegationRepository struct {
	database *gorm.DB
}

func NewTenantDelegationRepository(database *gorm.DB) (*TenantDelegationRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: tenant delegation database is required")
	}
	return &TenantDelegationRepository{database: database}, nil
}

func AutoMigrateTenantDelegation(ctx context.Context, database *gorm.DB) error {
	if database == nil {
		return errors.New("access persistence: database is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return database.WithContext(ctx).AutoMigrate(&tenantDelegationRecord{})
}

func (repository *TenantDelegationRepository) CreateOrGetActive(ctx context.Context, delegation *domain.TenantDelegation) (domain.TenantDelegation, error) {
	if repository == nil || repository.database == nil || delegation == nil {
		return domain.TenantDelegation{}, errors.New("access persistence: tenant delegation repository unavailable")
	}
	ownerTenantID, err := trustedDelegationOwner(ctx)
	if err != nil {
		return domain.TenantDelegation{}, err
	}
	delegation.OwnerTenantID = ownerTenantID
	permissions := append([]string(nil), delegation.Permissions...)
	sort.Strings(permissions)
	delegation.Permissions = permissions
	permissionsJSON, err := json.Marshal(permissions)
	if err != nil {
		return domain.TenantDelegation{}, err
	}
	activeKey := delegationActiveKey(ownerTenantID, delegation.GranteeTenantID, delegation.ResourceKind, delegation.ResourceID)
	row := tenantDelegationRecord{
		ID: delegation.ID, OwnerTenantID: ownerTenantID, GranteeTenantID: delegation.GranteeTenantID,
		ResourceKind: delegation.ResourceKind, ResourceID: delegation.ResourceID,
		PermissionsJSON: string(permissionsJSON), Status: delegation.Status, Version: delegation.Version,
		ExpiresAt: delegation.ExpiresAt, ActiveKey: &activeKey, CreatedAt: delegation.CreatedAt, UpdatedAt: delegation.UpdatedAt,
	}
	return createOrResolveActiveDelegation(repository.database.WithContext(ctx), row, activeKey, *delegation)
}

// createOrResolveActiveDelegation keeps the database unique key as the final
// serialization boundary for one effective authority tuple. Expiry is temporal
// validity, not a revoke transition, so an expired historical row keeps its
// status but relinquishes active_key before a fresh grant is inserted.
func createOrResolveActiveDelegation(db *gorm.DB, row tenantDelegationRecord, activeKey string, requested domain.TenantDelegation) (domain.TenantDelegation, error) {
	const maxAttempts = 4
	for attempt := 0; attempt < maxAttempts; attempt++ {
		candidate := row
		if err := db.Create(&candidate).Error; err == nil {
			return delegationRecordDomain(candidate)
		} else if !isActiveDelegationDuplicate(err) {
			return domain.TenantDelegation{}, err
		}

		var occupied tenantDelegationRecord
		lookupErr := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("owner_tenant_id = ? AND active_key = ? AND status = ?", requested.OwnerTenantID, activeKey, domain.TenantDelegationStatusActive).
			First(&occupied).Error
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			// A concurrent transaction may have released the old key between the
			// duplicate check and this locking read. Retry the canonical insert.
			continue
		}
		if lookupErr != nil {
			return domain.TenantDelegation{}, lookupErr
		}
		existing, convertErr := delegationRecordDomain(occupied)
		if convertErr != nil {
			return domain.TenantDelegation{}, convertErr
		}
		now := time.Now().UTC()
		if existing.Expired(now) {
			released := db.Model(&tenantDelegationRecord{}).
				Where("id = ? AND owner_tenant_id = ? AND status = ? AND active_key = ? AND expires_at IS NOT NULL AND expires_at <= ?", occupied.ID, requested.OwnerTenantID, domain.TenantDelegationStatusActive, activeKey, now).
				UpdateColumn("active_key", nil)
			if released.Error != nil {
				return domain.TenantDelegation{}, released.Error
			}
			// Whether this transaction released the key or another contender won
			// first, retry against the unique constraint. The next iteration will
			// either create the fresh authority or converge on the winning row.
			continue
		}
		if !sameDelegationAuthority(existing, requested) {
			return domain.TenantDelegation{}, ports.ErrTenantDelegationConflict
		}
		return existing, nil
	}
	return domain.TenantDelegation{}, ports.ErrTenantDelegationConflict
}

func isActiveDelegationDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 && strings.Contains(mysqlErr.Message, "uniq_delegation_active")
}

func (repository *TenantDelegationRepository) Get(ctx context.Context, id string) (domain.TenantDelegation, error) {
	if repository == nil || repository.database == nil {
		return domain.TenantDelegation{}, errors.New("access persistence: tenant delegation repository unavailable")
	}
	ownerTenantID, err := trustedDelegationOwner(ctx)
	if err != nil {
		return domain.TenantDelegation{}, err
	}
	var row tenantDelegationRecord
	if err := repository.database.WithContext(ctx).
		Where("owner_tenant_id = ? AND id = ?", ownerTenantID, strings.TrimSpace(id)).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.TenantDelegation{}, ports.ErrTenantDelegationNotFound
		}
		return domain.TenantDelegation{}, err
	}
	return delegationRecordDomain(row)
}

func (repository *TenantDelegationRepository) List(ctx context.Context) ([]domain.TenantDelegation, error) {
	if repository == nil || repository.database == nil {
		return nil, errors.New("access persistence: tenant delegation repository unavailable")
	}
	ownerTenantID, err := trustedDelegationOwner(ctx)
	if err != nil {
		return nil, err
	}
	var rows []tenantDelegationRecord
	if err := repository.database.WithContext(ctx).
		Where("owner_tenant_id = ?", ownerTenantID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.TenantDelegation, 0, len(rows))
	for _, row := range rows {
		value, err := delegationRecordDomain(row)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (repository *TenantDelegationRepository) Update(ctx context.Context, delegation *domain.TenantDelegation, expectedVersion uint64) error {
	if repository == nil || repository.database == nil || delegation == nil || expectedVersion == 0 {
		return errors.New("access persistence: tenant delegation update requires repository, value and version")
	}
	ownerTenantID, err := trustedDelegationOwner(ctx)
	if err != nil {
		return err
	}
	var activeKey any
	if delegation.Status == domain.TenantDelegationStatusActive {
		activeKey = delegationActiveKey(ownerTenantID, delegation.GranteeTenantID, delegation.ResourceKind, delegation.ResourceID)
	} else {
		activeKey = nil
	}
	result := repository.database.WithContext(ctx).Model(&tenantDelegationRecord{}).
		Where("owner_tenant_id = ? AND id = ? AND version = ?", ownerTenantID, delegation.ID, expectedVersion).
		Updates(map[string]any{
			"status": delegation.Status,
			"active_key": activeKey,
			"updated_at": delegation.UpdatedAt,
			"version": gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := repository.database.WithContext(ctx).Model(&tenantDelegationRecord{}).
			Where("owner_tenant_id = ? AND id = ?", ownerTenantID, delegation.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ports.ErrTenantDelegationNotFound
		}
		return ports.ErrTenantDelegationConflict
	}
	delegation.OwnerTenantID = ownerTenantID
	delegation.Version = expectedVersion + 1
	return nil
}

func NewTenantDelegationRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[ports.TenantDelegationRepositories], error) {
	if database == nil {
		return nil, errors.New("access persistence: database is required")
	}
	return requestscope.GORMRepositories(func(_ context.Context, transaction *gorm.DB) (ports.TenantDelegationRepositories, error) {
		delegation, err := NewTenantDelegationRepository(transaction)
		if err != nil {
			return ports.TenantDelegationRepositories{}, err
		}
		return ports.TenantDelegationRepositories{Delegation: delegation}, nil
	}), nil
}

func trustedDelegationOwner(ctx context.Context) (string, error) {
	principal, ok := identity.FromContext(ctx)
	if !ok || !principal.Authenticated || strings.TrimSpace(principal.TenantID) == "" {
		return "", errors.New("access persistence: trusted tenant principal is required")
	}
	return strings.TrimSpace(principal.TenantID), nil
}

func delegationActiveKey(ownerTenantID, granteeTenantID, resourceKind, resourceID string) string {
	material := strings.Join([]string{ownerTenantID, granteeTenantID, resourceKind, resourceID}, "\x00")
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:])
}

func sameDelegationAuthority(existing, requested domain.TenantDelegation) bool {
	if existing.OwnerTenantID != requested.OwnerTenantID || existing.GranteeTenantID != requested.GranteeTenantID || existing.ResourceKind != requested.ResourceKind || existing.ResourceID != requested.ResourceID {
		return false
	}
	if len(existing.Permissions) != len(requested.Permissions) {
		return false
	}
	for index := range existing.Permissions {
		if existing.Permissions[index] != requested.Permissions[index] {
			return false
		}
	}
	if existing.ExpiresAt == nil || requested.ExpiresAt == nil {
		return existing.ExpiresAt == nil && requested.ExpiresAt == nil
	}
	return existing.ExpiresAt.Equal(*requested.ExpiresAt)
}

func delegationRecordDomain(row tenantDelegationRecord) (domain.TenantDelegation, error) {
	var permissions []string
	if err := json.Unmarshal([]byte(row.PermissionsJSON), &permissions); err != nil {
		return domain.TenantDelegation{}, err
	}
	return domain.TenantDelegation{
		ID: row.ID, OwnerTenantID: row.OwnerTenantID, GranteeTenantID: row.GranteeTenantID,
		ResourceKind: row.ResourceKind, ResourceID: row.ResourceID, Permissions: permissions,
		Status: row.Status, Version: row.Version, ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}
